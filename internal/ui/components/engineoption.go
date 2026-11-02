package components

import (
	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// EngineOption is a radio row choosing an engine in the inspector. An
// engine that isn't installed shows a download hint instead of a radio;
// a note, when set, replaces the vendor line.
func EngineOption(m *engine.Manifest, selected, installed bool, note string, onClick func()) giode.Element {
	return Clickable("engine-option", m.ID, m.Name, onClick, func(s giode.PressState) giode.Element {
		bg, border := giode.Color("transparent"), theme.Faint()
		switch {
		case selected:
			bg, border = theme.Tint(theme.Primary, 0.12), theme.Primary
		case s.Hovered:
			bg = theme.Hover()
		}
		var mark giode.Element
		switch {
		case !installed:
			mark = theme.Icon("download", 15, theme.Muted)
		case selected:
			mark = giode.Box(giode.Styles{
				Width: 18, Height: 18, BorderRadius: 9, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
				BackgroundGradient: theme.BrandGradient(),
			}, theme.Icon("check", 12, "#FFFFFF"))
		default:
			mark = giode.Box(giode.Styles{Width: 18, Height: 18, BorderRadius: 9, BorderColor: theme.Border, BorderWidth: 2})
		}
		sub := m.Vendor
		switch {
		case !installed:
			sub = tr.T("Not installed · open Engines")
		case note != "":
			sub = note
		}
		nameColor := giode.Color(theme.Text)
		if !installed {
			nameColor = theme.Muted
		}
		return giode.Row(giode.Styles{
			Width: -1, Align: giode.AlignCenter, Gap: 12, Wrap: giode.WrapNoWrap,
			Background: bg, BorderColor: border, BorderWidth: 1, BorderRadius: 12, Padding: giode.SymmetricInset(12, 10),
		},
			giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
				giode.Text(m.Name, theme.Label().With(giode.Styles{Color: nameColor})),
				giode.Text(sub, theme.Caption().With(giode.Styles{TextOverflow: giode.TextOverflowEllipsis})),
			),
			mark,
		)
	})
}
