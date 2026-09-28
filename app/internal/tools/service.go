package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"common/biz"
	coretools "core/tools"
	"model"
	"model/shared"

	toolpkg "github.com/cloudwego/eino/components/tool"

	"github.com/google/uuid"
	"github.com/mszlu521/thunder/errs"
	"github.com/mszlu521/thunder/event"
	"github.com/mszlu521/thunder/logs"
	"github.com/mszlu521/thunder/res"
)

const databaseTimeout = 5 * time.Second

type service struct {
	repo repository
}

func newService() *service {
	return &service{repo: newModels()}
}

func (s *service) createTool(parent context.Context, userID uuid.UUID, request *CreateToolRequest) (*ToolResponse, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return nil, errs.ErrParam
	}
	toolType := request.ToolType
	if toolType == "" {
		toolType = model.SystemToolType
	}
	if toolType != model.SystemToolType && toolType != model.McpToolType {
		return nil, biz.ErrInvalidToolType
	}

	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	now := time.Now()
	tool := &model.Tool{
		BaseModel:   model.BaseModel{ID: uuid.New(), CreatedAt: now, UpdatedAt: now},
		CreatorID:   userID,
		ToolType:    toolType,
		IsEnable:    true,
		Name:        name,
		Description: strings.TrimSpace(request.Description),
	}
	if request.IsEnable != nil {
		tool.IsEnable = *request.IsEnable
	}

	if toolType == model.McpToolType {
		if request.McpConfig == nil || strings.TrimSpace(request.McpConfig.Url) == "" {
			return nil, biz.ErrMcpConfigRequired
		}
		if err := validateMcpConfig(request.McpConfig); err != nil {
			return nil, errs.ErrParam
		}
		tool.McpConfig = request.McpConfig
	} else {
		// 系统工具的真实名称、描述和参数以注册表为准，避免前端展示名和模型调用名不一致。
		registered := coretools.FindTool(name)
		if registered == nil {
			return nil, biz.ErrToolNotRegistered
		}
		info, err := registered.Info(ctx)
		if err != nil || info == nil {
			logs.Errorf("获取工具信息失败: %v", err)
			return nil, biz.ErrToolNotRegistered
		}
		existing, err := s.repo.getByName(ctx, userID, info.Name)
		if err != nil {
			logs.Errorf("获取工具失败: %v", err)
			return nil, errs.DBError
		}
		if existing != nil {
			return nil, biz.ErrToolAlreadyExists
		}
		tool.Name = info.Name
		tool.Description = info.Desc
		tool.ParametersSchema = registered.Params()
		tool.IsEnable = true
	}

	if err := s.repo.create(ctx, tool); err != nil {
		logs.Errorf("创建工具失败: %v", err)
		return nil, errs.DBError
	}
	return convertToolToResponse(tool), nil
}

func (s *service) updateTool(parent context.Context, userID, id uuid.UUID, request *UpdateToolRequest) (*ToolResponse, error) {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	tool, err := s.repo.get(ctx, id, userID)
	if err != nil {
		logs.Errorf("获取工具失败: %v", err)
		return nil, errs.DBError
	}
	if tool == nil {
		return nil, biz.ErrToolNotExist
	}

	// 系统工具的调用契约来自代码注册表，更新接口只允许改启用状态。
	if tool.ToolType == model.SystemToolType {
		if request.IsEnable != nil {
			tool.IsEnable = *request.IsEnable
		}
	} else {
		if request.Name != nil {
			name := strings.TrimSpace(*request.Name)
			if name == "" {
				return nil, errs.ErrParam
			}
			tool.Name = name
		}
		if request.Description != nil {
			tool.Description = strings.TrimSpace(*request.Description)
		}
		if request.ToolType != "" {
			if request.ToolType != model.McpToolType {
				return nil, biz.ErrInvalidToolType
			}
			tool.ToolType = request.ToolType
		}
		if request.ParametersSchema != nil {
			tool.ParametersSchema = *request.ParametersSchema
		}
		if request.McpConfig != nil {
			if err := validateMcpConfig(request.McpConfig); err != nil {
				return nil, errs.ErrParam
			}
			if request.McpConfig.Token == "" && tool.McpConfig != nil {
				request.McpConfig.Token = tool.McpConfig.Token
			}
			tool.McpConfig = request.McpConfig
		}
		if request.IsEnable != nil {
			tool.IsEnable = *request.IsEnable
		}
	}
	tool.UpdatedAt = time.Now()
	if err := s.repo.update(ctx, tool); err != nil {
		logs.Errorf("更新工具失败: %v", err)
		return nil, errs.DBError
	}
	return convertToolToResponse(tool), nil
}

func (s *service) listTools(parent context.Context, userID uuid.UUID, request *ToolListRequest) (*res.Page, error) {
	page, pageSize := request.Page, request.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	items, total, err := s.repo.list(ctx, toolFilter{
		CreatorID: userID,
		Name:      strings.TrimSpace(request.Name),
		ToolType:  strings.TrimSpace(request.Type),
		Limit:     pageSize,
		Offset:    (page - 1) * pageSize,
	})
	if err != nil {
		logs.Errorf("获取工具列表失败: %v", err)
		return nil, errs.DBError
	}
	response := make([]*ToolResponse, 0, len(items))
	for _, item := range items {
		response = append(response, convertToolToResponse(item))
	}
	return &res.Page{
		List:        response,
		Total:       total,
		CurrentPage: int64(page),
		PageSize:    int64(pageSize),
	}, nil
}

func (s *service) getTool(parent context.Context, userID, id uuid.UUID) (*ToolResponse, error) {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	tool, err := s.repo.get(ctx, id, userID)
	if err != nil {
		logs.Errorf("获取工具失败: %v", err)
		return nil, errs.DBError
	}
	if tool == nil {
		return nil, biz.ErrToolNotExist
	}
	return convertToolToResponse(tool), nil
}

func (s *service) deleteTool(parent context.Context, userID, id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	tool, err := s.repo.get(ctx, id, userID)
	if err != nil {
		logs.Errorf("获取工具失败: %v", err)
		return errs.DBError
	}
	if tool == nil {
		return biz.ErrToolNotExist
	}
	if err := s.repo.delete(ctx, id, userID); err != nil {
		logs.Errorf("删除工具失败: %v", err)
		return errs.DBError
	}
	return nil
}

func (s *service) testTool(parent context.Context, userID, id uuid.UUID, request *TestToolRequest) (*TestToolResponse, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tool, err := s.repo.get(ctx, id, userID)
	if err != nil {
		logs.Errorf("获取工具失败: %v", err)
		return nil, errs.DBError
	}
	if tool == nil {
		return nil, biz.ErrToolNotExist
	}
	if !tool.IsEnable {
		return nil, biz.ErrToolDisabled
	}
	if tool.ToolType == model.McpToolType {
		if tool.McpConfig == nil {
			return &TestToolResponse{Success: false, Message: "MCP 配置为空"}, nil
		}
		if strings.TrimSpace(request.ToolName) == "" {
			return nil, errs.ErrParam
		}
		config := &coretools.McpConfig{
			URL:            tool.McpConfig.Url,
			Type:           tool.McpConfig.Type,
			Token:          tool.McpConfig.Token,
			CredentialType: tool.McpConfig.CredentialType,
			ClientName:     "agent-platform",
			ClientVersion:  "1.0.0",
		}
		remoteTools, cli, err := coretools.GetMCPTools(ctx, config)
		if err != nil {
			logs.Errorf("MCP 工具测试连接失败, toolId=%s: %v", id, err)
			return &TestToolResponse{Success: false, Message: "MCP 工具连接失败"}, nil
		}
		defer cli.Close()
		var remoteTool toolpkg.BaseTool
		for _, item := range remoteTools {
			info, infoErr := item.Info(ctx)
			if infoErr == nil && info != nil && info.Name == request.ToolName {
				remoteTool = item
				break
			}
		}
		if remoteTool == nil {
			return &TestToolResponse{Success: false, Message: "MCP 工具不存在"}, nil
		}
		invokable, ok := remoteTool.(toolpkg.InvokableTool)
		if !ok {
			return &TestToolResponse{Success: false, Message: "MCP 工具不可执行"}, nil
		}
		params, err := json.Marshal(request.Params)
		if err != nil {
			return nil, errs.ErrParam
		}
		result, err := invokable.InvokableRun(ctx, string(params))
		if err != nil {
			// 调用阶段的错误来自远端工具（如参数校验失败），不含 Token，返回给工具所有者便于排查。
			logs.Errorf("MCP 工具测试执行失败, toolId=%s, toolName=%s: %v", id, request.ToolName, err)
			return &TestToolResponse{Success: false, Message: "MCP 工具执行失败: " + err.Error()}, nil
		}
		return &TestToolResponse{Success: true, Message: "success", Data: result}, nil
	}
	if tool.ToolType != model.SystemToolType {
		return nil, biz.ErrInvalidToolType
	}

	registered := coretools.FindTool(tool.Name)
	if registered == nil {
		return nil, biz.ErrToolNotRegistered
	}
	params, err := json.Marshal(request.Params)
	if err != nil {
		return nil, errs.ErrParam
	}
	result, err := registered.InvokableRun(ctx, string(params))
	if err != nil {
		logs.Errorf("工具 %s 测试失败: %v", tool.Name, err)
		return &TestToolResponse{Success: false, Message: err.Error()}, nil
	}
	return &TestToolResponse{Success: true, Message: "success", Data: result}, nil
}

func (s *service) getMcpTools(parent context.Context, userID, id uuid.UUID) ([]*McpToolResponse, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	stored, err := s.repo.get(ctx, id, userID)
	if err != nil {
		logs.Errorf("获取 MCP 工具配置失败: %v", err)
		return nil, errs.DBError
	}
	if stored == nil {
		return nil, biz.ErrToolNotExist
	}
	if stored.ToolType != model.McpToolType {
		return nil, biz.ErrInvalidToolType
	}
	if !stored.IsEnable {
		return nil, biz.ErrToolDisabled
	}
	if stored.McpConfig == nil {
		return nil, biz.ErrMcpConfigRequired
	}
	config := &coretools.McpConfig{
		URL:            stored.McpConfig.Url,
		Type:           stored.McpConfig.Type,
		Token:          stored.McpConfig.Token,
		CredentialType: stored.McpConfig.CredentialType,
		ClientName:     "agent-platform",
		ClientVersion:  "1.0.0",
	}
	remoteTools, cli, err := coretools.ListMCPTools(ctx, config)
	if err != nil {
		// 只记录配置 ID，底层错误不回传前端，避免暴露内部地址或认证细节。
		logs.Errorf("获取 MCP 工具失败, toolId=%s: %v", id, err)
		return nil, biz.ErrMcpConnectFailed
	}
	defer cli.Close()
	response := make([]*McpToolResponse, 0, len(remoteTools))
	for _, remote := range remoteTools {
		schemaBytes, err := json.Marshal(remote.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("转换 MCP 工具参数失败: %w", err)
		}
		inputSchema := model.JSON{}
		if err := json.Unmarshal(schemaBytes, &inputSchema); err != nil {
			return nil, fmt.Errorf("解析 MCP 工具参数失败: %w", err)
		}
		response = append(response, &McpToolResponse{
			Name:        remote.Name,
			Description: remote.Description,
			InputSchema: inputSchema,
		})
	}
	return response, nil
}

func validateMcpConfig(config *model.McpConfig) error {
	if config == nil || strings.TrimSpace(config.Url) == "" {
		return biz.ErrMcpConfigRequired
	}
	parsed, err := url.Parse(config.Url)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return fmt.Errorf("invalid MCP URL")
	}
	if config.Type != "" && config.Type != "sse" && config.Type != "streamable_http" {
		return fmt.Errorf("unsupported MCP transport type")
	}
	if config.CredentialType != "" && config.CredentialType != "bearer" {
		return fmt.Errorf("unsupported MCP credential type")
	}
	return nil
}
func convertToolToResponse(tool *model.Tool) *ToolResponse {
	config := model.JSON{}
	if tool.McpConfig != nil {
		config["type"] = tool.McpConfig.Type
		config["url"] = tool.McpConfig.Url
		config["authenticationRequired"] = tool.McpConfig.AuthenticationRequired
		config["credentialType"] = tool.McpConfig.CredentialType
		config["tokenConfigured"] = tool.McpConfig.Token != ""
	}
	return &ToolResponse{
		ID:               tool.ID.String(),
		Name:             tool.Name,
		Description:      tool.Description,
		Type:             string(tool.ToolType),
		IsEnable:         tool.IsEnable,
		ParametersSchema: tool.ParametersSchema,
		Config:           config,
		CreatedAt:        tool.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:        tool.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

// PublicService 只对其他模块暴露跨模块查询，避免智能体模块直接依赖工具仓储。
type PublicService struct {
	repo repository
}

func NewPublicService() *PublicService {
	return &PublicService{repo: newModels()}
}

func (s *PublicService) GetToolsInIds(e event.Event) (any, error) {
	request, ok := e.Data.(*shared.ToolsReq)
	if !ok {
		return nil, errs.ErrParam
	}
	if len(request.Ids) == 0 {
		return []*model.Tool{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), databaseTimeout)
	defer cancel()
	return s.repo.getToolsInIds(ctx, request.Ids)
}
