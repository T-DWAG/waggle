package knowledge

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"core/rag"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/mszlu521/thunder/config"
)

// settings 知识库运行参数，来自 config.yml 的 knowledge 段；缺省值与文档一致。
type settings struct {
	StorageDir         string
	MaxFileSize        int64
	ChunkSize          int
	ChunkOverlap       int
	EmbeddingBatchSize int
	TopK               int
	MinScore           float64
	VectorWeight       float64
	KeywordWeight      float64
	ContextMaxRunes    int
	RetrieveTimeout    time.Duration
}

func loadSettings() settings {
	s := settings{
		StorageDir:         config.GetString("knowledge.storageDir"),
		MaxFileSize:        int64(config.GetInt("knowledge.maxFileSizeMB")) << 20,
		ChunkSize:          config.GetInt("knowledge.chunkSize"),
		ChunkOverlap:       config.GetInt("knowledge.chunkOverlap"),
		EmbeddingBatchSize: config.GetInt("knowledge.embeddingBatchSize"),
		TopK:               config.GetInt("knowledge.topK"),
		MinScore:           config.GetFloat64("knowledge.minScore"),
		VectorWeight:       config.GetFloat64("knowledge.vectorWeight"),
		KeywordWeight:      config.GetFloat64("knowledge.keywordWeight"),
		ContextMaxRunes:    config.GetInt("knowledge.contextMaxRunes"),
		RetrieveTimeout:    time.Duration(config.GetInt("knowledge.retrieveTimeoutSec")) * time.Second,
	}
	if s.StorageDir == "" {
		s.StorageDir = "./data/knowledge"
	}
	if s.MaxFileSize <= 0 {
		s.MaxFileSize = 20 << 20
	}
	if s.ChunkSize <= 0 {
		s.ChunkSize = 800
	}
	if s.EmbeddingBatchSize <= 0 {
		s.EmbeddingBatchSize = 16
	}
	if s.TopK <= 0 {
		s.TopK = 5
	}
	if s.ContextMaxRunes <= 0 {
		s.ContextMaxRunes = 4000
	}
	if s.RetrieveTimeout <= 0 {
		s.RetrieveTimeout = 6 * time.Second
	}
	return s
}

// runtime 进程级共享资源：ES 客户端与原文存储。
// 懒加载且失败可重试——ES 启动晚于应用时，不应该让整个进程起不来，只让知识库接口返回 4003。
type runtime struct {
	mu       sync.Mutex
	once     sync.Once
	client   *elasticsearch.Client
	store    rag.ObjectStore
	settings settings
}

var rt = &runtime{}

// getRuntime 配置只读一次；必须在 config.SetViper 之后调用（handler 运行时自然满足）。
func getRuntime() *runtime {
	rt.once.Do(func() { rt.settings = loadSettings() })
	return rt
}

func (r *runtime) esClient(ctx context.Context) (*elasticsearch.Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client != nil {
		return r.client, nil
	}
	password := config.GetString("elasticsearch.password")
	if env := strings.TrimSpace(config.GetString("elasticsearch.passwordEnv")); env != "" {
		if value := os.Getenv(env); value != "" {
			password = value
		}
	}
	addresses := config.GetStringSlice("elasticsearch.addresses")
	if len(addresses) == 0 {
		addresses = []string{"http://localhost:9200"}
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	client, err := rag.NewClient(pingCtx, &rag.ESConfig{
		Addresses: addresses,
		Username:  config.GetString("elasticsearch.username"),
		Password:  password,
	})
	if err != nil {
		return nil, err
	}
	r.client = client
	return client, nil
}

func (r *runtime) objectStore() (rag.ObjectStore, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.store != nil {
		return r.store, nil
	}
	store, err := rag.NewLocalStore(r.settings.StorageDir)
	if err != nil {
		return nil, fmt.Errorf("init object store: %w", err)
	}
	r.store = store
	return store, nil
}
