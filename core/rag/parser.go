package rag

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	docxparser "github.com/cloudwego/eino-ext/components/document/parser/docx"
	htmlparser "github.com/cloudwego/eino-ext/components/document/parser/html"
	pdfparser "github.com/cloudwego/eino-ext/components/document/parser/pdf"
	"github.com/cloudwego/eino/components/document/parser"
)

// 支持的文件类型。扩展名统一小写、不带点。
const (
	FileTypeMarkdown = "md"
	FileTypeText     = "txt"
	FileTypePDF      = "pdf"
	FileTypeDocx     = "docx"
	FileTypeHTML     = "html"
)

// SupportedFileTypes 用于上传校验和前端 accept 提示。
var SupportedFileTypes = []string{FileTypeMarkdown, FileTypeText, FileTypePDF, FileTypeDocx, FileTypeHTML}

// NormalizeFileType 从文件名得到归一化的类型：.markdown→md，.htm→html。
func NormalizeFileType(filename string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	switch ext {
	case "markdown":
		return FileTypeMarkdown
	case "htm":
		return FileTypeHTML
	default:
		return ext
	}
}

func IsSupportedFileType(fileType string) bool {
	for _, item := range SupportedFileTypes {
		if item == fileType {
			return true
		}
	}
	return false
}

// ParseResult 解析后的纯文本，以及它是否保留了 Markdown 标题结构。
type ParseResult struct {
	Text       string
	IsMarkdown bool
}

// Parse 把原始文件字节解析成文本。
// md/txt 直接读；pdf/docx/html 走 eino-ext 的 parser。docx 解析器输出的本来就是 Markdown，
// 所以 docx 也走标题切分——这是它比 pdf 切得好的原因。
func Parse(ctx context.Context, fileType string, content []byte) (*ParseResult, error) {
	switch fileType {
	case FileTypeMarkdown:
		return &ParseResult{Text: decodeText(content), IsMarkdown: true}, nil
	case FileTypeText:
		return &ParseResult{Text: decodeText(content)}, nil
	case FileTypeDocx:
		p, err := docxparser.NewDocxParser(ctx, &docxparser.Config{IncludeTables: true})
		if err != nil {
			return nil, err
		}
		text, err := runParser(ctx, p, content)
		if err != nil {
			return nil, err
		}
		return &ParseResult{Text: text, IsMarkdown: true}, nil
	case FileTypePDF:
		p, err := pdfparser.NewPDFParser(ctx, &pdfparser.Config{})
		if err != nil {
			return nil, err
		}
		text, err := runParser(ctx, p, content)
		if err != nil {
			return nil, err
		}
		return &ParseResult{Text: text}, nil
	case FileTypeHTML:
		p, err := htmlparser.NewParser(ctx, &htmlparser.Config{Selector: &htmlparser.BodySelector})
		if err != nil {
			return nil, err
		}
		text, err := runParser(ctx, p, content)
		if err != nil {
			return nil, err
		}
		return &ParseResult{Text: text}, nil
	default:
		return nil, fmt.Errorf("unsupported file type %q", fileType)
	}
}

// Split 根据解析结果选择切分策略。
func Split(result *ParseResult, options SplitOptions) []Chunk {
	if result == nil {
		return nil
	}
	if result.IsMarkdown {
		return SplitMarkdown(result.Text, options)
	}
	return SplitPlainText(result.Text, options)
}

func runParser(ctx context.Context, p parser.Parser, content []byte) (string, error) {
	docs, err := p.Parse(ctx, bytes.NewReader(content))
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(docs))
	for _, doc := range docs {
		if doc != nil && strings.TrimSpace(doc.Content) != "" {
			parts = append(parts, doc.Content)
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

// decodeText 去掉 UTF-8 BOM；非法 UTF-8 替换掉，避免 PG text 列写入失败。
func decodeText(content []byte) string {
	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(content) {
		return string(content)
	}
	return strings.ToValidUTF8(string(content), "\uFFFD")
}
