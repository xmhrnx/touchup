package pipeline

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"math"
	"os"

	"github.com/mehran9675/touchup/internal/filters"
	"github.com/mehran9675/touchup/internal/media"
)

// Filters run in TouchUp itself: clean-up on the source before the
// engine, the look on its result. See the filters package.

// decodeFrame reads a picture file's bytes, PNG or any format engines
// write.
func decodeFrame(data []byte) (*filters.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return filters.FromImage(img), nil
}

// encodeFrame writes a picture as a PNG at light compression: frames are
// read once and deleted.
func encodeFrame(m *filters.Image) ([]byte, error) {
	var buf bytes.Buffer
	err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, m.NRGBA())
	return buf.Bytes(), err
}

// filterFile applies fn to the picture at path and writes the result as
// a PNG at out, which may be path.
func filterFile(path, out string, fn func(*filters.Image)) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	m, err := decodeFrame(data)
	if err != nil {
		return err
	}
	fn(m)
	data, err = encodeFrame(m)
	if err != nil {
		return err
	}
	return os.WriteFile(out, data, 0o644)
}

// filterImage applies fn to a decoded image.
func filterImage(img image.Image, fn func(*filters.Image)) image.Image {
	m := filters.FromImage(img)
	fn(m)
	return m.NRGBA()
}

// filterVideo runs every frame of src through fn and encodes the
// results to out with join's settings, reporting progress in [from, to]
// of frames frames.
func filterVideo(ctx context.Context, o Options, src, out string, frames int, join media.JoinOptions,
	fn func(m *filters.Image, frame int) *filters.Image, report Reporter, stage string, from, to float64) error {
	reader, err := o.Tools.ReadFrames(ctx, src)
	if err != nil {
		return err
	}
	defer reader.Close()
	var writer *media.FrameWriter
	done := false
	defer func() {
		if writer != nil && !done {
			writer.Abort()
			os.Remove(out)
		}
	}()
	var buf []byte
	for i := 0; ; i++ {
		data, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		m, err := decodeFrame(data)
		if err != nil {
			return err
		}
		m = fn(m, i)
		if writer == nil {
			join.RawWidth, join.RawHeight = m.W, m.H
			if writer, err = o.Tools.WriteFrames(ctx, out, join); err != nil {
				return err
			}
		}
		buf = m.RGB24(buf)
		if err := writer.Write(buf); err != nil {
			return err
		}
		if i%4 == 0 {
			report(Update{Stage: stage, Frac: from + (to-from)*math.Min(1, float64(i+1)/float64(max(1, frames)))})
		}
	}
	if writer == nil {
		return errors.New("no frames could be read from the video")
	}
	done = true
	return writer.Close()
}
