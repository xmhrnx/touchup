// Package assets embeds TouchUp's fonts, icons, app icon and
// translations.
package assets

import (
	"embed"
	_ "embed"
)

// Fonts holds the Inter typeface (SIL Open Font License, fonts/OFL.txt).
//
//go:embed fonts/*.ttf
var Fonts embed.FS

// Icons holds Lucide icons (ISC License, icons/LICENSE).
//
//go:embed icons/*.svg
var Icons embed.FS

// Locales holds the translations, locales/<language>.json, keyed by
// the English text (see internal/tr).
//
//go:embed locales/*.json
var Locales embed.FS

// AppIcon is the window and app icon.
//
//go:embed icon.png
var AppIcon []byte
