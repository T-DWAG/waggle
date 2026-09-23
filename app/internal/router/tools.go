package router

import (
	"app/internal/tools"

	"github.com/gin-gonic/gin"
)

type ToolRouter struct{}

func (*ToolRouter) Register(engine *gin.Engine) {
	handler := tools.NewHandler()
	group := engine.Group("/api/v1/tools")
	{
		group.POST("", handler.CreateTool)
		group.GET("", handler.ListTools)
		// 静态路径必须在 /:id 之前，避免 test 被当成工具 ID。
		group.POST("/:id/test", handler.TestTool)
		group.PUT("/:id", handler.UpdateTool)
		group.DELETE("/:id", handler.DeleteTool)
		group.GET("/:id", handler.GetTool)
	}
}
