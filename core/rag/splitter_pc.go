package rag

// ParentChunk 一个父块及其全部子块。
type ParentChunk struct {
	Parent   Chunk   // Raw 是回填给模型的正文
	Children []Chunk // Content 带前缀（进向量），Raw 不带（展示）
}

// ParentChildOptions 父子分段参数。
type ParentChildOptions struct {
	ParentRunes  int // 父块字符上限，默认 1200
	ChildRunes   int // 子块字符上限，默认 300
	ChildOverlap int // 子块重叠，默认 40
	Title        string
}

func normalizeParentChild(o ParentChildOptions) ParentChildOptions {
	if o.ParentRunes <= 0 {
		o.ParentRunes = 1200
	}
	if o.ChildRunes <= 0 {
		o.ChildRunes = 300
	}
	if o.ChildRunes >= o.ParentRunes {
		o.ChildRunes = o.ParentRunes / 4
	}
	if o.ChildOverlap < 0 || o.ChildOverlap >= o.ChildRunes/2 {
		o.ChildOverlap = o.ChildRunes / 5
	}
	return o
}

// SplitParentChild 先按文档结构切父块，再把每个父块切成子块。
// 子块 Meta 继承父块的 doc_title / h1~h6 / paragraph，并在入库时由调用方补 parent_id。
// 父块之间不重叠：同一句话进两个父块，回填时会重复。
func SplitParentChild(result *ParseResult, options ParentChildOptions) []ParentChunk {
	options = normalizeParentChild(options)
	parents := Split(result, SplitOptions{MaxRunes: options.ParentRunes, Overlap: 0, Title: options.Title})
	groups := make([]ParentChunk, 0, len(parents))
	for _, parent := range parents {
		group := ParentChunk{Parent: parent}
		for _, part := range SplitLong(parent.Raw, options.ChildRunes, options.ChildOverlap) {
			group.Children = append(group.Children, Chunk{
				Content: applyPrefix(part, parent.Meta),
				Raw:     part,
				Meta:    cloneMeta(parent.Meta),
			})
		}
		if len(group.Children) == 0 {
			continue
		}
		groups = append(groups, group)
	}
	return groups
}

// CloneMeta 供 app 层复制切片元数据，避免多个切片共享同一个 map。
func CloneMeta(meta map[string]any) map[string]any { return cloneMeta(meta) }
