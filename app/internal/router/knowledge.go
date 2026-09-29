package router

import (
	"app/internal/knowledge"

	"github.com/gin-gonic/gin"
)

type KnowledgeRouter struct{}

func (*KnowledgeRouter) Register(engine *gin.Engine) {
	handler := knowledge.NewHandler()
	group := engine.Group("/api/v1/knowledge")
	{
		group.POST("", handler.CreateKnowledgeBase)
		// 静态段必须在 /:id 之前，否则 list 会被当成知识库 ID。
		group.POST("/list", handler.ListKnowledgeBases)
		group.POST("/:id/documents", handler.UploadDocument)
		group.GET("/:id/documents", handler.ListDocuments)
		group.GET("/:id/documents/:documentId/chunks", handler.ListDocumentChunks)
		group.POST("/:id/documents/:documentId/reindex", handler.ReindexDocument)
		group.PUT("/:id/documents/:documentId", handler.UpdateDocument)
		group.DELETE("/:id/documents/:documentId", handler.DeleteDocument)
		group.POST("/:id/search", handler.Search)
		group.PUT("/:id", handler.UpdateKnowledgeBase)
		group.DELETE("/:id", handler.DeleteKnowledgeBase)
		group.GET("/:id", handler.GetKnowledgeBase)
	}
}
