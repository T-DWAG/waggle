package tools

import (
	"context"

	"model"

	"github.com/google/uuid"
	"github.com/mszlu521/thunder/database"
	"github.com/mszlu521/thunder/gorms"
	"gorm.io/gorm"
)

type repository interface {
	create(ctx context.Context, tool *model.Tool) error
	update(ctx context.Context, tool *model.Tool) error
	get(ctx context.Context, id, creatorID uuid.UUID) (*model.Tool, error)
	getByName(ctx context.Context, creatorID uuid.UUID, name string) (*model.Tool, error)
	list(ctx context.Context, filter toolFilter) ([]*model.Tool, int64, error)
	delete(ctx context.Context, id, creatorID uuid.UUID) error
	getToolsInIds(ctx context.Context, ids []uuid.UUID) ([]*model.Tool, error)
}

type models struct {
	db *gorm.DB
}

func newModels() *models {
	return &models{db: database.GetPostgresDB().GormDB}
}

func (m *models) create(ctx context.Context, tool *model.Tool) error {
	return m.db.WithContext(ctx).Create(tool).Error
}

func (m *models) update(ctx context.Context, tool *model.Tool) error {
	return m.db.WithContext(ctx).Save(tool).Error
}

func (m *models) get(ctx context.Context, id, creatorID uuid.UUID) (*model.Tool, error) {
	var tool model.Tool
	err := m.db.WithContext(ctx).Where("id = ? AND creator_id = ?", id, creatorID).First(&tool).Error
	if gorms.IsRecordNotFoundError(err) {
		return nil, nil
	}
	return &tool, err
}

func (m *models) getByName(ctx context.Context, creatorID uuid.UUID, name string) (*model.Tool, error) {
	var tool model.Tool
	err := m.db.WithContext(ctx).Where("creator_id = ? AND name = ?", creatorID, name).First(&tool).Error
	if gorms.IsRecordNotFoundError(err) {
		return nil, nil
	}
	return &tool, err
}

func (m *models) list(ctx context.Context, filter toolFilter) ([]*model.Tool, int64, error) {
	var (
		items []*model.Tool
		total int64
	)
	query := m.db.WithContext(ctx).Model(&model.Tool{}).Where("creator_id = ?", filter.CreatorID)
	if filter.Name != "" {
		query = query.Where("name ILIKE ?", "%"+filter.Name+"%")
	}
	if filter.ToolType != "" {
		query = query.Where("tool_type = ?", filter.ToolType)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	err := query.Order("created_at DESC").Find(&items).Error
	return items, total, err
}

func (m *models) delete(ctx context.Context, id, creatorID uuid.UUID) error {
	return m.db.WithContext(ctx).Where("id = ? AND creator_id = ?", id, creatorID).Delete(&model.Tool{}).Error
}

func (m *models) getToolsInIds(ctx context.Context, ids []uuid.UUID) ([]*model.Tool, error) {
	var items []*model.Tool
	if len(ids) == 0 {
		return items, nil
	}
	err := m.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error
	return items, err
}
