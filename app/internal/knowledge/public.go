package knowledge

import (
	"context"
	"sync"

	"core/rag"
	"model/shared"

	"github.com/google/uuid"
	"github.com/mszlu521/thunder/errs"
	"github.com/mszlu521/thunder/event"
	"github.com/mszlu521/thunder/logs"
)

// processingSet 进程内的文档处理锁：同一文档同时只允许一个流水线，
// 防止连点「重索引」时两个 goroutine 交替写 ES 和 PG。单实例部署足够；多实例需换成分布式锁。
type processingSet struct {
	mu  sync.Mutex
	ids map[uuid.UUID]struct{}
}

var processing = &processingSet{ids: map[uuid.UUID]struct{}{}}

func (p *processingSet) acquire(id uuid.UUID) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.ids[id]; ok {
		return false
	}
	p.ids[id] = struct{}{}
	return true
}

func (p *processingSet) release(id uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.ids, id)
}

func (p *processingSet) has(id uuid.UUID) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.ids[id]
	return ok
}

// PublicService 给 agents 模块用的跨模块能力，走 event，避免 agents import knowledge 仓储。
type PublicService struct {
	svc *service
}

func NewPublicService() *PublicService {
	return &PublicService{svc: newService()}
}

// Search 事件 "searchKnowledgeBases"。
// 超时由这里控制（event 不带 ctx）：隐式检索是增强，不是前提，超时直接返回错误让调用方跳过。
func (p *PublicService) Search(e event.Event) (any, error) {
	params, ok := e.Data.(*shared.KnowledgeSearchParams)
	if !ok || params == nil {
		return nil, errs.ErrParam
	}
	if len(params.KnowledgeBaseIDs) == 0 {
		return &rag.RetrieveResult{}, nil
	}
	settings := p.svc.rt.settings
	ctx, cancel := context.WithTimeout(context.Background(), settings.RetrieveTimeout)
	defer cancel()
	kbs, err := p.svc.repo.getKnowledgeBasesInIDs(ctx, params.KnowledgeBaseIDs, params.UserID)
	if err != nil {
		logs.Errorf("load knowledge bases for search: %v", err)
		return nil, errs.DBError
	}
	hits, err := p.svc.searchKnowledgeBases(ctx, params.UserID, kbs, p.svc.searchOptions(params.Query, params.TopK))
	if err != nil {
		return nil, err
	}
	text, refs := rag.BuildContext(hits, settings.ContextMaxRunes)
	return &rag.RetrieveResult{Context: text, References: refs, Hits: hits}, nil
}

