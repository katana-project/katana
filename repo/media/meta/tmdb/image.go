package tmdb

import (
	"fmt"
	"github.com/katana-project/katana/repo/media/meta"
	"github.com/katana-project/tmdb"
)

type image struct {
	type_       meta.ImageType
	path        string
	description string
	config      *tmdb.ConfigurationDetailsOKImages
}

func (i *image) Type() meta.ImageType {
	return i.type_
}
func (i *image) Path() string {
	var (
		url string
		ok  bool
	)
	if url, ok = i.config.GetSecureBaseURL().Get(); !ok {
		if url, ok = i.config.GetBaseURL().Get(); !ok {
			return "" // no available base URL
		}
	}

	// TODO: configure the size?
	return fmt.Sprintf("%soriginal%s", url, i.path)
}
func (i *image) Remote() bool {
	return true
}
func (i *image) Description() string {
	return i.description
}
