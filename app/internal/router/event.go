package router

import (
	"context"
	"fmt"
	"time"

	"app/internal/agents"
	"app/internal/tools"
	"model/shared"

	"github.com/mszlu521/thunder/event"
	"github.com/mszlu521/thunder/logs"
)

// providerConfigQueryTimeout 事件里查厂商配置的超时（独立于 HTTP 请求生命周期）
const providerConfigQueryTimeout = 3 * time.Second

type Event struct {
}

func (*Event) Register() {
	event.Register("getProviderConfigByProvider", func(e event.Event) (any, error) {
		params, ok := e.Data.(*shared.LLMParams)
		if !ok {
			logs.Errorf("事件参数类型错误: %T", e.Data)
			// 必须返回 error：调用方（4.8 的 service）拿到 nil 会直接断言 panic
			return nil, fmt.Errorf("事件参数类型错误: %T", e.Data)
		}
		// 这里按 provider + 模型标识找一条可用的厂商配置
		ctx, cancel := context.WithTimeout(context.Background(), providerConfigQueryTimeout)
		defer cancel()
		repo := agents.NewModel()
		cfg, err := repo.GetProviderConfigByProvider(ctx, params)
		if err != nil {
			return nil, err
		}
		return &shared.ModelProviderResponse{ProvideConfig: cfg}, nil
	})
	event.Register("getToolsInIds", tools.NewPublicService().GetToolsInIds)
}
