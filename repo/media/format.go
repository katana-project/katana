package media

// Format is a media file container format.
type Format struct {
	// Name is the container format name.
	Name string
	// MIME is the format MIME type.
	MIME string
	// Extension is the format's preferred file extension, **without leading dots**.
	Extension string
}

var (
	// FormatMP4 is the MP4 container format (.mp4, video/mp4).
	FormatMP4 = &Format{
		Name:      "MP4",
		MIME:      "video/mp4",
		Extension: "mp4",
	}
	// FormatMKV is the Matroska container format (.mkv, video/x-matroska).
	FormatMKV = &Format{
		Name:      "MKV",
		MIME:      "video/x-matroska",
		Extension: "mkv",
	}

	formats = []*Format{FormatMP4, FormatMKV}
)

// Formats returns all default formats.
func Formats() []*Format {
	return formats
}
