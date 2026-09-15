package router

import (
	"app/internal/auths"

	"github.com/gin-gonic/gin"
)

type AuthRouter struct{}

func (u *AuthRouter) Register(engine *gin.Engine) {

	//路由组
	userGroup := engine.Group("/api/v1/auth")
	{
		userHandler := auths.NewHandler()
		userGroup.GET("/register", userHandler.Register)
	}
}
