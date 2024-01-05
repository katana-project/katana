package tmdb

import (
	"github.com/katana-project/katana/repo/media/meta"
	"github.com/katana-project/tmdb"
	"time"
)

type episodeMetadata struct {
	meta.MovieOrSeriesMetadata

	data   *tmdb.TvEpisodeDetailsResponse
	config *tmdb.ConfigurationDetailsResponse
}

func (em *episodeMetadata) Type() meta.Type {
	return meta.TypeEpisode
}
func (em *episodeMetadata) Title() string {
	return *em.data.JSON200.Name
}
func (em *episodeMetadata) OriginalTitle() string {
	return *em.data.JSON200.Name // no original title
}
func (em *episodeMetadata) Overview() string {
	return *em.data.JSON200.Overview
}
func (em *episodeMetadata) ReleaseDate() time.Time {
	if parsedTime, err := time.Parse(time.DateOnly, *em.data.JSON200.AirDate); err == nil {
		return parsedTime
	}

	return invalidTime
}
func (em *episodeMetadata) VoteRating() float32 {
	return *em.data.JSON200.VoteAverage
}
func (em *episodeMetadata) Images() []meta.Image {
	if em.config == nil {
		return nil
	}

	url := em.config.JSON200.Images.SecureBaseUrl
	if url == nil {
		url = em.config.JSON200.Images.BaseUrl
	}

	var images []meta.Image
	if em.data.JSON200.StillPath != nil {
		images = append(images, &image{
			type_:   meta.ImageTypeStill,
			path:    *em.data.JSON200.StillPath,
			desc:    "Still",
			baseUrl: *url,
		})
	}

	return images
}
func (em *episodeMetadata) Series() meta.MovieOrSeriesMetadata {
	return em.MovieOrSeriesMetadata
}
func (em *episodeMetadata) Season() int {
	return *em.data.JSON200.SeasonNumber
}
func (em *episodeMetadata) Episode() int {
	return *em.data.JSON200.EpisodeNumber
}
