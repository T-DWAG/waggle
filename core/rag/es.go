package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// 检索字段名。mapping 与 search body 共用这几个常量，避免两边写错。
const (
	FieldContent  = "content"
	FieldVector   = "content_vector"
	FieldKBID     = "kb_id"
	FieldDocID    = "doc_id"
	FieldDocName  = "doc_name"
	FieldFileType = "file_type"
	FieldPosition = "position"
	FieldMeta     = "metadata"
)

// ESConfig Elasticsearch 连接配置，来自 app/etc/config.yml 的 elasticsearch 段。
type ESConfig struct {
	Addresses  []string
	Username   string
	Password   string
	TimeoutSec int
}

// NewClient 构造 ES 客户端并 Ping 一次，让配置错误在启动时暴露。
func NewClient(ctx context.Context, config *ESConfig) (*elasticsearch.Client, error) {
	if config == nil || len(config.Addresses) == 0 {
		return nil, fmt.Errorf("elasticsearch addresses is empty")
	}
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: config.Addresses,
		Username:  config.Username,
		Password:  config.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("create elasticsearch client: %w", err)
	}
	res, err := client.Ping(client.Ping.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("ping elasticsearch: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("ping elasticsearch: %s", res.String())
	}
	return client, nil
}

// IndexName 一个知识库一个索引。去掉 uuid 里的横线，避免索引名里出现 '-'
// （ES 索引名允许 '-'，但去掉后更贴近 ES 自动生成的原始索引名风格，也便于正则匹配）。
func IndexName(kbID string) string {
	return "kb_" + strings.ReplaceAll(strings.ToLower(kbID), "-", "")
}

// IndexSpec 返回索引导出结构：settings + mappings。
// content 用 ik_max_word 写入、ik_smart 查询——写宽读窄，召回优先。
func IndexSpec(dimensions int) map[string]any {
	if dimensions <= 0 {
		dimensions = DefaultVectorDimensions
	}
	return map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]any{
			"properties": map[string]any{
				FieldContent: map[string]any{
					"type":            "text",
					"analyzer":        "ik_max_word",
					"search_analyzer": "ik_smart",
				},
				FieldVector: map[string]any{
					"type":       "dense_vector",
					"dims":       dimensions,
					"index":      true,
					"similarity": "cosine",
				},
				FieldKBID:     map[string]any{"type": "keyword"},
				FieldDocID:    map[string]any{"type": "keyword"},
				FieldDocName:  map[string]any{"type": "keyword"},
				FieldFileType: map[string]any{"type": "keyword"},
				FieldPosition: map[string]any{"type": "integer"},
				FieldMeta:     map[string]any{"type": "flattened"},
			},
		},
	}
}

// EnsureIndex 索引不存在则按 dimensions 建索引；已存在则校验向量维度是否一致。
// 维度不一致必须显式报错：ES 的 dims 建好后无法修改，静默继续只会写出垃圾数据。
func EnsureIndex(ctx context.Context, client *elasticsearch.Client, index string, dimensions int) error {
	exists, err := IndexExists(ctx, client, index)
	if err != nil {
		return err
	}
	if exists {
		current, err := VectorDimensions(ctx, client, index)
		if err != nil {
			return err
		}
		if current != 0 && dimensions != 0 && current != dimensions {
			return fmt.Errorf("index %s expects %d-dimensional vectors, got %d; recreate the knowledge base", index, current, dimensions)
		}
		return nil
	}
	body, err := json.Marshal(IndexSpec(dimensions))
	if err != nil {
		return fmt.Errorf("marshal index spec: %w", err)
	}
	res, err := esapi.IndicesCreateRequest{Index: index, Body: bytes.NewReader(body)}.Do(ctx, client)
	if err != nil {
		return fmt.Errorf("create index %s: %w", index, err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("create index %s: %s", index, res.String())
	}
	return nil
}

// VectorDimensions 读取索引里 content_vector 的 dims。
func VectorDimensions(ctx context.Context, client *elasticsearch.Client, index string) (int, error) {
	res, err := esapi.IndicesGetMappingRequest{Index: []string{index}}.Do(ctx, client)
	if err != nil {
		return 0, fmt.Errorf("get mapping for %s: %w", index, err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return 0, fmt.Errorf("get mapping for %s: %s", index, res.String())
	}
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, err
	}
	var parsed map[string]struct {
		Mappings struct {
			Properties map[string]struct {
				Dims int `json:"dims"`
			} `json:"properties"`
		} `json:"mappings"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return 0, fmt.Errorf("parse mapping for %s: %w", index, err)
	}
	if entry, ok := parsed[index]; ok {
		return entry.Mappings.Properties[FieldVector].Dims, nil
	}
	return 0, nil
}

func IndexExists(ctx context.Context, client *elasticsearch.Client, index string) (bool, error) {
	res, err := esapi.IndicesExistsRequest{Index: []string{index}}.Do(ctx, client)
	if err != nil {
		return false, fmt.Errorf("check index %s: %w", index, err)
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case 200:
		return true, nil
	case 404:
		return false, nil
	default:
		return false, fmt.Errorf("check index %s: %s", index, res.String())
	}
}

// DropIndex 删除知识库时调用。索引不存在不算错误（幂等）。
func DropIndex(ctx context.Context, client *elasticsearch.Client, index string) error {
	res, err := esapi.IndicesDeleteRequest{Index: []string{index}}.Do(ctx, client)
	if err != nil {
		return fmt.Errorf("delete index %s: %w", index, err)
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return nil
	}
	if res.IsError() {
		return fmt.Errorf("delete index %s: %s", index, res.String())
	}
	return nil
}

// DeleteByDocument 删除某个文档的全部切片。
// doc_id 在 mapping 里是 keyword，所以直接 term 精确匹配即可
// （07 笔记里写 doc_id.keyword 是因为它依赖 ES 自动推断 mapping，把 doc_id 建成了 text+keyword）。
func DeleteByDocument(ctx context.Context, client *elasticsearch.Client, index, documentID string) error {
	body, err := json.Marshal(map[string]any{
		"query": map[string]any{
			"term": map[string]any{FieldDocID: documentID},
		},
	})
	if err != nil {
		return err
	}
	res, err := esapi.DeleteByQueryRequest{Index: []string{index}, Body: bytes.NewReader(body), Refresh: ptr(true)}.Do(ctx, client)
	if err != nil {
		return fmt.Errorf("delete by query on %s: %w", index, err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("delete by query on %s: %s", index, res.String())
	}
	return nil
}

func ptr[T any](v T) *T { return &v }
