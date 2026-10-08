package rag

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitMarkdownKeepsHeadingPathAndIndependentMeta(t *testing.T) {
	content := "# 手册\n\n引言\n\n## 请假\n\n年假 5 天。\n\n### 病假\n\n需要证明。\n\n## 报销\n\n```bash\n# 这不是标题\necho hi\n```\n\n发票必须真实。\n"
	chunks := SplitMarkdown(content, SplitOptions{Title: "员工手册"})
	if len(chunks) != 4 {
		t.Fatalf("expected 4 chunks, got %d: %#v", len(chunks), chunks)
	}
	if got := HeadingPath(chunks[2].Meta); got != "手册 > 请假 > 病假" {
		t.Fatalf("unexpected heading path %q", got)
	}
	// 进入新的二级标题后，上一节的三级标题必须失效。
	if got := HeadingPath(chunks[3].Meta); got != "手册 > 报销" {
		t.Fatalf("deeper heading leaked: %q", got)
	}
	// 代码块里的 # 注释不能被当成标题。
	if !strings.Contains(chunks[3].Raw, "# 这不是标题") {
		t.Fatalf("code fence comment was treated as heading: %q", chunks[3].Raw)
	}
	if !strings.HasPrefix(chunks[1].Content, "《员工手册》\n章节: 手册 > 请假\n") {
		t.Fatalf("missing prefix: %q", chunks[1].Content)
	}
	// 每个切片的元数据是独立副本。
	chunks[0].Meta["h2"] = "polluted"
	if chunks[1].Meta["h2"] != "请假" {
		t.Fatalf("meta maps are shared between chunks")
	}
}

func TestSplitMarkdownMergesHeadingOnlySections(t *testing.T) {
	chunks := SplitMarkdown("# 临时\n\n## 一节\n\n正文。\n", SplitOptions{Title: "t"})
	if len(chunks) != 1 {
		t.Fatalf("heading-only section should merge into next, got %d: %#v", len(chunks), chunks)
	}
	if !strings.Contains(chunks[0].Raw, "# 临时") || HeadingPath(chunks[0].Meta) != "临时 > 一节" {
		t.Fatalf("unexpected merged chunk: %#v", chunks[0])
	}
	if got := SplitMarkdown("# 只有标题\n", SplitOptions{}); len(got) != 1 {
		t.Fatalf("title-only document should still produce 1 chunk, got %d", len(got))
	}
}

func TestSplitLongRespectsLimitAndOverlap(t *testing.T) {
	text := strings.Repeat("这是一句测试文本。", 60) // 540 runes
	parts := SplitLong(text, 200, 20)
	if len(parts) < 3 {
		t.Fatalf("expected at least 3 parts, got %d", len(parts))
	}
	for i, part := range parts {
		if n := len([]rune(part)); n > 200 {
			t.Fatalf("part %d too long: %d", i, n)
		}
		if i < len(parts)-1 && !strings.HasSuffix(part, "。") {
			t.Fatalf("part %d should end at sentence boundary: %q", i, part)
		}
	}
	if SplitLong("   ", 100, 10) != nil {
		t.Fatalf("blank text should produce no parts")
	}
}

func TestSplitPlainTextMergesShortParagraphs(t *testing.T) {
	chunks := SplitPlainText("第一段。\n\n第二段。\n\n第三段。", SplitOptions{Title: "说明", MaxRunes: 100})
	if len(chunks) != 1 {
		t.Fatalf("short paragraphs should merge into one chunk, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0].Raw, "第三段") {
		t.Fatalf("merged chunk lost content: %q", chunks[0].Raw)
	}
}

func TestFuseWeightsDedupesAndFilters(t *testing.T) {
	vector := []rawHit{
		{ID: "a", Index: "kb_1", Score: 0.9, Source: map[string]any{FieldContent: "A", FieldDocName: "d1"}}, // cos 0.8
		{ID: "b", Index: "kb_1", Score: 0.6, Source: map[string]any{FieldContent: "B"}},                     // cos 0.2
	}
	keyword := []rawHit{
		{ID: "b", Index: "kb_1", Score: 12, Source: map[string]any{FieldContent: "B"}},
		{ID: "c", Index: "kb_1", Score: 1, Source: map[string]any{FieldContent: "C"}},
	}
	hits := Fuse(vector, keyword, SearchOptions{TopK: 5, MinScore: 0.2})
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits after filter, got %d: %#v", len(hits), hits)
	}
	if hits[0].ID != "a" || hits[1].ID != "b" {
		t.Fatalf("unexpected order: %s, %s", hits[0].ID, hits[1].ID)
	}
	if hits[1].KeywordScore != 12 || hits[1].VectorScore < 0.19 {
		t.Fatalf("b should carry both channel scores: %#v", hits[1])
	}
	// 同 _id 不同索引不能被合并。
	merged := Fuse([]rawHit{{ID: "x", Index: "kb_1", Score: 1}}, []rawHit{{ID: "x", Index: "kb_2", Score: 5}}, SearchOptions{})
	if len(merged) != 2 {
		t.Fatalf("hits from different indexes must not merge")
	}
}

func TestBuildContextNumbersReferences(t *testing.T) {
	text, refs := BuildContext([]Hit{
		{DocumentName: "手册.md", Content: "年假 5 天", Score: 0.8, Meta: map[string]any{"h1": "手册", "h2": "请假"}},
		{DocumentName: "手册.md", Content: "  ", Score: 0.7},
		{DocumentName: "FAQ.md", Content: "报销走 OA", Score: 0.6},
	}, 1000)
	if len(refs) != 2 || refs[1].Index != 2 || refs[1].DocumentName != "FAQ.md" {
		t.Fatalf("unexpected references: %#v", refs)
	}
	if !strings.Contains(text, "[1] 来源: 手册.md · 手册 > 请假") || !strings.Contains(text, "[2] 来源: FAQ.md") {
		t.Fatalf("unexpected context: %s", text)
	}
	if text, refs := BuildContext(nil, 100); text != "" || refs != nil {
		t.Fatalf("empty hits should produce empty context")
	}
}

func TestLocalStoreRoundTripAndTraversal(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := ObjectKey("kb1", "doc1", "md")
	if err := store.Put(ctx, key, strings.NewReader("hello")); err != nil {
		t.Fatal(err)
	}
	reader, err := store.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 5)
	_, _ = reader.Read(buf)
	reader.Close()
	if string(buf) != "hello" {
		t.Fatalf("unexpected content %q", buf)
	}
	for _, bad := range []string{"../escape.txt", "kb/../../escape", filepath.Join("..", "x")} {
		if err := store.Put(ctx, bad, strings.NewReader("x")); err == nil {
			t.Fatalf("path traversal %q should be rejected", bad)
		}
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, key); err != ErrObjectNotFound {
		t.Fatalf("expected ErrObjectNotFound, got %v", err)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("delete should be idempotent: %v", err)
	}
}

func TestNormalizeFileTypeAndIndexName(t *testing.T) {
	if NormalizeFileType("A.Markdown") != "md" || NormalizeFileType("x.HTM") != "html" {
		t.Fatalf("unexpected file type normalization")
	}
	if IsSupportedFileType("exe") {
		t.Fatalf("exe must not be supported")
	}
	if got := IndexName("0F8A-11"); got != "kb_0f8a11" {
		t.Fatalf("unexpected index name %q", got)
	}
}

func TestParseMarkdownStripsBOM(t *testing.T) {
	result, err := Parse(context.Background(), FileTypeMarkdown, append([]byte{0xEF, 0xBB, 0xBF}, []byte("# T\nbody")...))
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsMarkdown || strings.HasPrefix(result.Text, "\uFEFF") {
		t.Fatalf("BOM not stripped or markdown flag lost: %#v", result)
	}
	if _, err := Parse(context.Background(), "exe", nil); err == nil {
		t.Fatalf("unsupported type should fail")
	}
}

func TestKnowledgeToolValidatesArguments(t *testing.T) {
	called := 0
	tool := &KnowledgeTool{Name: KnowledgeToolName, Search: func(ctx context.Context, query string, topK int) ([]Hit, error) {
		called++
		if topK != 10 {
			t.Fatalf("top_k should be clamped to 10, got %d", topK)
		}
		return nil, nil
	}}
	if _, err := tool.InvokableRun(context.Background(), `{"query":"  "}`); err == nil {
		t.Fatalf("blank query should fail")
	}
	result, err := tool.InvokableRun(context.Background(), `{"query":"年假","top_k":99}`)
	if err != nil || called != 1 || !strings.Contains(result, "没有找到") {
		t.Fatalf("unexpected result %q err %v", result, err)
	}
}

// TestIntegrationIndexAndSearch 需要真实 ES 与向量模型，环境变量不全时跳过：
//
//	RAG_IT_ES_ADDR / RAG_IT_ES_USER / RAG_IT_ES_PASSWORD
//	RAG_IT_EMBED_BASE / RAG_IT_EMBED_KEY / RAG_IT_EMBED_MODEL
//	RAG_IT_DOC（可选，Markdown 文件路径）
func TestIntegrationIndexAndSearch(t *testing.T) {
	addr, key := os.Getenv("RAG_IT_ES_ADDR"), os.Getenv("RAG_IT_EMBED_KEY")
	if addr == "" || key == "" {
		t.Skip("integration env not set")
	}
	ctx := context.Background()
	client, err := NewClient(ctx, &ESConfig{Addresses: []string{addr}, Username: os.Getenv("RAG_IT_ES_USER"), Password: os.Getenv("RAG_IT_ES_PASSWORD")})
	if err != nil {
		t.Fatal(err)
	}
	embedder, err := NewEmbedder(ctx, &EmbeddingConfig{Provider: ProviderSiliconFlow, Model: os.Getenv("RAG_IT_EMBED_MODEL"), APIBase: os.Getenv("RAG_IT_EMBED_BASE"), APIKey: key, TimeoutSec: 30})
	if err != nil {
		t.Fatal(err)
	}
	dims, err := ProbeDimension(ctx, embedder)
	if err != nil {
		t.Fatal(err)
	}
	index := IndexName("it-test-0000")
	_ = DropIndex(ctx, client, index)
	t.Cleanup(func() { _ = DropIndex(context.Background(), client, index) })
	if err := EnsureIndex(ctx, client, index, dims); err != nil {
		t.Fatal(err)
	}
	if err := EnsureIndex(ctx, client, index, dims+1); err == nil {
		t.Fatalf("dimension mismatch should be rejected")
	}

	content := "# MCP\n\n## 什么是 MCP\n\nMCP 是模型上下文协议，定义模型如何获取工具、资源与状态。\n\n## Transport\n\nMCP 支持 stdio 与 HTTP SSE 两种通信方式。\n"
	if path := os.Getenv("RAG_IT_DOC"); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		content = string(raw)
	}
	chunks := SplitMarkdown(content, SplitOptions{Title: "MCP测试文档"})
	storeChunks := make([]StoreChunk, 0, len(chunks))
	for i, chunk := range chunks {
		storeChunks = append(storeChunks, StoreChunk{ID: "it-" + string(rune('a'+i%26)) + strings.Repeat("x", i/26), Content: chunk.Content, Position: i, Meta: chunk.Meta})
	}
	if err := StoreDocument(ctx, client, embedder, &StoreRequest{Index: index, KnowledgeID: "it", DocumentID: "doc-1", DocumentName: "MCP测试文档.md", FileType: "md", Chunks: storeChunks}); err != nil {
		t.Fatal(err)
	}

	hits, err := Search(ctx, client, embedder, SearchOptions{Indexes: []string{index}, Query: "MCP 有哪些通信方式", TopK: 3, MinScore: 0.3})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || !strings.Contains(hits[0].Content, "stdio") {
		t.Fatalf("expected transport chunk first, got %#v", hits)
	}
	t.Logf("positive top1 score=%.3f vec=%.3f kw=%.3f section=%s", hits[0].Score, hits[0].VectorScore, hits[0].KeywordScore, HeadingPath(hits[0].Meta))

	negative, err := Search(ctx, client, embedder, SearchOptions{Indexes: []string{index}, Query: "迟到扣款规则", TopK: 3, MinScore: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	for _, hit := range negative {
		t.Logf("negative hit score=%.3f vec=%.3f", hit.Score, hit.VectorScore)
	}
	if len(negative) != 0 {
		t.Fatalf("unrelated query should be filtered, got %d hits", len(negative))
	}

	if err := DeleteByDocument(ctx, client, index, "doc-1"); err != nil {
		t.Fatal(err)
	}
	if count, _ := CountByDocument(ctx, client, index, "doc-1"); count != 0 {
		t.Fatalf("expected 0 chunks after delete, got %d", count)
	}
}

func TestLocalStoreDeletePrunesEmptyDirs(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStore(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	a, b := ObjectKey("kb1", "doc1", "md"), ObjectKey("kb1", "doc2", "md")
	for _, key := range []string{a, b} {
		if err := store.Put(ctx, key, strings.NewReader("x")); err != nil {
			t.Fatal(err)
		}
	}
	_ = store.Delete(ctx, a)
	if _, err := os.Stat(filepath.Join(root, "kb", "kb1")); err != nil {
		t.Fatalf("non-empty dir must be kept: %v", err)
	}
	_ = store.Delete(ctx, b)
	if _, err := os.Stat(filepath.Join(root, "kb")); !os.IsNotExist(err) {
		t.Fatalf("empty dirs should be pruned, got %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("root must be kept: %v", err)
	}
	if err := store.Put(ctx, a, strings.NewReader("x")); err != nil {
		t.Fatalf("put after prune: %v", err)
	}
}

func TestCleanExtractedText(t *testing.T) {
	good := "第一章 介绍\nMCP 是一种协议，支持 stdio、SSE。\n温度 25°C · 正常"
	out, err := CleanExtractedText(good + "\nÆÇØ`\u0088")
	if err != nil || out != good {
		t.Fatalf("garbled line should be dropped only: %q %v", out, err)
	}
	if _, err := CleanExtractedText("fqÈBU_^DQº>?Ò\nØÆà\u0088\nÁ\u008a\x1bqÈÊ\u009fÄUý\nok"); err != ErrUnreadableText {
		t.Fatalf("mostly garbled text should fail, got %v", err)
	}
}

func TestHTMLToMarkdownKeepsHeadingsAndBlocks(t *testing.T) {
	src := `<html><head><style>h1{}</style></head><body><div id="write">
<h1><span>MCP 详解</span></h1><p><strong>MCP</strong><span> 是协议。</span></p>
<h2>传输</h2><ul><li><p>stdio</p></li><li>SSE</li></ul>
<pre class="md-fences"><div class="CodeMirror"><textarea>junk</textarea><div class="CodeMirror-measure"><pre><span>xxxxxxxxxx</span></pre></div><div cm-not-content="true">x</div>
<pre class="CodeMirror-line"><span>func main() {</span></pre><pre class="CodeMirror-line"><span>}</span></pre></div></pre>
<table><tr><th>方式</th><th>说明</th></tr><tr><td>stdio</td><td>本地</td></tr></table>
<script>alert(1)</script></div></body></html>`
	md, err := HTMLToMarkdown([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# MCP 详解\n\nMCP 是协议。", "## 传输", "- stdio", "- SSE", "```\nfunc main() {\n}\n```", "| 方式 | 说明"} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in:\n%s", want, md)
		}
	}
	for _, bad := range []string{"junk", "alert", "h1{}", "xxxxxxxxxx"} {
		if strings.Contains(md, bad) {
			t.Fatalf("should drop %q:\n%s", bad, md)
		}
	}
	chunks := Split(&ParseResult{Text: md, IsMarkdown: true}, SplitOptions{Title: "t"})
	if len(chunks) == 0 || HeadingPath(chunks[len(chunks)-1].Meta) == "" {
		t.Fatalf("html chunks should carry heading path: %#v", chunks)
	}
}
