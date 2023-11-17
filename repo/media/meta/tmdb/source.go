package tmdb

import (
	"context"
	"github.com/go-faster/errors"
	"github.com/katana-project/katana/repo/media/meta"
	"github.com/katana-project/tmdb"
	"github.com/ogen-go/ogen/validate"
	"golang.org/x/text/language"
	"path/filepath"
	"strings"
)

type source struct {
	client  tmdb.Invoker
	lang    string
	credits bool
}

// NewSource creates a metadata source that resolves queries using The Movie Database's API.
func NewSource(client tmdb.Invoker, lang language.Tag, resolveCast bool) meta.Source {
	return &source{client: client, lang: lang.String(), credits: resolveCast}
}

// FromFile tries to resolve the file name as a query.
func (s *source) FromFile(path string) (meta.Metadata, error) {
	fileName := filepath.Base(path)
	nameWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	return s.FromQuery(meta.NewQuery(nameWithoutExt, meta.TypeUnknown, 0, 0))
}

// FromQuery tries to resolve the query using The Movie Database's API.
func (s *source) FromQuery(query meta.Query) (meta.Metadata, error) {
	switch query.Type() {
	case meta.TypeMovie:
		return s.searchMovie(query.Query())
	case meta.TypeSeries, meta.TypeEpisode:
		return s.searchSeries(query.Query(), query.Season(), query.Episode())
	}

	return s.searchMulti(query.Query())
}

func (s *source) fetchConfiguration() (*tmdb.ConfigurationDetailsOK, error) {
	config, err := s.client.ConfigurationDetails(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch configuration")
	}

	return config, nil
}

func (s *source) searchMulti(query string) (meta.Metadata, error) {
	res, err := s.client.SearchMulti(context.Background(), tmdb.SearchMultiParams{
		Query:    query,
		Language: tmdb.NewOptString(s.lang),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to search multi")
	}

	for _, result := range res.GetResults() {
		if id, ok := result.GetID().Get(); ok {
			if resultType, ok := result.GetMediaType().Get(); ok {
				switch resultType {
				case "movie":
					return s.fetchMovie(id)
				case "tv":
					return s.fetchSeries(id, -1, -1)
				}
			}
		}
	}

	return nil, nil
}

func (s *source) searchMovie(query string) (meta.Metadata, error) {
	res, err := s.client.SearchMovie(context.Background(), tmdb.SearchMovieParams{
		Query:    query,
		Language: tmdb.NewOptString(s.lang),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to search movie")
	}

	for _, result := range res.GetResults() {
		if id, ok := result.GetID().Get(); ok {
			return s.fetchMovie(id)
		}
	}

	return nil, nil
}

func (s *source) searchSeries(query string, season, episode int) (meta.Metadata, error) {
	res, err := s.client.SearchTv(context.Background(), tmdb.SearchTvParams{
		Query:    query,
		Language: tmdb.NewOptString(s.lang),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to search series")
	}

	for _, result := range res.GetResults() {
		if id, ok := result.GetID().Get(); ok {
			return s.fetchSeries(id, season, episode)
		}
	}

	return nil, nil
}

func (s *source) fetchMovie(id int) (meta.Metadata, error) {
	res, err := s.client.MovieDetails(context.Background(), tmdb.MovieDetailsParams{
		MovieID:  int32(id),
		Language: tmdb.NewOptString(s.lang),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch movie details")
	}

	var resCredits *tmdb.MovieCreditsOK
	if s.credits {
		resCredits, err = s.client.MovieCredits(context.Background(), tmdb.MovieCreditsParams{
			MovieID:  int32(id),
			Language: tmdb.NewOptString(s.lang),
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch movie credits")
		}
	}

	config, err := s.fetchConfiguration()
	if err != nil {
		return nil, err
	}

	var imageConfig *tmdb.ConfigurationDetailsOKImages
	if ic, ok := config.GetImages().Get(); ok {
		imageConfig = &ic
	}

	return &movieMetadata{
		data:        res,
		credits:     resCredits,
		imageConfig: imageConfig,
	}, nil
}

func (s *source) fetchSeries(id, season, episode int) (meta.Metadata, error) {
	res, err := s.client.TvSeriesDetails(context.Background(), tmdb.TvSeriesDetailsParams{
		SeriesID: int32(id),
		Language: tmdb.NewOptString(s.lang),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch series details")
	}

	var resCredits *tmdb.TvSeriesCreditsOK
	if s.credits {
		resCredits, err = s.client.TvSeriesCredits(context.Background(), tmdb.TvSeriesCreditsParams{
			SeriesID: int32(id),
			Language: tmdb.NewOptString(s.lang),
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch series credits")
		}
	}

	config, err := s.fetchConfiguration()
	if err != nil {
		return nil, err
	}

	var imageConfig *tmdb.ConfigurationDetailsOKImages
	if ic, ok := config.GetImages().Get(); ok {
		imageConfig = &ic
	}

	seriesMeta := &seriesMetadata{
		data:        res,
		credits:     resCredits,
		imageConfig: imageConfig,
	}
	if season >= 0 && episode >= 0 {
		episodeRes, err := s.client.TvEpisodeDetails(context.Background(), tmdb.TvEpisodeDetailsParams{
			SeriesID:      int32(id),
			SeasonNumber:  int32(season),
			EpisodeNumber: int32(episode),
			Language:      tmdb.NewOptString(s.lang),
		})
		if err != nil {
			var usce *validate.UnexpectedStatusCodeError
			if errors.As(err, &usce) && usce.StatusCode == 404 {
				return nil, nil // episode-season combination not found
			}

			return nil, errors.Wrap(err, "failed to fetch episode details")
		}

		return &episodeMetadata{seriesMetadata: seriesMeta, data: episodeRes}, nil
	}

	return seriesMeta, nil
}
