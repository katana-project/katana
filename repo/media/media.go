package media

import (
	"encoding/json"
	"fmt"
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
	id   string
	path string
	mime string
	meta meta.Metadata
}

// NewMedia creates a Media with set values.
func NewMedia(id, path string, mime string, meta0 meta.Metadata) Media {
	return &BasicMedia{
		id:   id,
		path: path,
		mime: mime,
		meta: meta0,
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
		id:   m.ID(),
		path: m.Path(),
		mime: m.MIME(),
		meta: m.Meta(),
	}
}

func (bm *BasicMedia) ID() string {
	return bm.id
}
func (bm *BasicMedia) Path() string {
	return bm.path
}
func (bm *BasicMedia) MIME() string {
	return bm.mime
}
func (bm *BasicMedia) Meta() meta.Metadata {
	return bm.meta
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
	switch m := bm.meta.(type) {
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
		ID:   bm.id,
		Path: bm.path,
		MIME: bm.mime,
		Meta: meta0,
	})
}

// UnmarshalJSON unmarshals JSON data into this struct.
func (bm *BasicMedia) UnmarshalJSON(bytes []byte) error {
	var helper basicMediaJSONHelper
	if err := json.Unmarshal(bytes, &helper); err != nil {
		return err
	}

	bm.id = helper.ID
	bm.path = helper.Path
	bm.mime = helper.MIME

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

		bm.meta = &metaData
	case meta.TypeMovie, meta.TypeSeries:
		var metaData meta.BasicMovieOrSeriesMetadata
		if err := json.Unmarshal(helper.Meta, &metaData); err != nil {
			return err
		}

		bm.meta = &metaData
	case meta.TypeEpisode:
		var metaData meta.BasicEpisodeMetadata
		if err := json.Unmarshal(helper.Meta, &metaData); err != nil {
			return err
		}

		bm.meta = &metaData
	default:
		return fmt.Errorf("unexpected metadata type %d", metaBase.Type)
	}

	return nil
}
