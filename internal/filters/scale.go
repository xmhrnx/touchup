package filters

import "math"

// Methods of Resize.
const (
	// EdgeAdaptive follows edges: each output pixel is interpolated with
	// a kernel stretched along the local edge and kept within its four
	// nearest source pixels, so lines stay crisp without stairs or halos.
	// It's AMD's EASU, the upscaler of FidelityFX Super Resolution 1 (MIT
	// license), and works the way TVs' scalers do: detail looks sharper
	// without any being invented.
	EdgeAdaptive = "edge"
	// Lanczos is the classic sharp windowed-sinc resampler.
	Lanczos = "lanczos"
	// Bicubic is the soft, safe resampler most programs use.
	Bicubic = "bicubic"
)

// Resize scales m to w×h with method. EdgeAdaptive enlarges at most 2x
// a pass, so bigger enlargements go in steps, which keeps edges clean.
func Resize(m *Image, w, h int, method string) *Image {
	if w == m.W && h == m.H {
		return m.Clone()
	}
	var out *Image
	switch method {
	case EdgeAdaptive:
		if w < m.W || h < m.H { // shrinking: the kernel isn't made for it
			out = resizeSeparable(m, w, h, lanczos3, 3)
			break
		}
		out = m
		for out.W != w || out.H != h {
			out = easu(out, min(w, out.W*2), min(h, out.H*2))
		}
	case Bicubic:
		out = resizeSeparable(m, w, h, catmullRom, 2)
	default:
		out = resizeSeparable(m, w, h, lanczos3, 3)
	}
	if m.Alpha != nil {
		out.Alpha = resizeAlpha(m, w, h)
	}
	return out
}

func easu(m *Image, w, h int) *Image {
	out := NewImage(w, h)
	sx, sy := float32(m.W)/float32(w), float32(m.H)/float32(h)
	// The 12 taps around the 2×2 quad f g / j k:
	//      b c
	//    e f g h
	//    i j k l
	//      n o
	type tap struct{ x, y int }
	taps := [12]tap{{0, -1}, {1, -1}, {-1, 0}, {0, 0}, {1, 0}, {2, 0}, {-1, 1}, {0, 1}, {1, 1}, {2, 1}, {0, 2}, {1, 2}}
	const b, c, e, f, g, hh, i, j, k, l, n, o = 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11
	rows(h, func(y0, y1 int) {
		var lum [12]float32
		var col [12][3]float32
		for y := y0; y < y1; y++ {
			py := (float32(y)+0.5)*sy - 0.5
			fy := float32(math.Floor(float64(py)))
			ty := py - fy
			for x := 0; x < w; x++ {
				px := (float32(x)+0.5)*sx - 0.5
				fx := float32(math.Floor(float64(px)))
				tx := px - fx
				ix, iy := int(fx), int(fy)
				for t, p := range taps {
					for ch := 0; ch < 3; ch++ {
						col[t][ch] = m.at(ix+p.x, iy+p.y, ch)
					}
					// EASU's cheap luma: green plus half red and blue.
					lum[t] = col[t][1] + 0.5*(col[t][0]+col[t][2])
				}
				// Direction and length of the edge, from gradients at the
				// four quad pixels, blended bilinearly.
				var dirX, dirY, length float32
				set := func(w, a, bl, cc, d, ee float32) {
					// a above, bl left, cc center, d right, ee below.
					dc, cb := d-cc, cc-bl
					lenX := max(abs32(dc), abs32(cb))
					if lenX > 0 {
						lenX = 1 / lenX
					}
					dx := d - bl
					dirX += dx * w
					lenX = clamp01(abs32(dx) * lenX)
					length += lenX * lenX * w
					ec, ca := ee-cc, cc-a
					lenY := max(abs32(ec), abs32(ca))
					if lenY > 0 {
						lenY = 1 / lenY
					}
					dy := ee - a
					dirY += dy * w
					lenY = clamp01(abs32(dy) * lenY)
					length += lenY * lenY * w
				}
				set((1-tx)*(1-ty), lum[b], lum[e], lum[f], lum[g], lum[j])
				set(tx*(1-ty), lum[c], lum[f], lum[g], lum[hh], lum[k])
				set((1-tx)*ty, lum[f], lum[i], lum[j], lum[k], lum[n])
				set(tx*ty, lum[g], lum[j], lum[k], lum[l], lum[o])

				d2 := dirX*dirX + dirY*dirY
				if d2 < 1.0/32768 {
					dirX, dirY = 1, 0
				} else {
					r := 1 / float32(math.Sqrt(float64(d2)))
					dirX, dirY = dirX*r, dirY*r
				}
				length *= 0.5
				length *= length
				// Stretch the kernel along the edge, and sharpen it across
				// edges (anisotropy and the lobe shape grow with length).
				stretch := (dirX*dirX + dirY*dirY) / max(abs32(dirX), abs32(dirY))
				lenX2 := 1 + (stretch-1)*length
				lenY2 := 1 - 0.5*length
				lob := 0.5 + ((1.0/4-0.04)-0.5)*length
				clp := 1 / lob

				var acc [3]float32
				var wsum float32
				for t, p := range taps {
					ox, oy := float32(p.x)-tx, float32(p.y)-ty
					vx := (ox*dirX + oy*dirY) * lenX2
					vy := (ox*-dirY + oy*dirX) * lenY2
					dd := min(vx*vx+vy*vy, clp)
					wB := 2.0/5*dd - 1
					wA := lob*dd - 1
					wB *= wB
					wA *= wA
					wB = 25.0/16*wB - (25.0/16 - 1)
					wt := wB * wA
					for ch := 0; ch < 3; ch++ {
						acc[ch] += col[t][ch] * wt
					}
					wsum += wt
				}
				oi := (y*w + x) * 3
				for ch := 0; ch < 3; ch++ {
					v := acc[ch] / wsum
					// No ringing: stay within the quad's range.
					lo := min(col[f][ch], col[g][ch], col[j][ch], col[k][ch])
					hi := max(col[f][ch], col[g][ch], col[j][ch], col[k][ch])
					out.Pix[oi+ch] = max(lo, min(hi, v))
				}
			}
		}
	})
	return out
}

func lanczos3(x float64) float64 {
	x = math.Abs(x)
	if x < 1e-8 {
		return 1
	}
	if x >= 3 {
		return 0
	}
	px := math.Pi * x
	return 3 * math.Sin(px) * math.Sin(px/3) / (px * px)
}

func catmullRom(x float64) float64 {
	x = math.Abs(x)
	switch {
	case x < 1:
		return 1.5*x*x*x - 2.5*x*x + 1
	case x < 2:
		return -0.5*x*x*x + 2.5*x*x - 4*x + 2
	}
	return 0
}

// weights are the taps of a 1-D resampling: for each output position,
// the first source index and the normalized weights from it.
type weights struct {
	first []int
	w     [][]float32
}

func makeWeights(src, dst int, kernel func(float64) float64, support float64) weights {
	scale := float64(src) / float64(dst)
	filterScale := max(1, scale) // widen the kernel when shrinking
	radius := support * filterScale
	ws := weights{first: make([]int, dst), w: make([][]float32, dst)}
	for o := 0; o < dst; o++ {
		center := (float64(o)+0.5)*scale - 0.5
		lo := int(math.Floor(center - radius + 1))
		hi := int(math.Floor(center + radius))
		row := make([]float32, 0, hi-lo+1)
		var sum float64
		vals := make([]float64, 0, hi-lo+1)
		for s := lo; s <= hi; s++ {
			v := kernel((float64(s) - center) / filterScale)
			vals = append(vals, v)
			sum += v
		}
		for _, v := range vals {
			row = append(row, float32(v/sum))
		}
		ws.first[o], ws.w[o] = lo, row
	}
	return ws
}

func resizeSeparable(m *Image, w, h int, kernel func(float64) float64, support float64) *Image {
	// Horizontal pass into a w×m.H picture, then the vertical pass.
	wx := makeWeights(m.W, w, kernel, support)
	mid := NewImage(w, m.H)
	rows(m.H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < w; x++ {
				var acc [3]float32
				for t, wt := range wx.w[x] {
					sx := clampInt(wx.first[x]+t, 0, m.W-1)
					i := (y*m.W + sx) * 3
					acc[0] += m.Pix[i] * wt
					acc[1] += m.Pix[i+1] * wt
					acc[2] += m.Pix[i+2] * wt
				}
				o := (y*w + x) * 3
				mid.Pix[o], mid.Pix[o+1], mid.Pix[o+2] = acc[0], acc[1], acc[2]
			}
		}
	})
	wy := makeWeights(m.H, h, kernel, support)
	out := NewImage(w, h)
	rows(h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < w; x++ {
				var acc [3]float32
				for t, wt := range wy.w[y] {
					sy := clampInt(wy.first[y]+t, 0, m.H-1)
					i := (sy*w + x) * 3
					acc[0] += mid.Pix[i] * wt
					acc[1] += mid.Pix[i+1] * wt
					acc[2] += mid.Pix[i+2] * wt
				}
				o := (y*w + x) * 3
				out.Pix[o], out.Pix[o+1], out.Pix[o+2] = clamp01(acc[0]), clamp01(acc[1]), clamp01(acc[2])
			}
		}
	})
	return out
}

// resizeAlpha scales the alpha channel bilinearly.
func resizeAlpha(m *Image, w, h int) []uint8 {
	out := make([]uint8, w*h)
	sx, sy := float64(m.W)/float64(w), float64(m.H)/float64(h)
	rows(h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			fy := max(0, (float64(y)+0.5)*sy-0.5)
			iy := min(int(fy), m.H-1)
			ty := fy - float64(iy)
			iy2 := min(iy+1, m.H-1)
			for x := 0; x < w; x++ {
				fx := max(0, (float64(x)+0.5)*sx-0.5)
				ix := min(int(fx), m.W-1)
				tx := fx - float64(ix)
				ix2 := min(ix+1, m.W-1)
				a := func(xx, yy int) float64 { return float64(m.Alpha[yy*m.W+xx]) }
				v := (a(ix, iy)*(1-tx)+a(ix2, iy)*tx)*(1-ty) + (a(ix, iy2)*(1-tx)+a(ix2, iy2)*tx)*ty
				out[y*w+x] = uint8(math.Round(v))
			}
		}
	})
	return out
}
