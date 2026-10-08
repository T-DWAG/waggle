package rag

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// HTMLToMarkdown 把 HTML 转成「够切片用」的 Markdown：
//   - h1~h6 → #～######，让 SplitMarkdown 能按标题切并带上章节路径；
//   - 块级元素（p/li/blockquote/tr/div…）之间留空行，避免标题和正文粘成一行；
//   - <pre> → 代码块。Typora 导出的 CodeMirror 结构里每行是一个内层 <pre>，逐行还原；
//   - 丢弃 head/script/style/textarea 等非正文内容。
//
// 只追求切片质量，不追求完整的 Markdown 还原（链接、强调等内联格式直接取文本）。
func HTMLToMarkdown(content []byte) (string, error) {
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return "", err
	}
	root := findElement(doc, atom.Body)
	if root == nil {
		root = doc
	}
	w := &mdWriter{}
	w.walk(root)
	return strings.TrimSpace(w.String()), nil
}

type mdWriter struct {
	out  strings.Builder
	line strings.Builder // 当前行（内联文本累积）
}

func (w *mdWriter) String() string {
	w.flushLine()
	return w.out.String()
}

// flushLine 把当前行写出（空行忽略）。
func (w *mdWriter) flushLine() {
	text := strings.TrimSpace(collapseSpaces(w.line.String()))
	w.line.Reset()
	if text != "" {
		w.out.WriteString(text)
		w.out.WriteString("\n")
	}
}

// block 结束一个块：写出当前行并留一个空行。
func (w *mdWriter) block() {
	w.flushLine()
	s := w.out.String()
	if s != "" && !strings.HasSuffix(s, "\n\n") {
		w.out.WriteString("\n")
	}
}

func (w *mdWriter) walk(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		w.line.WriteString(n.Data)
		return
	case html.ElementNode:
	default:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			w.walk(c)
		}
		return
	}

	switch n.DataAtom {
	case atom.Head, atom.Script, atom.Style, atom.Textarea, atom.Noscript, atom.Template, atom.Svg:
		return
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		w.block()
		level := int(n.Data[1] - '0')
		title := strings.TrimSpace(collapseSpaces(textOf(n)))
		if title != "" {
			w.out.WriteString(strings.Repeat("#", level) + " " + title + "\n\n")
		}
		return
	case atom.Pre:
		w.block()
		code := strings.Trim(strings.ReplaceAll(preText(n), "\u00a0", " "), "\n")
		if strings.TrimSpace(code) != "" {
			w.out.WriteString("```\n" + code + "\n```\n\n")
		}
		return
	case atom.Br:
		w.flushLine()
		return
	case atom.Li:
		w.flushLine()
		w.line.WriteString("- ")
	case atom.Td, atom.Th:
		w.line.WriteString(" | ")
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		w.walk(c)
	}

	switch n.DataAtom {
	case atom.P, atom.Div, atom.Blockquote, atom.Ul, atom.Ol, atom.Table, atom.Section,
		atom.Article, atom.Figure, atom.Hr, atom.Header, atom.Footer, atom.Main, atom.Nav, atom.Aside:
		w.block()
	case atom.Li, atom.Tr, atom.Dt, atom.Dd:
		w.flushLine()
	}
}

// preText 取代码块文本。Typora/CodeMirror 把每行放在内层 <pre class="CodeMirror-line"> 里，
// 内层 pre 之间补换行；同时跳过 textarea 和 cm-not-content 等编辑器辅助节点。
func preText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
			return
		}
		if node.Type == html.ElementNode {
			if node.DataAtom == atom.Textarea || hasAttr(node, "cm-not-content") || isEditorChrome(node) {
				return
			}
			if node.DataAtom == atom.Br {
				b.WriteString("\n")
				return
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if node != n && node.Type == html.ElementNode && node.DataAtom == atom.Pre {
			b.WriteString("\n")
		}
	}
	walk(n)
	return b.String()
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func findElement(n *html.Node, a atom.Atom) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == a {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElement(c, a); found != nil {
			return found
		}
	}
	return nil
}

func hasAttr(n *html.Node, key string) bool {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return true
		}
	}
	return false
}

// isEditorChrome CodeMirror 的辅助节点：测量用的 "xxxxxxxxxx"、行号栏等，不是代码内容。
func isEditorChrome(n *html.Node) bool {
	for _, attr := range n.Attr {
		if attr.Key != "class" {
			continue
		}
		for _, class := range strings.Fields(attr.Val) {
			switch class {
			case "CodeMirror-measure", "CodeMirror-gutters", "CodeMirror-linenumber", "CodeMirror-cursors":
				return true
			}
		}
	}
	return false
}

// collapseSpaces 把连续空白（含 HTML 源码里的换行缩进）压成一个空格。
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
