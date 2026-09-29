package knowledge

import (
	"context"

	"model"

	"github.com/google/uuid"
	"github.com/mszlu521/thunder/database"
	"github.com/mszlu521/thunder/gorms"
	"gorm.io/gorm"
)

// repository 只做数据库操作，不返回业务错误；「查不到」统一返回 nil, nil。
type repository interface {
	createKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error
	getKnowledgeBase(ctx context.Context, id, creatorID uuid.UUID) (*model.KnowledgeBase, error)
	getKnowledgeBasesInIDs(ctx context.Context, ids []uuid.UUID, creatorID uuid.UUID) ([]*model.KnowledgeBase, error)
	listKnowledgeBases(ctx context.Context, filter knowledgeFilter) ([]*model.KnowledgeBase, int64, error)
	updateKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error
	deleteKnowledgeBase(ctx context.Context, id uuid.UUID) error
	isKnowledgeBaseInUse(ctx context.Context, id uuid.UUID) (bool, error)
	sumDocumentSize(ctx context.Context, kbID uuid.UUID) (int64, error)
	refreshCounts(ctx context.Context, kbID uuid.UUID) error

	createDocument(ctx context.Context, doc *model.Document) error
	getDocument(ctx context.Context, kbID, docID uuid.UUID) (*model.Document, error)
	findDocumentByHash(ctx context.Context, kbID uuid.UUID, hash string) (*model.Document, error)
	listDocuments(ctx context.Context, filter documentFilter) ([]*model.Document, int64, error)
	countChunksByDocuments(ctx context.Context, docIDs []uuid.UUID) (map[uuid.UUID]int64, error)
	updateDocumentFields(ctx context.Context, docID uuid.UUID, fields map[string]any) error
	disabledDocumentIDs(ctx context.Context, kbIDs []uuid.UUID) ([]string, error)
	deleteDocument(ctx context.Context, doc *model.Document) error
	listDocumentIDs(ctx context.Context, kbID uuid.UUID) ([]uuid.UUID, error)

	replaceChunks(ctx context.Context, docID uuid.UUID, chunks []*model.DocumentChunk) error
	listChunks(ctx context.Context, docID uuid.UUID) ([]*model.DocumentChunk, error)
}

type models struct {
	db *gorm.DB
}

func newModels() *models {
	return &models{db: database.GetPostgresDB().GormDB}
}

func notFound(err error) bool {
	return gorms.IsRecordNotFoundError(err)
}

func (m *models) createKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error {
	return m.db.WithContext(ctx).Create(kb).Error
}

func (m *models) getKnowledgeBase(ctx context.Context, id, creatorID uuid.UUID) (*model.KnowledgeBase, error) {
	var kb model.KnowledgeBase
	err := m.db.WithContext(ctx).Where("id = ? AND creator_id = ?", id, creatorID).First(&kb).Error
	if notFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

func (m *models) getKnowledgeBasesInIDs(ctx context.Context, ids []uuid.UUID, creatorID uuid.UUID) ([]*model.KnowledgeBase, error) {
	var items []*model.KnowledgeBase
	if len(ids) == 0 {
		return items, nil
	}
	err := m.db.WithContext(ctx).Where("id IN ? AND creator_id = ?", ids, creatorID).Find(&items).Error
	return items, err
}

func (m *models) listKnowledgeBases(ctx context.Context, filter knowledgeFilter) ([]*model.KnowledgeBase, int64, error) {
	var (
		items []*model.KnowledgeBase
		total int64
	)
	query := m.db.WithContext(ctx).Model(&model.KnowledgeBase{}).Where("creator_id = ?", filter.CreatorID)
	if filter.Name != "" {
		query = query.Where("name ILIKE ?", "%"+filter.Name+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("created_at DESC").Limit(filter.Limit).Offset(filter.Offset).Find(&items).Error
	return items, total, err
}

func (m *models) updateKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error {
	return m.db.WithContext(ctx).Model(kb).Select("name", "description", "tags", "status", "updated_at").Updates(kb).Error
}

// deleteKnowledgeBase 事务里软删库、文档与切片，关联表硬删。
func (m *models) deleteKnowledgeBase(ctx context.Context, id uuid.UUID) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("kb_id = ?", id).Delete(&model.DocumentChunk{}).Error; err != nil {
			return err
		}
		if err := tx.Where("kb_id = ?", id).Delete(&model.Document{}).Error; err != nil {
			return err
		}
		if err := tx.Where("knowledge_base_id = ?", id).Delete(&model.AgentKnowledgeBase{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&model.KnowledgeBase{}).Error
	})
}

// isKnowledgeBaseInUse 只看启用中的关联：已解绑（disabled）的不阻止删除。
func (m *models) isKnowledgeBaseInUse(ctx context.Context, id uuid.UUID) (bool, error) {
	var count int64
	err := m.db.WithContext(ctx).Model(&model.AgentKnowledgeBase{}).
		Joins("JOIN agents ON agents.id = agent_knowledge_bases.agent_id AND agents.deleted_at IS NULL").
		Where("agent_knowledge_bases.knowledge_base_id = ? AND agent_knowledge_bases.status = ?", id, model.Enabled).
		Count(&count).Error
	return count > 0, err
}

func (m *models) sumDocumentSize(ctx context.Context, kbID uuid.UUID) (int64, error) {
	var total int64
	err := m.db.WithContext(ctx).Model(&model.Document{}).Where("kb_id = ?", kbID).
		Select("COALESCE(SUM(size), 0)").Scan(&total).Error
	return total, err
}

// refreshCounts 用聚合重算统计，而不是 +1/-1：
// 异步处理、重索引、删除交错时，增量计数一旦漏一次就永远不准。
func (m *models) refreshCounts(ctx context.Context, kbID uuid.UUID) error {
	return m.db.WithContext(ctx).Exec(`
UPDATE knowledge_bases SET
  document_count = (SELECT COUNT(*) FROM documents WHERE kb_id = ? AND deleted_at IS NULL),
  chunk_count = (SELECT COUNT(*) FROM document_chunks WHERE kb_id = ? AND deleted_at IS NULL),
  updated_at = NOW()
WHERE id = ?`, kbID, kbID, kbID).Error
}

func (m *models) createDocument(ctx context.Context, doc *model.Document) error {
	return m.db.WithContext(ctx).Create(doc).Error
}

func (m *models) getDocument(ctx context.Context, kbID, docID uuid.UUID) (*model.Document, error) {
	var doc model.Document
	err := m.db.WithContext(ctx).Where("id = ? AND kb_id = ?", docID, kbID).First(&doc).Error
	if notFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (m *models) findDocumentByHash(ctx context.Context, kbID uuid.UUID, hash string) (*model.Document, error) {
	var doc model.Document
	err := m.db.WithContext(ctx).Where("kb_id = ? AND file_hash = ?", kbID, hash).First(&doc).Error
	if notFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (m *models) listDocuments(ctx context.Context, filter documentFilter) ([]*model.Document, int64, error) {
	var (
		items []*model.Document
		total int64
	)
	query := m.db.WithContext(ctx).Model(&model.Document{}).Where("kb_id = ?", filter.KnowledgeBaseID)
	if filter.Name != "" {
		query = query.Where("name ILIKE ?", "%"+filter.Name+"%")
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("created_at DESC").Limit(filter.Limit).Offset(filter.Offset).Find(&items).Error
	return items, total, err
}

func (m *models) countChunksByDocuments(ctx context.Context, docIDs []uuid.UUID) (map[uuid.UUID]int64, error) {
	result := make(map[uuid.UUID]int64, len(docIDs))
	if len(docIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		DocumentID uuid.UUID
		Count      int64
	}
	err := m.db.WithContext(ctx).Model(&model.DocumentChunk{}).
		Select("document_id, COUNT(*) AS count").
		Where("document_id IN ?", docIDs).
		Group("document_id").Scan(&rows).Error
	for _, row := range rows {
		result[row.DocumentID] = row.Count
	}
	return result, err
}

func (m *models) updateDocumentFields(ctx context.Context, docID uuid.UUID, fields map[string]any) error {
	return m.db.WithContext(ctx).Model(&model.Document{}).Where("id = ?", docID).Updates(fields).Error
}

// disabledDocumentIDs 检索时要排除的文档：停用的，以及还没处理完的（ES 里可能只有半截）。
func (m *models) disabledDocumentIDs(ctx context.Context, kbIDs []uuid.UUID) ([]string, error) {
	var ids []string
	if len(kbIDs) == 0 {
		return ids, nil
	}
	err := m.db.WithContext(ctx).Model(&model.Document{}).
		Where("kb_id IN ?", kbIDs).
		Where("enabled = FALSE OR status <> ?", model.DocumentStatusCompleted).
		Pluck("id::text", &ids).Error
	return ids, err
}

func (m *models) deleteDocument(ctx context.Context, doc *model.Document) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("document_id = ?", doc.ID).Delete(&model.DocumentChunk{}).Error; err != nil {
			return err
		}
		return tx.Delete(doc).Error
	})
}

func (m *models) listDocumentIDs(ctx context.Context, kbID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := m.db.WithContext(ctx).Model(&model.Document{}).Where("kb_id = ?", kbID).Pluck("id", &ids).Error
	return ids, err
}

// replaceChunks 重索引时整体替换：硬删旧切片再批量插入，避免软删记录堆积。
func (m *models) replaceChunks(ctx context.Context, docID uuid.UUID, chunks []*model.DocumentChunk) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("document_id = ?", docID).Delete(&model.DocumentChunk{}).Error; err != nil {
			return err
		}
		if len(chunks) == 0 {
			return nil
		}
		return tx.CreateInBatches(chunks, 200).Error
	})
}

func (m *models) listChunks(ctx context.Context, docID uuid.UUID) ([]*model.DocumentChunk, error) {
	var chunks []*model.DocumentChunk
	err := m.db.WithContext(ctx).Where("document_id = ?", docID).Order("chunk_index ASC").Find(&chunks).Error
	return chunks, err
}
