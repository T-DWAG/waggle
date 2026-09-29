package rag

import (
	"time"

	ollamaembedding "github.com/cloudwego/eino-ext/components/embedding/ollama"
	openaiembedding "github.com/cloudwego/eino-ext/components/embedding/openai"
)

// newOpenAIEmbeddingConfig 构造 OpenAI 兼容的向量器配置。
// openai / qwen / siliconflow / dashscope 走的是同一份协议，差异只在 BaseURL 与 APIKey。
func newOpenAIEmbeddingConfig(config *EmbeddingConfig) *openaiembedding.EmbeddingConfig {
	embeddingConfig := &openaiembedding.EmbeddingConfig{
		APIKey:  config.APIKey,
		BaseURL: config.APIBase,
		Model:   config.Model,
	}
	if config.TimeoutSec > 0 {
		embeddingConfig.Timeout = time.Duration(config.TimeoutSec) * time.Second
	}
	if config.Dimensions > 0 {
		dims := config.Dimensions
		embeddingConfig.Dimensions = &dims
	}
	return embeddingConfig
}

// ollamaOptions 只保留 core 需要暴露的字段，避免把 ollama 专有参数泄漏到上层配置里。
type ollamaOptions struct {
	Model   string
	BaseURL string
}

func (o *ollamaOptions) build() *ollamaembedding.EmbeddingConfig {
	return &ollamaembedding.EmbeddingConfig{
		Model:   o.Model,
		BaseURL: o.BaseURL,
	}
}
