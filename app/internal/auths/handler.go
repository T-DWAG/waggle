package auths

import (
	"github.com/gin-gonic/gin"
	"github.com/mszlu521/thunder/errs"
	"github.com/mszlu521/thunder/req"
	"github.com/mszlu521/thunder/res"
	"github.com/mszlu521/thunder/tools/jwt"
)

type Handler struct {
	service *Service
}

func NewHandler() *Handler {
	return &Handler{
		service: NewService(),
	}
}

func (h *Handler) Register(c *gin.Context) {
	var reqData RegisterReq
	if err := req.JsonParam(c, &reqData); err != nil {
		return
	}
	resp, err := h.service.register(reqData)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, resp)
}

func (h *Handler) VerifyEmail(c *gin.Context) {
	var reqData VerifyEmailReq
	if err := req.QueryParam(c, &reqData); err != nil {
		return
	}

	_, err := h.service.verifyEmail(reqData.Token)
	if err != nil {
		res.Error(c, err)
		return
	}
	//如果成功 直接跳转登录页面
	c.Redirect(302, "http://localhost:5173/login")
}

func (h *Handler) Login(c *gin.Context) {
	var reqData LoginReq
	if err := req.JsonParam(c, &reqData); err != nil {
		return
	}
	resp, err := h.service.login(reqData)
	if err != nil {
		res.Error(c, err)
		return
	}
	res.Success(c, resp)
}

func (h *Handler) RefreshToken(c *gin.Context) {
	var reqData RefreshTokenReq
	if err := req.JsonParam(c, &reqData); err != nil {
		return
	}
	claims, err := jwt.ParseToken(reqData.RefreshToken)
	if err != nil {
		res.Error(c, errs.NewError(401, "令牌已过期"))
		return
	}
	// 查找用户
	user, err := h.service.repo.findByUserName(c.Request.Context(), claims.Username)
	if err != nil {
		res.Error(c, errs.NewError(401, "用户不存在"))
		return
	}

	// 生成新的访问令牌和刷新令牌
	resp, err := h.service.token(nil, user)
	if err != nil {
		res.Error(c, errs.NewError(500, "令牌生成失败"))
		return
	}

	res.Success(c, resp)
}

func (h *Handler) ForgotPassword(c *gin.Context) {
	var reqData ForgotPasswordReq
	if err := req.JsonParam(c, &reqData); err != nil {
		res.Error(c, err)
		return
	}

	err := h.service.forgotPassword(reqData)
	if err != nil {
		res.Error(c, err)
		return
	}

	res.Success(c, map[string]interface{}{
		"message": "验证码已发送到您的邮箱",
	})
}

func (h *Handler) VerifyCode(c *gin.Context) {
	var reqData VerifyCodeReq
	if err := req.JsonParam(c, &reqData); err != nil {
		res.Error(c, err)
		return
	}

	token, err := h.service.verifyCode(reqData)
	if err != nil {
		res.Error(c, err)
		return
	}

	res.Success(c, map[string]interface{}{
		"message": "验证码验证成功",
		"token":   token,
	})
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var reqData ResetPasswordReq
	if err := req.JsonParam(c, &reqData); err != nil {
		return
	}

	err := h.service.resetPassword(reqData)
	if err != nil {
		res.Error(c, err)
		return
	}

	res.Success(c, map[string]interface{}{
		"message": "密码重置成功",
	})
}
