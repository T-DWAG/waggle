package agents

import (
	"context"

	"model"
	"model/shared"

	"github.com/google/uuid"
	"github.com/mszlu521/thunder/database"
	"github.com/mszlu521/thunder/gorms"
	"gorm.io/gorm"
)

// Repository contains database-only operations. It never returns business errors.
type Repository interface {
	ListAgents(ctx context.Context, filter AgentFilter, creatorID uuid.UUID) ([]*model.Agent, int64, error)
	CreateAgent(ctx context.Context, agent *model.Agent) error
	GetAgentByIDAndCreator(ctx context.Context, id, creatorID uuid.UUID) (*model.Agent, error)
	UpdateAgent(ctx context.Context, agent *model.Agent) error

	ListProviderConfigs(ctx context.Context, filter ProviderConfigFilter) ([]*model.ProviderConfig, int64, error)
	CreateProviderConfig(ctx context.Context, config *model.ProviderConfig) error
	GetProviderConfigByIDAndUser(ctx context.Context, id, userID uuid.UUID) (*model.ProviderConfig, error)
	UpdateProviderConfig(ctx context.Context, config *model.ProviderConfig) error
	DeleteProviderConfig(ctx context.Context, id, userID uuid.UUID) error
	HasLLMsForProviderConfig(ctx context.Context, configID, userID uuid.UUID) (bool, error)
	GetProviderConfigByProvider(ctx context.Context, params *shared.LLMParams) (*model.ProviderConfig, error)

	ListLLMs(ctx context.Context, filter LLMFilter) ([]*model.LLM, int64, error)
	CreateLLM(ctx context.Context, llm *model.LLM) error
	GetLLMByIDAndUser(ctx context.Context, id, userID uuid.UUID) (*model.LLM, error)
	UpdateLLM(ctx context.Context, llm *model.LLM) error
	DeleteLLM(ctx context.Context, id, userID uuid.UUID) error
	GetActiveLLMByProviderAndName(ctx context.Context, userID uuid.UUID, provider, modelName string) (*model.LLM, error)

	GetAgentWithTools(ctx context.Context, id, creatorID uuid.UUID) (*model.Agent, error)
	DeleteAgentTools(ctx context.Context, agentID uuid.UUID) error
	CreateAgentTools(ctx context.Context, agentTools []model.AgentTool) error
}

type Model struct {
	db *gorm.DB
}

func NewModel() *Model {
	return &Model{db: database.GetPostgresDB().GormDB}
}

func recordNotFound(err error) bool {
	return gorms.IsRecordNotFoundError(err)
}

func (m *Model) ListAgents(ctx context.Context, filter AgentFilter, creatorID uuid.UUID) ([]*model.Agent, int64, error) {
	var (
		agents []*model.Agent
		total  int64
	)

	countQuery := m.db.WithContext(ctx).Model(&model.Agent{}).Where("creator_id = ?", creatorID)
	if filter.Name != "" {
		countQuery = countQuery.Where("name ILIKE ?", "%"+filter.Name+"%")
	}
	if filter.Status != "" {
		countQuery = countQuery.Where("status = ?", filter.Status)
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := m.db.WithContext(ctx).Where("creator_id = ?", creatorID)
	if filter.Name != "" {
		query = query.Where("name ILIKE ?", "%"+filter.Name+"%")
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	if err := query.Order("created_at DESC").Find(&agents).Error; err != nil {
		return nil, 0, err
	}
	return agents, total, nil
}

func (m *Model) CreateAgent(ctx context.Context, agent *model.Agent) error {
	return m.db.WithContext(ctx).Create(agent).Error
}

func (m *Model) GetAgentByIDAndCreator(ctx context.Context, id, creatorID uuid.UUID) (*model.Agent, error) {
	var agent model.Agent
	err := m.db.WithContext(ctx).Where("id = ? AND creator_id = ?", id, creatorID).First(&agent).Error
	if recordNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func (m *Model) UpdateAgent(ctx context.Context, agent *model.Agent) error {
	return m.db.WithContext(ctx).Save(agent).Error
}

func (m *Model) ListProviderConfigs(ctx context.Context, filter ProviderConfigFilter) ([]*model.ProviderConfig, int64, error) {
	var (
		configs []*model.ProviderConfig
		total   int64
	)

	countQuery := m.db.WithContext(ctx).Model(&model.ProviderConfig{}).Where("user_id = ?", filter.UserID)
	if filter.Name != "" {
		countQuery = countQuery.Where("name ILIKE ?", "%"+filter.Name+"%")
	}
	if filter.Provider != "" {
		countQuery = countQuery.Where("provider = ?", filter.Provider)
	}
	if filter.Status != "" {
		countQuery = countQuery.Where("status = ?", filter.Status)
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := m.db.WithContext(ctx).Where("user_id = ?", filter.UserID)
	if filter.Name != "" {
		query = query.Where("name ILIKE ?", "%"+filter.Name+"%")
	}
	if filter.Provider != "" {
		query = query.Where("provider = ?", filter.Provider)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	if err := query.Order("created_at DESC").Find(&configs).Error; err != nil {
		return nil, 0, err
	}
	return configs, total, nil
}

func (m *Model) CreateProviderConfig(ctx context.Context, config *model.ProviderConfig) error {
	return m.db.WithContext(ctx).Create(config).Error
}

func (m *Model) GetProviderConfigByIDAndUser(ctx context.Context, id, userID uuid.UUID) (*model.ProviderConfig, error) {
	var config model.ProviderConfig
	err := m.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&config).Error
	if recordNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (m *Model) UpdateProviderConfig(ctx context.Context, config *model.ProviderConfig) error {
	return m.db.WithContext(ctx).Save(config).Error
}

func (m *Model) DeleteProviderConfig(ctx context.Context, id, userID uuid.UUID) error {
	return m.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.ProviderConfig{}).Error
}

func (m *Model) HasLLMsForProviderConfig(ctx context.Context, configID, userID uuid.UUID) (bool, error) {
	var count int64
	err := m.db.WithContext(ctx).Model(&model.LLM{}).
		Where("provider_config_id = ? AND user_id = ?", configID, userID).
		Count(&count).Error
	return count > 0, err
}

// GetProviderConfigByProvider is called by router/event.go. The event data has
// no user ID, so the selected LLM and its provider configuration must be joined
// to ensure they belong to the same user.
func (m *Model) GetProviderConfigByProvider(ctx context.Context, params *shared.LLMParams) (*model.ProviderConfig, error) {
	var config model.ProviderConfig
	query := m.db.WithContext(ctx).
		Table("provider_configs").
		Select("provider_configs.*").
		Joins("JOIN llms ON llms.provider_config_id = provider_configs.id").
		Where("provider_configs.provider = ?", params.Provider).
		Where("provider_configs.status = ?", model.LLMStatusActive).
		Where("llms.model_name = ?", params.Model).
		Where("llms.model_type = ?", params.ModelType).
		Where("llms.status = ?", model.LLMStatusActive).
		Where("llms.user_id = provider_configs.user_id")
	if params.UserID != uuid.Nil {
		query = query.Where("provider_configs.user_id = ?", params.UserID)
	}
	err := query.Order("provider_configs.created_at DESC").First(&config).Error
	if recordNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (m *Model) ListLLMs(ctx context.Context, filter LLMFilter) ([]*model.LLM, int64, error) {
	var (
		llms  []*model.LLM
		total int64
	)

	countQuery := m.db.WithContext(ctx).Model(&model.LLM{}).Where("user_id = ?", filter.UserID)
	if filter.Name != "" {
		countQuery = countQuery.Where("name ILIKE ?", "%"+filter.Name+"%")
	}
	if filter.ProviderConfigID != nil {
		countQuery = countQuery.Where("provider_config_id = ?", *filter.ProviderConfigID)
	}
	if filter.ModelType != "" {
		countQuery = countQuery.Where("model_type = ?", filter.ModelType)
	}
	if filter.Status != "" {
		countQuery = countQuery.Where("status = ?", filter.Status)
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := m.db.WithContext(ctx).Where("user_id = ?", filter.UserID)
	if filter.Name != "" {
		query = query.Where("name ILIKE ?", "%"+filter.Name+"%")
	}
	if filter.ProviderConfigID != nil {
		query = query.Where("provider_config_id = ?", *filter.ProviderConfigID)
	}
	if filter.ModelType != "" {
		query = query.Where("model_type = ?", filter.ModelType)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	if err := query.Order("created_at DESC").Find(&llms).Error; err != nil {
		return nil, 0, err
	}
	return llms, total, nil
}

func (m *Model) CreateLLM(ctx context.Context, llm *model.LLM) error {
	return m.db.WithContext(ctx).Create(llm).Error
}

func (m *Model) GetLLMByIDAndUser(ctx context.Context, id, userID uuid.UUID) (*model.LLM, error) {
	var llm model.LLM
	err := m.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&llm).Error
	if recordNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &llm, nil
}

func (m *Model) UpdateLLM(ctx context.Context, llm *model.LLM) error {
	return m.db.WithContext(ctx).Save(llm).Error
}

func (m *Model) DeleteLLM(ctx context.Context, id, userID uuid.UUID) error {
	return m.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.LLM{}).Error
}

func (m *Model) GetAgentWithTools(ctx context.Context, id, creatorID uuid.UUID) (*model.Agent, error) {
	var agent model.Agent
	err := m.db.WithContext(ctx).
		Preload("Tools").
		Where("id = ? AND creator_id = ?", id, creatorID).
		First(&agent).Error
	if recordNotFound(err) {
		return nil, nil
	}
	return &agent, err
}

func (m *Model) DeleteAgentTools(ctx context.Context, agentID uuid.UUID) error {
	return m.db.WithContext(ctx).Where("agent_id = ?", agentID).Delete(&model.AgentTool{}).Error
}

func (m *Model) CreateAgentTools(ctx context.Context, agentTools []model.AgentTool) error {
	if len(agentTools) == 0 {
		return nil
	}
	return m.db.WithContext(ctx).CreateInBatches(&agentTools, len(agentTools)).Error
}

func (m *Model) GetActiveLLMByProviderAndName(ctx context.Context, userID uuid.UUID, provider, modelName string) (*model.LLM, error) {
	var llm model.LLM
	err := m.db.WithContext(ctx).
		Joins("JOIN provider_configs ON provider_configs.id = llms.provider_config_id").
		Where("llms.user_id = ?", userID).
		Where("llms.model_name = ?", modelName).
		Where("llms.model_type = ?", model.LLMTypeChat).
		Where("llms.status = ?", model.LLMStatusActive).
		Where("provider_configs.provider = ?", provider).
		Where("provider_configs.user_id = ?", userID).
		Where("provider_configs.status = ?", model.LLMStatusActive).
		First(&llm).Error
	if recordNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &llm, nil
}
