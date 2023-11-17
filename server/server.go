package server

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-faster/errors"
	"github.com/go-faster/jx"
	"github.com/katana-project/katana/config"
	"github.com/katana-project/katana/repo"
	"github.com/katana-project/katana/repo/media/meta"
	"github.com/katana-project/katana/repo/media/meta/tmdb"
	"github.com/katana-project/katana/server/api"
	tmdbClient "github.com/katana-project/tmdb"
	"github.com/mitchellh/mapstructure"
	"github.com/ogen-go/ogen/ogenerrors"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"golang.org/x/text/language"
	"net/http"
)

var tmdbApiUrl = "https://api.themoviedb.org/"

// Server is a REST server for the Katana API.
type Server struct {
	repos  map[string]repo.Repository
	logger *zap.Logger
}

// NewServer creates a new server with pre-defined repositories.
func NewServer(repos []repo.Repository, logger *zap.Logger) (*Server, error) {
	reposById := make(map[string]repo.Repository, len(repos))
	for _, r := range repos {
		repoId := r.ID()
		if _, ok := reposById[repoId]; ok {
			return nil, &ErrDuplicateRepo{
				ID:   repoId,
				Path: r.Path(),
			}
		}

		reposById[repoId] = r
	}

	return &Server{
		repos:  reposById,
		logger: logger,
	}, nil
}

// NewConfiguredServer creates a new server from configuration.
func NewConfiguredServer(cfg *config.Config, logger *zap.Logger) (*Server, error) {
	repos := make([]repo.Repository, 0, len(cfg.Repos))
	for repoId, repoConfig := range cfg.Repos {
		metaSources := make([]meta.Source, 0, len(repoConfig.Sources))
		for sourceName, options := range repoConfig.Sources {
			ms, err := NewConfiguredMetaSource(sourceName, options)
			if err != nil {
				return nil, errors.Wrap(err, "failed to configure metadata source")
			}

			metaSources = append(metaSources, ms)
		}

		var (
			sourcesLen = len(metaSources)
			metaSource = meta.NewDummySource()
		)
		if sourcesLen > 1 {
			metaSource = meta.NewCompositeSource(metaSources...)
		} else if sourcesLen == 1 {
			metaSource = metaSources[0]
		}

		repoName := repoConfig.Name
		if repoName == "" { // zero value
			repoName = repoId
		}

		r, err := repo.NewRepository(repoId, repoName, repoConfig.Path, metaSource, logger)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create repository")
		}

		if repoConfig.IndexPath != "" {
			r, err = repo.NewIndexedRepository(r, repoConfig.IndexPath, logger)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create indexed repository")
			}
		}

		if repoConfig.Watch {
			r, err = repo.NewWatchedRepository(r, logger)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create watched repository")
			}
		}

		if err := r.Scan(); err != nil {
			return nil, errors.Wrap(err, "failed to scan repository")
		}

		repos = append(repos, r)
	}

	return NewServer(repos, logger)
}

// DefaultErrorHandler is the default error handler that writes an api.Error object.
func DefaultErrorHandler(_ context.Context, w http.ResponseWriter, _ *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ogenerrors.ErrorCode(err))

	e := jx.GetEncoder()
	e.ObjStart()
	e.FieldStart("description")
	e.StrEscape(err.Error())
	e.ObjEnd()

	_, _ = w.Write(e.Bytes())
}

// NewRouter creates a new router.
func NewRouter(handler api.Handler, logger *zap.Logger) (http.Handler, error) {
	s, err := api.NewServer(handler, api.WithPathPrefix("/api"), api.WithErrorHandler(DefaultErrorHandler))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create api server")
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestLogger(&middleware.DefaultLogFormatter{
		Logger:  zap.NewStdLog(logger),
		NoColor: true,
	}))
	r.Use(middleware.Recoverer)
	r.Group(func(r chi.Router) {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"https://*", "http://*"},
			AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: false,
			MaxAge:           300,
		}))

		r.Mount("/api", s)
	})

	return r, nil
}

// NewConfiguredRouter creates a new server router from configuration.
func NewConfiguredRouter(cfg *config.Config, logger *zap.Logger) (http.Handler, error) {
	server, err := NewConfiguredServer(cfg, logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure server")
	}

	return NewRouter(server, logger)
}

func (s *Server) Repos() []repo.Repository {
	return maps.Values(s.repos)
}

func (s *Server) newError(description string) *api.Error {
	return &api.Error{
		Description: description,
	}
}

type tmdbSourceOptions struct {
	Key  string `mapstructure:"key"`
	URL  string `mapstructure:"url"`
	Lang string `mapstructure:"lang"`
}

type tmdbSecuritySource struct {
	key tmdbClient.Sec0
}

func (tss *tmdbSecuritySource) Sec0(_ context.Context, _ string) (tmdbClient.Sec0, error) {
	return tss.key, nil
}

// NewConfiguredMetaSource creates a metadata source from configuration.
func NewConfiguredMetaSource(name string, options map[string]interface{}) (meta.Source, error) {
	switch name {
	case "literal":
		return meta.NewLiteralSource(), nil
	case "tmdb":
		var parsedOpts tmdbSourceOptions
		if err := mapstructure.WeakDecode(options, &parsedOpts); err != nil {
			return nil, errors.Wrapf(err, "failed to decode metadata source %s options", name)
		}

		url := parsedOpts.URL
		if url == "" { // zero value
			url = tmdbApiUrl
		}

		sec0 := &tmdbSecuritySource{key: tmdbClient.Sec0{Token: parsedOpts.Key}}
		client, err := tmdbClient.NewClient(url, sec0)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create tmdb api client")
		}

		langStr := parsedOpts.Lang
		if langStr == "" { // zero value
			langStr = "en-US"
		}

		lang, err := language.Parse(langStr)
		if err != nil {
			return nil, errors.Errorf("failed to parse tmdb api language preference")
		}

		return tmdb.NewSource(client, lang, true), nil
	case "analysis":
		metaSources := make([]meta.Source, 0, len(options))
		for sourceName, sourceOptions0 := range options {
			sourceOptions, ok := sourceOptions0.(map[string]interface{})
			if !ok {
				return nil, errors.Errorf("failed to parse metadata sub-source %s options", sourceName)
			}

			ms, err := NewConfiguredMetaSource(sourceName, sourceOptions)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to configure metadata sub-source %s", sourceName)
			}

			metaSources = append(metaSources, ms)
		}

		var (
			sourcesLen = len(metaSources)
			metaSource = meta.NewDummySource()
		)
		if sourcesLen > 1 {
			metaSource = meta.NewCompositeSource(metaSources...)
		} else if sourcesLen == 1 {
			metaSource = metaSources[0]
		}

		return meta.NewFileAnalysisSource(metaSource), nil
	}

	return nil, errors.Errorf("unknown metadata source %s", name)
}