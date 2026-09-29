package rag

import (
	"fmt"
	"strings"
)

// Reference 返回给前端/模型的引用信息。
type Reference struct {
	Index        int     `json:"index"`
	KnowledgeID  string  `json:"kbId"`
	DocumentID   string  `json:"documentId"`
	DocumentName string  `json:"documentName"`
	Section      string  `json:"section"`
	Score        float64 `json:"score"`
}

// BuildContext 把检索结果渲染成填进 {ragContext} 的文本，并返回编号一致的引用列表。
//
// 设计：每条片段前带 [n] 编号 + 出处，system prompt 里要求模型用 [n] 标注来源；
// maxRunes 控制注入总量，避免「大海捞针」——宁可少给，不要把整本手册塞进上下文。
func BuildContext(hits []Hit, maxRunes int) (string, []Reference) {
	if len(hits) == 0 {
		return "", nil
	}
	if maxRunes <= 0 {
		maxRunes = 4000
	}
	var (
		builder    strings.Builder
		references []Reference
		used       int
	)
	builder.WriteString("以下是与用户问题相关的知识库片段，按相关度排序。回答时请在引用处标注对应编号，例如 [1]。\n")
	builder.WriteString("如果片段不足以回答问题，请明确说明知识库中没有相关信息，不要编造。\n\n")
	for _, hit := range hits {
		content := strings.TrimSpace(hit.Content)
		if content == "" {
			continue
		}
		remaining := maxRunes - used
		if remaining <= 80 {
			break
		}
		if runes := []rune(content); len(runes) > remaining {
			content = string(runes[:remaining]) + "…"
		}
		index := len(references) + 1
		section := HeadingPath(hit.Meta)
		source := hit.DocumentName
		if section != "" {
			source += " · " + section
		}
		builder.WriteString(fmt.Sprintf("[%d] 来源: %s (相关度 %.2f)\n%s\n\n", index, source, hit.Score, content))
		used += len([]rune(content))
		references = append(references, Reference{
			Index:        index,
			KnowledgeID:  hit.KnowledgeID,
			DocumentID:   hit.DocumentID,
			DocumentName: hit.DocumentName,
			Section:      section,
			Score:        hit.Score,
		})
	}
	if len(references) == 0 {
		return "", nil
	}
	return strings.TrimSpace(builder.String()), references
}

// FormatReferences 生成回答末尾的「参考来源」尾注，SSE 里单独推一条。
func FormatReferences(references []Reference) string {
	if len(references) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("\n\n参考来源:\n")
	for _, ref := range references {
		line := fmt.Sprintf("[%d] %s", ref.Index, ref.DocumentName)
		if ref.Section != "" {
			line += " · " + ref.Section
		}
		builder.WriteString(line + "\n")
	}
	return strings.TrimRight(builder.String(), "\n")
}

// RetrieveResult 跨模块检索结果（knowledge → agents 的事件出参）。
// 放在 core 而不是 model/shared：它引用 Hit/Reference，model 层不能依赖 core。
type RetrieveResult struct {
	Context    string
	References []Reference
	Hits       []Hit
}
