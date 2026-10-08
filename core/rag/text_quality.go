package rag

import (
	"errors"
	"strings"
	"unicode"
)

// ErrUnreadableText 解析出的文本大面积乱码（常见于字体缺少 ToUnicode 映射的 PDF，
// 例如 Typora 导出的 Type3 字体）。这种文本写进向量库只会污染召回，直接判失败。
var ErrUnreadableText = errors.New("文本无法正确提取（疑似字体编码问题导致乱码），请转成 Markdown 或 Word 后再上传")

const (
	garbledLineRatio = 0.15 // 单行可疑字符占比 ≥ 此值视为乱码行，整行丢弃
	maxDroppedRatio  = 0.30 // 丢弃的乱码字符占全文 > 此值，说明整份文档不可用
	minReadableRunes = 20   // 清洗后可读字符太少，同样不可用
)

// isSuspiciousRune 判断字符是否像「编码错位」产物：
// Latin-1 补充区（中文文档里几乎不会出现 Æ Ç Ø 这类字母）、C0/C1 控制符、私用区、替换符。
func isSuspiciousRune(c rune) bool {
	switch {
	case c == '\uFFFD', unicode.In(c, unicode.Co):
		return true
	case c < 0x20:
		return c != '\n' && c != '\t' && c != '\r'
	case c >= 0x7F && c <= 0xFF:
		// 常见的合法符号放行
		return !strings.ContainsRune("\u00a0·×÷°±«»§©®", c)
	}
	return false
}

// CleanExtractedText 逐行去掉乱码行，返回清洗后的文本。
// 乱码过多（丢弃比例超阈值或剩余可读内容过少）时返回 ErrUnreadableText。
func CleanExtractedText(text string) (string, error) {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	total, dropped := 0, 0
	for _, line := range lines {
		n, bad := 0, 0
		for _, c := range line {
			if unicode.IsSpace(c) {
				continue
			}
			n++
			if isSuspiciousRune(c) {
				bad++
			}
		}
		total += n
		if n > 0 && float64(bad)/float64(n) >= garbledLineRatio {
			dropped += n
			continue
		}
		kept = append(kept, line)
	}
	if total == 0 {
		return "", nil
	}
	if float64(dropped)/float64(total) > maxDroppedRatio || total-dropped < minReadableRunes {
		return "", ErrUnreadableText
	}
	return strings.Join(kept, "\n"), nil
}
