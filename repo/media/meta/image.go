package meta

// Image is an image file.
type Image interface {
	// Path returns the image's path.
	Path() string
	// Remote returns whether Path returns a remote URL.
	Remote() bool
	// Description returns the image's description.
	Description() string
}

// BasicImage is a JSON-serializable Image.
type BasicImage struct {
	Path_        string `json:"path"`
	Remote_      bool   `json:"remote"`
	Description_ string `json:"description"`
}

// NewImage creates an Image with set values.
func NewImage(path string, remote bool, description string) Image {
	return &BasicImage{
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
		Path_:        i.Path(),
		Remote_:      i.Remote(),
		Description_: i.Description(),
	}
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
