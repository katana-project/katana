package mux

import (
	"github.com/katana-project/katana/repo"
	"github.com/katana-project/katana/repo/media"
	"github.com/katana-project/mux"
	"path/filepath"
)

var (
	formats = make(map[*media.Format]*muxFormat)
)

func init() {
	for _, fmt := range media.Formats() {
		var ext string
		if len(fmt.Extensions) != 0 {
			ext = fmt.Extensions[0]
		}

		var (
			muxer   = mux.FindMuxer(fmt.Name, ext, fmt.MIME)
			demuxer = mux.FindDemuxer(fmt.Name, ext, fmt.MIME)
		)
		if muxer == nil && demuxer == nil {
			continue // don't include missing formats
		}

		formats[fmt] = &muxFormat{muxer: muxer, demuxer: demuxer}
	}
}

// muxFormat is a muxer + demuxer combination.
type muxFormat struct {
	muxer   *mux.Muxer
	demuxer *mux.Demuxer
}

type muxRepository struct {
	repo.Repository

	path    string
	inPlace bool
}

func NewMuxRepository(repo repo.Repository, path string) repo.MuxingRepository {
	inPlace := path == "" // zero value
	if !inPlace {
		path = filepath.Clean(path)
	}

	return &muxRepository{
		Repository: repo,
		path:       path,
		inPlace:    inPlace,
	}
}

func (mr *muxRepository) Capabilities() repo.Capability {
	return mr.Repository.Capabilities() | repo.CapabilityRemux | repo.CapabilityTranscode
}

func (mr *muxRepository) Remux(m media.Media, fmt media.Format) (media.Media, error) {
	//TODO implement me
	panic("implement me")
}
