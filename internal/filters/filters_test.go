package filters

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// gradient is a w×h picture with a smooth horizontal ramp and a sharp
// vertical edge in the middle.
func gradient(w, h int) *Image {
	n := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(40 + x*100/w)
			if x >= w/2 {
				v += 100
			}
			n.SetNRGBA(x, y, color.NRGBA{v, v / 2, 255 - v, 255})
		}
	}
	return FromNRGBA(n)
}

func diff(a, b *Image) float64 {
	var sum float64
	for i := range a.Pix {
		sum += math.Abs(float64(a.Pix[i] - b.Pix[i]))
	}
	return sum / float64(len(a.Pix))
}

func TestZeroSettingsChangeNothing(t *testing.T) {
	src := gradient(64, 48)
	m := src.Clone()
	var s Settings
	if s.Active() {
		t.Fatal("zero settings are active")
	}
	s.Cleanup(m, 3)
	s.Look(m, 3)
	if d := diff(src, m); d != 0 {
		t.Errorf("changed by %v", d)
	}
}

func TestEveryFilterChangesSomething(t *testing.T) {
	src := gradient(64, 48)
	// Noise and blocks for the clean-up filters to find.
	noisy := src.Clone()
	for i := range noisy.Pix {
		noisy.Pix[i] = clamp01(noisy.Pix[i] + (noise(uint32(i), 0, 0, 9)-0.5)*0.03)
	}
	for name, s := range map[string]Settings{
		"deinterlace": {Deinterlace: true}, "deblock": {Deblock: 100}, "denoise": {Denoise: 60},
		"deband": {Deband: 80}, "sharpen": {Sharpen: 80}, "brightness": {Brightness: 30},
		"contrast": {Contrast: -40}, "saturation": {Saturation: 50}, "warmth": {Warmth: 50}, "grain": {Grain: 50},
	} {
		m := noisy.Clone()
		s.Cleanup(m, 1)
		s.Look(m, 1)
		if diff(noisy, m) == 0 {
			t.Errorf("%s changed nothing", name)
		}
		for _, v := range m.Pix {
			if v < 0 || v > 1 || v != v {
				t.Fatalf("%s left a value of %v", name, v)
			}
		}
	}
}

func TestDenoiseKeepsEdges(t *testing.T) {
	src := gradient(64, 16)
	m := src.Clone()
	Denoise(m, 1)
	// Across the hard edge, the step stays nearly as big.
	before := src.at(32, 8, 0) - src.at(31, 8, 0)
	after := m.at(32, 8, 0) - m.at(31, 8, 0)
	if after < before*0.8 {
		t.Errorf("edge step fell from %.3f to %.3f", before, after)
	}
}

func TestGrainChangesByFrame(t *testing.T) {
	a, b := gradient(32, 32), gradient(32, 32)
	Grain(a, 1, 1.5, 1)
	Grain(b, 1, 1.5, 2)
	if diff(a, b) == 0 {
		t.Error("grain is the same in two frames")
	}
	c := gradient(32, 32)
	Grain(c, 1, 1.5, 1)
	if diff(a, c) != 0 {
		t.Error("grain differs for the same frame")
	}
}

func TestResize(t *testing.T) {
	src := gradient(40, 30)
	src.Alpha = make([]uint8, 40*30)
	for _, method := range []string{EdgeAdaptive, Lanczos, Bicubic} {
		for _, size := range [][2]int{{160, 120}, {100, 75}, {20, 15}, {40, 30}} {
			out := Resize(src, size[0], size[1], method)
			if out.W != size[0] || out.H != size[1] || len(out.Pix) != size[0]*size[1]*3 || len(out.Alpha) != size[0]*size[1] {
				t.Fatalf("%s to %v: got %dx%d", method, size, out.W, out.H)
			}
			// Flat areas keep their color.
			x0, y0 := 2*out.W/10, out.H/2
			want := src.at(2*src.W/10, src.H/2, 2)
			if got := out.at(x0, y0, 2); math.Abs(float64(got-want)) > 0.04 {
				t.Errorf("%s to %v: flat area went from %.3f to %.3f", method, size, want, got)
			}
		}
	}
}

func TestEdgeAdaptiveDoesNotRing(t *testing.T) {
	src := gradient(32, 32)
	out := Resize(src, 128, 128, EdgeAdaptive)
	lo, hi := float32(1), float32(0)
	for _, v := range src.Pix {
		lo, hi = min(lo, v), max(hi, v)
	}
	for _, v := range out.Pix {
		if v < lo-1e-6 || v > hi+1e-6 {
			t.Fatalf("value %v outside the source's range [%v, %v]", v, lo, hi)
		}
	}
}

func TestConversions(t *testing.T) {
	src := gradient(17, 9)
	back := FromRGB24(src.RGB24(nil), 17, 9)
	if d := diff(src, back); d > 1.0/255 {
		t.Errorf("RGB24 round trip differs by %v", d)
	}
	if d := diff(src, FromNRGBA(src.NRGBA())); d > 1.0/255 {
		t.Errorf("NRGBA round trip differs by %v", d)
	}
}
