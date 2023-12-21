package v1

import (
	"context"
	"fmt"
	"github.com/erni27/imcache"
	"github.com/go-faster/errors"
	"github.com/go-faster/jx"
	"github.com/katana-project/katana/repo"
	"github.com/katana-project/katana/server/api/v1"
	"github.com/ogen-go/ogen/ogenerrors"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"net/http"
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
			return nil, errors.New(fmt.Sprintf("duplicate repository ID %s", repoId))
		}

		reposById[repoId] = r
	}

	return &Server{
		repos:  reposById,
		logger: logger,
	}, nil
}

// DefaultErrorHandler is the default error handler that writes a v1.Error object.
func DefaultErrorHandler(_ context.Context, w http.ResponseWriter, _ *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ogenerrors.ErrorCode(err))

	e := jx.GetEncoder()
	e.ObjStart()
	e.FieldStart("type")
	v, _ := v1.ErrorTypeInternalError.MarshalText()
	e.ByteStr(v)
	e.FieldStart("description")
	e.StrEscape(err.Error())
	e.ObjEnd()

	_, _ = w.Write(e.Bytes())
}

// NewRouter creates a new v1 API router.
func NewRouter(handler v1.Handler) (http.Handler, error) {
	s, err := v1.NewServer(handler, v1.WithPathPrefix("/v1"), v1.WithErrorHandler(DefaultErrorHandler))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create api server")
	}

	return s, nil
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

func (s *Server) newError(type_ v1.ErrorType, description string) *v1.Error {
	return &v1.Error{
		Type:        type_,
		Description: description,
	}
}
