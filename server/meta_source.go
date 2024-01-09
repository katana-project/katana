package server

import (
	"fmt"
	"github.com/erni27/imcache"
	"github.com/katana-project/katana/internal/errors"
	"github.com/katana-project/katana/repo/media/meta"
	"github.com/katana-project/katana/repo/media/meta/tmdb"
	tmdbClient "github.com/katana-project/tmdb"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/text/language"
	"reflect"
	"time"
)

var (
	// tmdbDefaultCacheExp is the default API response cache expiration.
	tmdbDefaultCacheExp = imcache.WithExpiration(5 * time.Minute)
)

// tmdbSourceOptions are the configuration options of the TMDB metadata source.
type tmdbSourceOptions struct {
	// Key is the TMDB API key.
	Key string `mapstructure:"key"`
	// URL is the base URL of the TMDB API, **must not include a version suffix**, defaults to "https://api.themoviedb.org/".
	URL string `mapstructure:"url"`
	// Lang is the preferred language of the API query results, in a BCP 47 format, defaults to "en-US".
	Lang string `mapstructure:"lang"`
	// CacheExp is the API response cache expiration duration in seconds, defaults to 5 minutes (60*5).
	CacheExp int `mapstructure:"cache_exp"`
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
			url = tmdbClient.DefaultServerBaseURL
		}

		client, err := tmdbClient.NewClientWithResponses(url, tmdbClient.WithToken(parsedOpts.Key))
		if err != nil {
			return nil, errors.Wrap(err, "failed to create tmdb api client")
		}

		langStr := parsedOpts.Lang
		if langStr == "" { // zero value
			langStr = "en-US"
		}

		lang, err := language.Parse(langStr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse tmdb api language preference")
		}

		var (
			cacheExp     = tmdbDefaultCacheExp
			cacheExpTime = time.Duration(parsedOpts.CacheExp) * time.Second
		)
		if cacheExpTime > 0 {
			cacheExp = imcache.WithExpiration(cacheExpTime)
		}

		return tmdb.NewSource(client, lang, cacheExp), nil
	case "analysis":
		metaSources := make([]meta.Source, 0, len(options))
		for sourceName, sourceOptions0 := range options {
			sourceOptions, ok := sourceOptions0.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf(
					"failed to parse metadata sub-source %s options, expected map[string]interface{}, got %s",
					sourceName, reflect.TypeOf(sourceOptions0).String(),
				)
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

	return nil, fmt.Errorf("unknown metadata source %s", name)
}
