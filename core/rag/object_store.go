package rag

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ObjectStore 原始文件存储。重索引、下载原文都依赖它能「读回来」，
// 所以接口里必须有 Get——thunder/upload 的 OSS 封装只有 Upload/Delete/URL，不满足。
type ObjectStore interface {
	Put(ctx context.Context, key string, reader io.Reader) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// ErrObjectNotFound 原文丢失：重索引时要给出明确错误，而不是静默重切出 0 片。
var ErrObjectNotFound = errors.New("object not found")

// LocalStore 本地磁盘实现。key 形如 kb/<kbID>/<docID>.<ext>。
type LocalStore struct {
	root string
}

func NewLocalStore(root string) (*LocalStore, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("local store root is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create local store root: %w", err)
	}
	return &LocalStore{root: abs}, nil
}

// resolve 防目录穿越：key 来自服务端生成，但仍按不可信输入处理。
func (s *LocalStore) resolve(key string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(strings.TrimLeft(key, "/\\")))
	if cleaned == "." || strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("invalid object key")
	}
	full := filepath.Join(s.root, cleaned)
	if !strings.HasPrefix(full, s.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid object key")
	}
	return full, nil
}

func (s *LocalStore) Put(_ context.Context, key string, reader io.Reader) error {
	path, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// 先写临时文件再 rename：进程中途崩溃也不会留下半截原文。
	tmp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if errors.Is(err, os.ErrNotExist) {
		// 极少数情况下目录刚被并发的 Delete 清理掉，重建后再试一次。
		if err = os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
			tmp, err = os.CreateTemp(filepath.Dir(path), ".upload-*")
		}
	}
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, reader); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (s *LocalStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrObjectNotFound
	}
	return file, err
}

func (s *LocalStore) Delete(_ context.Context, key string) error {
	path, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	s.pruneEmptyDirs(filepath.Dir(path))
	return nil
}

// pruneEmptyDirs 自下而上删掉空目录（如删库后的 kb/<kbID>），最多到 root 为止。
// os.Remove 只删空目录，非空即停；并发上传时 Put 会 MkdirAll 重建，不会丢文件。
func (s *LocalStore) pruneEmptyDirs(dir string) {
	for strings.HasPrefix(dir, s.root+string(os.PathSeparator)) {
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// ObjectKey 统一原文 key 规则。
func ObjectKey(kbID, docID, fileType string) string {
	return fmt.Sprintf("kb/%s/%s.%s", kbID, docID, fileType)
}
