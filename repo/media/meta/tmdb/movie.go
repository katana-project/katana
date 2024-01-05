package tmdb

import (
	"github.com/katana-project/katana/repo/media/meta"
	"github.com/katana-project/tmdb"
	"golang.org/x/text/language"
	"time"
)

var (
	invalidTime = time.UnixMilli(0)
)

type movieMetadata struct {
	data    *tmdb.MovieDetailsResponse
	credits *tmdb.MovieCreditsResponse
	config  *tmdb.ConfigurationDetailsResponse
}

func (mm *movieMetadata) Type() meta.Type {
	return meta.TypeMovie
}
func (mm *movieMetadata) Title() string {
	return *mm.data.JSON200.Title
}
func (mm *movieMetadata) OriginalTitle() string {
	return *mm.data.JSON200.OriginalTitle
}
func (mm *movieMetadata) Overview() string {
	return *mm.data.JSON200.Overview
}
func (mm *movieMetadata) ReleaseDate() time.Time {
	if parsedTime, err := time.Parse(time.DateOnly, *mm.data.JSON200.ReleaseDate); err == nil {
		return parsedTime
	}

	return invalidTime
}
func (mm *movieMetadata) VoteRating() float32 {
	return *mm.data.JSON200.VoteAverage
}
func (mm *movieMetadata) Genres() []string {
	var (
		genres     = *mm.data.JSON200.Genres
		genreNames = make([]string, 0, len(genres))
	)
	for _, genre := range genres {
		genreNames = append(genreNames, *genre.Name)
	}

	return genreNames
}
func (mm *movieMetadata) Cast() []meta.CastMember {
	if mm.credits == nil {
		return nil
	}

	imageUrl := mm.config.JSON200.Images.SecureBaseUrl
	if imageUrl == nil {
		imageUrl = mm.config.JSON200.Images.BaseUrl
	}

	var (
		cast        = *mm.credits.JSON200.Cast
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
func (mm *movieMetadata) Languages() []language.Tag {
	var (
		languages    = *mm.data.JSON200.SpokenLanguages
		languageTags = make([]language.Tag, 0, len(languages))
	)
	for _, lang := range languages {
		if tag, err := language.Parse(*lang.Iso6391); err == nil {
			languageTags = append(languageTags, tag)
		}
	}

	return languageTags
}
func (mm *movieMetadata) Countries() []language.Region {
	var (
		countries = *mm.data.JSON200.ProductionCountries
		regions   = make([]language.Region, 0, len(countries))
	)
	for _, country := range countries {
		if region, err := language.ParseRegion(*country.Iso31661); err == nil {
			regions = append(regions, region)
		}
	}

	return regions
}
func (mm *movieMetadata) Images() []meta.Image {
	if mm.config == nil {
		return nil
	}

	url := mm.config.JSON200.Images.SecureBaseUrl
	if url == nil {
		url = mm.config.JSON200.Images.BaseUrl
	}

	var images []meta.Image
	if mm.data.JSON200.BackdropPath != nil {
		images = append(images, &image{
			type_:   meta.ImageTypeBackdrop,
			path:    *mm.data.JSON200.BackdropPath,
			desc:    "Backdrop",
			baseUrl: *url,
		})
	}
	if mm.data.JSON200.PosterPath != nil {
		images = append(images, &image{
			type_:   meta.ImageTypePoster,
			path:    *mm.data.JSON200.PosterPath,
			desc:    "Poster",
			baseUrl: *url,
		})
	}

	return images
}
