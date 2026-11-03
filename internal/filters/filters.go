package filters

// Settings are the filters to apply. Amounts are percentages: 0 to 100,
// or -100 to 100 for adjustments that go both ways. The zero value
// changes nothing.
type Settings struct {
	// Clean-up, applied to the source before upscaling.
	Deinterlace bool `json:"deinterlace,omitempty"`
	// BottomFieldFirst keeps the odd lines when deinterlacing.
	BottomFieldFirst bool `json:"bottomFieldFirst,omitempty"`
	Deblock          int  `json:"deblock,omitempty"`
	Denoise          int  `json:"denoise,omitempty"`
	Deband           int  `json:"deband,omitempty"`

	// The look, applied to the result.
	Sharpen    int `json:"sharpen,omitempty"`
	Brightness int `json:"brightness,omitempty"`
	Contrast   int `json:"contrast,omitempty"`
	Saturation int `json:"saturation,omitempty"`
	Warmth     int `json:"warmth,omitempty"`
	Grain      int `json:"grain,omitempty"`
	// GrainSize is the grain's size in tenths of a pixel of the result;
	// 0 means 15 (1.5 pixels).
	GrainSize int `json:"grainSize,omitempty"`
}

// Active reports whether any filter is on.
func (s Settings) Active() bool { return s.HasCleanup() || s.HasLook() }

// HasCleanup reports whether any clean-up filter is on.
func (s Settings) HasCleanup() bool {
	return s.Deinterlace || s.Deblock > 0 || s.Denoise > 0 || s.Deband > 0
}

// HasLook reports whether any look filter is on.
func (s Settings) HasLook() bool {
	return s.Sharpen > 0 || s.Brightness != 0 || s.Contrast != 0 || s.Saturation != 0 || s.Warmth != 0 || s.Grain > 0
}

func pct(v int) float32 { return float32(max(-100, min(100, v))) / 100 }

// Cleanup applies the clean-up filters to m, frame being the frame's
// number in a video (0 for a picture), in the order that suits them:
// fields first, then blocks, then noise, then bands.
func (s Settings) Cleanup(m *Image, frame int) {
	if s.Deinterlace {
		Deinterlace(m, s.BottomFieldFirst)
	}
	Deblock(m, pct(s.Deblock))
	Denoise(m, pct(s.Denoise))
	Deband(m, pct(s.Deband), frame)
}

// Look applies the look filters to m: sharpness first, so it works on
// the picture as it is, then color, then grain last, as on film.
func (s Settings) Look(m *Image, frame int) {
	Sharpen(m, pct(s.Sharpen))
	Color(m, pct(s.Brightness), pct(s.Contrast), pct(s.Warmth))
	Saturate(m, pct(s.Saturation))
	size := float32(s.GrainSize) / 10
	if s.GrainSize <= 0 {
		size = 1.5
	}
	Grain(m, pct(s.Grain), size, frame)
}
