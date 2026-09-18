package core

import "encoding/json"

type AgentMessage struct {
	Action           string `json:"action"`
	AgentName        string `json:"agentName"`
	ToolName         string `json:"toolName"`
	IsErr            bool   `json:"isErr"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoningContent"`
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
