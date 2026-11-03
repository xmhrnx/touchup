package filters

// Sharpen brings out detail without the halos of an unsharp mask, with
// AMD's robust contrast-adaptive sharpening (RCAS, from FidelityFX
// Super Resolution 1, MIT license): each pixel is pushed away from its
// four neighbors only as far as keeps it inside their range, and less
// where the area looks like noise. amount is in [0, 1].
func Sharpen(m *Image, amount float32) {
	if amount <= 0 {
		return
	}
	const limit = 0.25 - 1.0/16 // the strongest lobe RCAS allows
	src := m.Clone()
	rows(m.H, func(y0, y1 int) {
		var b, d, e, f, h [3]float32
		for y := y0; y < y1; y++ {
			for x := 0; x < m.W; x++ {
				for c := 0; c < 3; c++ {
					b[c], d[c], e[c] = src.at(x, y-1, c), src.at(x-1, y, c), src.at(x, y, c)
					f[c], h[c] = src.at(x+1, y, c), src.at(x, y+1, c)
				}
				lobe := float32(-limit)
				lobeMax := float32(-1e9)
				for c := 0; c < 3; c++ {
					mn := min(b[c], d[c], f[c], h[c])
					mx := max(b[c], d[c], f[c], h[c])
					hitMin := mn / (4 * max(mx, 1e-5))
					hitMax := (1 - mx) / min(4*mn-4, -1e-5)
					lobeMax = max(lobeMax, max(-hitMin, hitMax))
				}
				lobe = max(lobe, min(lobeMax, 0))
				// Less in noise: where the center stands out from its
				// neighbors relative to the local range.
				bl, dl, el, fl, hl := luma(b[0], b[1], b[2]), luma(d[0], d[1], d[2]), luma(e[0], e[1], e[2]), luma(f[0], f[1], f[2]), luma(h[0], h[1], h[2])
				nz := abs32(0.25*(bl+dl+fl+hl) - el)
				rng := max(bl, dl, el, fl, hl) - min(bl, dl, el, fl, hl)
				if rng > 1e-5 {
					nz = clamp01(nz / rng)
				} else {
					nz = 0
				}
				lobe *= (1 - 0.5*nz) * amount
				i := (y*m.W + x) * 3
				for c := 0; c < 3; c++ {
					m.Pix[i+c] = clamp01((lobe*(b[c]+d[c]+f[c]+h[c]) + e[c]) / (4*lobe + 1))
				}
			}
		}
	})
}

// Color adjusts brightness, contrast and warmth, each in [-1, 1].
func Color(m *Image, brightness, contrast, warmth float32) {
	if brightness == 0 && contrast == 0 && warmth == 0 {
		return
	}
	k := 1 + contrast
	if contrast > 0 {
		k = 1 + 2*contrast
	}
	lift := brightness * 0.25
	wr, wb := 1+0.12*warmth, 1-0.12*warmth
	rows(m.H, func(y0, y1 int) {
		for i := y0 * m.W * 3; i < y1*m.W*3; i += 3 {
			r := (m.Pix[i]-0.5)*k + 0.5 + lift
			g := (m.Pix[i+1]-0.5)*k + 0.5 + lift
			b := (m.Pix[i+2]-0.5)*k + 0.5 + lift
			m.Pix[i], m.Pix[i+1], m.Pix[i+2] = clamp01(r*wr), clamp01(g), clamp01(b*wb)
		}
	})
}

// Saturate scales colors away from (amount > 0) or toward (amount < 0)
// gray of the same brightness; amount is in [-1, 1], where -1 is black
// and white.
func Saturate(m *Image, amount float32) {
	if amount == 0 {
		return
	}
	k := 1 + amount
	rows(m.H, func(y0, y1 int) {
		for i := y0 * m.W * 3; i < y1*m.W*3; i += 3 {
			l := luma(m.Pix[i], m.Pix[i+1], m.Pix[i+2])
			for c := 0; c < 3; c++ {
				m.Pix[i+c] = clamp01(l + (m.Pix[i+c]-l)*k)
			}
		}
	})
}

// Grain adds film grain: soft, monochrome noise that's strongest in the
// midtones, as on film, and different every frame. amount is in [0, 1];
// size is the grain's size in pixels, at least 1.
func Grain(m *Image, amount, size float32, frame int) {
	if amount <= 0 {
		return
	}
	size = max(1, size)
	// The grain is drawn on a coarser grid for bigger grains and blended
	// between its points.
	gw, gh := int(float32(m.W)/size)+2, int(float32(m.H)/size)+2
	field := make([]float32, gw*gh)
	for y := 0; y < gh; y++ {
		for x := 0; x < gw; x++ {
			u1 := noise(uint32(x), uint32(y), uint32(frame), 11)
			u2 := noise(uint32(x), uint32(y), uint32(frame), 12)
			field[y*gw+x] = gauss(u1, u2)
		}
	}
	strength := amount * 0.08
	rows(m.H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			fy := float32(y) / size
			gy := int(fy)
			ty := fy - float32(gy)
			for x := 0; x < m.W; x++ {
				fx := float32(x) / size
				gx := int(fx)
				tx := fx - float32(gx)
				n := (field[gy*gw+gx]*(1-tx)+field[gy*gw+gx+1]*tx)*(1-ty) +
					(field[(gy+1)*gw+gx]*(1-tx)+field[(gy+1)*gw+gx+1]*tx)*ty
				i := (y*m.W + x) * 3
				l := luma(m.Pix[i], m.Pix[i+1], m.Pix[i+2])
				v := n * strength * (0.35 + 2.6*l*(1-l))
				for c := 0; c < 3; c++ {
					m.Pix[i+c] = clamp01(m.Pix[i+c] + v)
				}
			}
		}
	})
}
