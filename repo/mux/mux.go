package mux

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"github.com/katana-project/ffmpeg/avutil"
	"github.com/katana-project/katana/internal/errors"
	"github.com/katana-project/katana/internal/sync"
	"github.com/katana-project/katana/repo"
	"github.com/katana-project/katana/repo/media"
	"github.com/katana-project/mux"
	"go.uber.org/multierr"
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

		formats[f] = &format{
			Format:  f,
			muxer:   muxer,
			demuxer: demuxer,
		}
	}
}

// format is a muxer + demuxer combination.
type format struct {
	*media.Format

	muxer   *mux.Muxer
	demuxer *mux.Demuxer
}

// muxRepo is a repo.MuxingRepository implementation that uses the mux library.
type muxRepo struct {
	repo.MutableRepository

	path, remuxPath, transcodePath string

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

// NewRepository creates a new mux-backed repo.MutableRepository.
func NewRepository(r repo.MutableRepository, cap repo.Capability, path string, logger *zap.Logger) (repo.MutableRepository, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make path absolute")
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

	return &muxRepo{
		MutableRepository: r,
		path:              absPath,
		remuxPath:         remuxPath,
		transcodePath:     transcodePath,
		cap:               cap & capMask,
		logger:            logger,
	}, nil
}

func (mr *muxRepo) Capabilities() repo.Capability {
	return mr.MutableRepository.Capabilities() | mr.cap
}

func (mr *muxRepo) Scan() error {
	if err := mr.MutableRepository.Scan(); err != nil {
		return err
	}

	var (
		items  = mr.MutableRepository.Items() // snapshot repo items
		hashes = make(map[string]struct{}, len(items))
	)
	for _, item := range items {
		hash, err := makeHash(item.Path())
		if err != nil {
			return errors.Wrap(err, "failed to make hash")
		}

		hashes[hash] = struct{}{}
	}

	err := mr.walkCache(func(path string, d fs.DirEntry) error {
		var (
			name = d.Name()
			hash = strings.TrimSuffix(name, filepath.Ext(name))
		)
		if _, ok := hashes[hash]; !ok { // doesn't exist in repo, remove
			_, err := mr.mu.Do(path, func() (interface{}, error) {
				return nil, os.Remove(path)
			})
			if mr.logger != nil {
				mr.logger.Info(
					"removed unused cache file",
					zap.String("repo", mr.MutableRepository.ID()),
					zap.String("repo_path", mr.MutableRepository.Path()),
					zap.String("path", path),
				)
			}

			return err
		}

		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to walk + delete media")
	}

	return nil
}

func (mr *muxRepo) Remove(m media.Media) error {
	hash, err := makeHash(m.Path())
	if err != nil {
		return errors.Wrap(err, "failed to make hash")
	}

	if err := mr.MutableRepository.Remove(m); err != nil {
		return err
	}

	return mr.remove(hash)
}

func (mr *muxRepo) RemovePath(path string) error {
	hash, err := makeHash(path)
	if err != nil {
		return errors.Wrap(err, "failed to make hash")
	}

	if err := mr.MutableRepository.RemovePath(path); err != nil {
		return err
	}

	return mr.remove(hash)
}

func makeHash(path string) (_ string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", errors.Wrap(err, "failed to open file")
	}
	defer func() {
		if err0 := f.Close(); err0 != nil {
			err = multierr.Append(err, errors.Wrap(err0, "failed to close file"))
		}
	}()

	fi, err := f.Stat()
	if err != nil {
		return "", errors.Wrap(err, "failed to stat file")
	}

	h := md5.New()
	if _, err = io.Copy(h, io.LimitReader(f, 1024*1024)); err != nil {
		return "", errors.Wrap(err, "failed to read file")
	}

	_ = binary.Write(h, binary.LittleEndian, fi.Size())
	return hex.EncodeToString(h.Sum(nil)), err
}

func (mr *muxRepo) remove(hash string) error {
	err := mr.walkCache(func(path string, d fs.DirEntry) error {
		name := d.Name()
		if strings.TrimSuffix(name, filepath.Ext(name)) == hash {
			_, err := mr.mu.Do(path, func() (interface{}, error) {
				return nil, os.Remove(path)
			})
			return err
		}

		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to walk + delete media")
	}

	return nil
}

type walkFunc func(path string, d fs.DirEntry) error

func (mr *muxRepo) walkCache(fn walkFunc) error {
	if err := walkFiles(mr.remuxPath, fn); err != nil {
		return err
	}
	if err := walkFiles(mr.transcodePath, fn); err != nil {
		return err
	}

	return nil
}

func walkFiles(path string, fn walkFunc) error {
	return filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			err = fn(path, d)
		}
		return err
	})
}

func (mr *muxRepo) Remux(id string, format *media.Format) (media.Media, error) {
	m := mr.MutableRepository.Get(id)
	if m == nil {
		return nil, nil
	}

	mFmt := m.Format()
	// FAST PATH: MIME type already matches
	if mFmt.MIME == format.MIME {
		return m, nil
	}

	path := m.Path()

	hash, err := makeHash(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make hash")
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

func (mr *muxRepo) remux(muxer *mux.Muxer, src, dst string) (err error) {
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
		} else if mr.logger != nil { // codec not supported in container, strip
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

func (mr *muxRepo) Mutable() repo.MutableRepository {
	return mr
}
