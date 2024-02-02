package repo

import (
	"github.com/gabriel-vasile/mimetype"
	"github.com/katana-project/katana/config"
	"github.com/katana-project/katana/internal/errors"
	"github.com/katana-project/katana/repo/media"
	"github.com/katana-project/katana/repo/media/meta"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"io/fs"
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
	CapabilityRemux
	// CapabilityTranscode is a flag of a repository that is able to transcode media.
	CapabilityTranscode
)

// Capabilities translates capabilities from the configuration.
func Capabilities(caps []config.Capability) Capability {
	var c Capability
	for _, capability := range caps {
		switch capability {
		case config.CapabilityWatch:
			c |= CapabilityWatch
		case config.CapabilityRemux:
			c |= CapabilityRemux
		case config.CapabilityTranscode:
			c |= CapabilityTranscode
		}
	}

	return c
}

// Has checks whether a Capability can be addressed from this one.
func (c Capability) Has(flag Capability) bool {
	return (c & flag) != 0
}

// Repository is an immutable media repository or an immutable view of one.
type Repository interface {
	// ID returns the repository ID, alphanumeric, lowercase, non-blank ([a-z0-9-_]).
	ID() string
	// Name returns the repository name.
	Name() string
	// Path returns the path to the root directory of this repository, absolute.
	Path() string
	// Capabilities returns the capabilities of this repository.
	Capabilities() Capability

	// Get tries to get media by its ID in this repository, returns nil if not found.
	Get(id string) media.Media
	// Find tries to find media of an absolute or relative path in this repository, returns nil if not found.
	Find(path string) media.Media
	// Items returns the pieces of media in this repository.
	Items() []media.Media

	// Remux remuxes media to the desired container format and returns the remuxed media or nil, if the ID wasn't found.
	// ErrUnsupportedOperation may be returned if the repository does not have the CapabilityRemux capability.
	Remux(id string, format *media.Format) (media.Media, error)

	// Source returns the metadata source for this repository.
	Source() meta.Source

	// Close cleans up residual data after the repository.
	// The repository should not be used any further after calling Close.
	Close() error

	// Mutable tries to assert this repository view to a MutableRepository, returns nil if not possible.
	Mutable() MutableRepository
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

// MutableRepository is a mutable media repository.
type MutableRepository interface {
	Repository

	// Scan tries to recursively discover missing media from the repository root directory.
	Scan() error
	// Add adds media to the repository.
	Add(m media.Media) error
	// AddPath adds media at the supplied path to the repository.
	AddPath(path string) error
	// Remove removes media from the repository.
	Remove(m media.Media) error
	// RemovePath removes media with the supplied absolute path from the repository.
	RemovePath(path string) error
}

// NopMutable wraps a Repository and no-ops unimplemented mutation functions.
func NopMutable(r Repository) MutableRepository {
	if nmr, ok := r.(*nopMutableRepo); ok {
		return nmr // no need to wrap again
	}

	return &nopMutableRepo{Repository: r}
}

// mutableRepo is a Repository wrapper that no-ops all mutating calls.
type nopMutableRepo struct {
	Repository
}

func (nmr *nopMutableRepo) Scan() error {
	return errors.ErrUnsupported
}
func (nmr *nopMutableRepo) Add(_ media.Media) error {
	return errors.ErrUnsupported
}
func (nmr *nopMutableRepo) AddPath(_ string) error {
	return errors.ErrUnsupported
}
func (nmr *nopMutableRepo) Remove(_ media.Media) error {
	return errors.ErrUnsupported
}
func (nmr *nopMutableRepo) RemovePath(_ string) error {
	return errors.ErrUnsupported
}
func (nmr *nopMutableRepo) Mutable() MutableRepository {
	return nmr
}

// mutableRepo is an implementation of a MutableRepository.
type mutableRepo struct {
	id         string
	name       string
	path       string
	metaSource meta.Source
	logger     *zap.Logger

	mu sync.RWMutex

	// these two should be kept in sync - use addItem and removeItem
	itemsById   map[string]media.Media
	itemsByPath map[string]media.Media
}

// NewRepository creates a file-based CRUD repository.
func NewRepository(id, name, path string, metaSource meta.Source, logger *zap.Logger) (MutableRepository, error) {
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

	return &mutableRepo{
		id:          id,
		name:        name,
		path:        absPath,
		itemsById:   make(map[string]media.Media),
		itemsByPath: make(map[string]media.Media),
		logger:      logger,
		metaSource:  metaSource,
	}, nil
}

func (mr *mutableRepo) ID() string {
	return mr.id
}

func (mr *mutableRepo) Name() string {
	return mr.name
}

func (mr *mutableRepo) Path() string {
	return mr.path
}

func (mr *mutableRepo) Capabilities() Capability {
	return 0
}

func (mr *mutableRepo) addItem(id, path string, m media.Media) {
	mr.itemsById[id] = m
	mr.itemsByPath[path] = m
}

func (mr *mutableRepo) removeItem(id, path string) bool {
	length := len(mr.itemsById) - 1
	delete(mr.itemsById, id)
	delete(mr.itemsByPath, path)

	return len(mr.itemsById) == length
}

func (mr *mutableRepo) checkFormat(path string, format *media.Format) error {
	group := strings.SplitN(format.MIME, "/", 2)[0]
	if !slices.Contains(allowedMimeGroups, group) {
		return &ErrInvalidMediaType{
			Path: path,
			Type: format.MIME,
		}
	}

	return nil
}

func (mr *mutableRepo) detectAndCheckFormat(path string) (*media.Format, error) {
	t, err := mimetype.DetectFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to detect MIME type")
	}

	format := media.FindUnsupportedFormat(t.String(), filepath.Ext(path))
	return format, mr.checkFormat(path, format)
}

func (mr *mutableRepo) Scan() error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if mr.logger != nil {
		scanTime := time.Now()
		defer func() {
			mr.logger.Info(
				"finished repository scan",
				zap.String("id", mr.id),
				zap.String("path", mr.path),
				zap.Int64("elapsed_ms", time.Since(scanTime).Milliseconds()),
			)
		}()
	}

	err := filepath.WalkDir(mr.path, func(path string, d fs.DirEntry, err error) error {
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
			relPath, err := filepath.Rel(mr.path, path)
			if err != nil {
				return err // shouldn't be possible
			}

			if _, ok := mr.itemsByPath[relPath]; !ok {
				format, err := mr.detectAndCheckFormat(path)
				if err != nil {
					var eimt ErrInvalidMediaType
					if errors.Is(err, &eimt) { // invalid MIME type, skip
						if mr.logger != nil {
							mr.logger.Warn(
								"invalid MIME type, skipping",
								zap.String("repo", mr.id),
								zap.String("repo_path", mr.path),
								zap.String("path", relPath),
								zap.String("type", eimt.Type),
							)
						}

						return nil
					}

					return err // wrapped in checkFormat already
				}

				m, err := mr.metaSource.FromFile(path)
				if err != nil {
					return errors.Wrap(err, "failed to discover metadata")
				}

				id := media.SanitizeID(d.Name())
				mr.addItem(id, relPath, media.NewMedia(id, path, m, format))
			}
		}

		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to walk repository files")
	}

	return nil
}

func (mr *mutableRepo) Get(id string) media.Media {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	return mr.itemsById[id]
}

func (mr *mutableRepo) Find(path string) media.Media {
	relPath := path
	if filepath.IsAbs(path) { // relativize path
		var err error
		if relPath, err = filepath.Rel(mr.path, path); err != nil {
			return nil // fast path: can't be made relative
		}
	}

	mr.mu.RLock()
	defer mr.mu.RUnlock()

	return mr.itemsByPath[relPath]
}

func (mr *mutableRepo) add(id, path string, m media.Media) error {
	if _, err := os.Stat(path); err != nil { // catches non-existent files
		return errors.Wrap(err, "failed to stat file")
	}

	relPath, err := filepath.Rel(mr.path, path)
	if err != nil {
		return &ErrInvalidMediaPath{
			Path: path,
			Root: mr.path,
		}
	}

	mr.mu.Lock()
	defer mr.mu.Unlock()

	if _, ok := mr.itemsById[id]; ok {
		return &ErrDuplicateID{
			ID:   id,
			Repo: mr.path,
		}
	}
	if _, ok := mr.itemsByPath[relPath]; ok {
		return &ErrDuplicatePath{
			Path: relPath,
			Repo: mr.path,
		}
	}

	mr.addItem(id, relPath, m)
	if mr.logger != nil {
		mr.logger.Info(
			"added media to repository",
			zap.String("repo", mr.id),
			zap.String("repo_path", mr.path),
			zap.String("id", id),
			zap.String("path", relPath),
		)
	}

	return nil
}

func (mr *mutableRepo) Add(m media.Media) error {
	id := m.ID()
	if !media.ValidID(id) {
		return &ErrInvalidID{
			ID:       id,
			Expected: "^[a-z0-9-_]+$", // media.idPattern
		}
	}

	path := m.Path()
	if err := mr.checkFormat(path, m.Format()); err != nil {
		return errors.Wrap(err, "failed format check")
	}

	return mr.add(id, path, m)
}

func (mr *mutableRepo) AddPath(path string) error {
	format, err := mr.detectAndCheckFormat(path)
	if err != nil {
		return errors.Wrap(err, "failed format check")
	}

	m, err := mr.metaSource.FromFile(path)
	if err != nil {
		return errors.Wrap(err, "failed to discover metadata")
	}

	id := media.SanitizeID(filepath.Base(path))
	return mr.add(id, path, media.NewMedia(id, path, m, format))
}

func (mr *mutableRepo) Remove(m media.Media) error {
	id := m.ID()
	relPath, err := filepath.Rel(mr.path, m.Path())
	if err != nil {
		return nil // fast path: can't be made relative
	}

	mr.mu.Lock()
	defer mr.mu.Unlock()

	ok := mr.removeItem(id, relPath)
	if ok && mr.logger != nil { // don't log anything if it were a no-op
		mr.logger.Info(
			"removed media from repository",
			zap.String("repo", mr.id),
			zap.String("repo_path", mr.path),
			zap.String("id", id),
			zap.String("path", relPath),
		)
	}

	return nil
}

func (mr *mutableRepo) RemovePath(path string) error {
	relPath, err := filepath.Rel(mr.path, path)
	if err != nil {
		return nil // fast path: can't be made relative
	}

	mr.mu.Lock()
	defer mr.mu.Unlock()

	m, ok := mr.itemsByPath[relPath]
	if !ok {
		return nil // fast path: path not in repository
	}

	mr.removeItem(m.ID(), relPath)
	if mr.logger != nil {
		mr.logger.Info(
			"removed media from repository",
			zap.String("repo", mr.id),
			zap.String("repo_path", mr.path),
			zap.String("path", relPath),
		)
	}

	return nil
}

func (mr *mutableRepo) Items() []media.Media {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	return maps.Values(mr.itemsById)
}

func (mr *mutableRepo) Remux(_ string, _ *media.Format) (media.Media, error) {
	return nil, &ErrUnsupportedOperation{
		Operation: "remux",
		Repo:      mr.id,
	}
}

func (mr *mutableRepo) Source() meta.Source {
	return mr.metaSource
}

func (mr *mutableRepo) Close() error {
	return nil
}

func (mr *mutableRepo) Mutable() MutableRepository {
	return mr
}
