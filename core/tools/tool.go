package tools

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/mszlu521/thunder/ai/einos"
)

// InvokeParamTool 复用 thunder 的定义：可执行，并且能导出参数 schema。
type InvokeParamTool = einos.InvokeParamTool

var registry = &Registry{}

type Registry struct {
	tools []InvokeParamTool
}

// RegisterSystemTools 在进程启动时注册内置工具。重复调用会整体替换注册表。
func RegisterSystemTools(inputs ...InvokeParamTool) {
	tools := make([]InvokeParamTool, 0, len(inputs))
	registry = &Registry{tools: append(tools, inputs...)}
}

// FindTool 按 eino ToolInfo.Name 查找，不按数据库里的展示名称查找。
func FindTool(toolName string) InvokeParamTool {
	for _, item := range registry.tools {
		info, err := item.Info(context.Background())
		if err != nil || info == nil || info.Name != toolName {
			continue
		}
		return item
	}
	return nil
}

func GetTools() []tool.BaseTool {
	tools := make([]tool.BaseTool, 0, len(registry.tools))
	for _, item := range registry.tools {
		tools = append(tools, item)
	}
	return tools
}

func GetToolByName(name string) tool.BaseTool {
	return FindTool(name)
}
