package knowledge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"common/biz"
	"core/rag"
	"model"
	"model/shared"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/google/uuid"
	"github.com/mszlu521/thunder/errs"
	"github.com/mszlu521/thunder/event"
	"github.com/mszlu521/thunder/logs"
	"github.com/mszlu521/thunder/res"
)

const (
	databaseTimeout = 5 * time.Second
	// createTimeout 建库要走一次 embedding 探针 + 建 ES 索引，比普通 DB 操作慢。
	createTimeout = 30 * time.Second
	// processTimeout 单个文档的异步处理上限（解析 + 向量化 + 写 ES）。
	processTimeout = 10 * time.Minute
)

type service struct {
	repo repository
	rt   *runtime
}

func newService() *service {
	return &service{repo: newModels(), rt: getRuntime()}
}

// ---------- 知识库 ----------

func (s *service) createKnowledgeBase(parent context.Context, userID uuid.UUID, request *CreateKnowledgeBaseRequest) (*KnowledgeBaseResponse, error) {
	name := strings.TrimSpace(request.Name)
	provider := strings.TrimSpace(request.EmbeddingModelProvider)
	modelName := strings.TrimSpace(request.EmbeddingModelName)
	if name == "" || provider == "" || modelName == "" {
		return nil, errs.ErrParam
	}
	ctx, cancel := context.WithTimeout(parent, createTimeout)
	defer cancel()

	embedder, err := s.embedderFor(ctx, userID, provider, modelName)
	if err != nil {
		return nil, err
	}
	dims, err := rag.ProbeDimension(ctx, embedder)
	if err != nil {
		logs.Errorf("probe embedding dimension %s/%s: %v", provider, modelName, err)
		return nil, biz.ErrEmbeddingModelInvalid
	}
	client, err := s.rt.esClient(ctx)
	if err != nil {
		logs.Errorf("connect elasticsearch: %v", err)
		return nil, biz.ErrVectorStoreUnavailable
	}

	id := uuid.New()
	now := time.Now()
	kb := &model.KnowledgeBase{
		BaseModel:              model.BaseModel{ID: id, CreatedAt: now, UpdatedAt: now},
		CreatorID:              userID,
		Name:                   name,
		Description:            strings.TrimSpace(request.Description),
		EmbeddingModelProvider: provider,
		EmbeddingModelName:     modelName,
		EmbeddingDimension:     dims,
		StorageType:            model.StorageTypeElasticSearch,
		IndexName:              rag.IndexName(id.String()),
		Tags:                   cleanTags(request.Tags),
		Status:                 model.KnowledgeBaseStatusActive,
	}
	// 先建索引后落库：反过来会在 PG 里留下一个永远检索不到的库。
	if err := rag.EnsureIndex(ctx, client, kb.IndexName, dims); err != nil {
		logs.Errorf("create index %s: %v", kb.IndexName, err)
		return nil, biz.ErrVectorStoreUnavailable
	}
	if err := s.repo.createKnowledgeBase(ctx, kb); err != nil {
		logs.Errorf("create knowledge base: %v", err)
		if dropErr := rag.DropIndex(context.Background(), client, kb.IndexName); dropErr != nil {
			logs.Warnf("rollback index %s: %v", kb.IndexName, dropErr)
		}
		return nil, errs.DBError
	}
	return toKnowledgeBaseResponse(kb, 0), nil
}

func (s *service) listKnowledgeBases(parent context.Context, userID uuid.UUID, request *ListKnowledgeBasesRequest) (*res.Page, error) {
	page, pageSize := normalizePage(request.Page, request.PageSize)
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	items, total, err := s.repo.listKnowledgeBases(ctx, knowledgeFilter{
		CreatorID: userID, Name: strings.TrimSpace(request.Name),
		Limit: pageSize, Offset: (page - 1) * pageSize,
	})
	if err != nil {
		logs.Errorf("list knowledge bases: %v", err)
		return nil, errs.DBError
	}
	list := make([]*KnowledgeBaseResponse, 0, len(items))
	for _, item := range items {
		list = append(list, toKnowledgeBaseResponse(item, 0))
	}
	return &res.Page{List: list, Total: total, CurrentPage: int64(page), PageSize: int64(pageSize)}, nil
}

func (s *service) getKnowledgeBase(parent context.Context, userID, id uuid.UUID) (*KnowledgeBaseResponse, error) {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	kb, err := s.mustKnowledgeBase(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	size, err := s.repo.sumDocumentSize(ctx, id)
	if err != nil {
		logs.Errorf("sum document size: %v", err)
		return nil, errs.DBError
	}
	return toKnowledgeBaseResponse(kb, size), nil
}

func (s *service) updateKnowledgeBase(parent context.Context, userID, id uuid.UUID, request *UpdateKnowledgeBaseRequest) (*KnowledgeBaseResponse, error) {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	kb, err := s.mustKnowledgeBase(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if (request.EmbeddingModelProvider != nil && strings.TrimSpace(*request.EmbeddingModelProvider) != kb.EmbeddingModelProvider) ||
		(request.EmbeddingModelName != nil && strings.TrimSpace(*request.EmbeddingModelName) != kb.EmbeddingModelName) {
		return nil, biz.ErrEmbeddingModelImmutable
	}
	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if name == "" {
			return nil, errs.ErrParam
		}
		kb.Name = name
	}
	if request.Description != nil {
		kb.Description = strings.TrimSpace(*request.Description)
	}
	if request.Tags != nil {
		kb.Tags = cleanTags(*request.Tags)
	}
	if request.Status != nil {
		status := model.KnowledgeBaseStatus(*request.Status)
		if status != model.KnowledgeBaseStatusActive && status != model.KnowledgeBaseStatusDisabled {
			return nil, errs.ErrParam
		}
		kb.Status = status
	}
	kb.UpdatedAt = time.Now()
	if err := s.repo.updateKnowledgeBase(ctx, kb); err != nil {
		logs.Errorf("update knowledge base: %v", err)
		return nil, errs.DBError
	}
	return toKnowledgeBaseResponse(kb, 0), nil
}

func (s *service) deleteKnowledgeBase(parent context.Context, userID, id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	kb, err := s.mustKnowledgeBase(ctx, userID, id)
	if err != nil {
		return err
	}
	inUse, err := s.repo.isKnowledgeBaseInUse(ctx, id)
	if err != nil {
		logs.Errorf("check knowledge base usage: %v", err)
		return errs.DBError
	}
	if inUse {
		return biz.ErrKnowledgeBaseInUse
	}
	docIDs, err := s.repo.listDocumentIDs(ctx, id)
	if err != nil {
		logs.Errorf("list document ids: %v", err)
		return errs.DBError
	}
	if err := s.repo.deleteKnowledgeBase(ctx, id); err != nil {
		logs.Errorf("delete knowledge base: %v", err)
		return errs.DBError
	}
	// PG 是权威数据，已删成功；ES 与原文清理失败只告警，不回滚。
	if client, err := s.rt.esClient(ctx); err != nil {
		logs.Warnf("drop index %s skipped: %v", kb.IndexName, err)
	} else if err := rag.DropIndex(ctx, client, kb.IndexName); err != nil {
		logs.Warnf("drop index %s: %v", kb.IndexName, err)
	}
	if store, err := s.rt.objectStore(); err == nil {
		for _, docID := range docIDs {
			s.removeSources(ctx, store, id, docID)
		}
	}
	return nil
}

// ---------- 文档 ----------

func (s *service) uploadDocument(parent context.Context, userID, kbID uuid.UUID, header *multipart.FileHeader) (*DocumentResponse, error) {
	if header == nil {
		return nil, errs.ErrParam
	}
	name := filepath.Base(strings.TrimSpace(header.Filename))
	fileType := rag.NormalizeFileType(name)
	if name == "" || name == "." || !rag.IsSupportedFileType(fileType) {
		return nil, biz.ErrFileTypeNotSupported
	}
	if header.Size > s.rt.settings.MaxFileSize {
		return nil, biz.ErrFileTooLarge
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	kb, err := s.mustKnowledgeBase(ctx, userID, kbID)
	if err != nil {
		return nil, err
	}

	file, err := header.Open()
	if err != nil {
		return nil, errs.ErrParam
	}
	defer file.Close()
	// 多读 1 字节：客户端声明的 Size 不可信，以实际读到的为准。
	content, err := io.ReadAll(io.LimitReader(file, s.rt.settings.MaxFileSize+1))
	if err != nil {
		return nil, errs.ErrParam
	}
	if int64(len(content)) > s.rt.settings.MaxFileSize {
		return nil, biz.ErrFileTooLarge
	}
	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])

	existing, err := s.repo.findDocumentByHash(ctx, kbID, hash)
	if err != nil {
		logs.Errorf("find document by hash: %v", err)
		return nil, errs.DBError
	}
	if existing != nil {
		return nil, biz.ErrDocumentDuplicated
	}
	store, err := s.rt.objectStore()
	if err != nil {
		logs.Errorf("object store: %v", err)
		return nil, biz.ErrVectorStoreUnavailable
	}

	now := time.Now()
	doc := &model.Document{
		BaseModel:       model.BaseModel{ID: uuid.New(), CreatedAt: now, UpdatedAt: now},
		KnowledgeBaseID: kbID,
		CreatorID:       userID,
		Name:            name,
		FileType:        fileType,
		Size:            int64(len(content)),
		FileHash:        hash,
		Status:          model.DocumentStatusPending,
		Enabled:         true,
	}
	doc.StorageKey = rag.ObjectKey(kbID.String(), doc.ID.String(), fileType)
	// 先存原文再落库：原文写失败就不会出现「有记录但无法重索引」的文档。
	if err := store.Put(ctx, doc.StorageKey, bytes.NewReader(content)); err != nil {
		logs.Errorf("store document source: %v", err)
		return nil, biz.ErrVectorStoreUnavailable
	}
	if err := s.repo.createDocument(ctx, doc); err != nil {
		logs.Errorf("create document: %v", err)
		_ = store.Delete(context.Background(), doc.StorageKey)
		return nil, errs.DBError
	}
	_ = s.repo.refreshCounts(ctx, kbID)
	s.processAsync(kb, doc.ID)
	return toDocumentResponse(doc, 0), nil
}

func (s *service) listDocuments(parent context.Context, userID, kbID uuid.UUID, request *ListDocumentsRequest) (*res.Page, error) {
	page, pageSize := normalizePage(request.Page, request.PageSize)
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	if _, err := s.mustKnowledgeBase(ctx, userID, kbID); err != nil {
		return nil, err
	}
	items, total, err := s.repo.listDocuments(ctx, documentFilter{
		KnowledgeBaseID: kbID, Name: strings.TrimSpace(request.Name), Status: strings.TrimSpace(request.Status),
		Limit: pageSize, Offset: (page - 1) * pageSize,
	})
	if err != nil {
		logs.Errorf("list documents: %v", err)
		return nil, errs.DBError
	}
	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	counts, err := s.repo.countChunksByDocuments(ctx, ids)
	if err != nil {
		logs.Errorf("count chunks: %v", err)
		return nil, errs.DBError
	}
	list := make([]*DocumentResponse, 0, len(items))
	for _, item := range items {
		list = append(list, toDocumentResponse(item, counts[item.ID]))
	}
	return &res.Page{List: list, Total: total, CurrentPage: int64(page), PageSize: int64(pageSize)}, nil
}

// updateDocument 启停文档。停用不删 ES 切片，检索时按 doc_id 排除，恢复无需重新向量化。
func (s *service) updateDocument(parent context.Context, userID, kbID, docID uuid.UUID, request *UpdateDocumentRequest) (*DocumentResponse, error) {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	doc, err := s.mustDocument(ctx, userID, kbID, docID)
	if err != nil {
		return nil, err
	}
	if request.Enabled != nil {
		if err := s.repo.updateDocumentFields(ctx, doc.ID, map[string]any{"enabled": *request.Enabled, "updated_at": time.Now()}); err != nil {
			logs.Errorf("update document: %v", err)
			return nil, errs.DBError
		}
		doc.Enabled = *request.Enabled
	}
	return toDocumentResponse(doc, 0), nil
}

func (s *service) deleteDocument(parent context.Context, userID, kbID, docID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	doc, err := s.mustDocument(ctx, userID, kbID, docID)
	if err != nil {
		return err
	}
	if processing.has(docID) {
		return biz.ErrDocumentProcessing
	}
	if err := s.repo.deleteDocument(ctx, doc); err != nil {
		logs.Errorf("delete document: %v", err)
		return errs.DBError
	}
	_ = s.repo.refreshCounts(ctx, kbID)
	kb, _ := s.repo.getKnowledgeBase(ctx, kbID, userID)
	if kb != nil {
		if client, err := s.rt.esClient(ctx); err != nil {
			logs.Warnf("delete es chunks of %s skipped: %v", docID, err)
		} else if err := rag.DeleteByDocument(ctx, client, kb.IndexName, docID.String()); err != nil {
			logs.Warnf("delete es chunks of %s: %v", docID, err)
		}
	}
	if store, err := s.rt.objectStore(); err == nil {
		_ = store.Delete(ctx, doc.StorageKey)
	}
	return nil
}

// reindexDocument 从原文重新解析切片：切片参数改了也能真正生效。
func (s *service) reindexDocument(parent context.Context, userID, kbID, docID uuid.UUID) (*DocumentResponse, error) {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	kb, err := s.mustKnowledgeBase(ctx, userID, kbID)
	if err != nil {
		return nil, err
	}
	doc, err := s.mustDocument(ctx, userID, kbID, docID)
	if err != nil {
		return nil, err
	}
	if processing.has(docID) {
		return nil, biz.ErrDocumentProcessing
	}
	if doc.StorageKey == "" {
		return nil, biz.ErrDocumentSourceMissing
	}
	if err := s.repo.updateDocumentFields(ctx, doc.ID, map[string]any{"status": model.DocumentStatusPending, "error_message": "", "updated_at": time.Now()}); err != nil {
		logs.Errorf("mark document pending: %v", err)
		return nil, errs.DBError
	}
	doc.Status = model.DocumentStatusPending
	doc.ErrorMessage = ""
	s.processAsync(kb, doc.ID)
	return toDocumentResponse(doc, 0), nil
}

func (s *service) listChunks(parent context.Context, userID, kbID, docID uuid.UUID) ([]*ChunkResponse, error) {
	ctx, cancel := context.WithTimeout(parent, databaseTimeout)
	defer cancel()
	if _, err := s.mustDocument(ctx, userID, kbID, docID); err != nil {
		return nil, err
	}
	chunks, err := s.repo.listChunks(ctx, docID)
	if err != nil {
		logs.Errorf("list chunks: %v", err)
		return nil, errs.DBError
	}
	list := make([]*ChunkResponse, 0, len(chunks))
	for _, chunk := range chunks {
		list = append(list, &ChunkResponse{
			ID: chunk.ID, ChunkIndex: chunk.ChunkIndex, Content: chunk.Content, TokenCount: chunk.TokenCount,
			Section: rag.HeadingPath(chunk.MetaInfo), MetaInfo: chunk.MetaInfo, Status: string(chunk.Status),
		})
	}
	return list, nil
}

// ---------- 异步处理 ----------

// processAsync 用 context.Background()：请求返回后请求 ctx 即被取消，沿用它会让向量化中途失败。
func (s *service) processAsync(kb *model.KnowledgeBase, docID uuid.UUID) {
	if !processing.acquire(docID) {
		return
	}
	kbCopy := *kb
	go func() {
		defer processing.release(docID)
		ctx, cancel := context.WithTimeout(context.Background(), processTimeout)
		defer cancel()
		defer func() {
			if recovered := recover(); recovered != nil {
				logs.Errorf("panic while processing document %s: %v", docID, recovered)
				s.markFailed(docID, "internal error")
			}
		}()
		if err := s.processDocument(ctx, &kbCopy, docID); err != nil {
			logs.Errorf("process document %s: %v", docID, err)
			s.markFailed(docID, userFacingError(err))
		}
		_ = s.repo.refreshCounts(context.Background(), kbCopy.ID)
	}()
}

func (s *service) processDocument(ctx context.Context, kb *model.KnowledgeBase, docID uuid.UUID) error {
	doc, err := s.repo.getDocument(ctx, kb.ID, docID)
	if err != nil || doc == nil {
		return fmt.Errorf("load document: %w", err)
	}
	if err := s.repo.updateDocumentFields(ctx, docID, map[string]any{"status": model.DocumentStatusProcessing, "error_message": "", "updated_at": time.Now()}); err != nil {
		return err
	}
	store, err := s.rt.objectStore()
	if err != nil {
		return err
	}
	reader, err := store.Get(ctx, doc.StorageKey)
	if err != nil {
		return stepError{"读取原文", err}
	}
	content, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		return stepError{"读取原文", err}
	}
	parsed, err := rag.Parse(ctx, doc.FileType, content)
	if err != nil {
		return stepError{"解析文件", err}
	}
	title := strings.TrimSuffix(doc.Name, filepath.Ext(doc.Name))
	settings := s.rt.settings
	chunks := rag.Split(parsed, rag.SplitOptions{MaxRunes: settings.ChunkSize, Overlap: settings.ChunkOverlap, Title: title})
	if len(chunks) == 0 {
		return stepError{"切片", errors.New("文档没有可索引的文本")}
	}

	embedder, err := s.embedderFor(ctx, kb.CreatorID, kb.EmbeddingModelProvider, kb.EmbeddingModelName)
	if err != nil {
		return stepError{"加载向量模型", err}
	}
	client, err := s.rt.esClient(ctx)
	if err != nil {
		return stepError{"连接向量存储", err}
	}
	if err := rag.EnsureIndex(ctx, client, kb.IndexName, kb.EmbeddingDimension); err != nil {
		return stepError{"校验索引", err}
	}
	// 重索引或上次失败残留：先清掉该文档在 ES 里的旧切片。
	if err := rag.DeleteByDocument(ctx, client, kb.IndexName, docID.String()); err != nil {
		return stepError{"清理旧切片", err}
	}

	now := time.Now()
	records := make([]*model.DocumentChunk, 0, len(chunks))
	storeChunks := make([]rag.StoreChunk, 0, len(chunks))
	totalTokens := 0
	for i, chunk := range chunks {
		id := uuid.New()
		tokens := estimateTokens(chunk.Raw)
		totalTokens += tokens
		records = append(records, &model.DocumentChunk{
			BaseModel:       model.BaseModel{ID: id, CreatedAt: now, UpdatedAt: now},
			DocumentID:      docID,
			KnowledgeBaseID: kb.ID,
			ElasticSearchID: id.String(),
			ChunkIndex:      i,
			Content:         chunk.Raw,
			TokenCount:      tokens,
			MetaInfo:        model.JSON(chunk.Meta),
			Status:          model.ChunkStatusEmbedded,
		})
		storeChunks = append(storeChunks, rag.StoreChunk{ID: id.String(), Content: chunk.Content, Position: i, Meta: chunk.Meta})
	}
	if err := rag.StoreDocument(ctx, client, embedder, &rag.StoreRequest{
		Index: kb.IndexName, KnowledgeID: kb.ID.String(), DocumentID: docID.String(),
		DocumentName: doc.Name, FileType: doc.FileType, Chunks: storeChunks, BatchSize: settings.EmbeddingBatchSize,
	}); err != nil {
		_ = rag.DeleteByDocument(context.Background(), client, kb.IndexName, docID.String())
		return stepError{"向量化写入", err}
	}
	if err := s.repo.replaceChunks(ctx, docID, records); err != nil {
		_ = rag.DeleteByDocument(context.Background(), client, kb.IndexName, docID.String())
		return stepError{"保存切片", err}
	}
	return s.repo.updateDocumentFields(ctx, docID, map[string]any{
		"status": model.DocumentStatusCompleted, "token_count": totalTokens, "error_message": "",
		"meta_info": model.JSON{"chunkSize": settings.ChunkSize, "chunkOverlap": settings.ChunkOverlap, "chunks": len(records)},
		"updated_at": time.Now(),
	})
}

func (s *service) markFailed(docID uuid.UUID, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), databaseTimeout)
	defer cancel()
	if err := s.repo.updateDocumentFields(ctx, docID, map[string]any{
		"status": model.DocumentStatusFailed, "error_message": message, "updated_at": time.Now(),
	}); err != nil {
		logs.Errorf("mark document %s failed: %v", docID, err)
	}
}

// ---------- 检索 ----------

func (s *service) search(parent context.Context, userID, kbID uuid.UUID, request *SearchRequest) (*SearchResponse, error) {
	query := strings.TrimSpace(request.Query)
	if query == "" {
		return nil, errs.ErrParam
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	kb, err := s.mustKnowledgeBase(ctx, userID, kbID)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	options := s.searchOptions(query, request.TopK)
	// 召回箱默认不过滤：调参时要看到低分结果才知道阈值该设多少。
	options.MinScore = 0
	if request.MinScore != nil {
		options.MinScore = *request.MinScore
	}
	for _, id := range request.DocumentIDs {
		options.OnlyDocIDs = append(options.OnlyDocIDs, id.String())
	}
	hits, err := s.searchKnowledgeBases(ctx, userID, []*model.KnowledgeBase{kb}, options)
	if err != nil {
		return nil, err
	}
	results := make([]SearchResult, 0, len(hits))
	for _, hit := range hits {
		results = append(results, SearchResult{
			ChunkID: hit.ID, DocumentID: hit.DocumentID, DocumentName: hit.DocumentName, FileType: hit.FileType,
			Position: hit.Position, Section: rag.HeadingPath(hit.Meta), Content: hit.Content,
			Score: hit.Score, VectorScore: hit.VectorScore, KeywordScore: hit.KeywordScore, Metadata: hit.Meta,
		})
	}
	return &SearchResponse{
		Query: query, KnowledgeBaseID: kbID, Mode: "hybrid", Took: time.Since(started).Milliseconds(),
		Total: len(results), Results: results,
	}, nil
}

func (s *service) searchOptions(query string, topK int) rag.SearchOptions {
	settings := s.rt.settings
	if topK <= 0 {
		topK = settings.TopK
	}
	return rag.SearchOptions{
		Query: query, TopK: topK, MinScore: settings.MinScore,
		VectorWeight: settings.VectorWeight, KeywordWeight: settings.KeywordWeight,
	}
}

// searchKnowledgeBases 跨库检索。向量只能在同一个模型空间里比较，
// 所以按 embedding 模型分组：同模型的库一次多索引查询，不同模型分别查后按融合分合并。
func (s *service) searchKnowledgeBases(ctx context.Context, userID uuid.UUID, kbs []*model.KnowledgeBase, options rag.SearchOptions) ([]rag.Hit, error) {
	client, err := s.rt.esClient(ctx)
	if err != nil {
		logs.Errorf("connect elasticsearch: %v", err)
		return nil, biz.ErrVectorStoreUnavailable
	}
	groups := map[string][]*model.KnowledgeBase{}
	order := []string{}
	kbIDs := make([]uuid.UUID, 0, len(kbs))
	for _, kb := range kbs {
		if kb == nil || kb.Status != model.KnowledgeBaseStatusActive {
			continue
		}
		key := kb.EmbeddingModelProvider + "\x00" + kb.EmbeddingModelName
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], kb)
		kbIDs = append(kbIDs, kb.ID)
	}
	if len(kbIDs) == 0 {
		return []rag.Hit{}, nil
	}
	excluded, err := s.repo.disabledDocumentIDs(ctx, kbIDs)
	if err != nil {
		logs.Errorf("load disabled documents: %v", err)
		return nil, errs.DBError
	}
	options.ExcludeDocIDs = excluded

	var all []rag.Hit
	for _, key := range order {
		group := groups[key]
		embedder, err := s.embedderFor(ctx, userID, group[0].EmbeddingModelProvider, group[0].EmbeddingModelName)
		if err != nil {
			return nil, err
		}
		groupOptions := options
		groupOptions.Indexes = nil
		for _, kb := range group {
			groupOptions.Indexes = append(groupOptions.Indexes, kb.IndexName)
		}
		hits, err := rag.Search(ctx, client, embedder, groupOptions)
		if err != nil {
			logs.Errorf("search knowledge bases: %v", err)
			return nil, biz.ErrVectorStoreUnavailable
		}
		all = append(all, hits...)
	}
	sortHits(all)
	topK := options.TopK
	if topK <= 0 {
		topK = s.rt.settings.TopK
	}
	if len(all) > topK {
		all = all[:topK]
	}
	return all, nil
}

// embedderFor 通过事件拿厂商配置：knowledge 不直接依赖 agents 的仓储，与聊天模型同一条路径。
func (s *service) embedderFor(ctx context.Context, userID uuid.UUID, provider, modelName string) (embedding.Embedder, error) {
	result, err := event.Trigger("getProviderConfigByProvider", &shared.LLMParams{
		UserID: userID, ModelType: model.LLMTypeEmbedding, Provider: provider, Model: modelName,
	})
	if err != nil {
		logs.Errorf("get embedding provider config: %v", err)
		return nil, errs.DBError
	}
	response, ok := result.(*shared.ModelProviderResponse)
	if !ok || response.ProvideConfig == nil || response.ProvideConfig.Status != model.LLMStatusActive {
		return nil, biz.ErrEmbeddingModelInvalid
	}
	config := response.ProvideConfig
	embedder, err := rag.NewEmbedder(ctx, &rag.EmbeddingConfig{
		Provider: config.Provider, Model: modelName, APIBase: config.APIBase, APIKey: config.APIKey, TimeoutSec: 60,
	})
	if err != nil {
		logs.Errorf("build embedder %s/%s: %v", provider, modelName, err)
		return nil, biz.ErrEmbeddingModelInvalid
	}
	return embedder, nil
}

// ---------- 工具函数 ----------

func (s *service) mustKnowledgeBase(ctx context.Context, userID, id uuid.UUID) (*model.KnowledgeBase, error) {
	kb, err := s.repo.getKnowledgeBase(ctx, id, userID)
	if err != nil {
		logs.Errorf("get knowledge base: %v", err)
		return nil, errs.DBError
	}
	if kb == nil {
		return nil, biz.ErrKnowledgeBaseNotFound
	}
	return kb, nil
}

// mustDocument 先校验库归属再查文档：文档表的 creator_id 只是冗余，隔离以库为准。
func (s *service) mustDocument(ctx context.Context, userID, kbID, docID uuid.UUID) (*model.Document, error) {
	if _, err := s.mustKnowledgeBase(ctx, userID, kbID); err != nil {
		return nil, err
	}
	doc, err := s.repo.getDocument(ctx, kbID, docID)
	if err != nil {
		logs.Errorf("get document: %v", err)
		return nil, errs.DBError
	}
	if doc == nil {
		return nil, biz.ErrDocumentNotFound
	}
	return doc, nil
}

func (s *service) removeSources(ctx context.Context, store rag.ObjectStore, kbID, docID uuid.UUID) {
	for _, fileType := range rag.SupportedFileTypes {
		_ = store.Delete(ctx, rag.ObjectKey(kbID.String(), docID.String(), fileType))
	}
}

// stepError 带上失败步骤，前端能看到「卡在哪一步」；底层错误细节只进日志。
type stepError struct {
	step string
	err  error
}

func (e stepError) Error() string { return e.step + ": " + e.err.Error() }
func (e stepError) Unwrap() error { return e.err }

func userFacingError(err error) string {
	var step stepError
	if errors.As(err, &step) {
		var bizErr *errs.Errors
		if errors.As(step.err, &bizErr) {
			return step.step + "失败: " + bizErr.Msg
		}
		if step.step == "切片" || step.step == "解析文件" {
			return step.step + "失败: " + step.err.Error()
		}
		return step.step + "失败，详情见服务端日志"
	}
	return "处理失败，详情见服务端日志"
}

// estimateTokens 粗估：中文按 1 字 1 token，其余按 4 字符 1 token。只用于展示和容量预估。
func estimateTokens(text string) int {
	cjk, other := 0, 0
	for _, r := range text {
		if r >= 0x2E80 {
			cjk++
		} else {
			other++
		}
	}
	return cjk + (other+3)/4
}

func sortHits(hits []rag.Hit) {
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].Score > hits[j-1].Score; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
}

func cleanTags(tags []string) model.StringArrayJSON {
	result := make(model.StringArrayJSON, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		result = append(result, tag)
	}
	return result
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func toKnowledgeBaseResponse(kb *model.KnowledgeBase, totalSize int64) *KnowledgeBaseResponse {
	tags := []string(kb.Tags)
	if tags == nil {
		tags = []string{}
	}
	return &KnowledgeBaseResponse{
		ID: kb.ID, Name: kb.Name, Description: kb.Description,
		EmbeddingModelProvider: kb.EmbeddingModelProvider, EmbeddingModelName: kb.EmbeddingModelName,
		EmbeddingDimension: kb.EmbeddingDimension, StorageType: string(kb.StorageType), IndexName: kb.IndexName,
		DocumentCount: kb.DocumentCount, ChunkCount: kb.ChunkCount, TotalSize: totalSize, Tags: tags,
		Status: string(kb.Status), CreatedAt: kb.CreatedAt, UpdatedAt: kb.UpdatedAt,
	}
}

func toDocumentResponse(doc *model.Document, chunkCount int64) *DocumentResponse {
	return &DocumentResponse{
		ID: doc.ID, KnowledgeBaseID: doc.KnowledgeBaseID, Name: doc.Name, FileType: doc.FileType,
		Size: doc.Size, TokenCount: doc.TokenCount, ChunkCount: chunkCount, Status: string(doc.Status),
		ErrorMessage: doc.ErrorMessage, Enabled: doc.Enabled, CreatedAt: doc.CreatedAt, UpdatedAt: doc.UpdatedAt,
	}
}
