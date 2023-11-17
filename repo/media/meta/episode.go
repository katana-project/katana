package meta

// EpisodeMetadata is a metadata object of a series' episode.
type EpisodeMetadata interface {
	Metadata

	// Series returns the underlying series' metadata.
	Series() MovieOrSeriesMetadata
	// Season returns the season number, such as 1.
	Season() int
	// Episode returns the episode number, such as 1.
	Episode() int
}

// BasicEpisodeMetadata is an EpisodeMetadata implementation with defined values.
type BasicEpisodeMetadata struct {
	*BasicMetadata

	Series_  *BasicMovieOrSeriesMetadata `json:"series"`
	Season_  int                         `json:"season"`
	Episode_ int                         `json:"episode"`
}

// NewEpisodeMetadata creates an EpisodeMetadata with set values.
func NewEpisodeMetadata(m Metadata, series MovieOrSeriesMetadata, season, episode int) EpisodeMetadata {
	return &BasicEpisodeMetadata{
		BasicMetadata: NewBasicMetadata(m),
		Series_:       NewBasicMovieOrSeriesMetadata(series),
		Season_:       season,
		Episode_:      episode,
	}
}

// NewBasicEpisodeMetadata wraps a EpisodeMetadata object into BasicEpisodeMetadata.
func NewBasicEpisodeMetadata(em EpisodeMetadata) *BasicEpisodeMetadata {
	if em == nil {
		return nil
	}
	if bem, ok := em.(*BasicEpisodeMetadata); ok {
		return bem
	}

	return &BasicEpisodeMetadata{
		BasicMetadata: NewBasicMetadata(em),
		Series_:       NewBasicMovieOrSeriesMetadata(em.Series()),
		Season_:       em.Season(),
		Episode_:      em.Episode(),
	}
}

func (bem *BasicEpisodeMetadata) Series() MovieOrSeriesMetadata {
	return bem.Series_
}
func (bem *BasicEpisodeMetadata) Season() int {
	return bem.Season_
}
func (bem *BasicEpisodeMetadata) Episode() int {
	return bem.Episode_
}
