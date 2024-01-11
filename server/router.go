package server

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/katana-project/katana/config"
	"github.com/katana-project/katana/internal/errors"
	"github.com/katana-project/katana/repo"
	"github.com/katana-project/katana/repo/index"
	"github.com/katana-project/katana/repo/media/meta"
	"github.com/katana-project/katana/repo/mux"
	"github.com/katana-project/katana/repo/watch"
	"github.com/katana-project/katana/server/v1"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"io"
	"net/http"
)

// HandlerCloser is a http.Handler that is notified of the HTTP server shutting down.
type HandlerCloser interface {
	http.Handler
	io.Closer
}

// handlerCloser is an implementation of HandlerCloser.
type handlerCloser struct {
	http.Handler
	io.Closer
}

// NewRouter creates a new router from configuration.
func NewRouter(repos []repo.Repository, logger *zap.Logger) (HandlerCloser, error) {
	v1Srv, err := v1.NewServer(repos, logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create v1 api handler")
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestLogger(&middleware.DefaultLogFormatter{
		Logger:  zap.NewStdLog(logger),
		NoColor: true,
	}))
	r.Use(middleware.Recoverer)
	r.Route("/api", func(r chi.Router) {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"https://*", "http://*"},
			AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: false,
			MaxAge:           300,
		}))

		r.Mount("/v1", v1.NewRouter("/api/v1", v1Srv))
	})

	return &handlerCloser{
		Handler: r,
		Closer:  v1Srv,
	}, nil
}

// NewConfiguredRouter creates a new router from configuration.
func NewConfiguredRouter(cfg *config.Config, logger *zap.Logger) (HandlerCloser, error) {
	repos := make(map[string]repo.Repository, len(cfg.Repos))
	for repoId, repoConfig := range cfg.Repos {
		if _, ok := repos[repoId]; ok {
			return nil, &ErrDuplicateRepo{
				ID:   repoId,
				Path: repoConfig.Path,
			}
		}

		metaSources := make([]meta.Source, 0, len(repoConfig.Sources))
		for sourceName, options := range repoConfig.Sources {
			ms, err := NewConfiguredMetaSource(sourceName, options)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to configure metadata source %s", sourceName)
			}

			metaSources = append(metaSources, ms)
		}

		metaSource := meta.NewCompositeSource(metaSources...)
		r, err := repo.NewRepository(repoId, repoConfig.Name, repoConfig.Path, metaSource, logger)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create repository")
		}

		if repoConfig.Capable(config.CapabilityRemux) || repoConfig.Capable(config.CapabilityTranscode) {
			r, err = mux.NewRepository(r, repo.Capabilities(repoConfig.Capabilities), repoConfig.CachePath, logger)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create mux repository")
			}
		}

		if repoConfig.IndexPath != "" { // zero value
			r, err = index.NewRepository(r, repoConfig.IndexPath, logger)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create indexed repository")
			}
		}

		if repoConfig.Capable(config.CapabilityWatch) {
			r, err = watch.NewRepository(r, logger)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create watched repository")
			}
		}

		go func() {
			if err := r.Scan(); err != nil {
				logger.Error("failed to scan repository", zap.String("repo", repoId), zap.Error(err))
			}
		}()

		repos[repoId] = r
	}

	return NewRouter(maps.Values(repos), logger)
}
