package components

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/i18n"
)

// actionState is the persistent part of an Action.
type actionState struct {
	b       *giode.ButtonElement
	onClick func()
}

// Action returns a button kept under (scope, key), built once by build,
// whose click handler is refreshed every frame. For buttons in lists.
// Buttons are kept per color mode and language, since build resolves
// theme colors and labels.
func Action(scope string, key any, build func() *giode.ButtonElement, onClick func()) *giode.ButtonElement {
	st := giode.Keyed(scope+"/"+modeName(), key, func() *actionState {
		a := &actionState{b: build()}
		a.b.OnClick(func() {
			if a.onClick != nil {
				a.onClick()
			}
		})
		return a
	})
	st.onClick = onClick
	return st.b
}

// modeName names the color mode and language that stateful components
// were built for, so keyed ones are rebuilt when either changes.
func modeName() string {
	mode := "dark"
	if giode.CurrentMode() == giode.ModeLight {
		mode = "light"
	}
	return mode + "/" + i18n.Locale()
}
