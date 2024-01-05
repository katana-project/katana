package tmdb

import "github.com/katana-project/katana/repo/media/meta"

type castMember struct {
	name, role string
	img        *image
}

func (cm *castMember) Name() string {
	return cm.name
}
func (cm *castMember) Role() string {
	return cm.role
}
func (cm *castMember) Image() meta.Image {
	return cm.img
}
