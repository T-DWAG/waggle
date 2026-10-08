package rag

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

// Chunk 一个待入库的切片。
type Chunk struct {
	// Content 真正写进向量和 ES content 字段的文本，含「《文档》路径: …」前缀。
	Content string
	// Raw 不含前缀的原文，落 PG 供展示/编辑。
	Raw string
	// Meta 切片级元数据：doc_title、h1~h6、paragraph。每个切片独立一份。
	Meta map[string]any
}

// SplitOptions 切片参数。
type SplitOptions struct {
	MaxRunes int    // 单切片字符上限，超过则二次切分
	Overlap  int    // 二次切分时的重叠字符，避免句子被切断后两边都不完整
	Title    string // 文档级标题，通常是文件名去掉扩展名
}

var (
	markdownHeading = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	codeFence       = regexp.MustCompile("^\\s*(```|~~~)")
	blankLines      = regexp.MustCompile(`\n\s*\n`)
)

// SplitMarkdown 按标题层级切分 Markdown。
//
// 与 07 笔记的差异：
//  1. 每个切片拿到「自己的」元数据副本。笔记里 chunkMetadata 被循环反复写 doc_id/position，
//     下标取模 chunkMetadata[i%len] 会让不同切片共享同一个 map，最后写入的值覆盖前面所有切片。
//  2. 代码块里的 `# 注释` 不会被误判成标题。
//  3. 超长小节走 SplitLong 二次切分，而不是整段丢给 embedding（语义稀释）。
func SplitMarkdown(content string, options SplitOptions) []Chunk {
	options = normalizeSplitOptions(options)
	var (
		chunks   []Chunk
		builder  strings.Builder
		headings = map[int]string{}
		inFence  bool
	)

	flush := func() {
		text := strings.TrimSpace(builder.String())
		if text == "" {
			builder.Reset()
			return
		}
		// 只有标题行、没有正文的小节（如「# 手册」紧跟「## 第一章」）不单独成片：
		// 保留在 builder 里并入下一节，否则会产出只有一行标题的噪声切片。
		if isHeadingOnly(text) {
			return
		}
		builder.Reset()
		meta := map[string]any{}
		if options.Title != "" {
			meta["doc_title"] = options.Title
		}
		for level := 1; level <= 6; level++ {
			if title := headings[level]; title != "" {
				meta[fmt.Sprintf("h%d", level)] = title
			}
		}
		for _, part := range SplitLong(text, options.MaxRunes, options.Overlap) {
			chunks = append(chunks, Chunk{Content: applyPrefix(part, meta), Raw: part, Meta: cloneMeta(meta)})
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(strings.ReplaceAll(content, "\r\n", "\n")))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if codeFence.MatchString(line) {
			inFence = !inFence
		}
		if !inFence {
			if matches := markdownHeading.FindStringSubmatch(line); len(matches) == 3 {
				flush()
				level := len(matches[1])
				headings[level] = strings.TrimSpace(matches[2])
				// 进入新章节时更深层标题全部失效，否则上一节的三级标题会一路跟到文末。
				for deeper := level + 1; deeper <= 6; deeper++ {
					delete(headings, deeper)
				}
			}
		}
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	flush()
	// 文末残留的纯标题段也要落一片，否则整篇只有标题的文档会得到 0 片。
	if rest := strings.TrimSpace(builder.String()); rest != "" && isHeadingOnly(rest) {
		meta := map[string]any{}
		if options.Title != "" {
			meta["doc_title"] = options.Title
		}
		chunks = append(chunks, Chunk{Content: applyPrefix(rest, meta), Raw: rest, Meta: meta})
	}
	return chunks
}

func isHeadingOnly(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !markdownHeading.MatchString(line) {
			return false
		}
	}
	return true
}

// SplitPlainText 用于没有标题结构的文本（txt/pdf/docx/html 解析结果）：按空行分段，
// 再把相邻短段落合并到 MaxRunes，过长的二次切分。
func SplitPlainText(content string, options SplitOptions) []Chunk {
	options = normalizeSplitOptions(options)
	paragraphs := blankLines.Split(strings.ReplaceAll(content, "\r\n", "\n"), -1)

	var (
		chunks []Chunk
		buffer strings.Builder
		index  int
	)
	emit := func(text string) {
		for _, part := range SplitLong(text, options.MaxRunes, options.Overlap) {
			meta := map[string]any{"paragraph": index}
			if options.Title != "" {
				meta["doc_title"] = options.Title
			}
			chunks = append(chunks, Chunk{Content: applyPrefix(part, meta), Raw: part, Meta: meta})
			index++
		}
	}
	for _, paragraph := range paragraphs {
		text := strings.TrimSpace(paragraph)
		if text == "" {
			continue
		}
		// 短段落合并：一个切片只有一句话时，向量太「尖」反而召回不到上下文。
		if buffer.Len() > 0 && len([]rune(buffer.String()))+len([]rune(text))+1 > options.MaxRunes {
			emit(buffer.String())
			buffer.Reset()
		}
		if buffer.Len() > 0 {
			buffer.WriteString("\n")
		}
		buffer.WriteString(text)
	}
	if buffer.Len() > 0 {
		emit(buffer.String())
	}
	return chunks
}

// SplitLong 把超长文本切开，块间保留 overlap 个字符。
// 断点优先级：换行 > 句末标点 > 逗号类 > 硬切——尽量保住「语义完整」。
func SplitLong(text string, maxRunes, overlap int) []string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil
	}
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return []string{string(runes)}
	}
	var parts []string
	for start := 0; start < len(runes); {
		end := start + maxRunes
		if end >= len(runes) {
			parts = append(parts, string(runes[start:]))
			break
		}
		cut := findBreak(runes, start+maxRunes/2, end)
		parts = append(parts, string(runes[start:cut]))
		next := cut - overlap
		if next <= start {
			next = cut
		}
		start = next
	}
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// findBreak 在 (low, end] 内从后往前找最自然的断点；找不到就硬切在 end。
// low 取块的一半位置，避免为了找标点把块切得过短。
func findBreak(runes []rune, low, end int) int {
	for _, set := range []string{"\n", "。！？；.!?;", "，,、：:"} {
		for i := end; i > low; i-- {
			if strings.ContainsRune(set, runes[i-1]) {
				return i
			}
		}
	}
	return end
}

func normalizeSplitOptions(options SplitOptions) SplitOptions {
	if options.MaxRunes <= 0 {
		options.MaxRunes = 800
	}
	if options.Overlap < 0 {
		options.Overlap = 0
	}
	if options.Overlap >= options.MaxRunes/2 {
		options.Overlap = options.MaxRunes / 5
	}
	return options
}

// applyPrefix 把「文档 + 标题路径」拼到切片正文前面。
// 这一段同时进向量和 ES content：既修代词丢失，也让检索结果本身带出处。
func applyPrefix(content string, meta map[string]any) string {
	var builder strings.Builder
	if title, _ := meta["doc_title"].(string); title != "" {
		builder.WriteString("《" + title + "》\n")
	}
	if path := HeadingPath(meta); path != "" {
		builder.WriteString("章节: " + path + "\n")
	}
	if builder.Len() == 0 {
		return content
	}
	return builder.String() + content
}

// HeadingPath 把 h1~h6 拼成 "一级 > 二级 > 三级"，用于前缀和引用展示。
func HeadingPath(meta map[string]any) string {
	var path []string
	for level := 1; level <= 6; level++ {
		if title, _ := meta[fmt.Sprintf("h%d", level)].(string); title != "" {
			path = append(path, title)
		}
	}
	return strings.Join(path, " > ")
}

func cloneMeta(meta map[string]any) map[string]any {
	cloned := make(map[string]any, len(meta))
	for key, value := range meta {
		cloned[key] = value
	}
	return cloned
}
