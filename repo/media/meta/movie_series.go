package meta

import (
	"golang.org/x/text/language"
)

// MovieOrSeriesMetadata is a metadata object of a movie or series.
type MovieOrSeriesMetadata interface {
	Metadata

	// Genres returns the genre names.
	Genres() []string
	// Cast returns the cast members.
	Cast() []CastMember
	// Languages returns tags of the languages spoken in the movie or series.
	Languages() []language.Tag
	// Countries returns tags of the regions that took part in producing the movie or series.
	Countries() []language.Region
}

// BasicMovieOrSeriesMetadata is a JSON-serializable MovieOrSeriesMetadata.
type BasicMovieOrSeriesMetadata struct {
	*BasicMetadata

	Genres_    []string           `json:"genres"`
	Cast_      []*BasicCastMember `json:"cast"`
	Languages_ []string           `json:"languages"`
	Countries_ []string           `json:"countries"`
}

// NewMovieOrSeriesMetadata creates a MovieOrSeriesMetadata with set values.
func NewMovieOrSeriesMetadata(
	m Metadata,
	genres []string,
	castMembers []CastMember,
	languages []language.Tag,
	countries []language.Region,
) MovieOrSeriesMetadata {
	castMembers0 := make([]*BasicCastMember, len(castMembers))
	for i, cm := range castMembers {
		castMembers0[i] = NewBasicCastMember(cm)
	}

	languages0 := make([]string, len(languages))
	for i, lang := range languages {
		languages0[i] = lang.String()
	}

	countries0 := make([]string, len(countries))
	for i, country := range countries {
		countries0[i] = country.String()
	}

	return &BasicMovieOrSeriesMetadata{
		BasicMetadata: NewBasicMetadata(m),
		Genres_:       genres,
		Cast_:         castMembers0,
		Languages_:    languages0,
		Countries_:    countries0,
	}
}

// NewBasicMovieOrSeriesMetadata wraps a MovieOrSeriesMetadata into BasicMovieOrSeriesMetadata.
func NewBasicMovieOrSeriesMetadata(msm MovieOrSeriesMetadata) *BasicMovieOrSeriesMetadata {
	if msm == nil {
		return nil
	}
	if bmsm, ok := msm.(*BasicMovieOrSeriesMetadata); ok {
		return bmsm
	}

	var (
		castMembers  = msm.Cast()
		castMembers0 = make([]*BasicCastMember, len(castMembers))
	)
	for i, cm := range castMembers {
		castMembers0[i] = NewBasicCastMember(cm)
	}

	var (
		languages  = msm.Languages()
		languages0 = make([]string, len(languages))
	)
	for i, lang := range languages {
		languages0[i] = lang.String()
	}

	var (
		countries  = msm.Countries()
		countries0 = make([]string, len(countries))
	)
	for i, country := range countries {
		countries0[i] = country.String()
	}

	return &BasicMovieOrSeriesMetadata{
		BasicMetadata: NewBasicMetadata(msm),
		Genres_:       msm.Genres(),
		Cast_:         castMembers0,
		Languages_:    languages0,
		Countries_:    countries0,
	}
}

func (bmsm *BasicMovieOrSeriesMetadata) Genres() []string {
	return bmsm.Genres_
}
func (bmsm *BasicMovieOrSeriesMetadata) Cast() []CastMember {
	castMembers := make([]CastMember, len(bmsm.Cast_))
	for i, cm := range bmsm.Cast_ {
		castMembers[i] = cm
	}

	return castMembers
}
func (bmsm *BasicMovieOrSeriesMetadata) Languages() []language.Tag {
	languages := make([]language.Tag, len(bmsm.Languages_))
	for i, lang := range bmsm.Languages_ {
		tag, err := language.Parse(lang)
		if err != nil {
			tag = language.Und
		}

		languages[i] = tag
	}

	return languages
}
func (bmsm *BasicMovieOrSeriesMetadata) Countries() []language.Region {
	countries := make([]language.Region, len(bmsm.Countries_))
	for i, country := range bmsm.Countries_ {
		region, err := language.ParseRegion(country)
		if err != nil {
			region = language.Region{}
		}

		countries[i] = region
	}

	return countries
}
