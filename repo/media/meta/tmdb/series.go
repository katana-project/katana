package tmdb

import (
	"github.com/katana-project/katana/repo/media/meta"
	"github.com/katana-project/tmdb"
	"golang.org/x/text/language"
	"time"
)

type seriesMetadata struct {
	data    *tmdb.TvSeriesDetailsResponse
	credits *tmdb.TvSeriesCreditsResponse
	config  *tmdb.ConfigurationDetailsResponse
}

func (sm *seriesMetadata) Type() meta.Type {
	return meta.TypeSeries
}
func (sm *seriesMetadata) Title() string {
	return *sm.data.JSON200.Name
}
func (sm *seriesMetadata) OriginalTitle() string {
	return *sm.data.JSON200.OriginalName
}
func (sm *seriesMetadata) Overview() string {
	return *sm.data.JSON200.Overview
}
func (sm *seriesMetadata) ReleaseDate() time.Time {
	if parsedTime, err := time.Parse(time.DateOnly, *sm.data.JSON200.FirstAirDate); err == nil {
		return parsedTime
	}

	return invalidTime
}
func (sm *seriesMetadata) VoteRating() float32 {
	return *sm.data.JSON200.VoteAverage
}
func (sm *seriesMetadata) Genres() []string {
	var (
		genres     = *sm.data.JSON200.Genres
		genreNames = make([]string, len(genres))
	)
	for i, genre := range genres {
		genreNames[i] = *genre.Name
	}

	return genreNames
}
func (sm *seriesMetadata) Cast() []meta.CastMember {
	if sm.credits == nil {
		return nil
	}

	imageUrl := sm.config.JSON200.Images.SecureBaseUrl
	if imageUrl == nil {
		imageUrl = sm.config.JSON200.Images.BaseUrl
	}

	var (
		cast        = *sm.credits.JSON200.Cast
		castMembers = make([]meta.CastMember, len(cast))
	)
	for i, member := range cast {
		var img *image
		if member.ProfilePath != nil {
			img = &image{
				type_:   meta.ImageTypeAvatar,
				path:    *member.ProfilePath,
				desc:    *member.Name,
				baseUrl: *imageUrl,
			}
		}

		castMembers[i] = &castMember{
			name: *member.Name,
			role: *member.Character,
			img:  img,
		}
	}

	return castMembers
}
func (sm *seriesMetadata) Languages() []language.Tag {
	var (
		languages    = *sm.data.JSON200.SpokenLanguages
		languageTags = make([]language.Tag, 0, len(languages))
	)
	for _, lang := range languages {
		if tag, err := language.Parse(*lang.Iso6391); err == nil {
			languageTags = append(languageTags, tag)
		}
	}

	return languageTags
}
func (sm *seriesMetadata) Countries() []language.Region {
	var (
		countries = *sm.data.JSON200.ProductionCountries
		regions   = make([]language.Region, 0, len(countries))
	)
	for _, country := range countries {
		if region, err := language.ParseRegion(*country.Iso31661); err == nil {
			regions = append(regions, region)
		}
	}

	return regions
}
func (sm *seriesMetadata) Images() []meta.Image {
	if sm.config == nil {
		return nil
	}

	url := sm.config.JSON200.Images.SecureBaseUrl
	if url == nil {
		url = sm.config.JSON200.Images.BaseUrl
	}

	var images []meta.Image
	if sm.data.JSON200.BackdropPath != nil {
		images = append(images, &image{
			type_:   meta.ImageTypeBackdrop,
			path:    *sm.data.JSON200.BackdropPath,
			desc:    "Backdrop",
			baseUrl: *url,
		})
	}
	if sm.data.JSON200.PosterPath != nil {
		images = append(images, &image{
			type_:   meta.ImageTypePoster,
			path:    *sm.data.JSON200.PosterPath,
			desc:    "Poster",
			baseUrl: *url,
		})
	}

	return images
}
