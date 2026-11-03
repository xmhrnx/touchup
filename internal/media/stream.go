package media

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mehran9675/touchup/internal/proc"
)

// Videos are upscaled a few frames at a time, so their frames never all
// sit on disk: a FrameReader hands out the source's frames one by one
// from a single FFmpeg decode, and a FrameWriter encodes the results as
// they come, with the source's audio.

// FrameReader decodes a video's frames as PNGs, one at a time. FFmpeg
// waits while frames aren't being read, so nothing piles up.
type FrameReader struct {
	cmd    *exec.Cmd
	out    *bufio.Reader
	stderr *bytes.Buffer
	cancel context.CancelFunc
}

// ReadFrames starts decoding the first video stream of path.
func (t Tools) ReadFrames(ctx context.Context, path string) (*FrameReader, error) {
	if t.FFmpeg == "" {
		return nil, ErrNoFFmpeg
	}
	ctx, cancel := context.WithCancel(ctx)
	cmd := proc.Command(ctx, t.FFmpeg, "-hide_banner", "-nostdin", "-loglevel", "error",
		"-i", path, "-map", "0:v:0", "-fps_mode", "passthrough", "-pix_fmt", "rgb24",
		// Light compression: frames are read once and deleted.
		"-f", "image2pipe", "-c:v", "png", "-compression_level", "1", "pipe:1")
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	return &FrameReader{cmd: cmd, out: bufio.NewReaderSize(out, 1<<20), stderr: stderr, cancel: cancel}, nil
}

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// Next returns the next frame as a PNG file's bytes, or io.EOF after the
// last.
func (r *FrameReader) Next() ([]byte, error) {
	var buf bytes.Buffer
	sig := make([]byte, len(pngSignature))
	if _, err := io.ReadFull(r.out, sig); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, r.finish()
		}
		return nil, r.fail(err)
	}
	if !bytes.Equal(sig, pngSignature) {
		return nil, r.fail(errors.New("FFmpeg sent something that isn't a PNG"))
	}
	buf.Write(sig)
	head := make([]byte, 8)
	for {
		if _, err := io.ReadFull(r.out, head); err != nil {
			return nil, r.fail(err)
		}
		n := binary.BigEndian.Uint32(head[:4])
		buf.Write(head)
		// The chunk's data and its CRC.
		if _, err := io.CopyN(&buf, r.out, int64(n)+4); err != nil {
			return nil, r.fail(err)
		}
		if string(head[4:]) == "IEND" {
			return buf.Bytes(), nil
		}
	}
}

// finish waits for FFmpeg after the last frame and reports how it ended.
func (r *FrameReader) finish() error {
	if err := r.cmd.Wait(); err != nil {
		return toolError("ffmpeg", err, r.stderr.String())
	}
	return io.EOF
}

func (r *FrameReader) fail(err error) error {
	r.Close()
	if msg := r.stderr.String(); msg != "" {
		return toolError("ffmpeg", err, msg)
	}
	return fmt.Errorf("reading frames: %w", err)
}

// Close stops decoding.
func (r *FrameReader) Close() {
	r.cancel()
	r.cmd.Wait()
}

// FrameWriter encodes PNG frames into a video as they're written.
type FrameWriter struct {
	cmd    *exec.Cmd
	in     io.WriteCloser
	stderr *bytes.Buffer
	cancel context.CancelFunc
	frames int
}

// WriteFrames starts encoding a video at out from frames in the format
// of o.FrameExt (PNG by default), with o's settings; Duration is unused.
func (t Tools) WriteFrames(ctx context.Context, out string, o JoinOptions) (*FrameWriter, error) {
	if t.FFmpeg == "" {
		return nil, ErrNoFFmpeg
	}
	args := []string{"-hide_banner", "-loglevel", "error", "-y",
		"-f", "image2pipe", "-framerate", strconv.FormatFloat(o.FPS, 'f', 6, 64), "-c:v", frameCodec(o.FrameExt), "-i", "pipe:0"}
	args = append(args, encodeArgs(o)...)
	args = append(args, out)
	ctx, cancel := context.WithCancel(ctx)
	cmd := proc.Command(ctx, t.FFmpeg, args...)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	return &FrameWriter{cmd: cmd, in: in, stderr: stderr, cancel: cancel}, nil
}

// frameCodec is FFmpeg's decoder for image files with extension ext.
func frameCodec(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "mjpeg"
	case ".webp":
		return "webp"
	case ".bmp":
		return "bmp"
	case ".tif", ".tiff":
		return "tiff"
	}
	return "png"
}

// Write encodes the next frame, an image file's bytes.
func (w *FrameWriter) Write(png []byte) error {
	if _, err := w.in.Write(png); err != nil {
		w.Abort()
		if msg := w.stderr.String(); msg != "" {
			return toolError("ffmpeg", err, msg)
		}
		return err
	}
	w.frames++
	return nil
}

// Frames is how many frames have been written.
func (w *FrameWriter) Frames() int { return w.frames }

// Close finishes the video.
func (w *FrameWriter) Close() error {
	w.in.Close()
	err := w.cmd.Wait()
	w.cancel()
	if err != nil {
		return toolError("ffmpeg", err, w.stderr.String())
	}
	return nil
}

// Abort stops encoding, leaving an unfinished file.
func (w *FrameWriter) Abort() {
	w.cancel()
	w.in.Close()
	w.cmd.Wait()
}
