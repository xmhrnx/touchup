package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// EmptyState fills a page or panel that has nothing to show yet.
func EmptyState(icon, title, text string, actions ...giode.Element) giode.Element {
	children := []giode.Element{
		IconBubble(icon, theme.Primary, 56),
		giode.Text(title, theme.Heading().With(giode.Styles{FontSize: 18})),
		giode.Box(giode.Styles{MaxWidth: 380},
			giode.Text(text, theme.Secondary().With(giode.Styles{TextAlign: giode.TextAlignCenter}))),
	}
	if len(actions) > 0 {
		children = append(children, giode.Row(giode.Styles{Gap: 10, Margin: giode.Inset{Top: 6}}, actions...))
	}
	return giode.Stack(giode.Styles{
		Width: -1, Height: -1, Align: giode.AlignCenter, Justify: giode.JustifyCenter, Gap: 10,
	}, children...)
}
