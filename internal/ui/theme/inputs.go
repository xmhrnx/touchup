package theme

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/input"
	"github.com/xmhrnx/giode/components/segmented"
)

// Field is a text field in TouchUp's style.
func Field(placeholder string) *input.Input {
	return giode.Input.Text(placeholder, giode.Styles{
		Background:   Raised(),
		Color:        Text,
		BorderColor:  Border,
		BorderWidth:  1,
		BorderRadius: 10,
		FontFamily:   Sans,
		FontSize:     13,
		Padding:      giode.SymmetricInset(12, 9),
	})
}

// Segmented is a segmented control in TouchUp's style. It fills its
// parent, so put it in a box of the width it should have.
func Segmented(options ...string) *segmented.Control {
	return giode.Input.Segmented(options...).Styles(giode.Styles{
		Width: -1, Background: Sunken(), BorderColor: Raised(), Color: Text, FontSize: 12.5,
	})
}
