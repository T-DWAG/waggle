package auths

import "model"

type RegisterReq struct {
	Username string `json:"username" binding:"required" validate:"required"`
	Password string `json:"password" binding:"required" validate:"required"`
	Email    string `json:"email" binding:"required" validate:"required,email"`
}

type RegisterResp struct {
	Message string `json:"message"`
}

type VerifyEmailReq struct {
	Token string `json:"token" form:"token" binding:"required" validate:"required"`
}

type LoginReq struct {
	Username string `json:"username" binding:"required" validate:"required"`
	Password string `json:"password" binding:"required" validate:"required"`
}

type LoginResp struct {
	Expire        int64         `json:"expire"`
	RefreshExpire int64         `json:"refreshExpire"`
	Token         string        `json:"token"`
	RefreshToken  string        `json:"refreshToken"`
	UserInfo      model.UserDTO `json:"userInfo"`
	Message       string        `json:"message,omitempty"`
}

type RefreshTokenReq struct {
	RefreshToken string `json:"refreshToken" binding:"required" validate:"required"`
}

type ForgotPasswordReq struct {
	Email string `json:"email" binding:"required" validate:"required,email"`
}

type VerifyCodeReq struct {
	Email string `json:"email" binding:"required" validate:"required,email"`
	Code  string `json:"code" binding:"required" validate:"required,len=6"`
}

type ResetPasswordReq struct {
	Email       string `json:"email" binding:"required" validate:"required,email"`
	Token       string `json:"token" binding:"required" validate:"required"`
	NewPassword string `json:"newPassword" binding:"required" validate:"required,min=6"`
}
