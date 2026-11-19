// Package theme defines TouchUp's look: colors, type, icons and the
// shared styles components build on.
package theme

import "github.com/xmhrnx/giode"

// Brand colors, the same in both modes.
const (
	Violet = "#7C5CFF"
	Pink   = "#EC4899"
	Cyan   = "#22D3EE"
)

var dark = giode.Palette{
	Primary:    Violet,
	Secondary:  "#A1A1B5",
	Background: "#09090D",
	Surface:    "#131319",
	Text:       "#F4F4F7",
	Border:     "#24242E",
	OnPrimary:  "#FFFFFF",
	Muted:      "#8A8A9B",
	Accent:     Pink,
	Success:    "#34D399",
	Warning:    "#FBBF24",
	Danger:     "#F87171",
	Info:       "#60A5FA",
}

var light = giode.Palette{
	Primary:    "#6D4AFF",
	Secondary:  "#5B5B6E",
	Background: "#F6F6F9",
	Surface:    "#FFFFFF",
	Text:       "#14141B",
	Border:     "#E3E3EA",
	OnPrimary:  "#FFFFFF",
	Muted:      "#6B6B7D",
	Accent:     "#DB2777",
	Success:    "#059669",
	Warning:    "#D97706",
	Danger:     "#DC2626",
	Info:       "#2563EB",
}

// Install sets both palettes and switches to mode.
func Install(mode giode.Mode) {
	giode.SetPalette(giode.ModeDark, dark)
	giode.SetPalette(giode.ModeLight, light)
	giode.SetMode(mode)
}

// Theme color tokens, which follow the active mode.
const (
	Background = "theme.background"
	Surface    = "theme.surface"
	Text       = "theme.text"
	Muted      = "theme.muted"
	Border     = "theme.border"
	Primary    = "theme.primary"
	OnPrimary  = "theme.onprimary"
	Accent     = "theme.accent"
	Success    = "theme.success"
	Warning    = "theme.warning"
	Danger     = "theme.danger"
	Info       = "theme.info"
)

// Raised is a surface one step above Surface: inputs, hovered rows.
func Raised() giode.Color { return giode.Mix(Text, Surface, 0.05) }

// Sunken is a surface below Surface: wells, tracks, the stage.
func Sunken() giode.Color { return giode.Mix(Background, Surface, 0.55) }

// Hover is the fill of a hovered row on Surface.
func Hover() giode.Color { return giode.Mix(Text, Surface, 0.08) }

// Faint is a hairline on Surface, softer than Border.
func Faint() giode.Color { return giode.Mix(Text, Surface, 0.07) }

// Tint mixes amount of color into Surface: selected rows, soft badges.
func Tint(color giode.Color, amount float64) giode.Color { return giode.Mix(color, Surface, amount) }

// BrandGradient is the violet-to-pink gradient of primary actions.
func BrandGradient() *giode.LinearGradient { return giode.Linear(120, Violet, Pink) }
