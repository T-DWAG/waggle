package rag

import "sort"

// ParentRef 回填所需的父块信息，由 service 从 PG 查出后传入。
type ParentRef struct {
	ID      string
	Content string // 父块 Raw
	Meta    map[string]any
}

// ParentIDOf 取命中子块的 parent_id；flat 切片返回 ""。
func ParentIDOf(hit Hit) string {
	id, _ := hit.Meta["parent_id"].(string)
	return id
}

// CollapseToParents 把子块命中回填为父块命中：
//   - 无 parent_id 的命中原样保留；
//   - 同一 parent_id 只保留最高分的那条，Content 换成父块正文，ID 换成父块 ID；
//   - 子块原文放进 Hit.Matched；
//   - 父块找不到（并发删除、越权被丢弃）→ 保留子块本身，不丢结果。
//
// 返回结果按 Score 降序，不截断。
func CollapseToParents(hits []Hit, parents map[string]ParentRef) []Hit {
	out := make([]Hit, 0, len(hits))
	best := map[string]int{} // parent_id -> out 下标
	for _, hit := range hits {
		pid := ParentIDOf(hit)
		parent, ok := parents[pid]
		if pid == "" || !ok {
			out = append(out, hit)
			continue
		}
		if idx, seen := best[pid]; seen {
			if hit.Score > out[idx].Score {
				out[idx] = expand(hit, parent)
			}
			continue
		}
		best[pid] = len(out)
		out = append(out, expand(hit, parent))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

func expand(hit Hit, parent ParentRef) Hit {
	hit.Matched = hit.Content
	hit.Content = parent.Content
	hit.ID = parent.ID
	if len(parent.Meta) > 0 {
		meta := cloneMeta(parent.Meta)
		meta["parent_id"] = parent.ID
		hit.Meta = meta
	}
	return hit
}
