package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// FormField labels a control, with a hint underneath.
func FormField(label, hint string, control giode.Element) giode.Element {
	children := []giode.Element{giode.Text(label, theme.Label().With(giode.Styles{FontSize: 12.5})), control}
	if hint != "" {
		children = append(children, giode.Text(hint, theme.Caption()))
	}
	return giode.Stack(giode.Styles{Gap: 6, Width: -1}, children...)
}
