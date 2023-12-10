package server

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-faster/errors"
	"github.com/katana-project/katana/config"
	"github.com/katana-project/katana/repo"
	"github.com/katana-project/katana/repo/media/meta"
	"github.com/katana-project/katana/repo/mux"
	"github.com/katana-project/katana/server/v1"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"net/http"
)

// NewRouter creates a new router from configuration.
func NewRouter(repos []repo.Repository, logger *zap.Logger) (http.Handler, error) {
	v1Srv, err := v1.NewConfiguredRouter(repos, logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create v1 api router")
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

		r.Mount("/v1", http.StripPrefix("/api", v1Srv))
	})

	return r, nil
}

// NewConfiguredRouter creates a new router from configuration.
func NewConfiguredRouter(cfg *config.Config, logger *zap.Logger) (http.Handler, error) {
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
			ms, err := NewConfiguredMetaSource(string(sourceName), options)
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
			r, err = repo.NewIndexedRepository(r, repoConfig.IndexPath, logger)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create indexed repository")
			}
		}

		if repoConfig.Capable(config.CapabilityWatch) {
			r, err = repo.NewWatchedRepository(r, logger)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create watched repository")
			}
		}

		if err := r.Scan(); err != nil {
			return nil, errors.Wrap(err, "failed to scan repository")
		}

		repos[repoId] = r
	}

	return NewRouter(maps.Values(repos), logger)
}
