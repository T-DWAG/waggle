package biz

import "github.com/mszlu521/thunder/errs"

var (
	ErrUserNameExisted  = errs.NewError(1001, "用户名已存在")
	ErrEmailExisted     = errs.NewError(1002, "邮箱已存在")
	ErrPasswordFormat   = errs.NewError(1003, "密码格式错误")
	ErrInvalidToken     = errs.NewError(1004, "无效的token")
	ErrUserNotFound     = errs.NewError(1005, "用户不存在")
	ErrEmailNotVerified = errs.NewError(1006, "邮箱未验证")
	ErrTokenGen         = errs.NewError(1007, "token生成失败")

	ErrAgentNotFound             = errs.NewError(2001, "智能体不存在")
	ErrProviderConfigNotFound    = errs.NewError(2002, "厂商配置不存在")
	ErrLLMNotFound               = errs.NewError(2003, "模型不存在")
	ErrProviderConfigUnavailable = errs.NewError(2004, "厂商配置不可用")
	ErrModelUnavailable          = errs.NewError(2005, "模型不可用")
	ErrAgentModelIncomplete      = errs.NewError(2006, "智能体模型配置不完整")
	ErrUnsupportedProvider       = errs.NewError(2007, "不支持的模型厂商")
	ErrProviderConfigInUse       = errs.NewError(2008, "厂商配置正在被模型使用")
	ErrAgentNotReady             = errs.NewError(2009, "智能体尚未发布或配置未完成")
)
