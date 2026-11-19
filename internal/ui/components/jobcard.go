package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// JobCardProps is what a JobCard shows.
type JobCardProps struct {
	Kind    media.Kind
	Name    string
	Label   string // engine · model · scale
	Status  string
	Tone    giode.Color
	Running bool
	Frac    float64
	Stage   string
	Line    string
	Clock   giode.Element // a RunClock while running
	Detail  string        // output path, error or timing
	Actions []giode.Element
}

// JobCard is one row of the queue.
func JobCard(p JobCardProps) giode.Element {
	icon, color := MediaIcon(p.Kind)
	head := giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 14, Wrap: giode.WrapNoWrap},
		IconBubble(icon, color, 44),
		giode.Stack(giode.Styles{Gap: 3, FlexGrow: 1},
			giode.Row(giode.Styles{Gap: 10, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
				giode.Box(giode.Styles{FlexGrow: 1},
					giode.Text(p.Name, theme.Label().With(giode.Styles{FontSize: 14, TextOverflow: giode.TextOverflowEllipsis}))),
				StatusBadge(p.Status, p.Tone),
			),
			giode.Text(p.Label, theme.Caption()),
		),
	)
	children := []giode.Element{head}
	if p.Running {
		children = append(children, giode.Stack(giode.Styles{Gap: 7, Width: -1},
			giode.Row(giode.Styles{Width: -1, Wrap: giode.WrapNoWrap},
				giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(p.Stage, theme.Secondary().With(giode.Styles{Color: theme.Text}))),
				giode.Text(percent(p.Frac), theme.Secondary()),
			),
			ProgressBar(p.Frac, 6),
			outputLine(p.Line),
			p.Clock,
		))
	}
	foot := []giode.Element{
		giode.Box(giode.Styles{FlexGrow: 1},
			giode.Text(p.Detail, theme.Code().With(giode.Styles{MaxLines: 3, TextOverflow: giode.TextOverflowEllipsis}))),
	}
	children = append(children, giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 8, Wrap: giode.WrapNoWrap},
		append(foot, p.Actions...)...))
	return Card(giode.Styles{Width: -1, Gap: 14, Padding: giode.UniformInset(18)}, children...)
}
