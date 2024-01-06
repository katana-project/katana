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
	// A repository with this flag can be safely asserted (Repository.Muxing) to be MuxingRepository.
	CapabilityRemux
	// CapabilityTranscode is a flag of a repository that is able to transcode media.
	// A repository with this flag can be safely asserted (Repository.Muxing) to be MuxingRepository.
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
	// RemovePath removes media with the supplied absolute path from the repository.
	RemovePath(path string) error
	// Items returns the pieces of media in this repository.
	Items() []media.Media
	// Source returns the metadata source for this repository.
	Source() meta.Source
	// Close cleans up residual data after the repository.
	// The repository should not be used any further after calling Close.
	Close() error

	// Mux tries to assert this repository to a MuxingRepository, returns nil if not possible.
	Mux() MuxingRepository
}

// MuxingRepository is a repository capable of remuxing and transcoding operations.
type MuxingRepository interface {
	Repository

	// Remux remuxes media to the desired container format and returns the remuxed media or nil, if the ID wasn't found.
	Remux(id string, format *media.Format) (media.Media, error)
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

// crudRepository is an implementation of a Repository.
type crudRepository struct {
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

func (cr *crudRepository) checkFormat(path string, format *media.Format) error {
	group := strings.SplitN(format.MIME, "/", 2)[0]
	if !slices.Contains(allowedMimeGroups, group) {
		return &ErrInvalidMediaType{
			Path: path,
			Type: format.MIME,
		}
	}

	return nil
}

func (cr *crudRepository) detectAndCheckFormat(path string) (*media.Format, error) {
	t, err := mimetype.DetectFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to detect MIME type")
	}

	format := media.FindUnsupportedFormat(t.String(), filepath.Ext(path))
	return format, cr.checkFormat(path, format)
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
				format, err := cr.detectAndCheckFormat(path)
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

					return err // wrapped in checkFormat already
				}

				m, err := cr.metaSource.FromFile(path)
				if err != nil {
					return errors.Wrap(err, "failed to discover metadata")
				}

				id := media.SanitizeID(d.Name())
				cr.addItem(id, relPath, media.NewMedia(id, path, m, format))
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
	if _, err := os.Stat(path); err != nil { // catches non-existent files
		return errors.Wrap(err, "failed to stat file")
	}

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
	if err := cr.checkFormat(path, m.Format()); err != nil {
		return errors.Wrap(err, "failed format check")
	}

	return cr.add(id, path, m)
}

func (cr *crudRepository) AddPath(path string) error {
	format, err := cr.detectAndCheckFormat(path)
	if err != nil {
		return errors.Wrap(err, "failed format check")
	}

	m, err := cr.metaSource.FromFile(path)
	if err != nil {
		return errors.Wrap(err, "failed to discover metadata")
	}

	id := media.SanitizeID(filepath.Base(path))
	return cr.add(id, path, media.NewMedia(id, path, m, format))
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

func (cr *crudRepository) Mux() MuxingRepository {
	return nil
}
