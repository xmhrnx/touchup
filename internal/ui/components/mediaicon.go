package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// MediaIcon returns the icon name and color of a kind of media.
func MediaIcon(kind media.Kind) (string, giode.Color) {
	switch kind {
	case media.Image:
		return "image", theme.Info
	case media.Video:
		return "film", theme.Accent
	case media.Audio:
		return "audio-lines", theme.Success
	}
	return "box", theme.Muted
}
