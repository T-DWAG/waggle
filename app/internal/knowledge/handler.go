package knowledge

import (
	"errors"
	"io"
	"net/http"

	"common/biz"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mszlu521/thunder/errs"
	"github.com/mszlu521/thunder/req"
	"github.com/mszlu521/thunder/res"
)

type handler struct {
	service *service
}

func NewHandler() *handler {
	return &handler{service: newService()}
}

func (h *handler) CreateKnowledgeBase(c *gin.Context) {
	var request CreateKnowledgeBaseRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	respond(c)(h.service.createKnowledgeBase(c.Request.Context(), userID, &request))
}

func (h *handler) ListKnowledgeBases(c *gin.Context) {
	var request ListKnowledgeBasesRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	respond(c)(h.service.listKnowledgeBases(c.Request.Context(), userID, &request))
}

func (h *handler) GetKnowledgeBase(c *gin.Context) {
	userID, id, ok := userAndID(c)
	if !ok {
		return
	}
	respond(c)(h.service.getKnowledgeBase(c.Request.Context(), userID, id))
}

func (h *handler) UpdateKnowledgeBase(c *gin.Context) {
	var request UpdateKnowledgeBaseRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	userID, id, ok := userAndID(c)
	if !ok {
		return
	}
	respond(c)(h.service.updateKnowledgeBase(c.Request.Context(), userID, id, &request))
}

func (h *handler) DeleteKnowledgeBase(c *gin.Context) {
	userID, id, ok := userAndID(c)
	if !ok {
		return
	}
	respond(c)(nil, h.service.deleteKnowledgeBase(c.Request.Context(), userID, id))
}

func (h *handler) UploadDocument(c *gin.Context) {
	userID, id, ok := userAndID(c)
	if !ok {
		return
	}
	// 请求体整体限流：比配置的文件上限多留 1MB 给 multipart 边界与其他字段。
	c.Request.Body = httpMaxBytes(c, h.service.rt.settings.MaxFileSize+(1<<20))
	header, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			res.Error(c, biz.ErrFileTooLarge)
			return
		}
		res.Error(c, errs.ErrParam)
		return
	}
	respond(c)(h.service.uploadDocument(c.Request.Context(), userID, id, header))
}

func (h *handler) ListDocuments(c *gin.Context) {
	var request ListDocumentsRequest
	if err := req.QueryParam(c, &request); err != nil {
		return
	}
	userID, id, ok := userAndID(c)
	if !ok {
		return
	}
	respond(c)(h.service.listDocuments(c.Request.Context(), userID, id, &request))
}

func (h *handler) UpdateDocument(c *gin.Context) {
	var request UpdateDocumentRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	userID, id, docID, ok := userAndDocument(c)
	if !ok {
		return
	}
	respond(c)(h.service.updateDocument(c.Request.Context(), userID, id, docID, &request))
}

func (h *handler) DeleteDocument(c *gin.Context) {
	userID, id, docID, ok := userAndDocument(c)
	if !ok {
		return
	}
	respond(c)(nil, h.service.deleteDocument(c.Request.Context(), userID, id, docID))
}

func (h *handler) ReindexDocument(c *gin.Context) {
	userID, id, docID, ok := userAndDocument(c)
	if !ok {
		return
	}
	respond(c)(h.service.reindexDocument(c.Request.Context(), userID, id, docID))
}

func (h *handler) ListDocumentChunks(c *gin.Context) {
	userID, id, docID, ok := userAndDocument(c)
	if !ok {
		return
	}
	respond(c)(h.service.listChunks(c.Request.Context(), userID, id, docID))
}

func (h *handler) Search(c *gin.Context) {
	var request SearchRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	userID, id, ok := userAndID(c)
	if !ok {
		return
	}
	respond(c)(h.service.search(c.Request.Context(), userID, id, &request))
}

// respond 把 (data, err) 统一转成 res.Success / res.Error。
func respond(c *gin.Context) func(data any, err error) {
	return func(data any, err error) {
		if err != nil {
			res.Error(c, err)
			return
		}
		res.Success(c, data)
	}
}

func userAndID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	var id uuid.UUID
	if err := req.Path(c, "id", &id); err != nil {
		return uuid.Nil, uuid.Nil, false
	}
	userID, ok := req.GetUserIdUUID(c)
	return userID, id, ok
}

func userAndDocument(c *gin.Context) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	userID, id, ok := userAndID(c)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	var docID uuid.UUID
	if err := req.Path(c, "documentId", &docID); err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return userID, id, docID, true
}

func httpMaxBytes(c *gin.Context, limit int64) io.ReadCloser {
	return http.MaxBytesReader(c.Writer, c.Request.Body, limit)
}
