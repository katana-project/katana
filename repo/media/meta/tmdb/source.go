package tmdb

import (
	"context"
	"fmt"
	"github.com/erni27/imcache"
	"github.com/katana-project/katana/internal/errors"
	"github.com/katana-project/katana/repo/media/meta"
	"github.com/katana-project/tmdb"
	"golang.org/x/text/language"
	"path/filepath"
	"strings"
)

type source struct {
	client tmdb.ClientWithResponsesInterface
	lang   string
	exp    imcache.Expiration

	config           *tmdb.ConfigurationDetailsResponse // TODO: expire?
	movieSeriesCache imcache.Cache[int, meta.MovieOrSeriesMetadata]
	episodeCache     imcache.Cache[episodeKey, meta.EpisodeMetadata]
}

type episodeKey struct {
	id, season, episode int
}

// NewSource creates a metadata source that resolves queries using The Movie Database's API.
func NewSource(client tmdb.ClientWithResponsesInterface, lang language.Tag, cacheExp imcache.Expiration) meta.Source {
	return &source{client: client, lang: lang.String(), exp: cacheExp}
}

// FromFile tries to resolve the file name as a query.
func (s *source) FromFile(path string) (meta.Metadata, error) {
	var (
		fileName       = filepath.Base(path)
		nameWithoutExt = strings.TrimSuffix(fileName, filepath.Ext(fileName))
	)

	return s.FromQuery(&meta.Query{Query: nameWithoutExt, Type: meta.TypeUnknown, Season: 0, Episode: 0})
}

// FromQuery tries to resolve the query using The Movie Database's API.
func (s *source) FromQuery(query *meta.Query) (meta.Metadata, error) {
	switch query.Type {
	case meta.TypeMovie:
		return s.searchMovie(query.Query)
	case meta.TypeSeries, meta.TypeEpisode:
		return s.searchSeries(query.Query, query.Season, query.Episode)
	}

	return s.searchMulti(query.Query)
}

func (s *source) fetchConfiguration() (*tmdb.ConfigurationDetailsResponse, error) {
	if s.config != nil {
		return s.config, nil
	}

	config, err := s.client.ConfigurationDetailsWithResponse(context.Background())
	if err == nil {
		err = s.checkStatus(config)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch configuration")
	}

	s.config = config
	return config, nil
}

func (s *source) searchMulti(query string) (meta.Metadata, error) {
	res, err := s.client.SearchMultiWithResponse(context.Background(), &tmdb.SearchMultiParams{
		Query:    query,
		Language: &s.lang,
	})
	if err == nil {
		err = s.checkStatus(res)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to search multi")
	}

	for _, result := range *res.JSON200.Results {
		id := *result.Id

		switch *result.MediaType {
		case "movie":
			return s.fetchMovie(id)
		case "tv":
			return s.fetchSeries(id)
		}
	}

	return nil, nil
}

func (s *source) searchMovie(query string) (meta.Metadata, error) {
	res, err := s.client.SearchMovieWithResponse(context.Background(), &tmdb.SearchMovieParams{
		Query:    query,
		Language: &s.lang,
	})
	if err == nil {
		err = s.checkStatus(res)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to search movie")
	}

	results := *res.JSON200.Results
	if len(results) > 0 {
		return s.fetchMovie(*(results[0].Id))
	}

	return nil, nil
}

func (s *source) searchSeries(query string, season, episode int) (meta.Metadata, error) {
	res, err := s.client.SearchTvWithResponse(context.Background(), &tmdb.SearchTvParams{
		Query:    query,
		Language: &s.lang,
	})
	if err == nil {
		err = s.checkStatus(res)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to search series")
	}

	results := *res.JSON200.Results
	if len(results) > 0 {
		id := *(results[0].Id)
		if season >= 0 && episode >= 0 {
			return s.fetchEpisode(id, season, episode)
		}

		return s.fetchSeries(id)
	}

	return nil, nil
}

func (s *source) fetchMovie(id int) (meta.MovieOrSeriesMetadata, error) {
	if m, ok := s.movieSeriesCache.Get(id); ok {
		return m, nil
	}

	res, err := s.client.MovieDetailsWithResponse(context.Background(), int32(id), &tmdb.MovieDetailsParams{Language: &s.lang})
	if err == nil {
		err = s.checkStatus(res)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch movie details")
	}

	resCredits, err := s.client.MovieCreditsWithResponse(context.Background(), int32(id), &tmdb.MovieCreditsParams{Language: &s.lang})
	if err == nil {
		err = s.checkStatus(res)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch movie credits")
	}

	config, err := s.fetchConfiguration()
	if err != nil {
		return nil, err
	}

	m := &movieMetadata{
		data:    res,
		credits: resCredits,
		config:  config,
	}
	s.movieSeriesCache.Set(id, m, s.exp)
	return m, nil
}

func (s *source) fetchSeries(id int) (meta.MovieOrSeriesMetadata, error) {
	if m, ok := s.movieSeriesCache.Get(id); ok {
		return m, nil
	}

	res, err := s.client.TvSeriesDetailsWithResponse(context.Background(), int32(id), &tmdb.TvSeriesDetailsParams{Language: &s.lang})
	if err == nil {
		err = s.checkStatus(res)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch series details")
	}

	resCredits, err := s.client.TvSeriesCreditsWithResponse(context.Background(), int32(id), &tmdb.TvSeriesCreditsParams{Language: &s.lang})
	if err == nil {
		err = s.checkStatus(res)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch series credits")
	}

	config, err := s.fetchConfiguration()
	if err != nil {
		return nil, err
	}

	m := &seriesMetadata{
		data:    res,
		credits: resCredits,
		config:  config,
	}
	s.movieSeriesCache.Set(id, m, s.exp)
	return m, nil
}

func (s *source) fetchEpisode(id, season, episode int) (meta.EpisodeMetadata, error) {
	key := episodeKey{id: id, season: season, episode: episode}
	if m, ok := s.episodeCache.Get(key); ok {
		return m, nil
	}

	seriesMeta, err := s.fetchSeries(id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch series metadata")
	}

	episodeRes, err := s.client.TvEpisodeDetailsWithResponse(context.Background(), int32(id), int32(season), int32(episode), &tmdb.TvEpisodeDetailsParams{Language: &s.lang})
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch episode details")
	}

	if episodeRes.StatusCode() == 404 {
		return nil, nil // episode-season combination not found
	}
	if err := s.checkStatus(episodeRes); err != nil {
		return nil, errors.Wrap(err, "failed to fetch episode details")
	}

	config, err := s.fetchConfiguration()
	if err != nil {
		return nil, err
	}

	m := &episodeMetadata{
		MovieOrSeriesMetadata: seriesMeta,
		data:                  episodeRes,
		config:                config,
	}
	s.episodeCache.Set(key, m, s.exp)
	return m, nil
}

func (s *source) checkStatus(resp tmdb.Response) error {
	code := resp.StatusCode()
	if code < 200 || code > 299 {
		return fmt.Errorf("non-2xx status code %d: %s", code, resp.Status())
	}

	return nil
}
