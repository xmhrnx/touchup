package components

import "github.com/xmhrnx/giode"

// clickState is the persistent part of a Clickable.
type clickState struct {
	p       *giode.PressableElement
	onClick func()
	render  func(giode.PressState) giode.Element
}

// Clickable makes content clickable inside lists whose rows come and go.
// Its press state is kept under (scope, key), while the handler and the
// content are refreshed every frame, so they can capture the row's
// current data.
func Clickable(scope string, key any, label string, onClick func(), render func(giode.PressState) giode.Element) giode.Element {
	st := giode.Keyed(scope, key, func() *clickState {
		c := &clickState{}
		c.p = giode.Pressable(func(s giode.PressState) giode.Element { return c.render(s) }).
			OnClick(func(giode.ClickEvent) {
				if c.onClick != nil {
					c.onClick()
				}
			})
		return c
	})
	st.onClick, st.render = onClick, render
	st.p.Label(label)
	return st.p
}
