package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// Banner is an inline message in a tone color ("theme.danger",
// "theme.warning", "theme.info"), with optional actions on the right.
func Banner(icon string, tone giode.Color, title, text string, actions ...giode.Element) giode.Element {
	body := []giode.Element{giode.Text(title, theme.Label())}
	if text != "" {
		body = append(body, giode.Text(text, theme.Secondary().With(giode.Styles{MaxLines: 6})))
	}
	row := []giode.Element{
		theme.Icon(icon, 18, string(tone)),
		giode.Stack(giode.Styles{Gap: 3, FlexGrow: 1}, body...),
	}
	return giode.Row(giode.Styles{
		Width: -1, Align: giode.AlignCenter, Gap: 12, Wrap: giode.WrapNoWrap,
		Background: theme.Tint(tone, 0.1), BorderColor: theme.Tint(tone, 0.35), BorderWidth: 1,
		BorderRadius: 12, Padding: giode.SymmetricInset(14, 12),
	}, append(row, actions...)...)
}
