package v1

import (
	"encoding/json"
	"fmt"
	"github.com/erni27/imcache"
	"github.com/katana-project/katana/repo"
	"github.com/katana-project/katana/server/api/v1"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"net/http"
)

// ErrorHandler handles translating errors to HTTP responses.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

var (
	DefaultRequestErrorHandler ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		e := v1.Error{Type: v1.BadRequest, Description: err.Error()}
		if err := json.NewEncoder(w).Encode(e); err != nil {
			_, _ = fmt.Fprintf(w, "{\"type\":\"%s\",\"description\":\"%s\"}", v1.InternalError, "failed to serialize error")
		}
	}

	DefaultResponseErrorHandler ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)

		e := v1.Error{Type: v1.InternalError, Description: err.Error()}
		if err := json.NewEncoder(w).Encode(e); err != nil {
			_, _ = fmt.Fprintf(w, "{\"type\":\"%s\",\"description\":\"%s\"}", v1.InternalError, "failed to serialize error")
		}
	}
)

// Server is a REST server for the Katana v1 API.
type Server struct {
	repos  map[string]repo.Repository
	logger *zap.Logger

	imageCache imcache.Cache[string, string] // non-remote image data, base64-encoded data:image URLs
}

// NewServer creates a new server with pre-defined repositories.
func NewServer(repos []repo.Repository, logger *zap.Logger) (*Server, error) {
	reposById := make(map[string]repo.Repository, len(repos))
	for _, r := range repos {
		repoId := r.ID()
		if _, ok := reposById[repoId]; ok {
			return nil, fmt.Errorf("duplicate repository name %s", repoId)
		}

		reposById[repoId] = r
	}

	return &Server{
		repos:  reposById,
		logger: logger,
	}, nil
}

// NewRouter creates a new v1 API router.
func NewRouter(baseUrl string, handler v1.StrictServerInterface) http.Handler {
	h := v1.NewStrictHandlerWithOptions(handler, nil, v1.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  DefaultRequestErrorHandler,
		ResponseErrorHandlerFunc: DefaultResponseErrorHandler,
	})

	return v1.HandlerWithOptions(h, v1.ChiServerOptions{
		BaseURL:          baseUrl,
		ErrorHandlerFunc: DefaultRequestErrorHandler,
	})
}

// Repos returns all repositories available to the server.
func (s *Server) Repos() []repo.Repository {
	return maps.Values(s.repos)
}

// Close cleans up residual data after the server.
func (s *Server) Close() (err error) {
	for _, r := range s.repos {
		err = multierr.Append(err, r.Close())
	}

	return err
}
