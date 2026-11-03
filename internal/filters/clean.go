package filters

import "math"

// Deinterlace rebuilds the second field of an interlaced frame from the
// first: each of its lines is interpolated from the lines above and
// below, along whichever nearby direction matches best, so diagonal
// edges stay smooth instead of turning into stairs (edge-based line
// averaging). bottomFirst keeps the odd lines instead of the even.
func Deinterlace(m *Image, bottomFirst bool) {
	src := m.Clone()
	start := 1
	if bottomFirst {
		start = 0
	}
	rows(m.H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			if y%2 != start%2 {
				continue
			}
			up, down := y-1, y+1
			if up < 0 {
				up = down
			}
			if down >= m.H {
				down = up
			}
			for x := 0; x < m.W; x++ {
				best, bestD := float32(math.MaxFloat32), 0
				for _, d := range [...]int{0, -1, 1, -2, 2} {
					diff := abs32(src.luma(x+d, up) - src.luma(x-d, down))
					// A small bias to the vertical keeps flat areas from
					// picking a direction by chance.
					if d != 0 {
						diff += 0.01 * float32(d*d)
					}
					if diff < best {
						best, bestD = diff, d
					}
				}
				for c := 0; c < 3; c++ {
					m.Pix[(y*m.W+x)*3+c] = (src.at(x+bestD, up, c) + src.at(x-bestD, down, c)) / 2
				}
			}
		}
	})
}

// Deblock smooths the 8-pixel block edges that heavy compression leaves,
// where the step across an edge is small enough to be an artifact
// rather than a real edge, as video codecs' own deblocking filters do.
// strength is in [0, 1].
func Deblock(m *Image, strength float32) {
	if strength <= 0 {
		return
	}
	alpha := 0.015 + 0.10*strength // the most a step across the edge may be
	beta := 0.008 + 0.035*strength // the most either side may vary
	tc := 0.01 + 0.05*strength     // the most a pixel may move
	// One pass across vertical edges, then one across horizontal ones.
	for _, vertical := range []bool{true, false} {
		src := m.Clone()
		lines, length := m.H, m.W
		if !vertical {
			lines, length = m.W, m.H
		}
		px := func(line, pos int) int {
			if vertical {
				return (line*m.W + pos) * 3
			}
			return (pos*m.W + line) * 3
		}
		lum := func(i int) float32 { return luma(src.Pix[i], src.Pix[i+1], src.Pix[i+2]) }
		rows(lines, func(l0, l1 int) {
			for line := l0; line < l1; line++ {
				for edge := 8; edge+1 < length; edge += 8 {
					p1, p0, q0, q1 := px(line, edge-2), px(line, edge-1), px(line, edge), px(line, edge+1)
					if abs32(lum(p0)-lum(q0)) >= alpha || abs32(lum(p1)-lum(p0)) >= beta || abs32(lum(q1)-lum(q0)) >= beta {
						continue
					}
					for c := 0; c < 3; c++ {
						a1, a0, b0, b1 := src.Pix[p1+c], src.Pix[p0+c], src.Pix[q0+c], src.Pix[q1+c]
						delta := ((b0-a0)*4 + (a1 - b1)) / 8
						delta = max(-tc, min(tc, delta))
						m.Pix[p0+c] = clamp01(a0 + delta)
						m.Pix[q0+c] = clamp01(b0 - delta)
						m.Pix[p1+c] = clamp01(a1 + delta/2)
						m.Pix[q1+c] = clamp01(b1 - delta/2)
					}
				}
			}
		})
	}
}

// Denoise smooths noise while keeping edges: each pixel becomes an
// average of its 5×5 neighborhood, weighted by distance and by how
// close each neighbor's brightness is to its own (a bilateral filter),
// so pixels across an edge barely count. strength is in [0, 1].
func Denoise(m *Image, strength float32) {
	if strength <= 0 {
		return
	}
	src := m.Clone()
	sigmaR := 0.015 + 0.09*strength
	inv := -1 / (2 * sigmaR * sigmaR)
	var spatial [5][5]float32
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			spatial[dy+2][dx+2] = float32(math.Exp(-float64(dx*dx+dy*dy) / (2 * 1.5 * 1.5)))
		}
	}
	rows(m.H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < m.W; x++ {
				c := src.luma(x, y)
				var r, g, b, wsum float32
				for dy := -2; dy <= 2; dy++ {
					for dx := -2; dx <= 2; dx++ {
						d := src.luma(x+dx, y+dy) - c
						w := spatial[dy+2][dx+2] * float32(math.Exp(float64(d*d*inv)))
						r += w * src.at(x+dx, y+dy, 0)
						g += w * src.at(x+dx, y+dy, 1)
						b += w * src.at(x+dx, y+dy, 2)
						wsum += w
					}
				}
				i := (y*m.W + x) * 3
				m.Pix[i], m.Pix[i+1], m.Pix[i+2] = r/wsum, g/wsum, b/wsum
			}
		}
	})
}

// Deband smooths the steps of color banding in skies and gradients: a
// pixel whose surroundings, sampled at a random distance and angle, are
// all within a few levels of it is replaced with their average, and a
// trace of noise keeps new steps from forming when it's saved in 8
// bits. Real detail differs by more than the threshold and is left
// alone. strength is in [0, 1].
func Deband(m *Image, strength float32, frame int) {
	if strength <= 0 {
		return
	}
	src := m.Clone()
	threshold := (1 + 4*strength) / 255
	radius := float32(6 + 14*strength)
	rows(m.H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < m.W; x++ {
				angle := noise(uint32(x), uint32(y), uint32(frame), 1) * 2 * math.Pi
				dist := (0.3 + 0.7*noise(uint32(x), uint32(y), uint32(frame), 2)) * radius
				dx := int(dist * float32(math.Cos(float64(angle))))
				dy := int(dist * float32(math.Sin(float64(angle))))
				i := (y*m.W + x) * 3
				for c := 0; c < 3; c++ {
					v := src.Pix[i+c]
					s1, s2 := src.at(x+dx, y+dy, c), src.at(x-dx, y-dy, c)
					s3, s4 := src.at(x-dy, y+dx, c), src.at(x+dy, y-dx, c)
					if abs32(s1-v) < threshold && abs32(s2-v) < threshold && abs32(s3-v) < threshold && abs32(s4-v) < threshold {
						v = (s1 + s2 + s3 + s4) / 4
					}
					dither := (noise(uint32(x), uint32(y), uint32(frame), 3+uint32(c)) - 0.5) / 255
					m.Pix[i+c] = clamp01(v + dither)
				}
			}
		}
	})
}
