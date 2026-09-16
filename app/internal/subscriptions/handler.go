package subscriptions

import (
	"time"

	"model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mszlu521/thunder/res"
)

type Handler struct {
}

func NewHandler() *Handler {
	return &Handler{}
}

type SubscriptionResponse struct {
	ID            uuid.UUID         `json:"id"`
	UserID        uuid.UUID         `json:"userId"`
	Plan          string            `json:"plan"`
	Duration      string            `json:"duration"`
	PaymentMethod string            `json:"paymentMethod"`
	StartDate     string            `json:"startDate"`
	EndDate       string            `json:"endDate"`
	CreatedAt     string            `json:"createdAt"`
	Configs       *model.PlanConfig `json:"configs"`
	UpdatedAt     string            `json:"updatedAt"`
}

func (h *Handler) GetUserSubscription(c *gin.Context) {
	res.Success(c, &SubscriptionResponse{
		Configs: &model.PlanConfig{
			MaxAgents:            10,
			MaxKnowledgeBaseSize: 10,
			MaxWorkflows:         10,
		},
		ID:            uuid.New(),
		UserID:        uuid.New(),
		Plan:          string(model.FreePlan),
		Duration:      string(model.Yearly),
		PaymentMethod: string(model.WeChatPay),
		StartDate:     time.Now().Format(time.DateTime),
		EndDate:       time.Now().Add(365 * 24 * time.Hour).Format(time.DateTime),
		CreatedAt:     time.Now().Format(time.DateTime),
		UpdatedAt:     time.Now().Format(time.DateTime),
	})
}
