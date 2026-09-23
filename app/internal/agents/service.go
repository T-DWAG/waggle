package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"common/biz"
	"core"
	"core/ai"
	"core/tools"
	"model"
	"model/shared"

	ollamaModel "github.com/cloudwego/eino-ext/components/model/ollama"
	openaiModel "github.com/cloudwego/eino-ext/components/model/openai"
	qwenModel "github.com/cloudwego/eino-ext/components/model/qwen"
	einoModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/mszlu521/thunder/errs"
	"github.com/mszlu521/thunder/event"
	"github.com/mszlu521/thunder/logs"
)

const databaseTimeout = 5 * time.Second

type Service struct {
	repo Repository
}

func NewService() *Service {
	return &Service{repo: NewModel()}
}

func (s *Service) ListAgents(parent context.Context, request SearchRequest, userID uuid.UUID) (*ListAgentResponse, error) {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	limit, offset := normalizePage(request.Page, request.PageSize)

	agents, total, err := s.repo.ListAgents(ctx, AgentFilter{
		Name: request.Name, Status: request.Status, Limit: limit, Offset: offset,
	}, userID)
	if err != nil {
		logs.Errorf("list agents: %v", err)
		return nil, errs.DBError
	}
	return &ListAgentResponse{Agents: agents, Total: total}, nil
}

func (s *Service) CreateAgent(parent context.Context, request CreateAgentRequest, userID uuid.UUID) (*model.Agent, error) {
	if strings.TrimSpace(request.Name) == "" {
		return nil, errs.ErrParam
	}
	if request.Status == "" {
		request.Status = model.Draft
	}
	if !validAgentStatus(request.Status) {
		return nil, errs.ErrParam
	}

	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	agent := &model.Agent{
		BaseModel:       model.BaseModel{ID: uuid.New()},
		CreatorID:       userID,
		Name:            strings.TrimSpace(request.Name),
		Description:     request.Description,
		Status:          request.Status,
		Visibility:      model.Private,
		ModelParameters: model.JSON{},
	}
	if err := s.repo.CreateAgent(ctx, agent); err != nil {
		logs.Errorf("create agent: %v", err)
		return nil, errs.DBError
	}
	return agent, nil
}

func (s *Service) GetAgent(parent context.Context, id, userID uuid.UUID) (*model.Agent, error) {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	agent, err := s.repo.GetAgentWithTools(ctx, id, userID)
	if err != nil {
		logs.Errorf("get agent: %v", err)
		return nil, errs.DBError
	}
	if agent == nil {
		return nil, biz.ErrAgentNotFound
	}
	return agent, nil
}

func (s *Service) UpdateAgent(parent context.Context, request UpdateAgentRequest, userID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	agent, err := s.repo.GetAgentByIDAndCreator(ctx, request.ID, userID)
	if err != nil {
		logs.Errorf("get agent for update: %v", err)
		return errs.DBError
	}
	if agent == nil {
		return biz.ErrAgentNotFound
	}

	if request.Name != nil {
		if strings.TrimSpace(*request.Name) == "" {
			return errs.ErrParam
		}
		agent.Name = strings.TrimSpace(*request.Name)
	}
	if request.Description != nil {
		agent.Description = *request.Description
	}
	if request.Icon != nil {
		agent.Icon = *request.Icon
	}
	if request.Status != nil {
		if !validAgentStatus(*request.Status) {
			return errs.ErrParam
		}
		agent.Status = *request.Status
	}
	if request.Visibility != nil {
		if !validAgentVisibility(*request.Visibility) {
			return errs.ErrParam
		}
		agent.Visibility = *request.Visibility
	}
	if request.SystemPrompt != nil {
		agent.SystemPrompt = *request.SystemPrompt
	}
	if request.ModelProvider != nil {
		agent.ModelProvider = strings.TrimSpace(*request.ModelProvider)
	}
	if request.ModelName != nil {
		agent.ModelName = strings.TrimSpace(*request.ModelName)
	}
	if request.ModelParameters != nil {
		agent.ModelParameters = *request.ModelParameters
	}
	if request.OpeningDialogue != nil {
		agent.OpeningDialogue = *request.OpeningDialogue
	}
	if request.SuggestedQuestions != nil {
		agent.SuggestedQuestions = *request.SuggestedQuestions
	}

	if agent.Status == model.Published {
		if err := s.validatePublishedAgent(ctx, agent, userID); err != nil {
			return err
		}
		if agent.PublishedAt == nil {
			now := time.Now()
			agent.PublishedAt = &now
		}
	}
	if err := s.repo.UpdateAgent(ctx, agent); err != nil {
		logs.Errorf("update agent: %v", err)
		return errs.DBError
	}
	return nil
}

func (s *Service) ListProviderConfigs(parent context.Context, request ProviderConfigListQuery, userID uuid.UUID) (*ListProviderConfigsResponse, error) {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	limit, offset := normalizePage(request.Page, request.PageSize)
	configs, total, err := s.repo.ListProviderConfigs(ctx, ProviderConfigFilter{
		UserID: userID, Name: request.Name, Provider: request.Provider, Status: request.Status, Limit: limit, Offset: offset,
	})
	if err != nil {
		logs.Errorf("list provider configs: %v", err)
		return nil, errs.DBError
	}
	response := make([]*ProviderConfigResponse, 0, len(configs))
	for _, config := range configs {
		response = append(response, toProviderConfigResponse(config))
	}
	return &ListProviderConfigsResponse{ProviderConfigs: response, Total: total}, nil
}

func (s *Service) CreateProviderConfig(parent context.Context, request CreateProviderConfigRequest, userID uuid.UUID) (*CreateProviderConfigResponse, error) {
	if strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.Provider) == "" {
		return nil, errs.ErrParam
	}
	if request.Status == "" {
		request.Status = model.LLMStatusActive
	}
	if !validLLMStatus(request.Status) {
		return nil, errs.ErrParam
	}
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	config := &model.ProviderConfig{
		BaseModel: model.BaseModel{ID: uuid.New()}, UserID: userID,
		Name: strings.TrimSpace(request.Name), Provider: strings.TrimSpace(request.Provider),
		Description: request.Description, APIKey: request.APIKey, APIBase: request.APIBase, Status: request.Status,
	}
	if err := s.repo.CreateProviderConfig(ctx, config); err != nil {
		logs.Errorf("create provider config: %v", err)
		return nil, errs.DBError
	}
	return &CreateProviderConfigResponse{ID: config.ID}, nil
}

func (s *Service) GetProviderConfig(parent context.Context, id, userID uuid.UUID) (*ProviderConfigResponse, error) {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	config, err := s.repo.GetProviderConfigByIDAndUser(ctx, id, userID)
	if err != nil {
		logs.Errorf("get provider config: %v", err)
		return nil, errs.DBError
	}
	if config == nil {
		return nil, biz.ErrProviderConfigNotFound
	}
	return toProviderConfigResponse(config), nil
}

func (s *Service) UpdateProviderConfig(parent context.Context, id, userID uuid.UUID, request UpdateProviderConfigRequest) error {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	config, err := s.repo.GetProviderConfigByIDAndUser(ctx, id, userID)
	if err != nil {
		logs.Errorf("get provider config for update: %v", err)
		return errs.DBError
	}
	if config == nil {
		return biz.ErrProviderConfigNotFound
	}
	if request.Name != nil {
		if strings.TrimSpace(*request.Name) == "" {
			return errs.ErrParam
		}
		config.Name = strings.TrimSpace(*request.Name)
	}
	if request.Description != nil {
		config.Description = *request.Description
	}
	if request.APIKey != nil {
		config.APIKey = *request.APIKey
	}
	if request.APIBase != nil {
		config.APIBase = *request.APIBase
	}
	if request.Status != nil {
		if !validLLMStatus(*request.Status) {
			return errs.ErrParam
		}
		config.Status = *request.Status
	}
	if err := s.repo.UpdateProviderConfig(ctx, config); err != nil {
		logs.Errorf("update provider config: %v", err)
		return errs.DBError
	}
	return nil
}

func (s *Service) DeleteProviderConfig(parent context.Context, id, userID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	config, err := s.repo.GetProviderConfigByIDAndUser(ctx, id, userID)
	if err != nil {
		logs.Errorf("get provider config for deletion: %v", err)
		return errs.DBError
	}
	if config == nil {
		return biz.ErrProviderConfigNotFound
	}
	inUse, err := s.repo.HasLLMsForProviderConfig(ctx, id, userID)
	if err != nil {
		logs.Errorf("check provider config usage: %v", err)
		return errs.DBError
	}
	if inUse {
		return biz.ErrProviderConfigInUse
	}
	if err := s.repo.DeleteProviderConfig(ctx, id, userID); err != nil {
		logs.Errorf("delete provider config: %v", err)
		return errs.DBError
	}
	return nil
}

func (s *Service) ListLLMs(parent context.Context, request LLMListQuery, userID uuid.UUID) (*ListLLMsResponse, error) {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	limit, offset := normalizePage(request.Page, request.PageSize)
	llms, total, err := s.repo.ListLLMs(ctx, LLMFilter{
		UserID: userID, Name: request.Name, ProviderConfigID: request.ProviderConfigID,
		ModelType: request.ModelType, Status: request.Status, Limit: limit, Offset: offset,
	})
	if err != nil {
		logs.Errorf("list llms: %v", err)
		return nil, errs.DBError
	}
	response := make([]*LLMResponse, 0, len(llms))
	for _, llm := range llms {
		response = append(response, toLLMResponse(llm))
	}
	return &ListLLMsResponse{LLMs: response, Total: total}, nil
}

func (s *Service) ListLLMsByProviderConfig(parent context.Context, configID, userID uuid.UUID) (*ListLLMsResponse, error) {
	return s.ListLLMs(parent, LLMListQuery{ProviderConfigID: &configID, Page: 1, PageSize: maxPageSize}, userID)
}

func (s *Service) CreateLLM(parent context.Context, request CreateLLMRequest, userID uuid.UUID) (*CreateLLMResponse, error) {
	if strings.TrimSpace(request.Name) == "" || strings.TrimSpace(request.ModelName) == "" || request.ProviderConfigID == uuid.Nil {
		return nil, errs.ErrParam
	}
	if request.ModelType == "" {
		request.ModelType = model.LLMTypeChat
	}
	if !validLLMType(request.ModelType) {
		return nil, errs.ErrParam
	}
	if request.Status == "" {
		request.Status = model.LLMStatusActive
	}
	if !validLLMStatus(request.Status) {
		return nil, errs.ErrParam
	}

	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	config, err := s.repo.GetProviderConfigByIDAndUser(ctx, request.ProviderConfigID, userID)
	if err != nil {
		logs.Errorf("get provider config for llm creation: %v", err)
		return nil, errs.DBError
	}
	if config == nil {
		return nil, biz.ErrProviderConfigNotFound
	}
	llm := &model.LLM{
		BaseModel: model.BaseModel{ID: uuid.New()}, UserID: userID, Name: strings.TrimSpace(request.Name),
		Description: request.Description, ProviderConfigID: request.ProviderConfigID,
		ModelName: strings.TrimSpace(request.ModelName), ModelType: request.ModelType,
		Config: request.Config, Status: request.Status,
	}
	if err := s.repo.CreateLLM(ctx, llm); err != nil {
		logs.Errorf("create llm: %v", err)
		return nil, errs.DBError
	}
	return &CreateLLMResponse{ID: llm.ID}, nil
}

func (s *Service) GetLLM(parent context.Context, id, userID uuid.UUID) (*GetLLMResponse, error) {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	llm, err := s.repo.GetLLMByIDAndUser(ctx, id, userID)
	if err != nil {
		logs.Errorf("get llm: %v", err)
		return nil, errs.DBError
	}
	if llm == nil {
		return nil, biz.ErrLLMNotFound
	}
	return &GetLLMResponse{LLM: toLLMResponse(llm)}, nil
}

func (s *Service) UpdateLLM(parent context.Context, id, userID uuid.UUID, request UpdateLLMRequest) error {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	llm, err := s.repo.GetLLMByIDAndUser(ctx, id, userID)
	if err != nil {
		logs.Errorf("get llm for update: %v", err)
		return errs.DBError
	}
	if llm == nil {
		return biz.ErrLLMNotFound
	}
	if request.Name != nil {
		if strings.TrimSpace(*request.Name) == "" {
			return errs.ErrParam
		}
		llm.Name = strings.TrimSpace(*request.Name)
	}
	if request.Description != nil {
		llm.Description = *request.Description
	}
	if request.ProviderConfigID != nil {
		config, err := s.repo.GetProviderConfigByIDAndUser(ctx, *request.ProviderConfigID, userID)
		if err != nil {
			logs.Errorf("get replacement provider config: %v", err)
			return errs.DBError
		}
		if config == nil {
			return biz.ErrProviderConfigNotFound
		}
		llm.ProviderConfigID = *request.ProviderConfigID
	}
	if request.ModelName != nil {
		if strings.TrimSpace(*request.ModelName) == "" {
			return errs.ErrParam
		}
		llm.ModelName = strings.TrimSpace(*request.ModelName)
	}
	if request.ModelType != nil {
		if !validLLMType(*request.ModelType) {
			return errs.ErrParam
		}
		llm.ModelType = *request.ModelType
	}
	if request.Config != nil {
		llm.Config = *request.Config
	}
	if request.Status != nil {
		if !validLLMStatus(*request.Status) {
			return errs.ErrParam
		}
		llm.Status = *request.Status
	}
	if err := s.repo.UpdateLLM(ctx, llm); err != nil {
		logs.Errorf("update llm: %v", err)
		return errs.DBError
	}
	return nil
}

func (s *Service) UpdateAgentTools(parent context.Context, userID, agentID uuid.UUID, request *ToolsRequest) ([]model.AgentTool, error) {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	agent, err := s.repo.GetAgentByIDAndCreator(ctx, agentID, userID)
	if err != nil {
		logs.Errorf("get agent for tool association: %v", err)
		return nil, errs.DBError
	}
	if agent == nil {
		return nil, biz.ErrAgentNotFound
	}

	ids := make([]uuid.UUID, 0, len(request.Tools))
	seen := make(map[uuid.UUID]struct{}, len(request.Tools))
	for _, item := range request.Tools {
		if item.ID == uuid.Nil {
			return nil, errs.ErrParam
		}
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		ids = append(ids, item.ID)
	}
	resolved, err := s.getToolsByIDs(ids)
	if err != nil {
		logs.Errorf("get tools for association: %v", err)
		return nil, errs.DBError
	}
	if len(resolved) != len(ids) {
		return nil, biz.ErrToolNotExist
	}

	if err := s.repo.DeleteAgentTools(ctx, agentID); err != nil {
		logs.Errorf("delete agent tools: %v", err)
		return nil, errs.DBError
	}
	agentTools := make([]model.AgentTool, 0, len(resolved))
	now := time.Now()
	for _, item := range resolved {
		if item.CreatorID != userID {
			return nil, biz.ErrToolNotExist
		}
		agentTools = append(agentTools, model.AgentTool{
			AgentID: agentID, ToolID: item.ID, Status: model.Enabled, CreatedAt: now,
		})
	}
	if err := s.repo.CreateAgentTools(ctx, agentTools); err != nil {
		logs.Errorf("create agent tools: %v", err)
		return nil, errs.DBError
	}
	return agentTools, nil
}

func (s *Service) getToolsByIDs(ids []uuid.UUID) ([]*model.Tool, error) {
	if len(ids) == 0 {
		return []*model.Tool{}, nil
	}
	result, err := event.Trigger("getToolsInIds", &shared.ToolsReq{Ids: ids})
	if err != nil {
		return nil, err
	}
	tools, ok := result.([]*model.Tool)
	if !ok {
		return nil, fmt.Errorf("unexpected tools event response: %T", result)
	}
	return tools, nil
}

func (s *Service) DeleteLLM(parent context.Context, id, userID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	llm, err := s.repo.GetLLMByIDAndUser(ctx, id, userID)
	if err != nil {
		logs.Errorf("get llm for deletion: %v", err)
		return errs.DBError
	}
	if llm == nil {
		return biz.ErrLLMNotFound
	}
	if err := s.repo.DeleteLLM(ctx, id, userID); err != nil {
		logs.Errorf("delete llm: %v", err)
		return errs.DBError
	}
	return nil
}

// AgentMessageStream 在模型返回工具调用时执行已关联工具，再把工具结果交回模型。
// 没有关联工具时保持原来的单次流式回答。
func (s *Service) AgentMessageStream(ctx context.Context, userID uuid.UUID, request ChatRequest) (<-chan string, <-chan error) {
	dataChan := make(chan string, 32)
	errorChan := make(chan error, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				sendError(ctx, errorChan, fmt.Errorf("internal server error"))
				logs.Errorf("panic in agent stream: %v", recovered)
			}
			close(dataChan)
			close(errorChan)
		}()

		agent, err := s.repo.GetAgentWithTools(ctx, request.AgentID, userID)
		if err != nil {
			logs.Errorf("get agent with tools: %v", err)
			sendError(ctx, errorChan, errs.DBError)
			return
		}
		if agent == nil {
			sendError(ctx, errorChan, biz.ErrAgentNotFound)
			return
		}
		if agent.Status != model.Published {
			sendError(ctx, errorChan, biz.ErrAgentNotReady)
			return
		}
		if err := s.validatePublishedAgent(ctx, agent, userID); err != nil {
			sendError(ctx, errorChan, err)
			return
		}

		result, err := event.Trigger("getProviderConfigByProvider", &shared.LLMParams{
			UserID:    userID,
			ModelType: model.LLMTypeChat,
			Provider:  agent.ModelProvider,
			Model:     agent.ModelName,
		})
		if err != nil {
			logs.Errorf("get provider config event: %v", err)
			sendError(ctx, errorChan, errs.DBError)
			return
		}
		providerResponse, ok := result.(*shared.ModelProviderResponse)
		if !ok {
			logs.Errorf("unexpected provider config event response: %T", result)
			sendError(ctx, errorChan, errs.DBError)
			return
		}
		config := providerResponse.ProvideConfig
		if config == nil || config.Status != model.LLMStatusActive {
			sendError(ctx, errorChan, biz.ErrProviderConfigUnavailable)
			return
		}

		chatModel, err := buildChatModel(ctx, agent, config)
		if err != nil {
			logs.Errorf("build chat model: %v", err)
			sendError(ctx, errorChan, biz.ErrUnsupportedProvider)
			return
		}
		agentTools := buildTools(agent)
		if len(agentTools) > 0 {
			toolInfos := make([]*schema.ToolInfo, 0, len(agentTools))
			for _, item := range agentTools {
				info, infoErr := item.Info(ctx)
				if infoErr != nil {
					logs.Errorf("load tool info: %v", infoErr)
					continue
				}
				toolInfos = append(toolInfos, info)
			}
			chatModel, err = chatModel.WithTools(toolInfos)
			if err != nil {
				logs.Errorf("bind tools: %v", err)
				sendError(ctx, errorChan, errors.New("工具绑定失败"))
				return
			}
		}

		messages := []*schema.Message{schema.SystemMessage(buildSystemPrompt(agent, agentTools)), schema.UserMessage(request.Message)}
		const maxToolRounds = 4
		for round := 0; ; round++ {
			stream, err := chatModel.Stream(ctx, messages)
			if err != nil {
				logs.Errorf("start model stream: %v", err)
				sendError(ctx, errorChan, errors.New("模型调用失败"))
				return
			}
			message, err := forwardStream(ctx, stream, dataChan, agent.Name)
			stream.Close()
			if err != nil {
				logs.Errorf("receive model stream: %v", err)
				sendError(ctx, errorChan, errors.New("模型流式响应失败"))
				return
			}
			if message == nil || len(message.ToolCalls) == 0 || round == maxToolRounds {
				return
			}

			messages = append(messages, message)
			for _, call := range message.ToolCalls {
				result, callErr := invokeAgentTool(ctx, agentTools, call)
				if callErr != nil {
					logs.Errorf("invoke tool %s: %v", call.Function.Name, callErr)
					result = "工具执行失败: " + callErr.Error()
					sendData(ctx, dataChan, core.BuildErrMessage(agent.Name, result))
				} else {
					sendData(ctx, dataChan, core.BuildContentMessage(agent.Name, call.Function.Name, "已调用工具 "+call.Function.Name))
				}
				messages = append(messages, schema.ToolMessage(result, call.ID))
			}
		}
	}()
	return dataChan, errorChan
}

func (s *Service) validatePublishedAgent(ctx context.Context, agent *model.Agent, userID uuid.UUID) error {
	if strings.TrimSpace(agent.ModelProvider) == "" || strings.TrimSpace(agent.ModelName) == "" {
		return biz.ErrAgentModelIncomplete
	}
	llm, err := s.repo.GetActiveLLMByProviderAndName(ctx, userID, agent.ModelProvider, agent.ModelName)
	if err != nil {
		logs.Errorf("validate agent model: %v", err)
		return errs.DBError
	}
	if llm == nil {
		return biz.ErrModelUnavailable
	}
	return nil
}

func buildChatModel(ctx context.Context, agent *model.Agent, config *model.ProviderConfig) (einoModel.ToolCallingChatModel, error) {
	params := agent.ModelParameters.ToModelParams()
	var (
		maxTokens   *int
		temperature *float32
		topP        *float32
	)
	if params.MaxTokens > 0 {
		value := params.MaxTokens
		maxTokens = &value
	}
	if params.Temperature != 0 {
		value := float32(params.Temperature)
		temperature = &value
	}
	if params.TopP != 0 {
		value := float32(params.TopP)
		topP = &value
	}

	switch config.Provider {
	case model.OpenAIProvider:
		return openaiModel.NewChatModel(ctx, &openaiModel.ChatModelConfig{
			APIKey: config.APIKey, BaseURL: config.APIBase, Model: agent.ModelName, Timeout: 90 * time.Second,
			MaxTokens: maxTokens, Temperature: temperature, TopP: topP,
		})
	case model.QwenProvider:
		return qwenModel.NewChatModel(ctx, &qwenModel.ChatModelConfig{
			APIKey: config.APIKey, BaseURL: config.APIBase, Model: agent.ModelName, Timeout: 90 * time.Second,
			MaxTokens: maxTokens, Temperature: temperature, TopP: topP,
		})
	case model.OllamaProvider:
		return ollamaModel.NewChatModel(ctx, &ollamaModel.ChatModelConfig{
			BaseURL: config.APIBase, Model: agent.ModelName, Timeout: 90 * time.Second,
		})
	default:
		return nil, fmt.Errorf("unsupported provider %q", config.Provider)
	}
}

func buildTools(agent *model.Agent) []tool.BaseTool {
	agentTools := make([]tool.BaseTool, 0, len(agent.Tools))
	for _, item := range agent.Tools {
		if !item.IsEnable {
			continue
		}
		switch item.ToolType {
		case model.SystemToolType:
			if systemTool := tools.FindTool(item.Name); systemTool != nil {
				agentTools = append(agentTools, systemTool)
			} else {
				logs.Warnf("system tool %s is not registered", item.Name)
			}
		default:
			logs.Warnf("unsupported tool type %s", item.ToolType)
		}
	}
	return agentTools
}

func buildSystemPrompt(agent *model.Agent, agentTools []tool.BaseTool) string {
	toolsInfo := "当前没有可用工具。"
	if len(agentTools) > 0 {
		toolsInfo = formatToolsDescription(agentTools)
	}
	prompt := strings.NewReplacer(
		"{role}", agent.SystemPrompt,
		"{ragContext}", "",
		"{toolsInfo}", toolsInfo,
		"{agentsInfo}", "",
	).Replace(ai.BaseSystemPrompt)
	return strings.TrimSpace(prompt)
}

func formatToolsDescription(agentTools []tool.BaseTool) string {
	var builder strings.Builder
	builder.WriteString("【可用工具列表】\n")
	for _, item := range agentTools {
		info, err := item.Info(context.Background())
		if err != nil || info == nil {
			continue
		}
		builder.WriteString(fmt.Sprintf("- name: `%s`\n", info.Name))
		builder.WriteString(fmt.Sprintf("  description: %q\n", info.Desc))
		if info.ParamsOneOf != nil {
			if params, err := info.ParamsOneOf.ToJSONSchema(); err == nil && params != nil {
				encoded, _ := json.Marshal(params)
				builder.WriteString(fmt.Sprintf("  parameters: %s\n", encoded))
			}
		}
		builder.WriteString("\n")
	}
	return builder.String()
}

func forwardStream(ctx context.Context, stream *schema.StreamReader[*schema.Message], output chan<- string, agentName string) (*schema.Message, error) {
	chunks := make([]*schema.Message, 0, 8)
	for {
		message, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if message == nil {
			continue
		}
		chunks = append(chunks, message)
		if message.ReasoningContent != "" {
			sendData(ctx, output, core.BuildReasoningMessage(agentName, message.ToolName, message.ReasoningContent))
		}
		if message.Content != "" {
			sendData(ctx, output, core.BuildContentMessage(agentName, message.ToolName, message.Content))
		}
	}
	if len(chunks) == 0 {
		return nil, nil
	}
	return schema.ConcatMessages(chunks)
}

func invokeAgentTool(ctx context.Context, agentTools []tool.BaseTool, call schema.ToolCall) (string, error) {
	for _, item := range agentTools {
		info, err := item.Info(ctx)
		if err != nil || info == nil || info.Name != call.Function.Name {
			continue
		}
		invokable, ok := item.(tool.InvokableTool)
		if !ok {
			return "", fmt.Errorf("tool %s is not invokable", info.Name)
		}
		arguments := call.Function.Arguments
		if strings.TrimSpace(arguments) == "" {
			arguments = "{}"
		}
		return invokable.InvokableRun(ctx, arguments)
	}
	return "", fmt.Errorf("tool %s is not associated with this agent", call.Function.Name)
}

func sendData(ctx context.Context, output chan<- string, data string) {
	select {
	case output <- data:
	case <-ctx.Done():
	}
}

func sendError(ctx context.Context, output chan<- error, err error) {
	select {
	case output <- err:
	case <-ctx.Done():
	}
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return pageSize, (page - 1) * pageSize
}

func validAgentStatus(status model.AgentStatus) bool {
	return status == model.Draft || status == model.Published || status == model.Archived
}

func validAgentVisibility(visibility model.AgentVisibility) bool {
	return visibility == model.Private || visibility == model.Public || visibility == model.LinkOnly
}

func validLLMStatus(status model.LLMStatus) bool {
	return status == model.LLMStatusActive || status == model.LLMStatusInactive
}

func validLLMType(modelType model.LLMType) bool {
	return modelType == model.LLMTypeChat || modelType == model.LLMTypeEmbedding || modelType == model.LLMTypeVision
}

func toProviderConfigResponse(config *model.ProviderConfig) *ProviderConfigResponse {
	return &ProviderConfigResponse{
		ID: config.ID, Name: config.Name, Provider: config.Provider, Description: config.Description,
		APIKey: maskAPIKey(config.APIKey), HasAPIKey: config.APIKey != "", APIBase: config.APIBase, Status: config.Status,
		CreatedAt: config.CreatedAt.Format(time.RFC3339), UpdatedAt: config.UpdatedAt.Format(time.RFC3339),
	}
}

func toLLMResponse(llm *model.LLM) *LLMResponse {
	return &LLMResponse{
		ID: llm.ID, Name: llm.Name, Description: llm.Description, ProviderConfigID: llm.ProviderConfigID,
		ModelName: llm.ModelName, ModelType: llm.ModelType, Config: llm.Config, Status: llm.Status,
		CreatedAt: llm.CreatedAt.Format(time.RFC3339), UpdatedAt: llm.UpdatedAt.Format(time.RFC3339),
	}
}

func maskAPIKey(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	if len(apiKey) <= 8 {
		return "****"
	}
	return apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
}
