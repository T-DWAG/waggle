package rag

import (
	"context"
	"fmt"
	"strings"

	einos "github.com/mszlu521/thunder/ai/einos"

	"github.com/cloudwego/eino/components/embedding"
)

// ProviderKey 与 model.ProviderConfig.Provider 的取值保持一致。
// model 层定义常量（model.SiliconFlowProvider 等），core 不 import model，
// 所以这里再声明一份字符串，由 app 层在构造配置时负责对齐。
const (
	ProviderOpenAI      = "openai"
	ProviderOllama      = "ollama"
	ProviderQwen        = "qwen"
	ProviderDashscope   = "dashscope"
	ProviderSiliconFlow = "siliconflow"
)

// EmbeddingConfig 是构造向量器所需的最小信息，由 app 层从数据库里取出来填。
// core 不依赖 model 的数据库结构，与 06 的 core/tools.McpConfig 同一思路。
type EmbeddingConfig struct {
	Provider   string
	Model      string
	APIBase    string
	APIKey     string
	TimeoutSec int
	Dimensions int // 可选：只在 provider 支持时透传
}

// NewEmbedder 按 provider 构造 eino 的 embedding.Embedder。
// siliconflow / qwen / dashscope 都是 OpenAI 兼容协议，统一走 openai 实现，
// 只是 baseURL 和 APIKey 不同——所以 provider 决定的是「用哪个实现类」，不是「发给谁」。
func NewEmbedder(ctx context.Context, config *EmbeddingConfig) (embedding.Embedder, error) {
	if config == nil || strings.TrimSpace(config.Model) == "" {
		return nil, fmt.Errorf("embedding model is empty")
	}
	provider := strings.ToLower(strings.TrimSpace(config.Provider))
	modelConfig := &einos.EmbeddingModelConfig{}
	loadProvider := provider

	switch provider {
	case ProviderOllama:
		modelConfig.OllamaConfig = (&ollamaOptions{Model: config.Model, BaseURL: config.APIBase}).build()
	case ProviderOpenAI, ProviderQwen, ProviderSiliconFlow, ProviderDashscope:
		modelConfig.OpenaiConfig = newOpenAIEmbeddingConfig(config)
		loadProvider = einos.EmbeddingOpenai
	default:
		// gemini / ark / qianfan / tencentcloud 需要各自的 SDK 客户端，按需再补。
		return nil, fmt.Errorf("unsupported embedding provider %s", provider)
	}

	embedder, err := einos.LoadEmbedding(ctx, loadProvider, modelConfig)
	if err != nil {
		return nil, fmt.Errorf("load embedding for %s/%s: %w", provider, config.Model, err)
	}
	if embedder == nil {
		return nil, fmt.Errorf("embedding client for %s/%s is nil", provider, config.Model)
	}
	return embedder, nil
}

// ProbeDimension 用一次真实调用量出向量维度。
// 维度写死在代码里迟早出错（同一厂商不同模型维度不同），所以建库时必须实测。
func ProbeDimension(ctx context.Context, embedder embedding.Embedder) (int, error) {
	vectors, err := embedder.EmbedStrings(ctx, []string{"dimension probe"})
	if err != nil {
		return 0, fmt.Errorf("probe embedding dimension: %w", err)
	}
	if len(vectors) != 1 || len(vectors[0]) == 0 {
		return 0, fmt.Errorf("unexpected embedding probe result")
	}
	return len(vectors[0]), nil
}
