package v1

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/erni27/imcache"
	"github.com/katana-project/katana/repo"
	"github.com/katana-project/katana/repo/media"
	"github.com/katana-project/katana/repo/media/meta"
	"github.com/katana-project/katana/server/api/v1"
	"go.uber.org/zap"
	"golang.org/x/text/language"
	"net/http"
	"os"
	"time"
)

// defaultStreamContentDisp is the default Content-Disposition header value for streamed content.
const defaultStreamContentDisp = "inline"

// imageCacheExp is the cache expiration period for non-remote images' data loaded into memory.
var imageCacheExp = imcache.WithExpiration(5 * time.Minute)

// GetRepos implements getRepos operation.
//
// Lists all repositories currently known to the server.
//
// GET /repos
func (s *Server) GetRepos(_ context.Context) ([]v1.Repository, error) {
	repos := make([]v1.Repository, 0, len(s.repos))
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
func (s *Server) GetRepoById(_ context.Context, params v1.GetRepoByIdParams) (v1.GetRepoByIdRes, error) {
	if r, ok := s.repos[params.ID]; ok {
		r0 := s.makeRepository(r)
		return &r0, nil
	}

	return s.newError(v1.ErrorTypeNotFound, "repository not found"), nil
}

// GetRepoMedia implements getRepoMedia operation.
//
// Gets a repository by its ID and lists its media.
//
// GET /repos/{id}/media
func (s *Server) GetRepoMedia(_ context.Context, params v1.GetRepoMediaParams) (v1.GetRepoMediaRes, error) {
	r, ok := s.repos[params.ID]
	if !ok {
		return s.newError(v1.ErrorTypeNotFound, "repository not found"), nil
	}

	var (
		imageMode = v1.ImageModeNone

		items     = r.Items()
		repoMedia = make([]v1.Media, len(items))
	)
	if im, ok := params.Images.Get(); ok {
		imageMode = im
	}
	for i, item := range items {
		repoMedia[i] = s.makeMedia(item, imageMode)
	}

	res := v1.GetRepoMediaOKApplicationJSON(repoMedia)
	return &res, nil
}

// GetRepoMediaById implements getRepoMediaById operation.
//
// Gets media by its ID in a repository.
//
// GET /repos/{repoId}/media/{mediaId}
func (s *Server) GetRepoMediaById(_ context.Context, params v1.GetRepoMediaByIdParams) (v1.GetRepoMediaByIdRes, error) {
	r, ok := s.repos[params.RepoId]
	if !ok {
		return s.newError(v1.ErrorTypeNotFound, "repository not found"), nil
	}

	m := r.Get(params.MediaId)
	if m == nil {
		return s.newError(v1.ErrorTypeNotFound, "media not found"), nil
	}

	m0 := s.makeMedia(m, v1.ImageModeAll)
	return &m0, nil
}

// GetRepoMediaStream implements getRepoMediaStream operation.
//
// Gets media by its ID in a repository and returns an HTTP media stream of the file.
//
// GET /repos/{repoId}/media/{mediaId}/stream/{format}
func (s *Server) GetRepoMediaStream(ctx context.Context, params v1.GetRepoMediaStreamParams) (v1.GetRepoMediaStreamRes, error) {
	rp, ok := s.repos[params.RepoId]
	if !ok {
		return s.newError(v1.ErrorTypeNotFound, "repository not found"), nil
	}

	var m media.Media
	if params.Format == v1.MediaFormatRaw {
		m = rp.Get(params.MediaId)
	} else {
		if !rp.Capabilities().Has(repo.CapabilityRemux) {
			return s.newError(v1.ErrorTypeMissingCapability, "missing 'remux' capability"), nil
		}

		format := media.FindFormat(string(params.Format))
		if format == nil {
			return s.newError(v1.ErrorTypeUnknownFormat, fmt.Sprintf("unknown format '%s'", params.Format)), nil
		}

		var err error
		m, err = rp.Mux().Remux(params.MediaId, format)
		if err != nil {
			return nil, err
		}
	}

	if m == nil {
		return s.newError(v1.ErrorTypeNotFound, "media not found"), nil
	}

	// takeover with http.ServeFile
	var (
		w = ctx.Value(v1.WriterCtxKey).(http.ResponseWriter)
		r = ctx.Value(v1.RequestCtxKey).(*http.Request)
	)

	w.Header().Set("Content-Type", m.MIME())
	w.Header().Set("Content-Disposition", defaultStreamContentDisp)

	http.ServeFile(w, r, m.Path())
	return nil, nil
}

func (s *Server) makeRepository(r repo.Repository) v1.Repository {
	return v1.Repository{
		ID:           r.ID(),
		Name:         r.Name(),
		Capabilities: s.makeCapabilities(r.Capabilities()),
	}
}

func (s *Server) makeCapabilities(c repo.Capability) []v1.RepositoryCapability {
	var caps []v1.RepositoryCapability
	if c.Has(repo.CapabilityWatch) {
		caps = append(caps, v1.RepositoryCapabilityWatch)
	}
	if c.Has(repo.CapabilityIndex) {
		caps = append(caps, v1.RepositoryCapabilityIndex)
	}
	if c.Has(repo.CapabilityRemux) {
		caps = append(caps, v1.RepositoryCapabilityRemux)
	}
	if c.Has(repo.CapabilityTranscode) {
		caps = append(caps, v1.RepositoryCapabilityTranscode)
	}

	return caps
}

func (s *Server) makeMedia(m media.Media, imageMode v1.ImageMode) v1.Media {
	var (
		repoMeta  = m.Meta()
		mediaMeta v1.NilMediaMeta
	)
	if repoMeta != nil {
		mediaMeta.SetTo(s.makeMediaMetadata(repoMeta, imageMode))
	} else {
		mediaMeta.SetToNull()
	}

	return v1.Media{
		ID:   m.ID(),
		Meta: mediaMeta,
	}
}

func (s *Server) makeMediaMetadata(m meta.Metadata, imageMode v1.ImageMode) v1.MediaMeta {
	mm := v1.MediaMeta{}

	switch metaVariant := m.(type) {
	case meta.EpisodeMetadata:
		mm.SetEpisodeMetadata(s.makeEpisodeMetadata(metaVariant, imageMode))
	case meta.MovieOrSeriesMetadata:
		switch metaVariant.Type() {
		case meta.TypeMovie:
			mm.SetMovieMetadata(s.makeMovieMetadata(metaVariant, imageMode))
		case meta.TypeSeries:
			mm.SetSeriesMetadata(s.makeSeriesMetadata(metaVariant, imageMode))
		default: // the metadata instance is breaking its contract, just force it to be generic
			mm.SetMetadata(s.makeMetadata(metaVariant, imageMode))
		}
	default:
		mm.SetMetadata(s.makeMetadata(metaVariant, imageMode))
	}

	return mm
}

func newNilString(v string) v1.NilString {
	s := v1.NewNilString(v)
	if v0, ok := s.Get(); ok && v0 == "" { // convert zero value to nil
		s.SetToNull()
	}

	return s
}

func (s *Server) makeMovieMetadata(m meta.MovieOrSeriesMetadata, imageMode v1.ImageMode) v1.MovieMetadata {
	return v1.MovieMetadata{
		Title:         m.Title(),
		OriginalTitle: newNilString(m.OriginalTitle()),
		Overview:      newNilString(m.Overview()),
		ReleaseDate:   m.ReleaseDate(),
		VoteRating:    m.VoteRating(),
		Images:        s.makeImages(m.Images(), imageMode),
		Genres:        m.Genres(),
		Cast:          s.makeCastMembers(m.Cast(), imageMode),
		Languages:     s.makeLanguages(m.Languages()),
		Countries:     s.makeCountries(m.Countries()),
	}
}

func (s *Server) makeImages(ims []meta.Image, imageMode v1.ImageMode) []v1.Image {
	if imageMode == v1.ImageModeNone {
		return nil
	}

	var images []v1.Image
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

		if imageMode == v1.ImageModeBasic && (im.Type == v1.ImageTypeAvatar || im.Type == v1.ImageTypeStill) {
			continue // basic only sends backdrops and posters
		}

		images = append(images, im)
	}

	return images
}

func (s *Server) makeImage(i meta.Image) (v1.Image, error) {
	var (
		path   = i.Path()
		remote = i.Remote()
	)
	if !remote {
		if path0, ok := s.imageCache.Get(path); ok {
			path = path0
		} else {
			b, err := os.ReadFile(path)
			if err != nil {
				return v1.Image{}, err
			}

			data := fmt.Sprintf(
				"data:%s;base64,%s",
				http.DetectContentType(b),
				base64.StdEncoding.EncodeToString(b),
			)
			s.imageCache.Set(path, data, imageCacheExp)
			path = data
		}
	}

	type_ := v1.ImageTypeUnknown
	switch i.Type() {
	case meta.ImageTypeStill:
		type_ = v1.ImageTypeStill
	case meta.ImageTypeBackdrop:
		type_ = v1.ImageTypeBackdrop
	case meta.ImageTypePoster:
		type_ = v1.ImageTypePoster
	case meta.ImageTypeAvatar:
		type_ = v1.ImageTypeAvatar
	}

	return v1.Image{
		Type:        type_,
		Path:        path,
		Remote:      remote,
		Description: newNilString(i.Description()),
	}, nil
}

func (s *Server) makeCastMembers(cms []meta.CastMember, imageMode v1.ImageMode) []v1.CastMember {
	if cms == nil {
		return nil
	}

	castMembers := make([]v1.CastMember, len(cms))
	for i, cm := range cms {
		var (
			image  = cm.Image()
			image0 v1.OptImage
		)
		if image != nil && imageMode == v1.ImageModeAll {
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

		castMembers[i] = v1.CastMember{
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

func (s *Server) makeSeriesMetadata(m meta.MovieOrSeriesMetadata, imageMode v1.ImageMode) v1.SeriesMetadata {
	return v1.SeriesMetadata{
		Title:         m.Title(),
		OriginalTitle: newNilString(m.OriginalTitle()),
		Overview:      newNilString(m.Overview()),
		ReleaseDate:   m.ReleaseDate(),
		VoteRating:    m.VoteRating(),
		Images:        s.makeImages(m.Images(), imageMode),
		Genres:        m.Genres(),
		Cast:          s.makeCastMembers(m.Cast(), imageMode),
		Languages:     s.makeLanguages(m.Languages()),
		Countries:     s.makeCountries(m.Countries()),
	}
}

func (s *Server) makeEpisodeMetadata(m meta.EpisodeMetadata, imageMode v1.ImageMode) v1.EpisodeMetadata {
	return v1.EpisodeMetadata{
		Title:         m.Title(),
		OriginalTitle: newNilString(m.OriginalTitle()),
		Overview:      newNilString(m.Overview()),
		ReleaseDate:   m.ReleaseDate(),
		VoteRating:    m.VoteRating(),
		Images:        s.makeImages(m.Images(), imageMode),
		Series:        s.makeSeriesMetadata(m.Series(), imageMode),
		Season:        m.Season(),
		Episode:       m.Episode(),
	}
}

func (s *Server) makeMetadata(m meta.Metadata, imageMode v1.ImageMode) v1.Metadata {
	return v1.Metadata{
		Title:         m.Title(),
		OriginalTitle: newNilString(m.OriginalTitle()),
		Overview:      newNilString(m.Overview()),
		ReleaseDate:   m.ReleaseDate(),
		VoteRating:    m.VoteRating(),
		Images:        s.makeImages(m.Images(), imageMode),
	}
}
