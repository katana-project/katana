package tmdb

import (
	"fmt"
	"github.com/katana-project/katana/repo/media/meta"
)

type image struct {
	type_               meta.ImageType
	path, desc, baseUrl string
}

func (i *image) Type() meta.ImageType {
	return i.type_
}
func (i *image) Path() string {
	// TODO: configure the size?
	return fmt.Sprintf("%soriginal%s", i.baseUrl, i.path)
}
func (i *image) Remote() bool {
	return true
}
func (i *image) Description() string {
	return i.desc
}
