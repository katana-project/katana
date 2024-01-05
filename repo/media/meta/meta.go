package meta

import (
	"time"
)

// Type is a type of metadata.
type Type uint

const (
	// TypeUnknown is an unknown media metadata type.
	TypeUnknown Type = iota
	// TypeMovie is a movie metadata type.
	TypeMovie
	// TypeSeries is a series metadata type.
	TypeSeries
	// TypeEpisode is a series episode metadata type.
	TypeEpisode
)

// Metadata is a media metadata object.
type Metadata interface {
	// Type returns the type of metadata.
	Type() Type
	// Title returns the title, such as "Bocchi the Rock!".
	Title() string
	// OriginalTitle returns the title as in the original release, such as "ぼっち・ざ・ろっく！".
	OriginalTitle() string
	// Overview returns the plot overview, such as "Hitori Gotoh, a shy, awkward, and lonely high school student dreams of being in a band despite her doubts and worries, but when she is recruited to be the guitarist of a group looking to make it big, she realises her dream may be able to be fulfilled and come true.".
	Overview() string
	// ReleaseDate returns the date of release, such as "2022-10-09".
	ReleaseDate() time.Time
	// VoteRating returns the average rating of the media, between 0 and 10, such as 8.7.
	VoteRating() float32
	// Images returns the promotional images of the media.
	Images() []Image
}

// BasicMetadata is a JSON-serializable Metadata with set values.
type BasicMetadata struct {
	Type_          Type          `json:"type"`
	Title_         string        `json:"title"`
	OriginalTitle_ string        `json:"original_title"`
	Overview_      string        `json:"overview"`
	ReleaseDate_   time.Time     `json:"release_date"`
	VoteRating_    float32       `json:"vote_rating"`
	Images_        []*BasicImage `json:"images"`
}

// NewMetadata creates a Metadata with set values.
func NewMetadata(type_ Type, title, originalTitle, overview string, releaseDate time.Time, voteRating float32, images []Image) Metadata {
	images0 := make([]*BasicImage, len(images))
	for i, image := range images {
		images0[i] = NewBasicImage(image)
	}

	return &BasicMetadata{
		Type_:          type_,
		Title_:         title,
		OriginalTitle_: originalTitle,
		Overview_:      overview,
		ReleaseDate_:   releaseDate,
		VoteRating_:    voteRating,
		Images_:        images0,
	}
}

// NewBasicMetadata wraps a Metadata object into BasicMetadata.
func NewBasicMetadata(m Metadata) *BasicMetadata {
	if m == nil {
		return nil
	}
	if bm, ok := m.(*BasicMetadata); ok {
		return bm
	}

	var (
		images  = m.Images()
		images0 = make([]*BasicImage, len(images))
	)
	for i, image := range images {
		images0[i] = NewBasicImage(image)
	}

	return &BasicMetadata{
		Type_:          m.Type(),
		Title_:         m.Title(),
		OriginalTitle_: m.OriginalTitle(),
		Overview_:      m.Overview(),
		ReleaseDate_:   m.ReleaseDate(),
		VoteRating_:    m.VoteRating(),
		Images_:        images0,
	}
}

func (bm *BasicMetadata) Type() Type {
	return bm.Type_
}
func (bm *BasicMetadata) Title() string {
	return bm.Title_
}
func (bm *BasicMetadata) OriginalTitle() string {
	return bm.OriginalTitle_
}
func (bm *BasicMetadata) Overview() string {
	return bm.Overview_
}
func (bm *BasicMetadata) ReleaseDate() time.Time {
	return bm.ReleaseDate_
}
func (bm *BasicMetadata) VoteRating() float32 {
	return bm.VoteRating_
}
func (bm *BasicMetadata) Images() []Image {
	images := make([]Image, len(bm.Images_))
	for i, image := range bm.Images_ {
		images[i] = image
	}

	return images
}
