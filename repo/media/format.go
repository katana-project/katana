package media

// Format is a media file container format.
type Format struct {
	// Name is the container format name.
	Name string
	// MIME is the format MIME type.
	MIME string
	// Extensions are the format file extensions.
	Extensions []string
}

var (
	// FormatMP4 is the MP4 container format (.mp4, video/mp4).
	FormatMP4 = &Format{
		Name:       "MP4",
		MIME:       "video/mp4",
		Extensions: []string{"mp4"},
	}
	// FormatMKV is the Matroska container format (.mkv, video/x-matroska).
	FormatMKV = &Format{
		Name:       "MKV",
		MIME:       "video/x-matroska",
		Extensions: []string{"mkv"},
	}

	formats = []*Format{FormatMP4, FormatMKV}
)

// Formats returns all default formats.
func Formats() []*Format {
	return formats
}
