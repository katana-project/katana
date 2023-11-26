package mux

import (
	"crypto/md5"
	"encoding/hex"
	"github.com/gabriel-vasile/mimetype"
	"github.com/go-faster/errors"
	"github.com/katana-project/ffmpeg/avutil"
	"github.com/katana-project/katana/repo"
	"github.com/katana-project/katana/repo/media"
	"github.com/katana-project/mux"
	"golang.org/x/sync/singleflight"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// formats are a mapping of supported formats to their mux variants.
var formats = make(map[*media.Format]*muxFormat)

func init() {
	avutil.SetLogLevel(avutil.LogError)

	for _, format := range media.Formats() {
		var (
			muxer   = mux.FindMuxer(format.Name, format.Extension, format.MIME)
			demuxer = mux.FindDemuxer(format.Name, format.Extension, format.MIME)
		)
		if muxer == nil && demuxer == nil {
			continue // don't include missing formats
		}

		formats[format] = &muxFormat{muxer: muxer, demuxer: demuxer}
	}
}

// muxFormat is a muxer + demuxer combination.
type muxFormat struct {
	muxer   *mux.Muxer
	demuxer *mux.Demuxer
}

// muxRepository is a repo.MuxingRepository implementation that uses the mux library.
type muxRepository struct {
	repo.Repository

	path      string
	remuxPath string

	sf singleflight.Group
}

// relocatedMedia is a media.Media delegate that changes the destination path.
type relocatedMedia struct {
	media.Media

	path string
}

func (rm *relocatedMedia) Path() string {
	return rm.path
}

// NewMuxRepository creates a new mux-backed repo.MuxingRepository.
func NewMuxRepository(repo repo.Repository, path string) (repo.MuxingRepository, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(absPath, 0); err != nil {
		return nil, errors.Wrap(err, "failed to make directories")
	}

	remuxPath := filepath.Join(absPath, "remux")
	if err := os.Mkdir(remuxPath, 0); err != nil {
		return nil, errors.Wrap(err, "failed to make remux directory")
	}

	return &muxRepository{
		Repository: repo,
		path:       absPath,
		remuxPath:  remuxPath,
	}, nil
}

func (mr *muxRepository) Capabilities() repo.Capability {
	return mr.Repository.Capabilities() | repo.CapabilityRemux | repo.CapabilityTranscode
}

func (mr *muxRepository) Remove(m media.Media) error {
	err := mr.Repository.Remove(m)
	if err != nil {
		return err
	}

	return /*mr.remove(m.Path())*/ nil // TODO: concurrency concerns
}

func (mr *muxRepository) RemovePath(path string) error {
	err := mr.Repository.RemovePath(path)
	if err != nil {
		return err
	}

	return /*mr.remove(path)*/ nil // TODO: concurrency concerns
}

// remove cleans all remuxed and transcoded variants of the supplied media path.
func (mr *muxRepository) remove(path string) error {
	relPath, err := filepath.Rel(mr.Repository.Path(), path)
	if err != nil {
		return errors.Wrap(err, "failed to make media path relative")
	}

	var (
		hash    = md5.Sum([]byte(relPath))
		hashHex = hex.EncodeToString(hash[:])
	)
	err = filepath.Walk(mr.path, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			fileName := info.Name()
			if strings.TrimSuffix(fileName, filepath.Ext(fileName)) == hashHex {
				return os.Remove(path)
			}
		}

		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to walk + delete media")
	}

	return nil
}

func (mr *muxRepository) Remux(id string, format *media.Format) (media.Media, error) {
	m := mr.Repository.Get(id)
	if m == nil {
		return nil, nil
	}

	// FAST PATH: extension already matches, no need to remux
	if filepath.Ext(m.Path())[1:] == format.Extension {
		return m, nil
	}

	path := m.Path()
	mime, err := mimetype.DetectFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to detect MIME type")
	}
	if mime.String() == format.MIME { // or MIME type
		return m, nil
	}

	relPath, err := filepath.Rel(mr.Repository.Path(), path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make media path relative")
	}

	var (
		hash        = md5.Sum([]byte(relPath))
		hashHex     = hex.EncodeToString(hash[:])
		remuxedPath = filepath.Join(mr.remuxPath, hashHex+"."+format.Extension)
	)
	v, err, _ := mr.sf.Do(remuxedPath, func() (interface{}, error) {
		remuxMedia := &relocatedMedia{Media: m, path: remuxedPath}
		if _, err := os.Stat(remuxMedia.path); err == nil {
			return remuxMedia, nil // already remuxed
		}

		muxDem, ok := formats[format]
		if !ok || muxDem.muxer == nil {
			return nil, &repo.ErrUnsupportedFormat{
				Format:    format.Name,
				Operation: "muxing",
			}
		}

		if err := mr.remux(muxDem.muxer, path, remuxMedia.path); err != nil {
			return nil, errors.Wrap(err, "failed to remux")
		}

		return remuxMedia, nil
	})
	if err != nil {
		return nil, err
	}

	return v.(media.Media), nil
}

func (mr *muxRepository) remux(muxer *mux.Muxer, src, dst string) (err error) {
	inCtx, err := mux.NewInputContext(src)
	if err != nil {
		return errors.Wrap(err, "failed to open input context")
	}
	defer inCtx.Close()

	outCtx, err := mux.NewOutputContext(muxer, dst)
	if err != nil {
		return errors.Wrap(err, "failed to open output context")
	}
	defer outCtx.Close()

	var (
		streamMapping   = make(map[int]int)
		lastStreamIndex = 0
	)
	for i, inStream := range inCtx.Streams() {
		if !muxer.SupportsCodec(inStream.Codec()) { // codec not supported in container, strip
			continue
		}

		outStream := outCtx.NewStream(nil)
		if err := inStream.CopyParameters(outStream); err != nil {
			return errors.Wrapf(err, "failed to copy stream %d parameters", i)
		}

		streamMapping[i] = lastStreamIndex
		lastStreamIndex++
	}

	pkt := mux.NewPacket()
	defer pkt.Close()

	for {
		err = inCtx.ReadFrame(pkt)
		if err != nil {
			if err != io.EOF {
				err = errors.Wrap(err, "failed to read frame")
			}

			break
		}

		streamIdx := pkt.StreamIndex()
		if remapId, ok := streamMapping[streamIdx]; ok {
			pkt.Rescale(
				inCtx.Stream(streamIdx).TimeBase(),
				outCtx.Stream(remapId).TimeBase(),
			)
			pkt.SetStreamIndex(remapId)
			pkt.ResetPos()

			if err := outCtx.WriteFrame(pkt); err != nil {
				err = errors.Wrap(err, "failed to write frame")
				break
			}
			// WriteFrame takes ownership of the packet and resets it, no need to clear here
		} else {
			if err := pkt.Clear(); err != nil {
				err = errors.Wrap(err, "failed to clear packet")
				break
			}
		}
	}
	if err == io.EOF {
		return nil
	}

	return err
}
