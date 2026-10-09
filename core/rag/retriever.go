package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// SearchOptions 一次混合检索的参数。
//
// 为什么不用 ES 的 rank.rrf：本机 ES 8.16 是 basic license，RRF 返回
// 403 "current license is non-compliant for [Reciprocal Rank Fusion (RRF)]"。
// knn 与 match 各自是免费的，所以在应用层双跑再融合。
type SearchOptions struct {
	Indexes       []string // 可以跨多个知识库一次查
	Query         string
	TopK          int
	ExcludeDocIDs []string // 已停用的文档
	OnlyDocIDs    []string // 只在指定文档里搜（可选）

	VectorWeight  float64 // 默认 0.7
	KeywordWeight float64 // 默认 0.3
	// MinScore 融合分下限；低于它的结果丢弃。0 表示不过滤。
	MinScore float64
	// KeywordSaturation BM25 归一化常数：norm = s / (s + k)。默认 6。
	// 用饱和函数而不是 min-max：min-max 会把「最好的弱匹配」也抬到 1.0，无关问题会被误召回。
	KeywordSaturation float64
	// NumCandidates knn 每分片候选数，默认 max(50, TopK*10)。
	NumCandidates int
}

// Hit 一条融合后的检索结果。
type Hit struct {
	ID           string         `json:"id"`
	Index        string         `json:"index"`
	KnowledgeID  string         `json:"kbId"`
	DocumentID   string         `json:"documentId"`
	DocumentName string         `json:"documentName"`
	FileType     string         `json:"fileType"`
	Position     int            `json:"position"`
	Content      string         `json:"content"`
	Meta         map[string]any `json:"metadata"`
	Score        float64        `json:"score"`        // 融合分
	VectorScore  float64        `json:"vectorScore"`  // 余弦相似度 [-1,1]
	KeywordScore float64        `json:"keywordScore"` // BM25 原始分
	Matched      string         `json:"matched,omitempty"` // parent_child：回填前命中的子块正文
}

// Search 混合检索：query 向量化 → knn 与 BM25 并发 → 按 _id 合并 → 加权 → 过滤 → 截断 TopK。
func Search(ctx context.Context, client *elasticsearch.Client, embedder embedding.Embedder, options SearchOptions) ([]Hit, error) {
	options = normalizeSearchOptions(options)
	if strings.TrimSpace(options.Query) == "" || len(options.Indexes) == 0 {
		return []Hit{}, nil
	}
	vectors, err := embedder.EmbedStrings(ctx, []string{options.Query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vectors) != 1 || len(vectors[0]) == 0 {
		return nil, fmt.Errorf("embed query: empty vector")
	}

	var (
		wg                      sync.WaitGroup
		vectorHits, keywordHits []rawHit
		vectorErr, keywordErr   error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		vectorHits, vectorErr = runSearch(ctx, client, options.Indexes, buildKnnBody(vectors[0], options))
	}()
	go func() {
		defer wg.Done()
		keywordHits, keywordErr = runSearch(ctx, client, options.Indexes, buildKeywordBody(options))
	}()
	wg.Wait()
	if vectorErr != nil {
		return nil, vectorErr
	}
	// 关键词通道失败不致命：向量结果仍然可用，只是少了兜底。
	if keywordErr != nil {
		keywordHits = nil
	}
	return Fuse(vectorHits, keywordHits, options), nil
}

// rawHit ES 单通道返回。
type rawHit struct {
	ID     string
	Index  string
	Score  float64
	Source map[string]any
}

// Fuse 把两路结果按 _id 合并、加权、排序。导出是为了能在不连 ES 的情况下单测。
func Fuse(vectorHits, keywordHits []rawHit, options SearchOptions) []Hit {
	options = normalizeSearchOptions(options)
	merged := map[string]*Hit{}
	order := make([]string, 0, len(vectorHits)+len(keywordHits))
	get := func(raw rawHit) *Hit {
		key := raw.Index + "/" + raw.ID
		if hit, ok := merged[key]; ok {
			return hit
		}
		hit := hitFromSource(raw)
		merged[key] = hit
		order = append(order, key)
		return hit
	}
	for _, raw := range vectorHits {
		// ES cosine 相似度的 _score = (1 + cos) / 2，这里还原成 cos，阈值才有直观含义。
		get(raw).VectorScore = 2*raw.Score - 1
	}
	for _, raw := range keywordHits {
		get(raw).KeywordScore = raw.Score
	}

	hits := make([]Hit, 0, len(merged))
	for _, key := range order {
		hit := merged[key]
		vector := hit.VectorScore
		if vector < 0 {
			vector = 0
		}
		keyword := hit.KeywordScore / (hit.KeywordScore + options.KeywordSaturation)
		hit.Score = options.VectorWeight*vector + options.KeywordWeight*keyword
		if options.MinScore > 0 && hit.Score < options.MinScore {
			continue
		}
		hits = append(hits, *hit)
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > options.TopK {
		hits = hits[:options.TopK]
	}
	return hits
}

func buildKnnBody(vector []float64, options SearchOptions) map[string]any {
	query := make([]float32, len(vector))
	for i, value := range vector {
		query[i] = float32(value)
	}
	knn := map[string]any{
		"field":          FieldVector,
		"query_vector":   query,
		"k":              options.TopK * 2,
		"num_candidates": options.NumCandidates,
	}
	if filter := buildFilter(options); filter != nil {
		// 写在 knn 里面是 pre-filter：先过滤再找近邻，停用文档不会挤占 k 个名额。
		knn["filter"] = filter
	}
	return map[string]any{
		"size":    options.TopK * 2,
		"knn":     knn,
		"_source": map[string]any{"excludes": []string{FieldVector}},
	}
}

func buildKeywordBody(options SearchOptions) map[string]any {
	boolQuery := map[string]any{
		"must": []any{map[string]any{"match": map[string]any{FieldContent: map[string]any{"query": options.Query}}}},
	}
	if filter := buildFilter(options); filter != nil {
		boolQuery["filter"] = []any{filter}
	}
	return map[string]any{
		"size":    options.TopK * 2,
		"query":   map[string]any{"bool": boolQuery},
		"_source": map[string]any{"excludes": []string{FieldVector}},
	}
}

func buildFilter(options SearchOptions) map[string]any {
	boolFilter := map[string]any{}
	if len(options.OnlyDocIDs) > 0 {
		boolFilter["must"] = []any{map[string]any{"terms": map[string]any{FieldDocID: options.OnlyDocIDs}}}
	}
	if len(options.ExcludeDocIDs) > 0 {
		boolFilter["must_not"] = []any{map[string]any{"terms": map[string]any{FieldDocID: options.ExcludeDocIDs}}}
	}
	if len(boolFilter) == 0 {
		return nil
	}
	return map[string]any{"bool": boolFilter}
}

func runSearch(ctx context.Context, client *elasticsearch.Client, indexes []string, body map[string]any) ([]rawHit, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	// ignore_unavailable：某个知识库索引还没建（刚建库、未上传）时不应该让整次检索失败。
	res, err := esapi.SearchRequest{
		Index:             indexes,
		Body:              bytes.NewReader(payload),
		IgnoreUnavailable: ptr(true),
	}.Do(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.IsError() {
		return nil, fmt.Errorf("search: %s", truncate(string(raw), 300))
	}
	var parsed struct {
		Hits struct {
			Hits []struct {
				ID     string         `json:"_id"`
				Index  string         `json:"_index"`
				Score  float64        `json:"_score"`
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}
	hits := make([]rawHit, 0, len(parsed.Hits.Hits))
	for _, item := range parsed.Hits.Hits {
		hits = append(hits, rawHit{ID: item.ID, Index: item.Index, Score: item.Score, Source: item.Source})
	}
	return hits, nil
}

func hitFromSource(raw rawHit) *Hit {
	hit := &Hit{ID: raw.ID, Index: raw.Index, Meta: map[string]any{}}
	src := raw.Source
	hit.Content, _ = src[FieldContent].(string)
	hit.KnowledgeID, _ = src[FieldKBID].(string)
	hit.DocumentID, _ = src[FieldDocID].(string)
	hit.DocumentName, _ = src[FieldDocName].(string)
	hit.FileType, _ = src[FieldFileType].(string)
	if position, ok := src[FieldPosition].(float64); ok {
		hit.Position = int(position)
	}
	if meta, ok := src[FieldMeta].(map[string]any); ok {
		hit.Meta = meta
	}
	return hit
}

func normalizeSearchOptions(options SearchOptions) SearchOptions {
	if options.TopK <= 0 {
		options.TopK = 5
	}
	if options.TopK > 50 {
		options.TopK = 50
	}
	if options.VectorWeight <= 0 && options.KeywordWeight <= 0 {
		options.VectorWeight, options.KeywordWeight = 0.7, 0.3
	}
	if options.KeywordSaturation <= 0 {
		options.KeywordSaturation = 6
	}
	if options.NumCandidates <= 0 {
		options.NumCandidates = options.TopK * 10
		if options.NumCandidates < 50 {
			options.NumCandidates = 50
		}
	}
	return options
}

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}
