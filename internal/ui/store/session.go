package store

import (
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"
	"time"

	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/pipeline"
	"github.com/mehran9675/touchup/internal/tr"
)

// Session is the file open in the studio.
type Session struct {
	Path string
	Name string
	Kind media.Kind
	Info media.Info

	Loading bool
	Err     string

	// Picture is the image itself, or the video frame at Time.
	Picture      image.Image
	Time         float64
	FrameLoading bool
	frameSeq     int

	// Film holds evenly spaced video thumbnails for the timeline.
	Film []image.Image

	// Peaks is the audio waveform; the section [SelStart, SelStart+SelLen)
	// is what a preview enhances.
	Peaks    []float32
	SelStart float64
	SelLen   float64

	Preview Preview

	ctx    context.Context
	cancel context.CancelFunc
}

// Preview is the state of a preview run and its result.
type Preview struct {
	Running bool
	Tuning  bool // the run is timing the engine's settings, not a preview
	Update  pipeline.Update
	Started time.Time
	Log     LiveLog // what the engine has printed so far
	Pace    Pace
	Err     string
	ErrLog  string // what the engine printed last, when it failed

	// At is the video time or audio start the result was made for.
	At     float64
	Length float64
	Label  string // engine, model and scale the result came from

	Frame *pipeline.FramePreview
	Audio *pipeline.AudioPreview

	cancel context.CancelFunc
}

// defaultSection is how many seconds of audio a preview takes at first.
const defaultSection = 10.0

// OpenDialog asks the user for a file and opens it.
func (s *Store) OpenDialog() {
	go func() {
		f, err := s.App.OpenFile(context.Background(), giode.FileFilter{Extensions: media.Extensions()})
		if errors.Is(err, giode.ErrCanceled) {
			return
		}
		if err != nil {
			s.fail(err)
			return
		}
		f.Close()
		named, ok := f.(interface{ Name() string })
		if !ok {
			s.fail(errors.New(tr.T("this file can't be opened from its location")))
			return
		}
		path := named.Name()
		s.post(func() { s.Open(path) })
	}()
}

// Open loads path into the studio, replacing the open file.
func (s *Store) Open(path string) {
	kind := media.Detect(path)
	if kind == media.Unknown {
		s.Toasts.Show(giode.ToastWarning, tr.T("TouchUp opens images, videos and audio files"))
		return
	}
	if fi, err := os.Stat(path); err != nil || fi.IsDir() {
		s.Toasts.Show(giode.ToastWarning, tr.T("That file can't be read"))
		return
	}
	s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	sess := &Session{
		Path: path, Name: filepath.Base(path), Kind: kind,
		Loading: true, ctx: ctx, cancel: cancel,
	}
	s.Session = sess
	s.Page = PageStudio
	if !s.Tools.Ready() {
		sess.Loading = false
		sess.Err = media.ErrNoFFmpeg.Error()
		return
	}
	go s.load(sess)
}

// Close closes the open file and stops its previews.
func (s *Store) Close() {
	sess := s.Session
	if sess == nil {
		return
	}
	sess.cancel()
	s.CancelPreview()
	s.Player.Stop()
	sess.Preview.Audio.Discard()
	s.Session = nil
}

// load probes the file and builds what the studio shows for it.
func (s *Store) load(sess *Session) {
	ctx := sess.ctx
	info, err := s.Tools.Probe(ctx, sess.Path)
	if err != nil {
		s.post(func() { sess.Loading, sess.Err = false, err.Error() })
		return
	}
	switch sess.Kind {
	case media.Image:
		img, err := s.Tools.LoadImage(ctx, sess.Path)
		s.post(func() {
			sess.Info, sess.Loading = info, false
			if err != nil {
				sess.Err = err.Error()
				return
			}
			sess.Picture = img
			sess.Info.Width, sess.Info.Height = img.Bounds().Dx(), img.Bounds().Dy()
		})
	case media.Video:
		if info.Width == 0 {
			s.post(func() { sess.Loading, sess.Err = false, tr.T("This file has no video stream") })
			return
		}
		at := min(info.Duration*0.2, 5)
		frame, err := s.Tools.FrameAt(ctx, sess.Path, at, 0)
		s.post(func() {
			sess.Info, sess.Loading, sess.Time = info, false, at
			if err != nil {
				sess.Err = err.Error()
				return
			}
			sess.Picture = frame
		})
		film := s.Tools.Filmstrip(ctx, sess.Path, info.Duration, 14, 96)
		s.post(func() { sess.Film = film })
	case media.Audio:
		if !info.HasAudio {
			s.post(func() { sess.Loading, sess.Err = false, tr.T("This file has no audio stream") })
			return
		}
		peaks, err := s.Tools.Waveform(ctx, sess.Path, 1400)
		s.post(func() {
			sess.Info, sess.Loading = info, false
			if err != nil {
				sess.Err = err.Error()
				return
			}
			sess.Peaks = peaks
			sess.SelLen = min(defaultSection, info.Duration)
			sess.SelStart = max(0, min(info.Duration*0.25, info.Duration-sess.SelLen))
		})
	}
}

// Seek moves the video to t seconds and loads that frame.
func (s *Store) Seek(t float64) {
	sess := s.Session
	if sess == nil || sess.Kind != media.Video {
		return
	}
	sess.Time = max(0, min(t, sess.Info.Duration-0.04))
	sess.frameSeq++
	seq := sess.frameSeq
	sess.FrameLoading = true
	at := sess.Time
	go func() {
		frame, err := s.Tools.FrameAt(sess.ctx, sess.Path, at, 0)
		s.post(func() {
			if seq != sess.frameSeq {
				return
			}
			sess.FrameLoading = false
			if err == nil {
				sess.Picture = frame
			}
		})
	}()
}

// SetSection moves the audio section to start at start, lasting length.
func (s *Store) SetSection(start, length float64) {
	sess := s.Session
	if sess == nil || sess.Kind != media.Audio {
		return
	}
	dur := sess.Info.Duration
	sess.SelLen = max(0.5, min(length, dur))
	sess.SelStart = max(0, min(start, dur-sess.SelLen))
}

// PreviewMatches reports whether the preview result belongs to what's
// selected now: the same video frame or audio section.
func (sess *Session) PreviewMatches() bool {
	p := sess.Preview
	switch sess.Kind {
	case media.Video:
		return p.Frame != nil && abs(p.At-sess.Time) < 0.02
	case media.Audio:
		return p.Audio != nil && abs(p.At-sess.SelStart) < 0.02 && abs(p.Length-sess.SelLen) < 0.02
	}
	return false
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
