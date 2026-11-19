package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// ProgressPanel floats over a stage while a preview runs: what's
// happening, how far along it is, the engine's latest output, how long
// it has run (a RunClock), and actions such as copying the log or
// stopping.
func ProgressPanel(title, stage string, frac float64, line string, clock giode.Element, actions ...giode.Element) giode.Element {
	return giode.Stack(giode.Styles{
		Width: 380, Gap: 12,
		Background: theme.Surface, BorderColor: theme.Faint(), BorderWidth: 1, BorderRadius: 16,
		Padding:   giode.UniformInset(20),
		BoxShadow: giode.BoxShadow{Y: 16, Blur: 48, Color: "#00000080"},
	},
		giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 12, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{
				Width: 36, Height: 36, BorderRadius: 11, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
				BackgroundGradient: theme.BrandGradient(),
			}, theme.Icon("sparkles", 18, "#FFFFFF")),
			giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
				giode.Text(title, theme.Label().With(giode.Styles{FontSize: 14})),
				giode.Text(stage, theme.Caption()),
			),
			giode.Text(percent(frac), theme.Label().With(giode.Styles{Color: theme.Muted})),
		),
		ProgressBar(frac, 6),
		giode.Stack(giode.Styles{Width: -1, Gap: 4}, outputLine(line), clock),
		giode.Row(giode.Styles{Width: -1, Gap: 4, Justify: giode.JustifyEnd}, actions...),
	)
}
