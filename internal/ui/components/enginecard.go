package components

import (
	"fmt"
	"strings"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/styles/properties"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/format"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// EngineCardProps is what an EngineCard shows.
type EngineCardProps struct {
	Engine *engine.Manifest
	// Width is the card's width in dp. Height is its height in dp, or 0
	// for its natural height; with a height the footer sits at the bottom.
	Width, Height int
	Installed     bool
	// Models is the card's model section, built by the page; nil when
	// the engine has no models to choose from.
	Models giode.Element
	// Footer is the bottom row: the install progress, an error, or the
	// card's buttons. It always takes the same height, so a card doesn't
	// grow while it installs.
	Footer giode.Element
}

// footerHeight is the height of an engine card's footer in dp.
const footerHeight = 44

// EngineCard presents an engine in the library.
func EngineCard(p EngineCardProps) giode.Element {
	m := p.Engine
	icon, color := MediaIcon(media.Image)
	if m.Handles("audio") {
		icon, color = MediaIcon(media.Audio)
	} else if m.Input == "video" {
		icon, color = MediaIcon(media.Video)
	}

	var badge giode.Element = giode.Box(giode.Styles{})
	switch {
	case p.Installed:
		badge = StatusBadge(tr.T("Installed"), theme.Success)
	case m.Custom:
		badge = StatusBadge(tr.T("Custom"), theme.Info)
	}

	tags := []giode.Element{}
	for _, k := range m.Media {
		tags = append(tags, Tag(MediaName(k), color))
	}
	for _, t := range m.Tags {
		tags = append(tags, Tag(tr.T(t), theme.Muted))
	}

	facts := []giode.Element{}
	if m.Requirements != "" {
		facts = append(facts, fact("cpu", tr.T(m.Requirements)))
	}
	detail := []string{}
	if m.Install.Size != "" {
		detail = append(detail, tr.T(m.Install.Size))
	}
	if len(m.Scales) > 0 {
		s := make([]string, len(m.Scales))
		for i, v := range m.Scales {
			s[i] = fmt.Sprintf("%dx", v)
		}
		detail = append(detail, strings.Join(s, " · "))
	}
	if m.License != "" {
		detail = append(detail, m.License)
	}
	if len(detail) > 0 {
		facts = append(facts, fact("package", strings.Join(detail, "  ·  ")))
	}

	children := []giode.Element{
		giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 12, Wrap: giode.WrapNoWrap},
			IconBubble(icon, color, 40),
			giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
				giode.Text(m.Name, theme.Heading()),
				giode.Text(m.Vendor, theme.Caption().With(giode.Styles{TextOverflow: giode.TextOverflowEllipsis})),
			),
			badge,
		),
		giode.Box(giode.Styles{MinHeight: 57},
			giode.Text(tr.T(m.Description), theme.Secondary().With(giode.Styles{MaxLines: 3, TextOverflow: giode.TextOverflowEllipsis}))),
		giode.Row(giode.Styles{Gap: 6}, tags...),
		giode.Stack(giode.Styles{Gap: 6}, facts...),
	}
	if p.Models != nil {
		children = append(children, p.Models)
	}
	// The footer group is the same element whether the card is measured
	// or stretched to its row, so both lay out the same children and
	// gaps; when stretched it grows and pushes the footer to the bottom.
	bottom := giode.Styles{Width: -1, Gap: 14, Justify: giode.JustifyEnd}
	if p.Height > 0 {
		bottom.FlexGrow = 1
	}
	children = append(children, giode.Stack(bottom,
		giode.Divider(giode.Styles{Color: theme.Faint()}),
		giode.Box(giode.Styles{Width: -1, Height: footerHeight, Align: giode.AlignCenter}, p.Footer),
	))
	return Card(giode.Styles{
		Width: properties.Width(p.Width), Height: properties.Height(p.Height), Gap: 14,
		Padding: giode.Inset{Top: 20, Right: 20, Bottom: 12, Left: 20},
	}, children...)
}

// InstallFooter is an engine card's footer while it installs: the stage,
// a progress bar and a cancel button, in the footer's fixed height.
func InstallFooter(stage string, frac float64, cancel giode.Element) giode.Element {
	return giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 12, Wrap: giode.WrapNoWrap},
		giode.Stack(giode.Styles{Gap: 7, FlexGrow: 1},
			giode.Row(giode.Styles{Width: -1, Wrap: giode.WrapNoWrap},
				giode.Box(giode.Styles{FlexGrow: 1},
					giode.Text(stage+"…", theme.Label().With(giode.Styles{FontSize: 12.5, TextOverflow: giode.TextOverflowEllipsis}))),
				giode.Text(percent(frac), theme.Caption()),
			),
			ProgressBar(frac, 5),
		),
		cancel,
	)
}

func fact(icon, text string) giode.Element {
	return giode.Row(giode.Styles{Gap: 8, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
		theme.Icon(icon, 14, theme.Muted),
		giode.Text(text, theme.Caption().With(giode.Styles{TextOverflow: giode.TextOverflowEllipsis})),
	)
}

func percent(f float64) string {
	if f < 0 {
		return ""
	}
	if f < 0.1 {
		// Long runs creep: 2.4% shows it moving where 2% wouldn't.
		return format.Percent(f, 1)
	}
	return format.Percent(f, 0)
}
