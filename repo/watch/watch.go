package watch

import (
	"github.com/fsnotify/fsnotify"
	"github.com/katana-project/katana/internal/errors"
	"github.com/katana-project/katana/repo"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// watchRepo is a wrapping repo.MutableRepository with a repo.CapabilityWatch capability.
type watchRepo struct {
	repo.MutableRepository

	logger  *zap.Logger
	watcher *fsnotify.Watcher
}

// NewRepository creates a repository with a filesystem watcher.
func NewRepository(repo repo.MutableRepository, logger *zap.Logger) (repo.MutableRepository, error) {
	if wr, ok := repo.(*watchRepo); ok {
		return wr, nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.Wrap(err, "failed to make watcher")
	}

	err = filepath.WalkDir(repo.Path(), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir // dot-prefixed files/directories are excluded from handling
			}

			return watcher.Add(path) // path is always absolute
		}

		return nil
	})
	if err != nil {
		watcher.Close()
		return nil, errors.Wrap(err, "failed to walk repository files")
	}

	wr := &watchRepo{
		MutableRepository: repo,
		logger:            logger,
		watcher:           watcher,
	}

	go wr.handleFsEvents()
	return wr, nil
}

func (wr *watchRepo) Capabilities() repo.Capability {
	return wr.MutableRepository.Capabilities() | repo.CapabilityWatch
}

func (wr *watchRepo) handleFsEvents() {
	var (
		waitFor = 100 * time.Millisecond
		timers  = make(map[string]*time.Timer)
		mu      sync.Mutex
	)

	for {
		select {
		case err, ok := <-wr.watcher.Errors:
			if !ok {
				return
			}

			if wr.logger != nil {
				wr.logger.Error(
					"filesystem watch error",
					zap.String("id", wr.ID()),
					zap.String("path", wr.Path()),
					zap.Error(err),
				)
			}
		case e, ok := <-wr.watcher.Events:
			if !ok {
				return
			}

			if e.Has(fsnotify.Create) || e.Has(fsnotify.Write) { // event deduplication - run handler 100ms after last event, else reset timer
				mu.Lock()
				t, ok := timers[e.Name]
				mu.Unlock()

				if !ok {
					t = time.AfterFunc(math.MaxInt64, func() {
						if err := wr.handleFsEvent(e); err != nil && wr.logger != nil {
							wr.logger.Error(
								"filesystem event handler error",
								zap.String("id", wr.ID()),
								zap.String("path", wr.Path()),
								zap.Error(err),
							)
						}

						mu.Lock()
						delete(timers, e.Name)
						mu.Unlock()
					})
					t.Stop()

					mu.Lock()
					timers[e.Name] = t
					mu.Unlock()
				}

				t.Reset(waitFor)
			} else if e.Has(fsnotify.Remove) || e.Has(fsnotify.Rename) { // no deduplication
				if err := wr.handleFsEvent(e); err != nil && wr.logger != nil {
					wr.logger.Error(
						"filesystem event handler error",
						zap.String("id", wr.ID()),
						zap.String("path", wr.Path()),
						zap.Error(err),
					)
				}
			}
		}
	}
}

func (wr *watchRepo) handleFsEvent(event fsnotify.Event) error {
	// event.Name is always absolute, since path supplied to watcher is absolute

	fileName := filepath.Base(event.Name)
	if strings.HasPrefix(fileName, ".") {
		if wr.logger != nil {
			wr.logger.Info(
				"ignored filesystem event, excluded file name",
				zap.String("path", event.Name),
				zap.String("repo", wr.ID()),
			)
		}
		return nil // dot-prefixed files/directories are excluded from handling
	}

	if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
		fi, err := os.Stat(event.Name)
		if err != nil {
			return err
		}

		if fi.IsDir() {
			if wr.logger != nil {
				wr.logger.Info(
					"adding filesystem watcher to directory",
					zap.String("path", event.Name),
					zap.String("repo", wr.ID()),
				)
			}
			return wr.watcher.Add(event.Name)
		}

		if err := wr.AddPath(event.Name); err != nil {
			return err
		}
	} else if event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove) {
		if slices.Contains(wr.watcher.WatchList(), event.Name) {
			if wr.logger != nil {
				wr.logger.Info(
					"removing filesystem watcher from directory",
					zap.String("path", event.Name),
					zap.String("repo", wr.ID()),
				)
			}
			return wr.watcher.Remove(event.Name)
		}

		if err := wr.RemovePath(event.Name); err != nil {
			return err
		}
	}

	return nil
}

func (wr *watchRepo) Close() (err error) {
	return multierr.Combine(wr.watcher.Close(), wr.MutableRepository.Close())
}

func (wr *watchRepo) Mutable() repo.MutableRepository {
	return wr
}
