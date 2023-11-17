package meta

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	dummySource0   = &dummySource{}
	literalSource0 = &literalSource{}

	episodePattern          = regexp.MustCompile("(?i)S(\\d+) ?[EX](\\d+)")
	encodingPattern         = regexp.MustCompile("(?i)[Hx]\\.?26\\d|HEVC|MPEG(?:-\\d)?|DivX|VP\\d|AV\\d")
	resolutionPattern       = regexp.MustCompile("(?i)(2160|1440|1080|720|480|360|240|144)([pi])")
	commonDelimiterReplacer = strings.NewReplacer("-", " ", "_", " ", ".", " ")
)

// Source is a source of Metadata.
type Source interface {
	// FromFile tries to resolve metadata for a media file, may return nil.
	FromFile(path string) (Metadata, error)
	// FromQuery tries to resolve metadata for a custom query, may return nil.
	FromQuery(query Query) (Metadata, error)
}

// Query is a search query for a movie or a series episode.
type Query interface {
	// Query returns the string used for searching the movie or series.
	Query() string
	// Type returns the type of metadata to search for, 0 (TypeUnknown) searches for all media.
	Type() Type
	// Season returns the season number, 0 means don't search for a specific episode.
	Season() int
	// Episode returns the episode number in the season, 0 means don't search for a specific episode.
	Episode() int
}

// BasicQuery is a JSON serializable Query, with set values.
type BasicQuery struct {
	Query_   string `json:"query"`
	Type_    Type   `json:"type"`
	Season_  int    `json:"season"`
	Episode_ int    `json:"episode"`
}

// NewQuery creates a Query with set values.
func NewQuery(query string, type_ Type, season, episode int) Query {
	return &BasicQuery{
		Query_:   query,
		Type_:    type_,
		Season_:  season,
		Episode_: episode,
	}
}

// NewBasicQuery wraps a Query object into BasicQuery.
func NewBasicQuery(mq Query) *BasicQuery {
	if mq == nil {
		return nil
	}
	if bmq, ok := mq.(*BasicQuery); ok {
		return bmq
	}

	return &BasicQuery{
		Query_:   mq.Query(),
		Type_:    mq.Type(),
		Season_:  mq.Season(),
		Episode_: mq.Episode(),
	}
}

func (bmq *BasicQuery) Query() string {
	return bmq.Query_
}
func (bmq *BasicQuery) Type() Type {
	return bmq.Type_
}
func (bmq *BasicQuery) Season() int {
	return bmq.Season_
}
func (bmq *BasicQuery) Episode() int {
	return bmq.Episode_
}

// dummySource is a Source that discovers nothing.
type dummySource struct {
}

// NewDummySource creates a Source that discovers nothing.
func NewDummySource() Source {
	return dummySource0
}

// FromFile always returns nil.
func (ds *dummySource) FromFile(_ string) (Metadata, error) {
	return nil, nil
}

// FromQuery always returns nil.
func (ds *dummySource) FromQuery(_ Query) (Metadata, error) {
	return nil, nil
}

// literalSource is a Source that creates rough metadata from queries.
type literalSource struct {
}

// NewLiteralSource creates a Source that creates rough metadata from queries.
func NewLiteralSource() Source {
	return literalSource0
}

// FromFile returns a literal metadata representation of the file name.
func (lms *literalSource) FromFile(path string) (Metadata, error) {
	fileName := filepath.Base(path)
	nameWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	return NewMetadata(TypeUnknown, nameWithoutExt, nameWithoutExt, "", time.Now(), 10, nil), nil
}

// FromQuery returns a literal metadata representation of the query.
func (lms *literalSource) FromQuery(query Query) (Metadata, error) {
	var (
		parentType = query.Type()
		season     = query.Season()
		episode    = query.Episode()
	)

	if season >= 0 && episode >= 0 {
		parentType = TypeSeries
	}

	genericMeta := NewMetadata(parentType, query.Query(), query.Query(), "", time.Now(), 10, nil)
	if season >= 0 && episode >= 0 {
		title := "S" + fmt.Sprintf("%02d", season) + "E" + fmt.Sprintf("%02d", episode)

		return NewEpisodeMetadata(
			NewMetadata(TypeEpisode, title, title, "", genericMeta.ReleaseDate(), 10, nil),
			NewMovieOrSeriesMetadata(genericMeta, nil, nil, nil, nil),
			season,
			episode,
		), nil
	}

	return genericMeta, nil
}

// CompositeSource is a Source that tries to resolve metadata from multiple sources.
type CompositeSource struct {
	// Sources are the sources to be resolved from, iterated in order.
	Sources []Source
}

// NewCompositeSource creates a metadata source that resolves results from multiple sources.
func NewCompositeSource(metaSources ...Source) Source {
	return &CompositeSource{Sources: metaSources}
}

// FromFile tries to resolve metadata for a media file from multiple sources, may return nil.
func (cs *CompositeSource) FromFile(path string) (Metadata, error) {
	for _, source := range cs.Sources {
		m, err := source.FromFile(path)
		if err != nil {
			return nil, err
		}
		if m != nil {
			return m, nil
		}
	}

	return nil, nil
}

// FromQuery tries to resolve metadata for a custom query from multiple sources, may return nil.
func (cs *CompositeSource) FromQuery(query Query) (Metadata, error) {
	for _, source := range cs.Sources {
		m, err := source.FromQuery(query)
		if err != nil {
			return nil, err
		}
		if m != nil {
			return m, nil
		}
	}

	return nil, nil
}

// fileAnalysisSource is a Source that tries to analyze file names.
type fileAnalysisSource struct {
	Source // FromQuery delegate
}

// NewFileAnalysisSource creates a metadata source that analyzes files, creates a query and delegates the query resolving to metaSource.
func NewFileAnalysisSource(metaSource Source) Source {
	return &fileAnalysisSource{Source: metaSource}
}

// FromFile tries to create a metadata query from a file and resolve it using FromQuery.
func (fas *fileAnalysisSource) FromFile(path string) (Metadata, error) {
	fileName := filepath.Base(path)
	nameWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	var (
		type_      = TypeUnknown
		season     = -1
		episode    = -1
		dirtyQuery = nameWithoutExt
	)

	episodeGroups := episodePattern.FindStringSubmatch(dirtyQuery)
	if episodeGroups != nil { // presume episode
		type_ = TypeEpisode
		season, _ = strconv.Atoi(strings.TrimLeft(episodeGroups[1], "0"))  // will never error
		episode, _ = strconv.Atoi(strings.TrimLeft(episodeGroups[2], "0")) // will never error
		dirtyQuery = nameWithoutExt[:strings.Index(dirtyQuery, episodeGroups[0])]
	} else {
		dirtyQuery = resolutionPattern.ReplaceAllLiteralString(nameWithoutExt, "") // step 1: remove resolution
		dirtyQuery = encodingPattern.ReplaceAllLiteralString(dirtyQuery, "")       // step 2: remove encoding format/codec
		dirtyQuery = stripBracketLike(dirtyQuery)
	}

	return fas.FromQuery(NewQuery(strings.Join(strings.Fields(commonDelimiterReplacer.Replace(dirtyQuery)), " "), type_, season, episode))
}

func stripBracketLike(s string) string {
	var (
		b          strings.Builder
		scrubUntil = '\000'
	)

	for _, char := range []rune(s) {
		switch char {
		case '(':
			scrubUntil = ')'
		case '[':
			scrubUntil = ']'
		case '{':
			scrubUntil = '}'
		case scrubUntil:
			scrubUntil = '\000'
			continue
		}

		if scrubUntil == '\000' {
			b.WriteRune(char)
		}
	}

	return b.String()
}
