package shell

import (
	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/components"
	"github.com/mehran9675/touchup/internal/ui/format"
	"github.com/mehran9675/touchup/internal/ui/store"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// Sidebar is the navigation column: brand, pages and a status card.
func Sidebar(s *store.Store) giode.Element {
	nav := func(p store.Page, icon, label string, count int) giode.Element {
		return components.NavItem(p, icon, label, s.Page == p, count, func() { s.Navigate(p) })
	}
	return giode.Stack(giode.Styles{
		Width: 236, Height: -1, Gap: 6,
		Background: theme.Surface, BorderColor: theme.Faint(), BorderWidth: 0,
		Padding: giode.Inset{Top: 22, Right: 14, Bottom: 16, Left: 14},
	},
		giode.Box(giode.Styles{Padding: giode.Inset{Left: 6, Bottom: 22}}, components.BrandMark()),
		giode.Box(giode.Styles{Padding: giode.Inset{Left: 12, Bottom: 6}}, giode.Text(tr.T("Workspace"), theme.Overline())),
		nav(store.PageStudio, "wand-sparkles", tr.T("Studio"), 0),
		nav(store.PageQueue, "list-checks", tr.T("Queue"), s.ActiveJobs()),
		giode.Box(giode.Styles{Padding: giode.Inset{Left: 12, Top: 18, Bottom: 6}}, giode.Text(tr.T("Library"), theme.Overline())),
		nav(store.PageEngines, "layers", tr.T("Engines"), 0),
		nav(store.PageSettings, "settings", tr.T("Settings"), 0),
		giode.Box(giode.Styles{FlexGrow: 1}),
		statusCard(s),
	)
}

// statusCard shows what's running, or whether TouchUp is ready.
func statusCard(s *store.Store) giode.Element {
	for _, j := range s.Jobs {
		if j.Status == store.JobRunning {
			return giode.Stack(giode.Styles{
				Width: -1, Gap: 8, Background: theme.Raised(), BorderRadius: 12, Padding: giode.UniformInset(12),
			},
				giode.Row(giode.Styles{Width: -1, Gap: 8, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
					giode.Spinner(giode.Styles{Width: 14, Height: 14, Color: theme.Primary}),
					giode.Box(giode.Styles{FlexGrow: 1},
						giode.Text(j.Name, theme.Label().With(giode.Styles{FontSize: 12, TextOverflow: giode.TextOverflowEllipsis}))),
				),
				components.ProgressBar(j.Update.Frac, 4),
				giode.Text(j.Update.Stage+" · "+format.Percent(float64(int(max(0, j.Update.Frac)*100))/100, 0), theme.Caption()),
			)
		}
	}
	installed := 0
	for _, m := range s.Registry.All() {
		if s.Installed(m.ID) {
			installed++
		}
	}
	tone, title, text := giode.Color(theme.Success), tr.T("Ready"), tr.N("{count} engines installed", installed)
	switch {
	case !s.Tools.Ready():
		tone, title, text = theme.Warning, tr.T("FFmpeg needed"), tr.T("Install FFmpeg or set its path in Settings")
	case installed == 0:
		tone, title, text = theme.Info, tr.T("Almost ready"), tr.T("Install an engine to start")
	}
	return giode.Row(giode.Styles{
		Width: -1, Gap: 10, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
		Background: theme.Raised(), BorderRadius: 12, Padding: giode.UniformInset(12),
	},
		giode.Box(giode.Styles{Width: 8, Height: 8, BorderRadius: 4, Background: tone}),
		giode.Stack(giode.Styles{Gap: 1, FlexGrow: 1},
			giode.Text(title, theme.Label().With(giode.Styles{FontSize: 12})),
			giode.Text(text, theme.Caption().With(giode.Styles{FontSize: 11})),
		),
	)
}
