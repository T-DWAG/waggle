package agents

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mszlu521/thunder/logs"
	"github.com/mszlu521/thunder/req"
	"github.com/mszlu521/thunder/res"
)

type Handler struct {
	service *Service
}

func NewHandler() *Handler {
	return &Handler{service: NewService()}
}

func (h *Handler) ListAgents(c *gin.Context) {
	var request SearchRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.ListAgents(c.Request.Context(), request, userID)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func (h *Handler) CreateAgent(c *gin.Context) {
	var request CreateAgentRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.CreateAgent(c.Request.Context(), request, userID)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func (h *Handler) GetAgent(c *gin.Context) {
	id, ok := pathUUID(c)
	if !ok {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.GetAgent(c.Request.Context(), id, userID)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func (h *Handler) UpdateAgent(c *gin.Context) {
	var request UpdateAgentRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	if err := h.service.UpdateAgent(c.Request.Context(), request, userID); err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, &SuccessResponse{Success: true})
}

func (h *Handler) ListProviderConfigs(c *gin.Context) {
	var request ProviderConfigListQuery
	if err := req.QueryParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.ListProviderConfigs(c.Request.Context(), request, userID)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func (h *Handler) CreateProviderConfig(c *gin.Context) {
	var request CreateProviderConfigRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.CreateProviderConfig(c.Request.Context(), request, userID)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func (h *Handler) GetProviderConfig(c *gin.Context) {
	id, ok := pathUUID(c)
	if !ok {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.GetProviderConfig(c.Request.Context(), id, userID)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func (h *Handler) UpdateProviderConfig(c *gin.Context) {
	id, ok := pathUUID(c)
	if !ok {
		return
	}
	var request UpdateProviderConfigRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	if err := h.service.UpdateProviderConfig(c.Request.Context(), id, userID, request); err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, &SuccessResponse{Success: true})
}

func (h *Handler) DeleteProviderConfig(c *gin.Context) {
	id, ok := pathUUID(c)
	if !ok {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	if err := h.service.DeleteProviderConfig(c.Request.Context(), id, userID); err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, &SuccessResponse{Success: true})
}

func (h *Handler) ListLLMs(c *gin.Context) {
	var request LLMListQuery
	if err := req.QueryParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.ListLLMs(c.Request.Context(), request, userID)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func (h *Handler) ListAllLLMs(c *gin.Context) {
	var request LLMListQuery
	if err := req.QueryParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.ListLLMs(c.Request.Context(), request, userID)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func (h *Handler) ListLLMsByProviderConfig(c *gin.Context) {
	var configID uuid.UUID
	if err := req.Path(c, "configId", &configID); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.ListLLMsByProviderConfig(c.Request.Context(), configID, userID)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func (h *Handler) CreateLLM(c *gin.Context) {
	var request CreateLLMRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.CreateLLM(c.Request.Context(), request, userID)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func (h *Handler) GetLLM(c *gin.Context) {
	id, ok := pathUUID(c)
	if !ok {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	response, err := h.service.GetLLM(c.Request.Context(), id, userID)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, response)
}

func (h *Handler) UpdateLLM(c *gin.Context) {
	id, ok := pathUUID(c)
	if !ok {
		return
	}
	var request UpdateLLMRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	if err := h.service.UpdateLLM(c.Request.Context(), id, userID, request); err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, &SuccessResponse{Success: true})
}

func (h *Handler) DeleteLLM(c *gin.Context) {
	id, ok := pathUUID(c)
	if !ok {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}
	if err := h.service.DeleteLLM(c.Request.Context(), id, userID); err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, &SuccessResponse{Success: true})
}

func (h *Handler) AgentMessage(c *gin.Context) {
	var request ChatRequest
	if err := req.JsonParam(c, &request); err != nil {
		return
	}
	userID, ok := req.GetUserIdUUID(c)
	if !ok {
		return
	}

	responseController := http.NewResponseController(c.Writer)
	if err := responseController.SetWriteDeadline(time.Time{}); err != nil {
		logs.Warn("failed to clear SSE write deadline", "err", err)
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	dataChan, errorChan := h.service.AgentMessageStream(ctx, userID, request)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !writeSSE(c, ": keep-alive\n\n") {
				cancel()
				return
			}
		case data, open := <-dataChan:
			if !open {
				writeSSE(c, "data: [DONE]\n\n")
				return
			}
			if !writeSSE(c, fmt.Sprintf("data: %s\n\n", data)) {
				cancel()
				return
			}
		case streamErr, open := <-errorChan:
			if !open {
				errorChan = nil
				continue
			}
			if streamErr != nil {
				// The regular JSON error envelope is not valid after SSE headers are sent.
				if !writeSSE(c, fmt.Sprintf("data: [ERROR] %s\n\n", streamErr.Error())) {
					cancel()
				}
				return
			}
		}
	}
}

func pathUUID(c *gin.Context) (uuid.UUID, bool) {
	var id uuid.UUID
	if err := req.Path(c, "id", &id); err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func writeSSE(c *gin.Context, payload string) bool {
	if _, err := c.Writer.WriteString(payload); err != nil {
		return false
	}
	c.Writer.Flush()
	return true
}
