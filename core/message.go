package core

import (
	"encoding/json"

	"core/rag"
)

type AgentMessage struct {
	Action           string `json:"action"`
	AgentName        string `json:"agentName"`
	ToolName         string `json:"toolName"`
	IsErr            bool   `json:"isErr"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoningContent"`
	// References 知识库引用，仅在 action=agent_references 时出现。
	References []rag.Reference `json:"references,omitempty"`
}

func BuildErrMessage(agentName string, errMsg string) string {
	msg := AgentMessage{
		Action:    "agent_answer",
		AgentName: agentName,
		IsErr:     true,
		Content:   errMsg,
	}
	bytes, _ := json.Marshal(msg)
	return string(bytes)
}

func BuildReasoningMessage(agentName string, toolName string, reasoning string) string {
	msg := AgentMessage{
		Action:           "agent_answer",
		AgentName:        agentName,
		ToolName:         toolName,
		ReasoningContent: reasoning,
	}
	bytes, _ := json.Marshal(msg)
	return string(bytes)
}

func BuildContentMessage(agentName string, toolName string, content string) string {
	msg := AgentMessage{
		Action:    "agent_answer",
		AgentName: agentName,
		ToolName:  toolName,
		Content:   content,
	}
	bytes, _ := json.Marshal(msg)
	return string(bytes)
}

// BuildReferencesMessage 回答结束后推一条引用消息，前端据此渲染「参考来源」，编号与 ragContext 里的 [n] 一致。
func BuildReferencesMessage(agentName string, references []rag.Reference) string {
	msg := AgentMessage{
		Action:     "agent_references",
		AgentName:  agentName,
		References: references,
	}
	bytes, _ := json.Marshal(msg)
	return string(bytes)
}
