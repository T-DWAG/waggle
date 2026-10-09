package rag

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func longMarkdown() *ParseResult {
	body := strings.Repeat("这是一个用于测试的句子，包含若干文字内容。", 80) // 约 1600 字
	return &ParseResult{IsMarkdown: true, Text: "# 手册\n\n## 请假\n\n" + body + "\n\n## 报销\n\n发票必须真实。\n"}
}

func TestSplitParentChild_ChildInsideParent(t *testing.T) {
	groups := SplitParentChild(longMarkdown(), ParentChildOptions{Title: "手册"})
	if len(groups) == 0 {
		t.Fatal("no groups")
	}
	for _, g := range groups {
		if len(g.Children) == 0 {
			t.Fatal("parent without children")
		}
		for _, c := range g.Children {
			if !strings.Contains(g.Parent.Raw, c.Raw) {
				t.Fatalf("child not inside parent: %q", c.Raw)
			}
		}
	}
}

func TestSplitParentChild_ShortParent(t *testing.T) {
	groups := SplitParentChild(&ParseResult{IsMarkdown: true, Text: "## 报销\n\n发票必须真实。\n"}, ParentChildOptions{})
	if len(groups) != 1 || len(groups[0].Children) != 1 {
		t.Fatalf("unexpected %#v", groups)
	}
	if groups[0].Children[0].Raw != groups[0].Parent.Raw {
		t.Fatalf("child raw %q != parent raw %q", groups[0].Children[0].Raw, groups[0].Parent.Raw)
	}
}

func TestSplitParentChild_NoParentOverlap(t *testing.T) {
	groups := SplitParentChild(longMarkdown(), ParentChildOptions{})
	for i := 1; i < len(groups); i++ {
		prev, cur := []rune(groups[i-1].Parent.Raw), []rune(groups[i].Parent.Raw)
		n := 20
		if len(prev) < n || len(cur) < n {
			continue
		}
		if string(prev[len(prev)-n:]) == string(cur[:n]) {
			t.Fatalf("parents %d and %d overlap", i-1, i)
		}
	}
}

func TestSplitParentChild_MetaIsolated(t *testing.T) {
	groups := SplitParentChild(longMarkdown(), ParentChildOptions{Title: "手册"})
	g := groups[0]
	if len(g.Children) < 2 {
		t.Fatal("need at least 2 children")
	}
	g.Children[0].Meta["parent_id"] = "x"
	if _, ok := g.Children[1].Meta["parent_id"]; ok {
		t.Fatal("sibling meta shared")
	}
	if _, ok := g.Parent.Meta["parent_id"]; ok {
		t.Fatal("parent meta shared")
	}
}

func TestSplitParentChild_Defaults(t *testing.T) {
	for _, g := range SplitParentChild(longMarkdown(), ParentChildOptions{}) {
		for _, c := range g.Children {
			if n := utf8.RuneCountInString(c.Raw); n > 300 {
				t.Fatalf("child has %d runes", n)
			}
		}
		if n := utf8.RuneCountInString(g.Parent.Raw); n > 1200 {
			t.Fatalf("parent has %d runes", n)
		}
	}
	if got := SplitParentChild(nil, ParentChildOptions{}); len(got) != 0 {
		t.Fatalf("nil result gave %d groups", len(got))
	}
}

func child(id, parent string, score float64, content string) Hit {
	return Hit{ID: id, Score: score, Content: content, Meta: map[string]any{"parent_id": parent}}
}

func TestCollapseToParents_Dedup(t *testing.T) {
	hits := []Hit{child("c1", "p1", 0.5, "a"), child("c2", "p1", 0.9, "b"), child("c3", "p1", 0.7, "c")}
	out := CollapseToParents(hits, map[string]ParentRef{"p1": {ID: "p1", Content: "PARENT"}})
	if len(out) != 1 {
		t.Fatalf("got %d", len(out))
	}
	if out[0].Score != 0.9 || out[0].Matched != "b" || out[0].Content != "PARENT" || out[0].ID != "p1" {
		t.Fatalf("unexpected %#v", out[0])
	}
}

func TestCollapseToParents_FlatPassthrough(t *testing.T) {
	flat := Hit{ID: "f", Score: 0.4, Content: "flat", Meta: map[string]any{}}
	out := CollapseToParents([]Hit{flat}, nil)
	if len(out) != 1 || out[0].Content != "flat" || out[0].Matched != "" {
		t.Fatalf("unexpected %#v", out)
	}
}

func TestCollapseToParents_MissingParent(t *testing.T) {
	out := CollapseToParents([]Hit{child("c1", "gone", 0.5, "a")}, map[string]ParentRef{})
	if len(out) != 1 || out[0].Content != "a" || out[0].Matched != "" {
		t.Fatalf("unexpected %#v", out)
	}
}

func TestCollapseToParents_Order(t *testing.T) {
	hits := []Hit{child("c1", "p1", 0.3, "a"), {ID: "f", Score: 0.6, Meta: map[string]any{}}, child("c2", "p2", 0.8, "b")}
	out := CollapseToParents(hits, map[string]ParentRef{"p1": {ID: "p1", Content: "P1"}, "p2": {ID: "p2", Content: "P2"}})
	if len(out) != 3 || out[0].ID != "p2" || out[1].ID != "f" || out[2].ID != "p1" {
		t.Fatalf("unexpected order %#v", out)
	}
}
