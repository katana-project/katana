package tmdb

import (
	"github.com/katana-project/katana/repo/media/meta"
	"github.com/katana-project/tmdb"
	"golang.org/x/text/language"
	"time"
)

type seriesMetadata struct {
	data        *tmdb.TvSeriesDetailsOK
	credits     *tmdb.TvSeriesCreditsOK
	imageConfig *tmdb.ConfigurationDetailsOKImages
}

func (sm *seriesMetadata) Type() meta.Type {
	return meta.TypeSeries
}
func (sm *seriesMetadata) Title() string {
	return sm.data.GetName().Or("")
}
func (sm *seriesMetadata) OriginalTitle() string {
	return sm.data.GetOriginalName().Or("")
}
func (sm *seriesMetadata) Overview() string {
	return sm.data.GetOverview().Or("")
}
func (sm *seriesMetadata) ReleaseDate() time.Time {
	if releaseDate, ok := sm.data.GetFirstAirDate().Get(); ok {
		if parsedTime, err := time.Parse(time.DateOnly, releaseDate); err == nil {
			return parsedTime
		}
	}

	return invalidTime
}
func (sm *seriesMetadata) VoteRating() float64 {
	return sm.data.GetVoteAverage().Or(10)
}
func (sm *seriesMetadata) Genres() []string {
	var genreNames []string
	for _, genre := range sm.data.GetGenres() {
		if name, ok := genre.GetName().Get(); ok {
			genreNames = append(genreNames, name)
		}
	}

	return genreNames
}
func (sm *seriesMetadata) Cast() []meta.CastMember {
	if sm.credits == nil {
		return nil
	}

	var (
		cast        = sm.credits.GetCast()
		castMembers = make([]meta.CastMember, len(cast))
	)
	for i, member := range cast {
		member0 := member // this needs to be here, a pointer to member ends up at the last value = nasty issues
		castMembers[i] = &seriesCastMember{
			data:        &member0,
			imageConfig: sm.imageConfig,
		}
	}

	return castMembers
}
func (sm *seriesMetadata) Languages() []language.Tag {
	var languageTags []language.Tag
	for _, lang := range sm.data.GetSpokenLanguages() {
		if code, ok := lang.GetIso6391().Get(); ok {
			if tag, err := language.Parse(code); err == nil {
				languageTags = append(languageTags, tag)
			}
		}
	}

	return languageTags
}
func (sm *seriesMetadata) Countries() []language.Region {
	var regions []language.Region
	for _, country := range sm.data.GetProductionCountries() {
		if code, ok := country.GetIso31661().Get(); ok {
			if region, err := language.ParseRegion(code); err == nil {
				regions = append(regions, region)
			}
		}
	}

	return regions
}
func (sm *seriesMetadata) Images() []meta.Image {
	if sm.imageConfig == nil {
		return nil
	}

	var images []meta.Image
	if backdropPath, ok := sm.data.GetBackdropPath().Get(); ok {
		images = append(images, &image{
			path:        backdropPath,
			description: "Backdrop",
			config:      sm.imageConfig,
		})
	}
	if posterPath, ok := sm.data.GetPosterPath().Get(); ok {
		images = append(images, &image{
			path:        posterPath,
			description: "Poster",
			config:      sm.imageConfig,
		})
	}

	return images
}

type seriesCastMember struct {
	data        *tmdb.TvSeriesCreditsOKCastItem
	imageConfig *tmdb.ConfigurationDetailsOKImages
}

func (scm *seriesCastMember) Name() string {
	return scm.data.GetName().Or("")
}
func (scm *seriesCastMember) Role() string {
	return scm.data.GetCharacter().Or("")
}
func (scm *seriesCastMember) Image() meta.Image {
	if scm.imageConfig == nil {
		return nil
	}

	if path, ok := scm.data.GetProfilePath().Get(); ok {
		return &image{
			path:        path,
			description: scm.data.GetName().Or(""),
			config:      scm.imageConfig,
		}
	}

	return nil
}
