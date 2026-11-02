package pipeline

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/tr"
)

// picture upscales the w×h PNG at src and returns a PNG of the planned
// size. Engines that can't split a picture themselves get pictures over
// their touchup.tileImage limit in overlapping parts, joined with
// feathered seams, so a large photo doesn't need more GPU memory than
// the engine has.
func (o Options) picture(ctx context.Context, ws *workspace, src string, w, h int, report Reporter, stage string, from, to float64) (string, error) {
	sz := o.size(w, h)
	limit := o.tileLimit()
	if limit <= 0 || (sz.Width <= limit && sz.Height <= limit) {
		j := o.job(src, filepath.Join(ws.out, "picture"+o.imageExt()), ws.in, ws.out, report, stage, from, to)
		j.OutputExt = o.imageExt()
		result, err := o.Engine.Run(ctx, o.sized(j, w, h))
		if err != nil || !sz.Exact {
			return result, err
		}
		sized := filepath.Join(ws.out, "picture-sized.png")
		return sized, o.Tools.ConvertImageSized(ctx, result, sized, sz.Width, sz.Height)
	}

	in, err := readPNG(src)
	if err != nil {
		return "", err
	}
	fx, fy := float64(sz.Width)/float64(w), float64(sz.Height)/float64(h)
	tile := max(16, int(float64(limit)/max(fx, fy)))
	overlap := max(8, tile/8)
	xs, ys := starts(w, tile, overlap), starts(h, tile, overlap)
	canvas := image.NewNRGBA(image.Rect(0, 0, sz.Width, sz.Height))
	outX := func(x int) int { return int(math.Round(float64(x) * fx)) }
	outY := func(y int) int { return int(math.Round(float64(y) * fy)) }

	type part struct {
		x0, y0, x1, y1, row, col int
		dir                      string
	}
	var parts []part
	for row, y0 := range ys {
		for col, x0 := range xs {
			n := len(parts)
			p := part{x0, y0, min(w, x0+tile), min(h, y0+tile), row, col, ws.path("part-" + strconv.Itoa(n))}
			if err := os.MkdirAll(p.dir, 0o755); err != nil {
				return "", err
			}
			parts = append(parts, p)
		}
	}
	// crop writes a part's piece of the source to path.
	crop := func(p part, path string) error {
		c := image.NewNRGBA(image.Rect(0, 0, p.x1-p.x0, p.y1-p.y0))
		draw.Draw(c, c.Bounds(), in, image.Pt(p.x0, p.y0), draw.Src)
		return writePNG(path, c)
	}
	// sizedJob is a job for a part, told the part's share of the result.
	sizedJob := func(j engine.Job, p part) engine.Job {
		j.OutputExt = o.imageExt()
		j = o.sized(j, p.x1-p.x0, p.y1-p.y0)
		j.Scale, j.OutWidth, j.OutHeight = sz.Scale, outX(p.x1)-outX(p.x0), outY(p.y1)-outY(p.y0)
		return j
	}

	results := make([]string, len(parts))
	if o.Engine.Batch {
		// One run over a folder of the parts, which all share a size:
		// the engine starts and loads its model once, not once a part.
		inDir, outDir := ws.path("parts-in"), ws.path("parts-out")
		for _, d := range []string{inDir, outDir} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return "", err
			}
		}
		names := make([]string, len(parts))
		for i, p := range parts {
			names[i] = fmt.Sprintf("part-%04d", i)
			if err := crop(p, filepath.Join(inDir, names[i]+".png")); err != nil {
				return "", err
			}
		}
		j := sizedJob(o.job(inDir, outDir, inDir, outDir, report, stage, from, to), parts[0])
		j.Progress = nil // the engine's own counts restart with every part
		stop := watchFolder(outDir, j.OutputExt, len(parts), func(f float64) {
			report(Update{Stage: stage, Frac: from + (to-from)*f})
		})
		_, err := o.Engine.Run(ctx, j)
		stop()
		if err != nil {
			return "", err
		}
		for i, name := range names {
			results[i] = filepath.Join(outDir, name+j.OutputExt)
			if _, err := os.Stat(results[i]); err != nil {
				return "", fmt.Errorf("%s: %w", tr.T("part {part} of {total}", "part", i+1, "total", len(parts)), err)
			}
		}
	}

	for n, p := range parts {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		result := results[n]
		if result == "" {
			label := stage + " · " + tr.T("part {part} of {total}", "part", n+1, "total", len(parts))
			pfrom := from + (to-from)*float64(n)/float64(len(parts))
			pto := from + (to-from)*float64(n+1)/float64(len(parts))
			report(Update{Stage: label, Frac: pfrom})
			tileIn := filepath.Join(p.dir, "in.png")
			if err := crop(p, tileIn); err != nil {
				return "", err
			}
			j := sizedJob(o.job(tileIn, filepath.Join(p.dir, "out"+o.imageExt()), p.dir, p.dir, report, label, pfrom, pto), p)
			var err error
			if result, err = o.Engine.Run(ctx, j); err != nil {
				return "", err
			}
		}
		ox0, oy0, ox1, oy1 := outX(p.x0), outY(p.y0), outX(p.x1), outY(p.y1)
		// Each part is brought to exactly its share of the result.
		exact := filepath.Join(p.dir, "exact.png")
		if err := o.Tools.ConvertImageSized(ctx, result, exact, ox1-ox0, oy1-oy0); err != nil {
			return "", err
		}
		up, err := readPNG(exact)
		if err != nil {
			return "", err
		}
		// Fade in over what the parts to the left and above left.
		left, top := 0, 0
		if p.col > 0 {
			left = outX(min(w, xs[p.col-1]+tile)) - ox0
		}
		if p.row > 0 {
			top = outY(min(h, ys[p.row-1]+tile)) - oy0
		}
		blend(canvas, up, image.Pt(ox0, oy0), left, top)
	}
	report(Update{Stage: stage, Frac: to})
	joined := filepath.Join(ws.out, "picture-joined.png")
	return joined, writePNG(joined, canvas)
}

// tileLimit is the engine's touchup.tileImage value: the largest output
// a side made in one run, or 0 for no limit.
func (o Options) tileLimit() int {
	v, err := strconv.Atoi(o.EngineOptions[engine.OptionTileImage])
	if err != nil {
		return 0
	}
	return v
}

// starts returns where tiles of size tile, sharing overlap pixels, begin
// along n pixels; the last ends at the edge.
func starts(n, tile, overlap int) []int {
	if n <= tile {
		return []int{0}
	}
	var out []int
	step := max(1, tile-overlap)
	for x := 0; x+tile < n; x += step {
		out = append(out, x)
	}
	return append(out, n-tile)
}

// blend draws src onto dst at at, fading it in over the first left
// columns and top rows, where an earlier part already drew.
func blend(dst, src *image.NRGBA, at image.Point, left, top int) {
	b := src.Bounds()
	for y := 0; y < b.Dy(); y++ {
		ay := 1.0
		if y < top {
			ay = (float64(y) + 0.5) / float64(top)
		}
		for x := 0; x < b.Dx(); x++ {
			a := ay
			if x < left {
				a *= (float64(x) + 0.5) / float64(left)
			}
			s := src.PixOffset(x, y)
			d := dst.PixOffset(at.X+x, at.Y+y)
			if a >= 1 {
				copy(dst.Pix[d:d+4], src.Pix[s:s+4])
				continue
			}
			for c := 0; c < 4; c++ {
				dst.Pix[d+c] = uint8(math.Round(float64(dst.Pix[d+c])*(1-a) + float64(src.Pix[s+c])*a))
			}
		}
	}
}

func readPNG(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filepath.Base(path), err)
	}
	if n, ok := img.(*image.NRGBA); ok {
		return n, nil
	}
	n := image.NewNRGBA(img.Bounds())
	draw.Draw(n, n.Bounds(), img, img.Bounds().Min, draw.Src)
	return n, nil
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
