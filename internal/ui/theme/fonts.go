package theme

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"

	"github.com/mehran9675/touchup/assets"
	"github.com/xmhrnx/giode"
)

// Font families.
const (
	Sans = "Inter"
	Mono = "Go Mono"
)

// LoadFonts registers Inter in four weights, with the Go fonts behind
// it for monospace text and any glyph Inter lacks.
func LoadFonts() error {
	weights := map[string]font.Weight{
		"Inter-Regular.ttf":  font.Normal,
		"Inter-Medium.ttf":   font.Medium,
		"Inter-SemiBold.ttf": font.SemiBold,
		"Inter-Bold.ttf":     font.Bold,
	}
	var faces []font.FontFace
	for file, weight := range weights {
		data, err := assets.Fonts.ReadFile("fonts/" + file)
		if err != nil {
			return err
		}
		face, err := opentype.Parse(data)
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		faces = append(faces, font.FontFace{
			Font: font.Font{Typeface: Sans, Weight: weight},
			Face: face,
		})
	}
	giode.RegisterFonts(append(faces, gofont.Collection()...))
	return nil
}
