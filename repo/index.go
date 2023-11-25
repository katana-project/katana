package repo

import (
	"encoding/json"
	"github.com/go-faster/errors"
	"github.com/katana-project/katana/repo/media"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type indexedRepository struct {
	Repository

	path       string
	oldPath    string
	pathParent string
	logger     *zap.Logger
	mu         sync.Mutex
}

type index struct {
	Items []*media.BasicMedia `json:"items"`
}

func NewIndexedRepository(repo Repository, path string, logger *zap.Logger) (Repository, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	dirPath := filepath.Dir(absPath)
	fileName := filepath.Base(absPath)
	ir := &indexedRepository{
		Repository: repo,
		path:       absPath,
		oldPath:    filepath.Join(dirPath, fileName+".old"),
		pathParent: dirPath,
		logger:     logger,
	}
	if err := ir.load(); err != nil {
		return ir, err
	}

	return ir, nil
}

func (ir *indexedRepository) Capabilities() Capability {
	return ir.Repository.Capabilities() | CapabilityIndex
}

func (ir *indexedRepository) load() error {
	if ir.logger != nil {
		loadTime := time.Now()
		defer func() {
			ir.logger.Info(
				"finished index load",
				zap.String("repo", ir.Repository.ID()),
				zap.String("repo_path", ir.Repository.Path()),
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

	repoPath := ir.Repository.Path()
	for _, item := range ix.Items {
		absItemPath := filepath.Join(repoPath, item.Path())

		// un-hack the Media contract for code reuse - you're not supposed to have relative paths in there
		absItem := media.NewBasicMedia(media.NewMedia(item.ID(), absItemPath, item.MIME(), item.Meta()))
		if err := ir.Repository.Add(absItem); err != nil {
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
				zap.String("repo", ir.Repository.ID()),
				zap.String("repo_path", ir.Repository.Path()),
				zap.String("path", ir.path),
				zap.Int64("elapsed_ms", time.Since(saveTime).Milliseconds()),
			)
		}()
	}

	var (
		path  = ir.Repository.Path()
		items = ir.Repository.Items()
		ix    = &index{Items: make([]*media.BasicMedia, len(items))}
	)
	for i, item := range items {
		relItemPath, err := filepath.Rel(path, item.Path())
		if err != nil {
			return err // shouldn't be possible
		}

		// hack the Media contract for code reuse - you're not supposed to have relative paths in there
		ix.Items[i] = media.NewBasicMedia(media.NewMedia(item.ID(), relItemPath, item.MIME(), item.Meta()))
	}

	bytes, err := json.Marshal(ix)
	if err != nil {
		return errors.Wrap(err, "failed to marshal index")
	}

	if err := os.MkdirAll(ir.pathParent, 0); err != nil {
		return errors.Wrap(err, "failed to make directories")
	}

	if err := ir.copyOld(); err != nil {
		return errors.Wrap(err, "failed to copy old index file")
	}

	if err := os.WriteFile(ir.path, bytes, 0); err != nil {
		return errors.Wrap(err, "failed to write index")
	}

	return nil
}

func (ir *indexedRepository) copyOld() error {
	if ir.logger != nil {
		copyTime := time.Now()
		defer func() {
			ir.logger.Info(
				"finished index copy",
				zap.String("repo", ir.Repository.ID()),
				zap.String("repo_path", ir.Repository.Path()),
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

// Scan tries to recursively discover missing media from the repository root directory.
func (ir *indexedRepository) Scan() error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	err := ir.Repository.Scan()
	if err != nil {
		return err
	}

	return ir.save()
}

// Add adds media to the repository.
func (ir *indexedRepository) Add(m media.Media) error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	err := ir.Repository.Add(m)
	if err != nil {
		return err
	}

	return ir.save()
}

// AddPath adds media at the supplied path to the repository.
func (ir *indexedRepository) AddPath(path string) error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	err := ir.Repository.AddPath(path)
	if err != nil {
		return err
	}

	return ir.save()
}

// Remove removes media from the repository.
func (ir *indexedRepository) Remove(m media.Media) error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	if err := ir.Repository.Remove(m); err != nil {
		return err
	}

	return ir.save()
}

// RemovePath removes media with the supplied path from the repository.
func (ir *indexedRepository) RemovePath(path string) error {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	if err := ir.Repository.RemovePath(path); err != nil {
		return err
	}

	return ir.save()
}
