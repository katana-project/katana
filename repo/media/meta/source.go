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
	FromQuery(query *Query) (Metadata, error)
}

// Query is a search query for a movie or a series episode.
type Query struct {
	// Query is the string used for searching the movie or series.
	Query string `json:"query"`
	// Type is the type of metadata to search for, 0 (TypeUnknown) searches for all media.
	Type Type `json:"type"`
	// Season is the season number, 0 means don't search for a specific episode.
	Season int `json:"season"`
	// Episode is the episode number in the season, 0 means don't search for a specific episode.
	Episode int `json:"episode"`
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
func (ds *dummySource) FromQuery(_ *Query) (Metadata, error) {
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
func (lms *literalSource) FromQuery(query *Query) (Metadata, error) {
	parentType := query.Type
	if query.Season >= 0 && query.Episode >= 0 {
		parentType = TypeSeries
	}

	genericMeta := NewMetadata(parentType, query.Query, query.Query, "", time.Now(), 10, nil)
	if query.Season >= 0 && query.Episode >= 0 {
		info := fmt.Sprintf("S%02dE%02d", query.Season, query.Episode)

		return NewEpisodeMetadata(
			NewMetadata(TypeEpisode, info, info, "", genericMeta.ReleaseDate(), 10, nil),
			NewMovieOrSeriesMetadata(genericMeta, nil, nil, nil, nil),
			query.Season,
			query.Episode,
		), nil
	}

	return genericMeta, nil
}

// compositeSource is a Source that tries to resolve metadata from multiple sources.
type compositeSource struct {
	// sources are the sources to be resolved from, iterated in order.
	sources []Source
}

// NewCompositeSource creates a metadata source that resolves results from multiple sources.
func NewCompositeSource(metaSources ...Source) Source {
	return &compositeSource{sources: metaSources}
}

// FromFile tries to resolve metadata for a media file from multiple sources, may return nil.
func (cs *compositeSource) FromFile(path string) (Metadata, error) {
	for _, source := range cs.sources {
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
func (cs *compositeSource) FromQuery(query *Query) (Metadata, error) {
	for _, source := range cs.sources {
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
	var (
		fileName       = filepath.Base(path)
		nameWithoutExt = strings.TrimSuffix(fileName, filepath.Ext(fileName))

		query = &Query{
			Query:   nameWithoutExt,
			Type:    TypeUnknown,
			Season:  -1,
			Episode: -1,
		}
	)

	episodeGroups := episodePattern.FindStringSubmatch(nameWithoutExt)
	if episodeGroups != nil { // presume episode
		query.Query = nameWithoutExt[:strings.Index(nameWithoutExt, episodeGroups[0])]
		query.Type = TypeEpisode
		query.Season, _ = strconv.Atoi(strings.TrimLeft(episodeGroups[1], "0"))  // will never error
		query.Episode, _ = strconv.Atoi(strings.TrimLeft(episodeGroups[2], "0")) // will never error
	} else {
		query.Query = resolutionPattern.ReplaceAllLiteralString(nameWithoutExt, "") // step 1: remove resolution
		query.Query = encodingPattern.ReplaceAllLiteralString(query.Query, "")      // step 2: remove encoding format/codec
		query.Query = stripBracketLike(query.Query)
	}

	query.Query = strings.Join(strings.Fields(commonDelimiterReplacer.Replace(query.Query)), " ")
	return fas.FromQuery(query)
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
