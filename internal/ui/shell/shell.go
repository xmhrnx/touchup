// Package shell is TouchUp's window layout: the sidebar, the current
// page and the toasts above them.
package shell

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/titlebar"

	"github.com/mehran9675/touchup/internal/ui/components"
	"github.com/mehran9675/touchup/internal/ui/pages"
	"github.com/mehran9675/touchup/internal/ui/store"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// Shell is the root of the UI.
type Shell struct {
	s        *store.Store
	studio   *pages.Studio
	engines  *pages.Engines
	queue    *pages.Queue
	settings *pages.Settings
	errors   *components.ErrorDetails
	options  *components.EngineSettings
	filters  *components.FilterSettings
	bar      *titlebar.TitleBar // nil with the system frame
}

// New builds every page once, and the title bar when the window is
// frameless.
func New(s *store.Store, dragging func() bool) *Shell {
	errs := components.NewErrorDetails()
	options := components.NewEngineSettings(s)
	filters := components.NewFilterSettings(s)
	sh := &Shell{
		s:        s,
		studio:   pages.NewStudio(s, dragging, errs, options, filters),
		engines:  pages.NewEngines(s, errs, options),
		queue:    pages.NewQueue(s, errs),
		settings: pages.NewSettings(s),
		errors:   errs,
		options:  options,
		filters:  filters,
	}
	s.ShowReport = func(title, subtitle, text string) {
		errs.ShowReport(title, subtitle, text, "gauge", theme.Success)
	}
	if CustomFrame() {
		sh.bar = newTitleBar(s)
	}
	return sh
}

// View lays out the window.
func (sh *Shell) View() giode.Element {
	var page giode.Element
	switch sh.s.Page {
	case store.PageEngines:
		page = sh.engines.View()
	case store.PageQueue:
		page = sh.queue.View()
	case store.PageSettings:
		page = sh.settings.View()
	default:
		page = sh.studio.View()
	}
	var bar giode.Element = giode.Box(giode.Styles{Display: giode.DisplayNone})
	if sh.bar != nil {
		bar = sh.bar.Title(title(sh.s))
	}
	root := []giode.Element{
		bar,
		giode.Row(giode.Styles{Width: -1, FlexGrow: 1, Wrap: giode.WrapNoWrap},
			Sidebar(sh.s),
			giode.Box(giode.Styles{Width: 1, Height: -1, Background: theme.Faint()}),
			giode.Box(giode.Styles{FlexGrow: 1, Height: -1}, page),
		),
	}
	// Overlays last, so they paint above everything, in a layer outside
	// the layout: an open dialog would otherwise take space from the page.
	// Gio blends in linear light, so the dialog's own scrim barely shows on
	// the dark theme; a deeper one goes under it.
	if sh.engines.DialogOpen() || sh.errors.Open() || sh.options.Open() || sh.filters.Open() {
		root = append(root, giode.Box(giode.Styles{
			Position: giode.PositionAbsolute,
			Top:      giode.Flush, Left: giode.Flush, Right: giode.Flush, Bottom: giode.Flush,
			Background: "#000000A0",
		}))
	}
	for _, o := range append(sh.engines.Overlays(), sh.options.Dialog(), sh.filters.Dialog(), sh.errors.Dialog()) {
		root = append(root, giode.Box(giode.Styles{
			Position: giode.PositionAbsolute,
			Top:      giode.Flush, Left: giode.Flush, Right: giode.Flush, Bottom: giode.Flush,
		}, o))
	}
	root = append(root, sh.s.Toasts)
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Background: theme.Background}, root...)
}
