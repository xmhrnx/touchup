package components

import (
	"fmt"

	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// NavItem is a sidebar link with an icon and an optional count.
func NavItem(key any, icon, label string, active bool, count int, onClick func()) giode.Element {
	return Clickable("nav-item", key, label, onClick, func(s giode.PressState) giode.Element {
		bg, fg := giode.Color("transparent"), giode.Color(theme.Muted)
		switch {
		case active:
			bg, fg = theme.Tint(theme.Primary, 0.16), theme.Text
		case s.Hovered:
			bg, fg = theme.Hover(), theme.Text
		}
		iconColor := fg
		if active {
			iconColor = theme.Primary
		}
		children := []giode.Element{
			theme.Icon(icon, 18, string(iconColor)),
			giode.Box(giode.Styles{FlexGrow: 1},
				giode.Text(label, giode.Styles{FontFamily: theme.Sans, FontSize: 13.5, FontWeight: giode.FontWeightMedium, Color: fg})),
		}
		if count > 0 {
			children = append(children, giode.Box(giode.Styles{
				BackgroundGradient: theme.BrandGradient(), BorderRadius: 999, Padding: giode.SymmetricInset(7, 1), MinWidth: 20,
			}, giode.Text(fmt.Sprint(count), giode.Styles{
				FontFamily: theme.Sans, FontSize: 11, FontWeight: giode.FontWeightBold, Color: "#FFFFFF", TextAlign: giode.TextAlignCenter,
			})))
		}
		return giode.Row(giode.Styles{
			Width: -1, Align: giode.AlignCenter, Gap: 12, Wrap: giode.WrapNoWrap,
			Background: bg, BorderRadius: 10, Padding: giode.SymmetricInset(12, 9),
		}, children...)
	})
}
