package router

import (
	"app/internal/subscriptions"

	"github.com/gin-gonic/gin"
)

// SubscriptionRouter 实现了 server.IRouter 接口
type SubscriptionRouter struct {
}

// Register 负责注册订阅相关的路由
func (s *SubscriptionRouter) Register(engine *gin.Engine) {
	handler := subscriptions.NewHandler()

	// 创建一个路由组
	subscriptionGroup := engine.Group("/api/v1/subscription")
	{
		// 获取指定用户订阅信息
		subscriptionGroup.GET("/current", handler.GetUserSubscription)
	}
}
