package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	einoTool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// SearchFunc 由 app 层注入：它知道用哪个向量器、哪些索引、排除哪些停用文档。
// core 只负责把它包装成模型可调用的工具，不关心数据库。
type SearchFunc func(ctx context.Context, query string, topK int) ([]Hit, error)

// KnowledgeTool 显式检索工具。隐式检索（对话前自动填 ragContext）用的是用户原话；
// 模型在多轮工具调用中发现需要换个说法再查时，才调用这个工具。
type KnowledgeTool struct {
	Name        string
	Description string
	Search      SearchFunc
	MaxRunes    int
}

var _ einoTool.InvokableTool = (*KnowledgeTool)(nil)

type knowledgeToolArgs struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"`
}

func (t *KnowledgeTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.Name,
		Desc: t.Description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {Type: schema.String, Desc: "检索用的问题或关键词，尽量具体", Required: true},
			"top_k": {Type: schema.Integer, Desc: "返回片段数量，默认 5，最大 10"},
		}),
	}, nil
}

func (t *KnowledgeTool) InvokableRun(ctx context.Context, arguments string, _ ...einoTool.Option) (string, error) {
	var args knowledgeToolArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return "", fmt.Errorf("query is required")
	}
	if args.TopK <= 0 {
		args.TopK = 5
	}
	if args.TopK > 10 {
		args.TopK = 10
	}
	hits, err := t.Search(ctx, args.Query, args.TopK)
	if err != nil {
		return "", err
	}
	text, _ := BuildContext(hits, t.MaxRunes)
	if text == "" {
		return "知识库中没有找到与该问题相关的内容。", nil
	}
	return text, nil
}

// KnowledgeToolName 一个智能体只挂一个检索工具，覆盖它绑定的全部知识库。
// 每库一个工具会让工具列表随绑定数膨胀，模型还要猜该查哪个库。
const KnowledgeToolName = "knowledge_search"
