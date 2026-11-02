package components

import (
	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// ModelRowProps is what a ModelRow shows.
type ModelRowProps struct {
	Name        string
	Description string
	Detail      string // size, or "Included"
	Recommended bool
	Installed   bool
	// Downloading is true while the model downloads; Stage and Frac
	// describe it. Err is a failed download's message.
	Downloading bool
	Stage       string
	Frac        float64
	Err         string
	// Action is the row's button, built by the page. Copy copies a failed
	// download's error, next to it.
	Action giode.Element
	Copy   giode.Element
}

// ModelRow is one model in an engine card: its name, what it's for, its
// size and state, and a button to download or remove it.
func ModelRow(p ModelRowProps) giode.Element {
	title := []giode.Element{
		giode.Text(p.Name, theme.Label().With(giode.Styles{FontSize: 12.5, TextOverflow: giode.TextOverflowEllipsis})),
	}
	if p.Recommended {
		title = append(title, Tag(tr.T("Recommended"), theme.Primary))
	}
	var status giode.Element = giode.Text(p.Detail, theme.Caption())
	if p.Installed {
		status = giode.Row(giode.Styles{Gap: 5, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			theme.Icon("circle-check", 12, theme.Success),
			giode.Text(tr.T("Installed · {detail}", "detail", p.Detail), theme.Caption().With(giode.Styles{Color: theme.Success})),
		)
	}
	info := []giode.Element{
		giode.Row(giode.Styles{Gap: 6, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap}, title...),
		giode.Text(p.Description, theme.Caption().With(giode.Styles{MaxLines: 2, TextOverflow: giode.TextOverflowEllipsis})),
	}
	switch {
	case p.Downloading:
		info = append(info,
			giode.Row(giode.Styles{Width: -1, Wrap: giode.WrapNoWrap, Margin: giode.Inset{Top: 2}},
				giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(p.Stage+"…", theme.Caption().With(giode.Styles{Color: theme.Text}))),
				giode.Text(percent(p.Frac), theme.Caption()),
			),
			ProgressBar(p.Frac, 4),
		)
	case p.Err != "":
		info = append(info, giode.Text(p.Err, theme.Caption().With(giode.Styles{Color: theme.Danger, MaxLines: 2, TextOverflow: giode.TextOverflowEllipsis})))
	default:
		info = append(info, status)
	}
	row := []giode.Element{giode.Stack(giode.Styles{Gap: 3, FlexGrow: 1}, info...)}
	if p.Copy != nil && p.Err != "" {
		row = append(row, p.Copy)
	}
	if p.Action != nil {
		row = append(row, p.Action)
	}
	bg := giode.Color(theme.Raised())
	if p.Installed {
		bg = theme.Tint(theme.Success, 0.06)
	}
	return giode.Row(giode.Styles{
		Width: -1, Align: giode.AlignCenter, Gap: 10, Wrap: giode.WrapNoWrap,
		Background: bg, BorderRadius: 10, Padding: giode.SymmetricInset(12, 10),
	}, row...)
}
