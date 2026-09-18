package shared

import "model"

// LLMParams 事件入参：要按什么条件找厂商配置
type LLMParams struct {
	ModelType model.LLMType `json:"modelType"`
	Provider  string        `json:"provider"`
	Model     string        `json:"model"`
}

// ModelProviderResponse 事件出参
type ModelProviderResponse struct {
	ProvideConfig *model.ProviderConfig `json:"provideConfig"`
}
