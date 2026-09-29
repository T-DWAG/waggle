package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	es8indexer "github.com/cloudwego/eino-ext/components/indexer/es8"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// StoreChunk 一个要写进 ES 的切片。ID 由调用方生成并同时写进 PG，保证两边一一对应。
type StoreChunk struct {
	ID       string
	Content  string
	Position int
	Meta     map[string]any
}

// StoreRequest 一个文档的整批切片。
type StoreRequest struct {
	Index        string
	KnowledgeID  string
	DocumentID   string
	DocumentName string
	FileType     string
	Chunks       []StoreChunk
	BatchSize    int
}

// StoreDocument 向量化并批量写入 ES。
//
// es8.Indexer 在 bulk 单条失败时只 log.Printf、仍然返回成功（见 indexer.go 的 OnFailure），
// 所以写完后必须按 doc_id 数一次：数量对不上就当失败，避免「状态 completed、实际 0 条」。
func StoreDocument(ctx context.Context, client *elasticsearch.Client, embedder embedding.Embedder, request *StoreRequest) error {
	if request == nil || len(request.Chunks) == 0 {
		return nil
	}
	batchSize := request.BatchSize
	if batchSize <= 0 {
		batchSize = 16
	}
	indexer, err := es8indexer.NewIndexer(ctx, &es8indexer.IndexerConfig{
		Client:    client,
		Index:     request.Index,
		BatchSize: batchSize,
		Embedding: embedder,
		DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]es8indexer.FieldValue, error) {
			meta := doc.MetaData
			return map[string]es8indexer.FieldValue{
				FieldContent:  {Value: doc.Content, EmbedKey: FieldVector},
				FieldKBID:     {Value: meta[FieldKBID]},
				FieldDocID:    {Value: meta[FieldDocID]},
				FieldDocName:  {Value: meta[FieldDocName]},
				FieldFileType: {Value: meta[FieldFileType]},
				FieldPosition: {Value: meta[FieldPosition]},
				FieldMeta:     {Value: meta[FieldMeta]},
			}, nil
		},
	})
	if err != nil {
		return fmt.Errorf("create es8 indexer: %w", err)
	}

	docs := make([]*schema.Document, 0, len(request.Chunks))
	for _, chunk := range request.Chunks {
		docs = append(docs, &schema.Document{
			ID:      chunk.ID,
			Content: chunk.Content,
			MetaData: map[string]any{
				FieldKBID:     request.KnowledgeID,
				FieldDocID:    request.DocumentID,
				FieldDocName:  request.DocumentName,
				FieldFileType: request.FileType,
				FieldPosition: chunk.Position,
				FieldMeta:     flattenMeta(chunk.Meta),
			},
		})
	}
	if _, err := indexer.Store(ctx, docs); err != nil {
		return fmt.Errorf("store chunks: %w", err)
	}
	if err := Refresh(ctx, client, request.Index); err != nil {
		return err
	}
	stored, err := CountByDocument(ctx, client, request.Index, request.DocumentID)
	if err != nil {
		return err
	}
	if stored != len(docs) {
		return fmt.Errorf("elasticsearch accepted %d of %d chunks; check index mapping and embedding dimension", stored, len(docs))
	}
	return nil
}

// Refresh 让刚写入的切片立即可检索；上传后前端马上搜索时依赖它。
func Refresh(ctx context.Context, client *elasticsearch.Client, index string) error {
	res, err := esapi.IndicesRefreshRequest{Index: []string{index}}.Do(ctx, client)
	if err != nil {
		return fmt.Errorf("refresh %s: %w", index, err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("refresh %s: %s", index, res.String())
	}
	return nil
}

// CountByDocument 统计某文档在 ES 里的切片数。
func CountByDocument(ctx context.Context, client *elasticsearch.Client, index, documentID string) (int, error) {
	body, _ := json.Marshal(map[string]any{"query": map[string]any{"term": map[string]any{FieldDocID: documentID}}})
	res, err := esapi.CountRequest{Index: []string{index}, Body: bytes.NewReader(body)}.Do(ctx, client)
	if err != nil {
		return 0, fmt.Errorf("count %s: %w", index, err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return 0, fmt.Errorf("count %s: %s", index, res.String())
	}
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, err
	}
	var parsed struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return 0, err
	}
	return parsed.Count, nil
}

// flattenMeta ES metadata 字段是 flattened 类型，只接受「值为标量」的对象。
// 这里把非字符串统一 fmt 成字符串，防止某个解析器塞进嵌套结构导致整条写入失败。
func flattenMeta(meta map[string]any) map[string]any {
	result := make(map[string]any, len(meta))
	for key, value := range meta {
		switch v := value.(type) {
		case nil:
			continue
		case string:
			result[key] = v
		default:
			result[key] = fmt.Sprint(v)
		}
	}
	return result
}
