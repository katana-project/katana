package repo

import (
	"encoding/json"
	"github.com/fsnotify/fsnotify"
	"github.com/gabriel-vasile/mimetype"
	"github.com/go-faster/errors"
	"github.com/katana-project/katana/repo/media"
	"github.com/katana-project/katana/repo/media/meta"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	allowedMimeGroups = []string{"video", "audio"}

	idPattern               = regexp.MustCompile("^[a-z0-9-_]+$")
	idCharExclusivePattern  = regexp.MustCompile("[^a-z0-9-_]")
	commonDelimiterReplacer = strings.NewReplacer(" ", "-", ".", "-")
)

// Capability is a collection of option flags (integers ORed together).
type Capability uint

const (
	// CapabilityWatch is a flag of a repository that is able to watch for filesystem changes.
	CapabilityWatch Capability = 1 << iota
	// CapabilityIndex is a flag of a repository that is able to persist its contents.
	CapabilityIndex
	// CapabilityRemux is a flag of a repository that is able to remux media.
	// A repository with this flag can be safely type asserted to be MuxingRepository.
	CapabilityRemux
	// CapabilityTranscode is a flag of a repository that is able to transcode media.
	// A repository with this flag can be safely type asserted to be MuxingRepository.
	CapabilityTranscode
)

// Has checks whether a Capability can be addressed from this one.
func (rc Capability) Has(flag Capability) bool {
	return (rc & flag) != 0
}

// Repository is a media repository.
type Repository interface {
	// ID returns the repository ID, alphanumeric, lowercase, non-blank ([a-z0-9-_]).
	ID() string
	// Name returns the repository name.
	Name() string
	// Path returns the path to the root directory of this repository, absolute.
	Path() string
	// Capabilities returns the capabilities of this repository.
	Capabilities() Capability
	// Scan tries to recursively discover missing media from the repository root directory.
	Scan() error
	// Get tries to get media by its ID in this repository, returns nil if not found.
	Get(id string) media.Media
	// Find tries to find media of an absolute or relative path in this repository, returns nil if not found.
	Find(path string) media.Media
	// Add adds media to the repository.
	Add(m media.Media) error
	// AddPath adds media at the supplied path to the repository.
	AddPath(path string) error
	// Remove removes media from the repository.
	Remove(m media.Media) error
	// RemovePath removes media with the supplied path from the repository.
	RemovePath(path string) error
	// Items returns the pieces of media in this repository.
	Items() []media.Media
	// Source returns the metadata source for this repository.
	Source() meta.Source
	// Close cleans up residual data after the repository.
	// The repository should not be used any further after calling Close.
	Close() error
}

// MuxingRepository is a repository capable of remuxing and transcoding operations.
type MuxingRepository interface {
	Repository
}

// ValidID checks whether the supplied string is a valid repository ID.
func ValidID(s string) bool {
	return idPattern.MatchString(s)
}

// SanitizeID sanitizes a string to be usable as a repository ID.
// Example: "My Shows" -> "my-shows"
func SanitizeID(s string) string {
	spaceLessLowerCase := strings.ToLower(commonDelimiterReplacer.Replace(s))

	return idCharExclusivePattern.ReplaceAllLiteralString(spaceLessLowerCase, "")
}

type crudRepository struct {
	id         string
	name       string
	path       string
	metaSource meta.Source
	logger     *zap.Logger
	mu         sync.RWMutex

	// these two should be kept in sync - use addItem and removeItem
	itemsById   map[string]media.Media
	itemsByPath map[string]media.Media
}

func NewRepository(id, name, path string, metaSource meta.Source, logger *zap.Logger) (Repository, error) {
	if !ValidID(id) {
		return nil, &ErrInvalidID{
			ID:       id,
			Expected: idPattern.String(),
		}
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(absPath, 0); err != nil {
		return nil, errors.Wrap(err, "failed to make directories")
	}

	return &crudRepository{
		id:          id,
		name:        name,
		path:        absPath,
		itemsById:   make(map[string]media.Media),
		itemsByPath: make(map[string]media.Media),
		logger:      logger,
		metaSource:  metaSource,
	}, nil
}

func (cr *crudRepository) ID() string {
	return cr.id
}

func (cr *crudRepository) Name() string {
	return cr.name
}

func (cr *crudRepository) Path() string {
	return cr.path
}

func (cr *crudRepository) Capabilities() Capability {
	return 0
}

func (cr *crudRepository) addItem(id, path string, m media.Media) {
	cr.itemsById[id] = m
	cr.itemsByPath[path] = m
}

func (cr *crudRepository) removeItem(id, path string) bool {
	desiredLen := len(cr.itemsById) - 1
	delete(cr.itemsById, id)
	delete(cr.itemsByPath, path)

	return len(cr.itemsById) == desiredLen
}

func (cr *crudRepository) checkMime(path, mime string) error {
	group := strings.SplitN(mime, "/", 2)[0]
	if !slices.Contains(allowedMimeGroups, group) {
		return &ErrInvalidMediaType{
			Path: path,
			Type: mime,
		}
	}

	return nil
}

func (cr *crudRepository) detectAndCheckMime(path string) (*mimetype.MIME, error) {
	t, err := mimetype.DetectFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to detect MIME type")
	}

	return t, cr.checkMime(path, t.String())
}

func (cr *crudRepository) Scan() error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if cr.logger != nil {
		scanTime := time.Now()
		defer func() {
			cr.logger.Info(
				"finished repository scan",
				zap.String("id", cr.id),
				zap.String("path", cr.path),
				zap.Int64("elapsed_ms", time.Since(scanTime).Milliseconds()),
			)
		}()
	}

	err := filepath.WalkDir(cr.path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if strings.HasPrefix(d.Name(), ".") { // dot-prefixed files/directories are excluded from handling
			if d.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		if !d.IsDir() {
			relPath, err := filepath.Rel(cr.path, path)
			if err != nil {
				return err // shouldn't be possible
			}

			if _, ok := cr.itemsByPath[relPath]; !ok {
				mime, err := cr.detectAndCheckMime(path)
				if err != nil {
					var eimt ErrInvalidMediaType
					if errors.Is(err, &eimt) { // invalid MIME type, skip
						if cr.logger != nil {
							cr.logger.Warn(
								"invalid MIME type, skipping",
								zap.String("repo", cr.id),
								zap.String("repo_path", cr.path),
								zap.String("path", relPath),
								zap.String("type", eimt.Type),
							)
						}

						return nil
					}

					return err // wrapped in checkMime already
				}

				m, err := cr.metaSource.FromFile(path)
				if err != nil {
					return errors.Wrap(err, "failed to discover metadata")
				}

				id := media.SanitizeID(d.Name())
				cr.addItem(id, relPath, media.NewMedia(id, path, mime.String(), m))
			}
		}

		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to walk repository files")
	}

	return nil
}

func (cr *crudRepository) Get(id string) media.Media {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	return cr.itemsById[id]
}

func (cr *crudRepository) Find(path string) media.Media {
	relPath := path
	if filepath.IsAbs(path) { // relativize path
		var err error
		if relPath, err = filepath.Rel(cr.path, path); err != nil {
			return nil // fast path: can't be made relative
		}
	}

	cr.mu.RLock()
	defer cr.mu.RUnlock()

	return cr.itemsByPath[relPath]
}

func (cr *crudRepository) add(id, path string, m media.Media) error {
	relPath, err := filepath.Rel(cr.path, path)
	if err != nil {
		return &ErrInvalidMediaPath{
			Path: path,
			Root: cr.path,
		}
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	if _, ok := cr.itemsById[id]; ok {
		return &ErrDuplicateID{
			ID:   id,
			Repo: cr.path,
		}
	}
	if _, ok := cr.itemsByPath[relPath]; ok {
		return &ErrDuplicatePath{
			Path: relPath,
			Repo: cr.path,
		}
	}

	cr.addItem(id, relPath, m)
	if cr.logger != nil {
		cr.logger.Info(
			"added media to repository",
			zap.String("repo", cr.id),
			zap.String("repo_path", cr.path),
			zap.String("id", id),
			zap.String("path", relPath),
		)
	}

	return nil
}

func (cr *crudRepository) Add(m media.Media) error {
	id := m.ID()
	if !media.ValidID(id) {
		return &ErrInvalidID{
			ID:       id,
			Expected: "^[a-z0-9-_]+$", // media.idPattern
		}
	}

	path := m.Path()
	if err := cr.checkMime(path, m.MIME()); err != nil {
		return errors.Wrap(err, "failed MIME type check")
	}

	return cr.add(id, path, m)
}

func (cr *crudRepository) AddPath(path string) error {
	mime, err := cr.detectAndCheckMime(path)
	if err != nil {
		return errors.Wrap(err, "failed MIME type check")
	}

	m, err := cr.metaSource.FromFile(path)
	if err != nil {
		return errors.Wrap(err, "failed to discover metadata")
	}

	id := media.SanitizeID(filepath.Base(path))
	return cr.add(id, path, media.NewMedia(id, path, mime.String(), m))
}

func (cr *crudRepository) Remove(m media.Media) error {
	id := m.ID()
	relPath, err := filepath.Rel(cr.path, m.Path())
	if err != nil {
		return nil // fast path: can't be made relative
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	ok := cr.removeItem(id, relPath)
	if ok && cr.logger != nil { // don't log anything if it were a no-op
		cr.logger.Info(
			"removed media from repository",
			zap.String("repo", cr.id),
			zap.String("repo_path", cr.path),
			zap.String("id", id),
			zap.String("path", relPath),
		)
	}

	return nil
}

func (cr *crudRepository) RemovePath(path string) error {
	relPath, err := filepath.Rel(cr.path, path)
	if err != nil {
		return nil // fast path: can't be made relative
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	m, ok := cr.itemsByPath[relPath]
	if !ok {
		return nil // fast path: path not in repository
	}

	cr.removeItem(m.ID(), relPath)
	if cr.logger != nil {
		cr.logger.Info(
			"removed media from repository",
			zap.String("repo", cr.id),
			zap.String("repo_path", cr.path),
			zap.String("path", relPath),
		)
	}

	return nil
}

func (cr *crudRepository) Items() []media.Media {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	return maps.Values(cr.itemsById)
}

func (cr *crudRepository) Source() meta.Source {
	return cr.metaSource
}

func (cr *crudRepository) Close() error {
	return nil
}

type watchedRepository struct {
	Repository

	logger  *zap.Logger
	watcher *fsnotify.Watcher
}

func NewWatchedRepository(repo Repository, logger *zap.Logger) (Repository, error) {
	if wr, ok := repo.(*watchedRepository); ok {
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

	wr := &watchedRepository{
		Repository: repo,
		logger:     logger,
		watcher:    watcher,
	}

	go wr.handleFsEvents()
	return wr, nil
}

func (wr *watchedRepository) Capabilities() Capability {
	return wr.Repository.Capabilities() | CapabilityWatch
}

func (wr *watchedRepository) handleFsEvents() {
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

func (wr *watchedRepository) handleFsEvent(event fsnotify.Event) error {
	// event.ID is always absolute, since path supplied to watcher is absolute

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

func (wr *watchedRepository) Close() error {
	if err := wr.watcher.Close(); err != nil {
		return err
	}

	return wr.Repository.Close()
}

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
