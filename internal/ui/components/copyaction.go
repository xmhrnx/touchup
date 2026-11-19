package components

import (
	"gioui.org/layout"

	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// copyState is the persistent part of a CopyAction.
type copyState struct {
	b, twin giode.Element
	text    func() string
}

// CopyAction returns a copy button kept under (scope, key) whose text is
// refreshed every frame, for errors in lists. An empty label makes it an
// icon-only button.
func CopyAction(scope string, key any, label string, text func() string) giode.Element {
	st := giode.Keyed(scope+"/"+modeName(), key, func() *copyState {
		c := &copyState{}
		c.b = theme.CopyButton(label, func() string { return c.text() })
		c.twin = theme.CopyButton(label, nil)
		return c
	})
	st.text = text
	return giode.Raw(st.layout)
}

// layout lays out the button. Passes that only measure, such as a grid
// sizing its rows, get a twin of the same size: a copy started between
// frames happens when the real button is laid out, and a measuring pass
// can't write to the clipboard.
func (c *copyState) layout(gtx layout.Context) layout.Dimensions {
	if !gtx.Source.Enabled() {
		return c.twin.Layout(gtx)
	}
	return c.b.Layout(gtx)
}
