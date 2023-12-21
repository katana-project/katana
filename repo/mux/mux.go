package mux

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/gabriel-vasile/mimetype"
	"github.com/go-faster/errors"
	"github.com/katana-project/ffmpeg/avutil"
	"github.com/katana-project/katana/repo"
	"github.com/katana-project/katana/repo/internal/sync"
	"github.com/katana-project/katana/repo/media"
	"github.com/katana-project/mux"
	"go.uber.org/zap"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// capMask is the mask for the repository capability input.
const capMask = repo.CapabilityRemux | repo.CapabilityTranscode

// formats are the media.Formats mapped to their mux variants.
var formats = make(map[*media.Format]*format)

func init() {
	avutil.SetLogLevel(avutil.LogWarning)

	for _, f := range media.Formats() {
		var (
			muxer   = mux.FindMuxer(f.Name, f.Extension, f.MIME)
			demuxer = mux.FindDemuxer(f.Name, f.Extension, f.MIME)
		)
		if muxer == nil && demuxer == nil {
			continue // don't include missing formats
		}

		formats[f] = &format{muxer: muxer, demuxer: demuxer}
	}
}

// format is a muxer + demuxer combination.
type format struct {
	muxer   *mux.Muxer
	demuxer *mux.Demuxer
}

// muxRepository is a repo.MuxingRepository implementation that uses the mux library.
type muxRepository struct {
	repo.Repository

	path                     string
	remuxPath, transcodePath string

	cap    repo.Capability
	logger *zap.Logger

	mu sync.KMutex
}

// relocatedMedia is a media.Media delegate that changes the destination path and MIME type.
type relocatedMedia struct {
	media.Media

	path, mime string
}

func (rm *relocatedMedia) Path() string {
	return rm.path
}

func (rm *relocatedMedia) MIME() string {
	return rm.mime
}

// NewRepository creates a new mux-backed repo.MuxingRepository.
func NewRepository(r repo.Repository, cap repo.Capability, path string, logger *zap.Logger) (repo.MuxingRepository, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(absPath, 0); err != nil {
		return nil, errors.Wrap(err, "failed to make directories")
	}

	var (
		remuxPath     string
		transcodePath string
	)
	if cap.Has(repo.CapabilityRemux) {
		remuxPath = filepath.Join(absPath, "remux")
		if _, err := os.Stat(remuxPath); errors.Is(err, fs.ErrNotExist) {
			if err := os.Mkdir(remuxPath, 0); err != nil {
				return nil, errors.Wrap(err, "failed to make remux directory")
			}
		}
	}
	if cap.Has(repo.CapabilityTranscode) {
		transcodePath = filepath.Join(absPath, "transcode")
		if _, err := os.Stat(transcodePath); errors.Is(err, fs.ErrNotExist) {
			if err := os.Mkdir(transcodePath, 0); err != nil {
				return nil, errors.Wrap(err, "failed to make transcode directory")
			}
		}
	}

	return &muxRepository{
		Repository:    r,
		path:          absPath,
		remuxPath:     remuxPath,
		transcodePath: transcodePath,
		cap:           cap & capMask,
		logger:        logger,
	}, nil
}

func (mr *muxRepository) Capabilities() repo.Capability {
	return mr.Repository.Capabilities() | mr.cap
}

// TODO: Scan() - clean up unused remux/transcode output

func (mr *muxRepository) Remove(m media.Media) error {
	err := mr.Repository.Remove(m)
	if err != nil {
		return err
	}

	return mr.remove(m.Path())
}

func (mr *muxRepository) RemovePath(path string) error {
	err := mr.Repository.RemovePath(path)
	if err != nil {
		return err
	}

	return mr.remove(path)
}

func (mr *muxRepository) makeHash(path string) (string, error) {
	relPath, err := filepath.Rel(mr.Repository.Path(), path)
	if err != nil {
		return "", errors.Wrap(err, "failed to make path relative")
	}

	fi, err := os.Stat(path)
	if err != nil {
		return "", errors.Wrap(err, "failed to stat file")
	}

	// fast hash using the relative path and file length
	sum := md5.Sum([]byte(fmt.Sprintf("%s,%d", relPath, fi.Size())))
	return hex.EncodeToString(sum[:]), nil
}

// remove cleans all remuxed and transcoded variants of the supplied media path.
func (mr *muxRepository) remove(path string) error {
	hash, err := mr.makeHash(path)
	if err != nil {
		return errors.Wrap(err, "failed to make checksum")
	}

	err = filepath.Walk(mr.path, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			fileName := info.Name()
			if strings.TrimSuffix(fileName, filepath.Ext(fileName)) == hash {
				_, err := mr.mu.Do(path, func() (interface{}, error) {
					return nil, os.Remove(path)
				})
				return err
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

	hash, err := mr.makeHash(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make checksum")
	}

	remuxedPath := filepath.Join(mr.remuxPath, hash+"."+format.Extension)
	res, err := mr.mu.Do(path, func() (interface{}, error) {
		remuxMedia := &relocatedMedia{
			Media: m,
			path:  remuxedPath,
			mime:  format.MIME,
		}
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

	return res.(media.Media), nil
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
		streams = inCtx.Streams()

		streamMapping   = make([]int, len(streams))
		lastStreamIndex = 0
	)
	for i, inStream := range streams {
		var (
			codec   = inStream.Codec()
			remapId = -1
		)
		if muxer.SupportsCodec(codec) {
			outStream := outCtx.NewStream(codec)
			if err := inStream.CopyParameters(outStream); err != nil {
				return errors.Wrapf(err, "failed to copy stream %d parameters", i)
			}

			remapId = lastStreamIndex
			lastStreamIndex++
		} else { // codec not supported in container, strip
			mr.logger.Warn(
				"skipping unsupported codec in stream",
				zap.String("codec", codec.Name()),
				zap.String("format", muxer.Name()),
				zap.Int("stream", i),
				zap.String("src", src),
				zap.String("dst", dst),
			)
		}

		streamMapping[i] = remapId
	}

	pkt := mux.NewPacket()
	defer pkt.Close()

	if err := outCtx.WriteHeader(); err != nil {
		return errors.Wrap(err, "failed to write header")
	}

	for {
		err = inCtx.ReadFrame(pkt)
		if err != nil {
			if err != io.EOF {
				err = errors.Wrap(err, "failed to read frame")
			}

			break
		}

		streamIdx := pkt.StreamIndex()
		if remapId := streamMapping[streamIdx]; remapId >= 0 {
			pkt.SetStreamIndex(remapId)

			pkt.Rescale(
				inCtx.Stream(streamIdx).TimeBase(),
				outCtx.Stream(remapId).TimeBase(),
			)
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
	if err == nil || err == io.EOF {
		if err := outCtx.WriteEnd(); err != nil {
			return errors.Wrap(err, "failed to write end")
		}

		return nil
	}

	return err
}

func (mr *muxRepository) Mux() repo.MuxingRepository {
	return mr
}
