package model

type SubscriptionPlan string

const (
	FreePlan       SubscriptionPlan = "free"
	BasicPlan      SubscriptionPlan = "basic"
	ProPlan        SubscriptionPlan = "pro"
	EnterprisePlan SubscriptionPlan = "enterprise"
)

type PlanConfig struct {
	MaxAgents            int64 `json:"maxAgents"`
	MaxWorkflows         int64 `json:"maxWorkflows"`
	MaxKnowledgeBaseSize int64 `json:"maxKnowledgeBaseSize"`
}

type PaymentDuration string

const (
	Monthly   PaymentDuration = "monthly"
	Quarterly PaymentDuration = "quarterly"
	Yearly    PaymentDuration = "yearly"
)

type PaymentMethod string

const (
	WeChatPay PaymentMethod = "wechatpay" //微信支付
)
