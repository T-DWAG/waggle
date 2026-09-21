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
		group.GET("/:id", handler.GetAgent)
	}
}
