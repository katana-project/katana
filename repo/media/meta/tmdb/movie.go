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
	data        *tmdb.MovieDetailsOK
	credits     *tmdb.MovieCreditsOK
	imageConfig *tmdb.ConfigurationDetailsOKImages
}

func (mm *movieMetadata) Type() meta.Type {
	return meta.TypeMovie
}
func (mm *movieMetadata) Title() string {
	return mm.data.GetTitle().Or("")
}
func (mm *movieMetadata) OriginalTitle() string {
	return mm.data.GetOriginalTitle().Or("")
}
func (mm *movieMetadata) Overview() string {
	return mm.data.GetOverview().Or("")
}
func (mm *movieMetadata) ReleaseDate() time.Time {
	if releaseDate, ok := mm.data.GetReleaseDate().Get(); ok {
		if parsedTime, err := time.Parse(time.DateOnly, releaseDate); err == nil {
			return parsedTime
		}
	}

	return invalidTime
}
func (mm *movieMetadata) VoteRating() float64 {
	return mm.data.GetVoteAverage().Or(10)
}
func (mm *movieMetadata) Genres() []string {
	var genreNames []string
	for _, genre := range mm.data.GetGenres() {
		if name, ok := genre.GetName().Get(); ok {
			genreNames = append(genreNames, name)
		}
	}

	return genreNames
}
func (mm *movieMetadata) Cast() []meta.CastMember {
	if mm.credits == nil {
		return nil
	}

	var (
		cast        = mm.credits.GetCast()
		castMembers = make([]meta.CastMember, len(cast))
	)
	for i, member := range cast {
		member0 := member // this needs to be here, a pointer to member ends up at the last value = nasty issues
		castMembers[i] = &movieCastMember{
			data:        &member0,
			imageConfig: mm.imageConfig,
		}
	}

	return castMembers
}
func (mm *movieMetadata) Languages() []language.Tag {
	var languageTags []language.Tag
	for _, lang := range mm.data.GetSpokenLanguages() {
		if code, ok := lang.GetIso6391().Get(); ok {
			if tag, err := language.Parse(code); err == nil {
				languageTags = append(languageTags, tag)
			}
		}
	}

	return languageTags
}
func (mm *movieMetadata) Countries() []language.Region {
	var regions []language.Region
	for _, country := range mm.data.GetProductionCountries() {
		if code, ok := country.GetIso31661().Get(); ok {
			if region, err := language.ParseRegion(code); err == nil {
				regions = append(regions, region)
			}
		}
	}

	return regions
}
func (mm *movieMetadata) Images() []meta.Image {
	if mm.imageConfig == nil {
		return nil
	}

	var images []meta.Image
	if backdropPath, ok := mm.data.GetBackdropPath().Get(); ok {
		images = append(images, &image{
			path:        backdropPath,
			description: "Backdrop",
			config:      mm.imageConfig,
		})
	}
	if posterPath, ok := mm.data.GetPosterPath().Get(); ok {
		images = append(images, &image{
			path:        posterPath,
			description: "Poster",
			config:      mm.imageConfig,
		})
	}

	return images
}

type movieCastMember struct {
	data        *tmdb.MovieCreditsOKCastItem
	imageConfig *tmdb.ConfigurationDetailsOKImages
}

func (mcm *movieCastMember) Name() string {
	return mcm.data.GetName().Or("")
}
func (mcm *movieCastMember) Role() string {
	return mcm.data.GetCharacter().Or("")
}
func (mcm *movieCastMember) Image() meta.Image {
	if mcm.imageConfig == nil {
		return nil
	}

	if path, ok := mcm.data.GetProfilePath().Get(); ok {
		return &image{
			path:        path,
			description: mcm.data.GetName().Or(""),
			config:      mcm.imageConfig,
		}
	}

	return nil
}
