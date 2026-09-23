package tools

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"common/biz"
	"core/tools"
	"model"
	"model/shared"

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
		tool.McpConfig = request.McpConfig
	} else {
		// 系统工具的真实名称、描述和参数以注册表为准，避免前端展示名和模型调用名不一致。
		registered := tools.FindTool(name)
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
			if strings.TrimSpace(request.McpConfig.Url) == "" {
				return nil, biz.ErrMcpConfigRequired
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
	if tool.ToolType != model.SystemToolType {
		return &TestToolResponse{Success: false, Message: "MCP 工具调用将在后续章节接入"}, nil
	}

	registered := tools.FindTool(tool.Name)
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

func convertToolToResponse(tool *model.Tool) *ToolResponse {
	config := model.JSON{}
	if tool.McpConfig != nil {
		encoded, err := json.Marshal(tool.McpConfig)
		if err == nil {
			_ = json.Unmarshal(encoded, &config)
		}
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
