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
	// Meta is the metadata object, may be nil.
	Meta() meta.Metadata
	// Format is the media format.
	Format() *Format
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
	ID_     string
	Path_   string
	Meta_   meta.Metadata
	Format_ *Format
}

// NewMedia creates a Media with set values.
func NewMedia(id, path string, meta0 meta.Metadata, format *Format) Media {
	return &BasicMedia{
		ID_:     id,
		Path_:   path,
		Meta_:   meta0,
		Format_: format,
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
		ID_:     m.ID(),
		Path_:   m.Path(),
		Meta_:   m.Meta(),
		Format_: m.Format(),
	}
}

func (bm *BasicMedia) ID() string {
	return bm.ID_
}
func (bm *BasicMedia) Path() string {
	return bm.Path_
}
func (bm *BasicMedia) Meta() meta.Metadata {
	return bm.Meta_
}
func (bm *BasicMedia) Format() *Format {
	return bm.Format_
}

// basicMediaJSONHelper is a helper struct for unmarshalling.
type basicMediaJSONHelper struct {
	ID     string          `json:"id"`
	Path   string          `json:"path"`
	Meta   json.RawMessage `json:"meta"`
	Format *Format         `json:"format"`
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
		ID     string        `json:"id"`
		Path   string        `json:"path"`
		Meta   meta.Metadata `json:"meta"`
		Format *Format       `json:"format"`
	}{
		ID:     bm.ID_,
		Path:   bm.Path_,
		Meta:   meta0,
		Format: bm.Format_,
	})
}

// UnmarshalJSON unmarshalls JSON data into this struct.
func (bm *BasicMedia) UnmarshalJSON(bytes []byte) error {
	var helper basicMediaJSONHelper
	if err := json.Unmarshal(bytes, &helper); err != nil {
		return err
	}

	bm.ID_ = helper.ID
	bm.Path_ = helper.Path
	bm.Format_ = helper.Format

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
		return fmt.Errorf("unexpected metadata type %d", metaBase.Type)
	}

	return nil
}
