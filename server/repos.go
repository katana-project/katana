package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/katana-project/katana/repo"
	"github.com/katana-project/katana/repo/media"
	"github.com/katana-project/katana/repo/media/meta"
	"github.com/katana-project/katana/server/api"
	"go.uber.org/zap"
	"golang.org/x/text/language"
	"net/http"
	"os"
)

const defaultStreamContentDisp = "inline"

// GetRepos implements getRepos operation.
//
// Lists all repositories currently known to the server.
//
// GET /repos
func (s *Server) GetRepos(_ context.Context) ([]api.Repository, error) {
	repos := make([]api.Repository, 0, len(s.repos))
	for _, r := range s.repos {
		repos = append(repos, s.makeRepository(r))
	}

	return repos, nil
}

// GetRepoById implements getRepoById operation.
//
// Gets a repository by its ID.
//
// GET /repos/{id}
func (s *Server) GetRepoById(_ context.Context, params api.GetRepoByIdParams) (api.GetRepoByIdRes, error) {
	if r, ok := s.repos[params.ID]; ok {
		r0 := s.makeRepository(r)
		return &r0, nil
	}

	return s.newError(api.ErrorTypeNotFound, "repository not found"), nil
}

// GetRepoMedia implements getRepoMedia operation.
//
// Gets a repository by its ID and lists its media.
//
// GET /repos/{id}/media
func (s *Server) GetRepoMedia(_ context.Context, params api.GetRepoMediaParams) (api.GetRepoMediaRes, error) {
	r, ok := s.repos[params.ID]
	if !ok {
		return s.newError(api.ErrorTypeNotFound, "repository not found"), nil
	}

	var (
		items     = r.Items()
		repoMedia = make([]api.Media, len(items))
	)
	for i, item := range items {
		repoMedia[i] = s.makeMedia(item)
	}

	res := api.GetRepoMediaOKApplicationJSON(repoMedia)
	return &res, nil
}

// GetRepoMediaById implements getRepoMediaById operation.
//
// Gets media by its ID in a repository.
//
// GET /repos/{repoId}/media/{mediaId}
func (s *Server) GetRepoMediaById(_ context.Context, params api.GetRepoMediaByIdParams) (api.GetRepoMediaByIdRes, error) {
	r, ok := s.repos[params.RepoId]
	if !ok {
		return s.newError(api.ErrorTypeNotFound, "repository not found"), nil
	}

	m := r.Get(params.MediaId)
	if m == nil {
		return s.newError(api.ErrorTypeNotFound, "media not found"), nil
	}

	m0 := s.makeMedia(m)
	return &m0, nil
}

// GetRepoMediaRawStream implements getRepoMediaRawStream operation.
//
// Gets media by its ID in a repository and returns an HTTP media stream of the original file.
//
// GET /repos/{repoId}/media/{mediaId}/stream/raw
func (s *Server) GetRepoMediaRawStream(ctx context.Context, params api.GetRepoMediaRawStreamParams) (api.GetRepoMediaRawStreamRes, error) {
	rp, ok := s.repos[params.RepoId]
	if !ok {
		return s.newError(api.ErrorTypeNotFound, "repository not found"), nil
	}

	m := rp.Get(params.MediaId)
	if m == nil {
		return s.newError(api.ErrorTypeNotFound, "media not found"), nil
	}

	// takeover with http.ServeFile
	var (
		w = ctx.Value(api.WriterCtxKey).(http.ResponseWriter)
		r = ctx.Value(api.RequestCtxKey).(*http.Request)
	)

	w.Header().Set("Content-Type", m.MIME())
	w.Header().Set("Content-Disposition", defaultStreamContentDisp)

	http.ServeFile(w, r, m.Path())
	return nil, nil
}

func (s *Server) makeRepository(r repo.Repository) api.Repository {
	return api.Repository{
		ID:           r.ID(),
		Name:         r.Name(),
		Capabilities: s.makeCapabilities(r.Capabilities()),
	}
}

func (s *Server) makeCapabilities(c repo.Capability) []api.RepositoryCapability {
	var caps []api.RepositoryCapability
	if c.Has(repo.CapabilityWatch) {
		caps = append(caps, api.RepositoryCapabilityWatch)
	}
	if c.Has(repo.CapabilityIndex) {
		caps = append(caps, api.RepositoryCapabilityIndex)
	}
	if c.Has(repo.CapabilityRemux) {
		caps = append(caps, api.RepositoryCapabilityRemux)
	}
	if c.Has(repo.CapabilityTranscode) {
		caps = append(caps, api.RepositoryCapabilityTranscode)
	}

	return caps
}

func (s *Server) makeMedia(m media.Media) api.Media {
	var (
		repoMeta  = m.Meta()
		mediaMeta api.NilMediaMeta
	)
	if repoMeta != nil {
		mediaMeta.SetTo(s.makeMediaMetadata(repoMeta))
	} else {
		mediaMeta.SetToNull()
	}

	return api.Media{
		ID:   m.ID(),
		Meta: mediaMeta,
	}
}

func (s *Server) makeMediaMetadata(m meta.Metadata) api.MediaMeta {
	mm := api.MediaMeta{}

	switch metaVariant := m.(type) {
	case meta.EpisodeMetadata:
		mm.SetEpisodeMetadata(s.makeEpisodeMetadata(metaVariant))
	case meta.MovieOrSeriesMetadata:
		switch metaVariant.Type() {
		case meta.TypeMovie:
			mm.SetMovieMetadata(s.makeMovieMetadata(metaVariant))
		case meta.TypeSeries:
			mm.SetSeriesMetadata(s.makeSeriesMetadata(metaVariant))
		default: // the metadata instance is breaking its contract, just force it to be generic
			mm.SetMetadata(s.makeMetadata(metaVariant))
		}
	default:
		mm.SetMetadata(s.makeMetadata(metaVariant))
	}

	return mm
}

func newNilString(v string) api.NilString {
	s := api.NewNilString(v)
	if v0, ok := s.Get(); ok && v0 == "" { // convert zero value to nil
		s.SetToNull()
	}

	return s
}

func (s *Server) makeMovieMetadata(m meta.MovieOrSeriesMetadata) api.MovieMetadata {
	return api.MovieMetadata{
		Title:         m.Title(),
		OriginalTitle: newNilString(m.OriginalTitle()),
		Overview:      newNilString(m.Overview()),
		ReleaseDate:   m.ReleaseDate(),
		VoteRating:    m.VoteRating(),
		Images:        s.makeImages(m.Images()),
		Genres:        m.Genres(),
		Cast:          s.makeCastMembers(m.Cast()),
		Languages:     s.makeLanguages(m.Languages()),
		Countries:     s.makeCountries(m.Countries()),
	}
}

func (s *Server) makeImages(ims []meta.Image) []api.Image {
	var images []api.Image
	for _, i := range ims {
		im, err := s.makeImage(i)
		if err != nil {
			s.logger.Error(
				"failed to make image, skipping",
				zap.String("path", i.Path()),
				zap.Bool("remote", i.Remote()),
				zap.String("description", i.Description()),
				zap.Error(err),
			)
			continue
		}

		images = append(images, im)
	}

	return images
}

func (s *Server) makeImage(i meta.Image) (api.Image, error) {
	var (
		path   = i.Path()
		remote = i.Remote()
	)
	if !remote {
		b, err := os.ReadFile(path)
		if err != nil {
			return api.Image{}, err
		}

		path = fmt.Sprintf("data:%s;base64,%s", http.DetectContentType(b), base64.StdEncoding.EncodeToString(b))
	}

	return api.Image{
		Path:        path,
		Remote:      remote,
		Description: newNilString(i.Description()),
	}, nil
}

func (s *Server) makeCastMembers(cms []meta.CastMember) []api.CastMember {
	if cms == nil {
		return nil
	}

	castMembers := make([]api.CastMember, len(cms))
	for i, cm := range cms {
		var (
			image  = cm.Image()
			image0 api.OptImage
		)
		if image != nil {
			im, err := s.makeImage(image)
			if err == nil {
				image0.SetTo(im)
			} else {
				s.logger.Error(
					"failed to make cast member image, skipping",
					zap.String("path", image.Path()),
					zap.Bool("remote", image.Remote()),
					zap.String("description", image.Description()),
					zap.Error(err),
				)
			}
		}

		castMembers[i] = api.CastMember{
			Name:  cm.Name(),
			Role:  cm.Role(),
			Image: image0,
		}
	}

	return castMembers
}

func (s *Server) makeLanguages(ts []language.Tag) []string {
	if ts == nil {
		return nil
	}

	tags := make([]string, len(ts))
	for i, tag := range ts {
		tags[i] = tag.String()
	}

	return tags
}

func (s *Server) makeCountries(rgs []language.Region) []string {
	if rgs == nil {
		return nil
	}

	regions := make([]string, len(rgs))
	for i, region := range rgs {
		regions[i] = region.String()
	}

	return regions
}

func (s *Server) makeSeriesMetadata(m meta.MovieOrSeriesMetadata) api.SeriesMetadata {
	return api.SeriesMetadata{
		Title:         m.Title(),
		OriginalTitle: newNilString(m.OriginalTitle()),
		Overview:      newNilString(m.Overview()),
		ReleaseDate:   m.ReleaseDate(),
		VoteRating:    m.VoteRating(),
		Images:        s.makeImages(m.Images()),
		Genres:        m.Genres(),
		Cast:          s.makeCastMembers(m.Cast()),
		Languages:     s.makeLanguages(m.Languages()),
		Countries:     s.makeCountries(m.Countries()),
	}
}

func (s *Server) makeEpisodeMetadata(m meta.EpisodeMetadata) api.EpisodeMetadata {
	return api.EpisodeMetadata{
		Title:         m.Title(),
		OriginalTitle: newNilString(m.OriginalTitle()),
		Overview:      newNilString(m.Overview()),
		ReleaseDate:   m.ReleaseDate(),
		VoteRating:    m.VoteRating(),
		Images:        s.makeImages(m.Images()),
		Series:        s.makeSeriesMetadata(m.Series()),
		Season:        m.Season(),
		Episode:       m.Episode(),
	}
}

func (s *Server) makeMetadata(m meta.Metadata) api.Metadata {
	return api.Metadata{
		Title:         m.Title(),
		OriginalTitle: newNilString(m.OriginalTitle()),
		Overview:      newNilString(m.Overview()),
		ReleaseDate:   m.ReleaseDate(),
		VoteRating:    m.VoteRating(),
		Images:        s.makeImages(m.Images()),
	}
}
