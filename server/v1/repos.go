package v1

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/erni27/imcache"
	"github.com/katana-project/katana/internal/errors"
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

func (s *Server) GetRepos(_ context.Context, _ v1.GetReposRequestObject) (v1.GetReposResponseObject, error) {
	repos := make([]v1.Repository, 0, len(s.repos))
	for _, r := range s.repos {
		repos = append(repos, s.wrapRepo(r))
	}

	return v1.GetRepos200JSONResponse(repos), nil
}

func (s *Server) GetRepoById(_ context.Context, request v1.GetRepoByIdRequestObject) (v1.GetRepoByIdResponseObject, error) {
	if r, ok := s.repos[request.Id]; ok {
		return v1.GetRepoById200JSONResponse(s.wrapRepo(r)), nil
	}

	return v1.GetRepoById400JSONResponse(v1.Error{Type: v1.NotFound, Description: "repository not found"}), nil
}

func (s *Server) GetRepoMedia(_ context.Context, request v1.GetRepoMediaRequestObject) (v1.GetRepoMediaResponseObject, error) {
	r, ok := s.repos[request.Id]
	if !ok {
		return v1.GetRepoMedia400JSONResponse(v1.Error{Type: v1.NotFound, Description: "repository not found"}), nil
	}

	var (
		imageMode = v1.None

		items     = r.Items()
		repoMedia = make([]v1.Media, len(items))
	)
	if request.Params.Images != nil {
		imageMode = *request.Params.Images
	}
	for i, item := range items {
		m, err := s.wrapMedia(item, imageMode)
		if err != nil {
			return nil, errors.Wrap(err, "failed to wrap media")
		}

		repoMedia[i] = m
	}

	return v1.GetRepoMedia200JSONResponse(repoMedia), nil
}

func (s *Server) GetRepoMediaById(_ context.Context, request v1.GetRepoMediaByIdRequestObject) (v1.GetRepoMediaByIdResponseObject, error) {
	r, ok := s.repos[request.RepoId]
	if !ok {
		return v1.GetRepoMediaById400JSONResponse(v1.Error{Type: v1.NotFound, Description: "repository not found"}), nil
	}

	m := r.Get(request.MediaId)
	if m == nil {
		return v1.GetRepoMediaById400JSONResponse(v1.Error{Type: v1.NotFound, Description: "media not found"}), nil
	}

	m0, err := s.wrapMedia(m, v1.All)
	if err != nil {
		return nil, errors.Wrap(err, "failed to wrap media")
	}

	return v1.GetRepoMediaById200JSONResponse(m0), nil
}

func (s *Server) GetRepoMediaStreams(_ context.Context, _ v1.GetRepoMediaStreamsRequestObject) (v1.GetRepoMediaStreamsResponseObject, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Server) GetRepoMediaStream(_ context.Context, request v1.GetRepoMediaStreamRequestObject) (v1.GetRepoMediaStreamResponseObject, error) {
	rp, ok := s.repos[request.RepoId]
	if !ok {
		return v1.GetRepoMediaStream400JSONResponse(v1.Error{Type: v1.NotFound, Description: "repository not found"}), nil
	}

	var m media.Media
	if request.Format == v1.Raw {
		m = rp.Get(request.MediaId)
	} else {
		if !rp.Capabilities().Has(repo.CapabilityRemux) {
			return v1.GetRepoMediaStream400JSONResponse(v1.Error{Type: v1.MissingCapability, Description: "missing 'remux' capability"}), nil
		}

		format := media.FindFormat(string(request.Format))
		if format == nil {
			return v1.GetRepoMediaStream400JSONResponse(v1.Error{Type: v1.UnknownFormat, Description: fmt.Sprintf("unknown format '%s'", request.Format)}), nil
		}

		var err error
		m, err = rp.Mux().Remux(request.MediaId, format)
		if err != nil {
			return nil, errors.Wrap(err, "failed to remux media")
		}
	}

	if m == nil {
		return v1.GetRepoMediaStream400JSONResponse(v1.Error{Type: v1.NotFound, Description: "media not found"}), nil
	}

	format := m.Format()
	return &streamResp{path: m.Path(), mime: format.MIME}, nil
}

type streamResp struct {
	path, mime string
}

func (sr *streamResp) VisitGetRepoMediaStreamResponse(w http.ResponseWriter, r *http.Request) error {
	f, err := os.Open(sr.path)
	if err != nil {
		return errors.Wrap(err, "failed to open media")
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return errors.Wrap(err, "failed to stat media")
	}

	w.Header().Set("Content-Type", sr.mime)
	w.Header().Set("Content-Disposition", defaultStreamContentDisp)

	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
	return nil
}

func (s *Server) wrapRepo(r repo.Repository) v1.Repository {
	return v1.Repository{
		Id:           r.ID(),
		Name:         r.Name(),
		Capabilities: s.wrapCaps(r.Capabilities()),
	}
}

func (s *Server) wrapCaps(c repo.Capability) []v1.RepositoryCapability {
	var caps []v1.RepositoryCapability
	if c.Has(repo.CapabilityWatch) {
		caps = append(caps, v1.Watch)
	}
	if c.Has(repo.CapabilityIndex) {
		caps = append(caps, v1.Index)
	}
	if c.Has(repo.CapabilityRemux) {
		caps = append(caps, v1.Remux)
	}
	if c.Has(repo.CapabilityTranscode) {
		caps = append(caps, v1.Transcode)
	}

	return caps
}

func (s *Server) wrapMedia(m media.Media, imageMode v1.ImageMode) (v1.Media, error) {
	var (
		err error

		repoMeta  = m.Meta()
		mediaMeta *v1.Media_Meta
	)
	if repoMeta != nil {
		mediaMeta, err = s.wrapMediaMeta(repoMeta, imageMode)
		if err != nil {
			return v1.Media{}, err
		}
	}

	return v1.Media{
		Id:   m.ID(),
		Meta: mediaMeta,
	}, nil
}

func (s *Server) wrapMediaMeta(m meta.Metadata, imageMode v1.ImageMode) (*v1.Media_Meta, error) {
	var (
		mm  = &v1.Media_Meta{}
		err error
	)

	switch metaVariant := m.(type) {
	case meta.EpisodeMetadata:
		err = mm.FromEpisodeMetadata(s.wrapEpisodeMeta(metaVariant, imageMode))
	case meta.MovieOrSeriesMetadata:
		switch metaVariant.Type() {
		case meta.TypeMovie:
			err = mm.FromMovieMetadata(s.wrapMovieMeta(metaVariant, imageMode))
		case meta.TypeSeries:
			err = mm.FromSeriesMetadata(s.wrapSeriesMeta(metaVariant, imageMode))
		default: // the metadata instance is breaking its contract, just force it to be generic
			err = mm.FromMetadata(s.wrapMeta(metaVariant, imageMode))
		}
	default:
		err = mm.FromMetadata(s.wrapMeta(metaVariant, imageMode))
	}

	return mm, err
}

func (s *Server) wrapMovieMeta(m meta.MovieOrSeriesMetadata, imageMode v1.ImageMode) v1.MovieMetadata {
	return v1.MovieMetadata{
		Title:         m.Title(),
		OriginalTitle: makeOptString(m.OriginalTitle()),
		Overview:      makeOptString(m.Overview()),
		ReleaseDate:   m.ReleaseDate(),
		VoteRating:    m.VoteRating(),
		Images:        s.wrapImages(m.Images(), imageMode),
		Genres:        m.Genres(),
		Cast:          s.wrapCastMembers(m.Cast(), imageMode),
		Languages:     s.wrapLanguages(m.Languages()),
		Countries:     s.wrapCountries(m.Countries()),
	}
}

func (s *Server) wrapImages(ims []meta.Image, imageMode v1.ImageMode) []v1.Image {
	if imageMode == v1.None {
		return nil
	}

	var images []v1.Image
	for _, i := range ims {
		type_ := i.Type()
		if imageMode == v1.Basic && type_ != meta.ImageTypeBackdrop && type_ != meta.ImageTypePoster {
			continue // basic only sends backdrops and posters
		}

		im, err := s.wrapImage(i)
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

func (s *Server) wrapImage(i meta.Image) (v1.Image, error) {
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
		Description: makeOptString(i.Description()),
	}, nil
}

func (s *Server) wrapCastMembers(cms []meta.CastMember, imageMode v1.ImageMode) []v1.CastMember {
	if cms == nil {
		return nil
	}

	castMembers := make([]v1.CastMember, len(cms))
	for i, cm := range cms {
		var (
			image  = cm.Image()
			image0 *v1.Image
		)
		if image != nil && imageMode == v1.All {
			im, err := s.wrapImage(image)
			if err != nil {
				s.logger.Error(
					"failed to make cast member image, skipping",
					zap.String("path", image.Path()),
					zap.Bool("remote", image.Remote()),
					zap.String("description", image.Description()),
					zap.Error(err),
				)
			} else {
				image0 = &im
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

func (s *Server) wrapLanguages(ts []language.Tag) []string {
	if ts == nil {
		return nil
	}

	tags := make([]string, len(ts))
	for i, tag := range ts {
		tags[i] = tag.String()
	}

	return tags
}

func (s *Server) wrapCountries(rgs []language.Region) []string {
	if rgs == nil {
		return nil
	}

	regions := make([]string, len(rgs))
	for i, region := range rgs {
		regions[i] = region.String()
	}

	return regions
}

func (s *Server) wrapSeriesMeta(m meta.MovieOrSeriesMetadata, imageMode v1.ImageMode) v1.SeriesMetadata {
	return v1.SeriesMetadata{
		Title:         m.Title(),
		OriginalTitle: makeOptString(m.OriginalTitle()),
		Overview:      makeOptString(m.Overview()),
		ReleaseDate:   m.ReleaseDate(),
		VoteRating:    m.VoteRating(),
		Images:        s.wrapImages(m.Images(), imageMode),
		Genres:        m.Genres(),
		Cast:          s.wrapCastMembers(m.Cast(), imageMode),
		Languages:     s.wrapLanguages(m.Languages()),
		Countries:     s.wrapCountries(m.Countries()),
	}
}

func (s *Server) wrapEpisodeMeta(m meta.EpisodeMetadata, imageMode v1.ImageMode) v1.EpisodeMetadata {
	return v1.EpisodeMetadata{
		Title:         m.Title(),
		OriginalTitle: makeOptString(m.OriginalTitle()),
		Overview:      makeOptString(m.Overview()),
		ReleaseDate:   m.ReleaseDate(),
		VoteRating:    m.VoteRating(),
		Images:        s.wrapImages(m.Images(), imageMode),
		Series:        s.wrapSeriesMeta(m.Series(), imageMode),
		Season:        m.Season(),
		Episode:       m.Episode(),
	}
}

func (s *Server) wrapMeta(m meta.Metadata, imageMode v1.ImageMode) v1.Metadata {
	return v1.Metadata{
		Title:         m.Title(),
		OriginalTitle: makeOptString(m.OriginalTitle()),
		Overview:      makeOptString(m.Overview()),
		ReleaseDate:   m.ReleaseDate(),
		VoteRating:    m.VoteRating(),
		Images:        s.wrapImages(m.Images(), imageMode),
	}
}
