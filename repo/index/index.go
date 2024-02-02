package index

import (
	"encoding/json"
	"github.com/katana-project/katana/internal/errors"
	"github.com/katana-project/katana/repo"
	"github.com/katana-project/katana/repo/media"
	"go.uber.org/zap"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// indexedRepository is a wrapping repo.MutableRepository with a repo.CapabilityIndex capability.
type indexedRepository struct {
	repo.MutableRepository

	path       string
	oldPath    string
	parentPath string
	logger     *zap.Logger

	mu sync.Mutex
}

// index is a JSON-serializable media index.
type index struct {
	Items []*media.BasicMedia `json:"items"`
}

// NewRepository creates a file-based indexing repository.
func NewRepository(repo repo.MutableRepository, path string, logger *zap.Logger) (repo.MutableRepository, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	dirPath := filepath.Dir(absPath)
	fileName := filepath.Base(absPath)
	ir := &indexedRepository{
		MutableRepository: repo,
		path:              absPath,
		oldPath:           filepath.Join(dirPath, fileName+".old"),
		parentPath:        dirPath,
		logger:            logger,
	}
	if err := ir.load(); err != nil {
		return ir, err
	}

	return ir, nil
}

func (ir *indexedRepository) Capabilities() repo.Capability {
	return ir.MutableRepository.Capabilities() | repo.CapabilityIndex
}

func (ir *indexedRepository) load() error {
	if ir.logger != nil {
		loadTime := time.Now()
		defer func() {
			ir.logger.Info(
				"finished index load",
				zap.String("repo", ir.MutableRepository.ID()),
				zap.String("repo_path", ir.MutableRepository.Path()),
				zap.String("path", ir.path),
				zap.Int64("elapsed_ms", time.Since(loadTime).Milliseconds()),
			)
		}()
	}

	bytes, err := os.ReadFile(ir.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return errors.Wrap(err, "failed to read index")
	}

	var ix index
	if err := json.Unmarshal(bytes, &ix); err != nil {
		return errors.Wrap(err, "failed to unmarshal index")
	}

	repoPath := ir.MutableRepository.Path()
	for _, item := range ix.Items {
		absItemPath := filepath.Join(repoPath, item.Path())
		if _, err := os.Stat(absItemPath); errors.Is(err, fs.ErrNotExist) {
			ir.logger.Warn(
				"non-existent index item, skipping",
				zap.String("repo", ir.MutableRepository.ID()),
				zap.String("repo_path", ir.MutableRepository.Path()),
				zap.String("index_path", ir.path),
				zap.String("path", absItemPath),
			)
			continue
		}

		// un-hack the Media contract for code reuse - you're not supposed to have relative paths in there
		absItem := media.NewBasicMedia(media.NewMedia(item.ID(), absItemPath, item.Meta(), item.Format()))
		if err := ir.MutableRepository.Add(absItem); err != nil {
			return errors.Wrap(err, "failed to add index item to repository")
		}
	}

	return nil
}

func (ir *indexedRepository) save() error {
	if ir.logger != nil {
		saveTime := time.Now()
		defer func() {
			ir.logger.Info(
				"finished index save",
				zap.String("repo", ir.MutableRepository.ID()),
				zap.String("repo_path", ir.MutableRepository.Path()),
				zap.String("path", ir.path),
				zap.Int64("elapsed_ms", time.Since(saveTime).Milliseconds()),
			)
		}()
	}

	var (
		path  = ir.MutableRepository.Path()
		items = ir.MutableRepository.Items()
		ix    = &index{Items: make([]*media.BasicMedia, len(items))}
	)
	for i, item := range items {
		relItemPath, err := filepath.Rel(path, item.Path())
		if err != nil {
			return err // shouldn't be possible
		}

		// hack the Media contract for code reuse - you're not supposed to have relative paths in there
		ix.Items[i] = media.NewBasicMedia(media.NewMedia(item.ID(), relItemPath, item.Meta(), item.Format()))
	}

	bytes, err := json.Marshal(ix)
	if err != nil {
		return errors.Wrap(err, "failed to marshal index")
	}

	if err := os.MkdirAll(ir.parentPath, 0); err != nil {
		return errors.Wrap(err, "failed to make directories")
	}

	if err := ir.copy(); err != nil {
		return errors.Wrap(err, "failed to copy old index file")
	}

	if err := os.WriteFile(ir.path, bytes, 0); err != nil {
		return errors.Wrap(err, "failed to write index")
	}

	return nil
}

func (ir *indexedRepository) copy() error {
	if ir.logger != nil {
		copyTime := time.Now()
		defer func() {
			ir.logger.Info(
				"finished index copy",
				zap.String("repo", ir.MutableRepository.ID()),
				zap.String("repo_path", ir.MutableRepository.Path()),
				zap.String("path", ir.path),
				zap.Int64("elapsed_ms", time.Since(copyTime).Milliseconds()),
			)
		}()
	}

	bytes, err := os.ReadFile(ir.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	return os.WriteFile(ir.oldPath, bytes, 0)
}

func (ir *indexedRepository) Scan() error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	err := ir.MutableRepository.Scan()
	if err != nil {
		return err
	}

	return ir.save()
}

func (ir *indexedRepository) Add(m media.Media) error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	err := ir.MutableRepository.Add(m)
	if err != nil {
		return err
	}

	return ir.save()
}

func (ir *indexedRepository) AddPath(path string) error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	err := ir.MutableRepository.AddPath(path)
	if err != nil {
		return err
	}

	return ir.save()
}

func (ir *indexedRepository) Remove(m media.Media) error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	if err := ir.MutableRepository.Remove(m); err != nil {
		return err
	}

	return ir.save()
}

func (ir *indexedRepository) RemovePath(path string) error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	if err := ir.MutableRepository.RemovePath(path); err != nil {
		return err
	}

	return ir.save()
}
