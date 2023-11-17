package tmdb

import (
	"github.com/katana-project/katana/repo/media/meta"
	"github.com/katana-project/tmdb"
	"time"
)

type episodeMetadata struct {
	*seriesMetadata

	data *tmdb.TvEpisodeDetailsOK
}

func (em *episodeMetadata) Type() meta.Type {
	return meta.TypeEpisode
}
func (em *episodeMetadata) Title() string {
	return em.data.GetName().Or("")
}
func (em *episodeMetadata) OriginalTitle() string {
	return em.data.GetName().Or("") // no original title
}
func (em *episodeMetadata) Overview() string {
	return em.data.GetOverview().Or("")
}
func (em *episodeMetadata) ReleaseDate() time.Time {
	if releaseDate, ok := em.data.GetAirDate().Get(); ok {
		if parsedTime, err := time.Parse(time.DateOnly, releaseDate); err == nil {
			return parsedTime
		}
	}

	return invalidTime
}
func (em *episodeMetadata) VoteRating() float64 {
	return em.data.GetVoteAverage().Or(10)
}
func (em *episodeMetadata) Images() []meta.Image {
	if em.imageConfig == nil {
		return nil
	}

	var images []meta.Image
	if stillPath, ok := em.data.GetStillPath().Get(); ok {
		images = append(images, &image{
			path:        stillPath,
			description: "Still",
			config:      em.imageConfig,
		})
	}

	return images
}
func (em *episodeMetadata) Series() meta.MovieOrSeriesMetadata {
	return em.seriesMetadata
}
func (em *episodeMetadata) Season() int {
	return em.data.GetSeasonNumber().Or(-1)
}
func (em *episodeMetadata) Episode() int {
	return em.data.GetEpisodeNumber().Or(-1)
}
