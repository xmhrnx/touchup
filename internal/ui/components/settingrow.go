package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// SettingRow is one setting: its name and explanation on the left, the
// control on the right.
func SettingRow(title, description string, control giode.Element) giode.Element {
	return giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 24, Wrap: giode.WrapNoWrap, Padding: giode.SymmetricInset(0, 6)},
		giode.Stack(giode.Styles{Gap: 3, FlexGrow: 1},
			giode.Text(title, theme.Label()),
			giode.Text(description, theme.Caption()),
		),
		control,
	)
}
