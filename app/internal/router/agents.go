package router

import (
	"app/internal/agents"

	"github.com/gin-gonic/gin"
)

type AgentsRouter struct{}

func (*AgentsRouter) Register(engine *gin.Engine) {
	handler := agents.NewHandler()
	group := engine.Group("/api/v1/agents")
	{
		group.POST("/list", handler.ListAgents)
		group.POST("/create", handler.CreateAgent)
		group.PUT("/update", handler.UpdateAgent)
		group.POST("/chat", handler.AgentMessage)
		// 批量关联必须在 /:id 之前注册，避免被详情路由截走。
		group.POST("/:id/tools/batch", handler.UpdateAgentTools)
		group.GET("/:id", handler.GetAgent)
	}
}
