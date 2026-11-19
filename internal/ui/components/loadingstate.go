package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// LoadingState is a centered spinner with a line of text.
func LoadingState(text string) giode.Element {
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Align: giode.AlignCenter, Justify: giode.JustifyCenter, Gap: 14},
		giode.Spinner(giode.Styles{Width: 28, Height: 28, Color: theme.Primary}),
		giode.Text(text, theme.Secondary()),
	)
}
