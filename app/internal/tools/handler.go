package tools

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mszlu521/thunder/req"
	"github.com/mszlu521/thunder/res"
)

type handler struct {
	service *service
}

func NewHandler() *handler {
	return &handler{service: newService()}
}

func (h *handler) CreateTool(c *gin.Context) {
	var request CreateToolRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.createTool(c.Request.Context(), userID, &request)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func (h *handler) UpdateTool(c *gin.Context) {
	var request UpdateToolRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	id, ok := pathUUID(c)
	if !ok {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.updateTool(c.Request.Context(), userID, id, &request)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func (h *handler) DeleteTool(c *gin.Context) {
	id, ok := pathUUID(c)
	if !ok {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	if err := h.service.deleteTool(c.Request.Context(), userID, id); err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, nil)
}

func (h *handler) GetTool(c *gin.Context) {
	id, ok := pathUUID(c)
	if !ok {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.getTool(c.Request.Context(), userID, id)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func (h *handler) ListTools(c *gin.Context) {
	var request ToolListRequest
	if err := req.QueryParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.listTools(c.Request.Context(), userID, &request)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func (h *handler) TestTool(c *gin.Context) {
	id, ok := pathUUID(c)
	if !ok {
		return
	}
	var request TestToolRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.testTool(c.Request.Context(), userID, id, &request)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func pathUUID(c *gin.Context) (uuid.UUID, bool) {
	var id uuid.UUID
	if err := req.Path(c, "id", &id); err != nil {
		return uuid.Nil, false
	}
	return id, true
}
