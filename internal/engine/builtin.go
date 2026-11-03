package engine

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // decoders for pictures given to built-in engines
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/mehran9675/touchup/internal/filters"
)

// Built-in engines (install type "builtin") are written in Go and run
// inside TouchUp: nothing to install or download. "classic" upscales
// without AI, with the filters package's scalers and sharpening;
// "rtxvideo" hands frames to NVIDIA's RTX Video Super Resolution in the
// graphics driver (rtxvideo_windows.go).

// Builtin reports whether the engine runs inside TouchUp.
func (m *Manifest) Builtin() bool { return m.Install.Type == "builtin" }

// Upscaler upscales pictures inside TouchUp. One is made for a job and
// closed after it; it may keep state, like a GPU device, in between.
type Upscaler interface {
	// Upscale returns img scaled to w×h.
	Upscale(img *filters.Image, w, h int) (*filters.Image, error)
	Close()
}

// NewUpscaler returns the built-in engine's upscaler, with its option
// values.
func (m *Manifest) NewUpscaler(values map[string]string) (Upscaler, error) {
	switch m.ID {
	case "classic":
		return classicUpscaler{m, values}, nil
	case "rtxvideo":
		return newRTXVideo()
	}
	return nil, fmt.Errorf("no built-in engine %q", m.ID)
}

// builtinSupported reports whether a built-in engine works on this
// system.
func builtinSupported(id string) bool {
	return id != "rtxvideo" || rtxVideoSupported
}

type classicUpscaler struct {
	m      *Manifest
	values map[string]string
}

func (c classicUpscaler) Upscale(img *filters.Image, w, h int) (*filters.Image, error) {
	return c.m.Classic(img, w, h, c.values), nil
}

func (classicUpscaler) Close() {}

// ClassicSettings are the classic engine's option values: the scaling
// method and how much detail to bring out, in [0, 1].
func (m *Manifest) ClassicSettings(values map[string]string) (method string, detail float32) {
	method = m.optionValue(values, "method")
	if d, err := strconv.Atoi(m.optionValue(values, "detail")); err == nil {
		detail = float32(d) / 100
	}
	return method, detail
}

// Classic upscales m to w×h with the classic engine's settings.
func (m *Manifest) Classic(img *filters.Image, w, h int, values map[string]string) *filters.Image {
	method, detail := m.ClassicSettings(values)
	out := filters.Resize(img, w, h, method)
	if w > img.W || h > img.H {
		filters.Sharpen(out, detail)
	}
	return out
}

func (m *Manifest) optionValue(values map[string]string, id string) string {
	if v, ok := values[id]; ok {
		return v
	}
	if o := m.OptionByID(id); o != nil {
		return o.Default
	}
	return ""
}

// runBuiltin runs a built-in engine on a picture, or on every picture in
// a folder for batch runs.
func (m *Manifest) runBuiltin(ctx context.Context, j Job) (string, error) {
	up, err := m.NewUpscaler(j.Options)
	if err != nil {
		return "", err
	}
	defer up.Close()
	one := func(in, out string) error {
		img, err := loadPicture(in)
		if err != nil {
			return err
		}
		w, h := j.OutWidth, j.OutHeight
		if w <= 0 || h <= 0 {
			w, h = img.W*max(1, j.Scale), img.H*max(1, j.Scale)
		}
		res, err := up.Upscale(img, w, h)
		if err != nil {
			return err
		}
		return savePNG(out, res.NRGBA())
	}
	info, err := os.Stat(j.Input)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return j.Output, one(j.Input, j.Output)
	}
	entries, err := os.ReadDir(j.Input)
	if err != nil {
		return "", err
	}
	var names []string
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !e.IsDir() && slices.Contains([]string{".png", ".jpg", ".jpeg", ".webp", ".bmp", ".tif", ".tiff"}, ext) {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	if err := os.MkdirAll(j.Output, 0o755); err != nil {
		return "", err
	}
	for i, name := range names {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		out := filepath.Join(j.Output, strings.TrimSuffix(name, filepath.Ext(name))+".png")
		if err := one(filepath.Join(j.Input, name), out); err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		if j.Progress != nil {
			j.Progress(float64(i+1) / float64(len(names)))
		}
	}
	return j.Output, nil
}

func loadPicture(path string) (*filters.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filepath.Base(path), err)
	}
	return filters.FromImage(img), nil
}

func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
