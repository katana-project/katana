package media

import (
	"strings"
)

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

	formatsByName = make(map[string]*Format, len(formats))
	formatsByMime = make(map[string]*Format, len(formats))
)

func init() {
	for _, format := range formats {
		formatsByName[strings.ToLower(format.Name)] = format
		formatsByMime[format.MIME] = format
	}
}

// Format is a media file container format.
type Format struct {
	// Name is the container format name.
	Name string `json:"name"`
	// MIME is the format MIME type.
	MIME string `json:"mime"`
	// Extension is the format's preferred file extension, **without leading dots**.
	Extension string `json:"extension"`
}

// Formats returns all default formats.
func Formats() []*Format {
	return formats
}

// FindFormat tries to find a format by its name, returns nil if not found.
func FindFormat(name string) *Format {
	if format, ok := formatsByName[strings.ToLower(name)]; ok {
		return format
	}

	return nil
}

// FindFormatMIME tries to find a format by its MIME type, returns nil if not found.
func FindFormatMIME(mime string) *Format {
	if format, ok := formatsByMime[mime]; ok {
		return format
	}

	return nil
}

// FindUnsupportedFormat tries to find a format by its MIME type, creating an unsupported Format if not found.
func FindUnsupportedFormat(mime, ext string) *Format {
	if format := FindFormatMIME(mime); format != nil {
		return format
	}

	return &Format{MIME: mime, Extension: ext}
}

// Supported returns whether the format is known to available de/muxers.
func (f *Format) Supported() bool {
	_, ok := formatsByMime[f.MIME]
	return ok
}
