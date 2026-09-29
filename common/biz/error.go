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

	ErrToolAlreadyExists = errs.NewError(3001, "工具已存在")
	ErrToolNotExist      = errs.NewError(3002, "工具不存在")
	ErrToolNotRegistered = errs.NewError(3003, "系统工具未注册")
	ErrToolDisabled      = errs.NewError(3004, "工具已停用")
	ErrInvalidToolType   = errs.NewError(3005, "不支持的工具类型")
	ErrMcpConfigRequired = errs.NewError(3006, "MCP 工具缺少配置")
	ErrMcpConnectFailed  = errs.NewError(3007, "MCP 服务连接失败")

	ErrKnowledgeBaseNotFound   = errs.NewError(4001, "知识库不存在")
	ErrEmbeddingModelInvalid   = errs.NewError(4002, "向量模型不可用")
	ErrVectorStoreUnavailable  = errs.NewError(4003, "向量存储不可用")
	ErrDocumentNotFound        = errs.NewError(4004, "文档不存在")
	ErrFileTypeNotSupported    = errs.NewError(4005, "不支持的文件类型")
	ErrFileTooLarge            = errs.NewError(4006, "文件超过大小限制")
	ErrDocumentDuplicated      = errs.NewError(4007, "知识库中已存在相同文件")
	ErrDocumentProcessing      = errs.NewError(4008, "文档正在处理中，请稍后再试")
	ErrEmbeddingModelImmutable = errs.NewError(4009, "知识库创建后不能更换向量模型")
	ErrDocumentSourceMissing   = errs.NewError(4010, "文档原文已丢失，无法重新索引")
	ErrKnowledgeBaseInUse      = errs.NewError(4011, "知识库正被智能体使用，请先解绑")
)
