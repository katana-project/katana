package meta

// ImageType is a type of image.
type ImageType uint

const (
	// ImageTypeUnknown is an unspecified image type.
	ImageTypeUnknown ImageType = iota
	// ImageTypeStill is a still image type (https://en.wikipedia.org/wiki/Film_still).
	ImageTypeStill
	// ImageTypeBackdrop is a background image type.
	ImageTypeBackdrop
	// ImageTypePoster is a poster image type.
	ImageTypePoster
	// ImageTypeAvatar is a person avatar image type.
	ImageTypeAvatar
)

// Image is an image file.
type Image interface {
	// Type returns the image's type.
	Type() ImageType
	// Path returns the image's path.
	Path() string
	// Remote returns whether Path returns a remote URL.
	Remote() bool
	// Description returns the image's description.
	Description() string
}

// BasicImage is a JSON-serializable Image.
type BasicImage struct {
	Type_        ImageType `json:"type"`
	Path_        string    `json:"path"`
	Remote_      bool      `json:"remote"`
	Description_ string    `json:"description"`
}

// NewImage creates an Image with set values.
func NewImage(type_ ImageType, path string, remote bool, description string) Image {
	return &BasicImage{
		Type_:        type_,
		Path_:        path,
		Remote_:      remote,
		Description_: description,
	}
}

// NewBasicImage wraps an Image into BasicImage.
func NewBasicImage(i Image) *BasicImage {
	if i == nil {
		return nil
	}
	if bi, ok := i.(*BasicImage); ok {
		return bi
	}

	return &BasicImage{
		Type_:        i.Type(),
		Path_:        i.Path(),
		Remote_:      i.Remote(),
		Description_: i.Description(),
	}
}

func (bi *BasicImage) Type() ImageType {
	return bi.Type_
}
func (bi *BasicImage) Path() string {
	return bi.Path_
}
func (bi *BasicImage) Remote() bool {
	return bi.Remote_
}
func (bi *BasicImage) Description() string {
	return bi.Description_
}
