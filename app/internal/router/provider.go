package router

import (
	"app/internal/agents"

	"github.com/gin-gonic/gin"
)

// ProviderRouter registers both provider configuration and LLM routes.
type ProviderRouter struct{}

func (*ProviderRouter) Register(engine *gin.Engine) {
	handler := agents.NewHandler()

	providerConfigs := engine.Group("/api/v1/provider-configs")
	{
		providerConfigs.GET("", handler.ListProviderConfigs)
		providerConfigs.POST("", handler.CreateProviderConfig)
		providerConfigs.GET("/:id", handler.GetProviderConfig)
		providerConfigs.PUT("/:id", handler.UpdateProviderConfig)
		providerConfigs.DELETE("/:id", handler.DeleteProviderConfig)
	}

	llms := engine.Group("/api/v1/llms")
	{
		llms.GET("", handler.ListLLMs)
		llms.POST("", handler.CreateLLM)
		// Static routes must come before /:id.
		llms.GET("/all", handler.ListAllLLMs)
		llms.GET("/config/:configId", handler.ListLLMsByProviderConfig)
		llms.GET("/:id", handler.GetLLM)
		llms.PUT("/:id", handler.UpdateLLM)
		llms.DELETE("/:id", handler.DeleteLLM)
	}
}
