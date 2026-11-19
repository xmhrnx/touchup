package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// PlayButton plays one side of an A/B comparison: a round play or stop
// glyph with a title and a detail line. accent fills it with the brand
// gradient. It fills its parent's width.
func PlayButton(key, title, detail string, playing, accent bool, onClick func()) giode.Element {
	return Clickable("play-button", key, title, onClick, func(s giode.PressState) giode.Element {
		bg := theme.Raised()
		if s.Hovered {
			bg = theme.Hover()
		}
		border := giode.Color(theme.Faint())
		if playing {
			border = theme.Primary
		}
		icon := "play"
		if playing {
			icon = "square"
		}
		disc := giode.Styles{Width: 40, Height: 40, BorderRadius: 20, Align: giode.AlignCenter, Justify: giode.JustifyCenter}
		iconColor := giode.Color(theme.Text)
		if accent {
			disc.BackgroundGradient = theme.BrandGradient()
			iconColor = "#FFFFFF"
		} else {
			disc.Background = theme.Hover()
		}
		return giode.Row(giode.Styles{
			Width: -1, Align: giode.AlignCenter, Gap: 12, Wrap: giode.WrapNoWrap,
			Background: bg, BorderColor: border, BorderWidth: 1, BorderRadius: 14, Padding: giode.SymmetricInset(12, 10),
		},
			giode.Box(disc, theme.Icon(icon, 16, string(iconColor))),
			giode.Stack(giode.Styles{Gap: 2},
				giode.Text(title, theme.Label()),
				giode.Text(detail, theme.Caption()),
			),
		)
	})
}
