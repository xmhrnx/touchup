package components

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/styles/properties"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// IconBubble is an icon on a rounded square tinted with its color.
func IconBubble(icon string, color giode.Color, size int) giode.Element {
	return giode.Box(giode.Styles{
		Width: properties.Width(size), Height: properties.Height(size),
		Align: giode.AlignCenter, Justify: giode.JustifyCenter,
		Background: theme.Tint(color, 0.16), BorderRadius: properties.BorderRadius(size * 3 / 10),
	}, theme.Icon(icon, size/2, color))
}
