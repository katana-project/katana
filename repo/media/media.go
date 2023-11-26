package media

import (
	"encoding/json"
	"github.com/go-faster/errors"
	"github.com/katana-project/katana/repo/media/meta"
	"regexp"
	"strings"
)

var (
	idPattern               = regexp.MustCompile("^[a-z0-9-_]+$")
	idCharExclusivePattern  = regexp.MustCompile("[^a-z0-9-_]")
	commonDelimiterReplacer = strings.NewReplacer(" ", "-", ".", "-")
)

// Media is a media file.
type Media interface {
	// ID returns the media ID, alphanumeric, lowercase, non-blank ([a-z0-9-_]).
	ID() string
	// Path is the path of this media file, absolute.
	Path() string
	// MIME is the MIME type of this media file.
	MIME() string
	// Meta is the metadata object, may be nil.
	Meta() meta.Metadata
}

// ValidID checks whether the supplied string is a valid media ID.
func ValidID(s string) bool {
	return idPattern.MatchString(s)
}

// SanitizeID sanitizes a string to be usable as a media ID.
// Example: "Test.mkv" -> "test-mkv"
func SanitizeID(s string) string {
	spaceLessLowerCase := strings.ToLower(commonDelimiterReplacer.Replace(s))

	return idCharExclusivePattern.ReplaceAllLiteralString(spaceLessLowerCase, "")
}

// BasicMedia is a JSON-serializable generic Media.
type BasicMedia struct {
	ID_   string
	Path_ string
	MIME_ string
	Meta_ meta.Metadata
}

// NewMedia creates a Media with set values.
func NewMedia(id, path string, mime string, meta0 meta.Metadata) Media {
	return &BasicMedia{
		ID_:   id,
		Path_: path,
		MIME_: mime,
		Meta_: meta0,
	}
}

// NewBasicMedia wraps Media into BasicMedia.
func NewBasicMedia(m Media) *BasicMedia {
	if m == nil {
		return nil
	}
	if bm, ok := m.(*BasicMedia); ok {
		return bm
	}

	return &BasicMedia{
		ID_:   m.ID(),
		Path_: m.Path(),
		MIME_: m.MIME(),
		Meta_: m.Meta(),
	}
}

func (bm *BasicMedia) ID() string {
	return bm.ID_
}
func (bm *BasicMedia) Path() string {
	return bm.Path_
}
func (bm *BasicMedia) MIME() string {
	return bm.MIME_
}
func (bm *BasicMedia) Meta() meta.Metadata {
	return bm.Meta_
}

// basicMediaJSONHelper is a helper struct for unmarshalling.
type basicMediaJSONHelper struct {
	ID   string          `json:"id"`
	Path string          `json:"path"`
	MIME string          `json:"mime"`
	Meta json.RawMessage `json:"meta"`
}

// metadataJSONHelper is a helper struct for figuring out the concrete metadata type when unmarshalling foreign JSON.
type metadataJSONHelper struct {
	Type meta.Type `json:"type"`
}

// MarshalJSON marshals JSON data from this struct.
func (bm *BasicMedia) MarshalJSON() ([]byte, error) {
	var meta0 meta.Metadata
	switch m := bm.Meta_.(type) {
	case meta.EpisodeMetadata:
		meta0 = meta.NewBasicEpisodeMetadata(m)
	case meta.MovieOrSeriesMetadata:
		meta0 = meta.NewBasicMovieOrSeriesMetadata(m)
	default:
		meta0 = meta.NewBasicMetadata(m)
	}

	return json.Marshal(&struct {
		ID   string        `json:"id"`
		Path string        `json:"path"`
		MIME string        `json:"mime"`
		Meta meta.Metadata `json:"meta"`
	}{
		ID:   bm.ID_,
		Path: bm.Path_,
		MIME: bm.MIME_,
		Meta: meta0,
	})
}

// UnmarshalJSON unmarshals JSON data into this struct.
func (bm *BasicMedia) UnmarshalJSON(bytes []byte) error {
	var helper basicMediaJSONHelper
	if err := json.Unmarshal(bytes, &helper); err != nil {
		return err
	}

	bm.ID_ = helper.ID
	bm.Path_ = helper.Path
	bm.MIME_ = helper.MIME

	var metaBase metadataJSONHelper
	if err := json.Unmarshal(helper.Meta, &metaBase); err != nil {
		return err
	}

	switch metaBase.Type {
	case meta.TypeUnknown:
		var metaData meta.BasicMetadata
		if err := json.Unmarshal(helper.Meta, &metaData); err != nil {
			return err
		}

		bm.Meta_ = &metaData
	case meta.TypeMovie, meta.TypeSeries:
		var metaData meta.BasicMovieOrSeriesMetadata
		if err := json.Unmarshal(helper.Meta, &metaData); err != nil {
			return err
		}

		bm.Meta_ = &metaData
	case meta.TypeEpisode:
		var metaData meta.BasicEpisodeMetadata
		if err := json.Unmarshal(helper.Meta, &metaData); err != nil {
			return err
		}

		bm.Meta_ = &metaData
	default:
		return errors.Errorf("unexpected metadata type %d", metaBase.Type)
	}

	return nil
}
