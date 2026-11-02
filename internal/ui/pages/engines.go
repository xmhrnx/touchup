package pages

import (
	"context"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/dialog"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/mehran9675/giode/components/segmented"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/components"
	"github.com/mehran9675/touchup/internal/ui/store"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// Engines is the engine library: install, remove and add engines.
type Engines struct {
	s      *store.Store
	filter *segmented.Control
	scroll *scroll.Scroll
	add    *giode.ButtonElement
	imp    *giode.ButtonElement
	form   *components.CustomEngineForm
	formOn bool
	dialog *dialog.Dialog
	// modelsOf is the engine whose models the models dialog shows.
	modelsOf     *engine.Manifest
	modelsOn     bool
	modelsDialog *dialog.Dialog
	modelsScroll *scroll.Scroll
	modelsClose  *giode.ButtonElement

	errors  *components.ErrorDetails
	options *components.EngineSettings
	// checking marks engines whose GPU check is running.
	checking map[string]bool
}

// NewEngines creates the page.
func NewEngines(s *store.Store, errs *components.ErrorDetails, options *components.EngineSettings) *Engines {
	e := &Engines{s: s, errors: errs, options: options, checking: map[string]bool{}, filter: theme.Segmented(tr.T("All"), tr.T("Image & video"), tr.T("Audio"), tr.T("Installed"))}
	e.scroll = giode.Scroll(e.grid, giode.Styles{ScrollBar: giode.ScrollBar{Width: 4, ThumbColor: theme.Border}})
	e.form = components.NewCustomEngineForm()
	e.form.OnSave = func(m *engine.Manifest) error {
		if err := s.AddCustomEngine(m); err != nil {
			return err
		}
		e.formOn = false
		return nil
	}
	e.form.OnCancel = func() { e.formOn = false }
	e.dialog = giode.Dialog(&e.formOn, func() giode.Element { return e.form }, giode.Styles{
		Background: theme.Surface, BorderRadius: 20, Padding: giode.UniformInset(26),
	})
	e.add = theme.PrimaryButton(tr.T("Add custom engine"), "plus").OnClick(func() {
		e.form.Reset()
		e.formOn = true
	})
	e.imp = theme.SecondaryButton(tr.T("Import manifest"), "file-code").OnClick(s.ImportManifest)
	e.modelsScroll = giode.Scroll(e.modelList, giode.Styles{ScrollBar: giode.ScrollBar{Width: 4, ThumbColor: theme.Border}}).FitContent(true)
	e.modelsClose = theme.SecondaryButton(tr.T("Done"), "").OnClick(func() { e.modelsOn = false })
	e.modelsDialog = giode.Dialog(&e.modelsOn, e.modelsView, giode.Styles{
		Background: theme.Surface, BorderRadius: 20, Padding: giode.UniformInset(24),
	})
	return e
}

// View lays out the page.
func (e *Engines) View() giode.Element {
	return giode.Stack(giode.Styles{Width: -1, Height: -1},
		giode.Stack(giode.Styles{Width: -1, Gap: 18, Padding: giode.Inset{Top: 28, Right: 32, Bottom: 8, Left: 32}},
			components.PageHeader(tr.T("Engines"),
				tr.T("Open-source models that run on this computer. Install what you need, or bring your own."),
				e.imp, e.add),
			giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
				giode.Box(giode.Styles{Width: 540}, e.filter),
			),
		),
		giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, e.scroll),
	)
}

// Overlays are the page's dialogs, laid out last in the window's root so
// they cover everything.
func (e *Engines) Overlays() []giode.Element {
	return []giode.Element{e.dialog, e.modelsDialog}
}

// DialogOpen reports whether one of the page's dialogs is showing.
func (e *Engines) DialogOpen() bool { return e.formOn || e.modelsOn }

func (e *Engines) grid() giode.Element {
	s := e.s
	var shown []*engine.Manifest
	for _, m := range s.Registry.All() {
		installed := s.Installed(m.ID)
		switch e.filter.Selected() {
		case 1:
			if !m.Handles("image") && !m.Handles("video") {
				continue
			}
		case 2:
			if !m.Handles("audio") {
				continue
			}
		case 3:
			if !installed {
				continue
			}
		}
		shown = append(shown, m)
	}
	if len(shown) == 0 {
		return giode.Box(giode.Styles{Width: -1, Height: 420},
			components.EmptyState("package", tr.T("No engines installed"),
				tr.T("Install one of the built-in engines or add your own to get started.")))
	}
	return giode.Box(giode.Styles{Width: -1, Padding: giode.Inset{Top: 12, Right: 32, Bottom: 32, Left: 32}},
		components.Grid(len(shown), 330, 18, func(i, width, height int) giode.Element {
			m := shown[i]
			return components.EngineCard(components.EngineCardProps{
				Engine: m, Width: width, Height: height, Installed: s.Installed(m.ID),
				Models: e.models(m), Footer: e.footer(m),
			})
		}))
}

// footer is a card's bottom row: install progress, an error, or buttons.
func (e *Engines) footer(m *engine.Manifest) giode.Element {
	s := e.s
	in := s.Installs[m.ID]
	row := func(children ...giode.Element) giode.Element {
		return giode.Row(giode.Styles{Width: -1, Gap: 8, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap}, children...)
	}
	spacer := giode.Box(giode.Styles{FlexGrow: 1})
	action := func(scope, label, icon string, build func(label, icon string) *giode.ButtonElement, fn func()) giode.Element {
		return components.Action(scope, m.ID, func() *giode.ButtonElement { return build(label, icon) }, fn)
	}
	website := func() giode.Element {
		if m.Homepage == "" {
			return giode.Box(giode.Styles{})
		}
		return action("engine-home", tr.T("Website"), "external-link", theme.GhostButton,
			func() { go s.App.OpenURL(context.Background(), m.Homepage) })
	}
	remove := func() giode.Element {
		if !m.Custom {
			return giode.Box(giode.Styles{})
		}
		return action("engine-remove", tr.T("Delete"), "trash", theme.DangerButton, func() { s.RemoveCustomEngine(m) })
	}

	switch {
	case in.Running():
		return components.InstallFooter(in.Status.Stage, in.Status.Frac,
			action("engine-cancel", tr.T("Cancel"), "x", theme.GhostButton, func() { s.CancelInstall(m.ID) }))
	case in != nil && in.Err != "":
		// The message is usually too long for a card: the card says what
		// happened and Details shows the rest.
		return row(
			theme.Icon("triangle-alert", 16, theme.Danger),
			giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(tr.T("Failed"), theme.Label().With(giode.Styles{
				FontSize: 12.5, Color: theme.Danger, MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis,
			}))),
			components.CopyAction("engine-copy", m.ID, "", func() string {
				return s.ErrorReport(tr.T("Couldn't install {engine}", "engine", m.Name), in.Err, in.Log)
			}),
			action("engine-details", tr.T("Details"), "", theme.GhostButton, func() {
				e.errors.Show(tr.T("Couldn't install {engine}", "engine", m.Name), s.ErrorReport(tr.T("Couldn't install {engine}", "engine", m.Name), in.Err, in.Log))
			}),
			action("engine-retry", tr.T("Retry"), "refresh-cw", theme.SecondaryButton, func() { s.InstallEngine(m) }),
		)
	case s.Installed(m.ID):
		uninstall := giode.Element(giode.Box(giode.Styles{}))
		if m.Install.Type != "none" {
			uninstall = action("engine-uninstall", tr.T("Uninstall"), "trash", theme.GhostButton, func() { s.UninstallEngine(m) })
		}
		// Engines with options get a Settings button; the website shrinks
		// to an icon to make room.
		settings := giode.Element(giode.Box(giode.Styles{}))
		if len(m.Options) > 0 {
			label := tr.T("Settings")
			if m.UsesTorch() {
				label = "" // next to Check GPU, an icon is all that fits
			}
			settings = action("engine-settings", label, "sliders-horizontal", theme.GhostButton, func() { e.options.Show(m) })
		}
		if !m.UsesTorch() {
			if len(m.Options) == 0 {
				return row(website(), spacer, uninstall, remove())
			}
			site := giode.Element(giode.Box(giode.Styles{}))
			if m.Homepage != "" {
				site = action("engine-home-icon", "", "external-link", theme.GhostButton,
					func() { go s.App.OpenURL(context.Background(), m.Homepage) })
			}
			return row(settings, site, spacer, uninstall, remove())
		}
		// PyTorch engines get a GPU check; the website shrinks to an icon
		// to make room.
		check := action("engine-gpu", tr.T("Check GPU"), "cpu", theme.GhostButton, func() {
			e.checking[m.ID] = true
			s.CheckGPU(m, func(report string, err error) {
				delete(e.checking, m.ID)
				if err != nil {
					report = err.Error()
				}
				e.errors.ShowReport(tr.T("GPU check · {engine}", "engine", m.Name),
					tr.T("What {engine}'s PyTorch sees. Copy it to report a problem.", "engine", m.Name), report, "cpu", theme.Primary)
			})
		}).(*giode.ButtonElement).Loading(e.checking[m.ID])
		site := giode.Element(giode.Box(giode.Styles{}))
		if m.Homepage != "" {
			site = action("engine-home-icon", "", "external-link", theme.GhostButton,
				func() { go s.App.OpenURL(context.Background(), m.Homepage) })
		}
		if len(m.Options) > 0 && m.Install.Type != "none" {
			uninstall = action("engine-uninstall-icon", "", "trash", theme.GhostButton, func() { s.UninstallEngine(m) })
		}
		return row(check, settings, site, spacer, uninstall, remove())
	case !m.Supported():
		return row(giode.Text(tr.T("Not available for this system"), theme.Caption()), spacer, remove())
	}
	verb := tr.T("Install")
	if m.Install.Type == "none" {
		verb = tr.T("Check again")
	}
	return row(action("engine-install", verb, "download", theme.PrimaryButton, func() { s.InstallEngine(m) }),
		website(), spacer, remove())
}

// models is a card's model summary, which opens the models dialog.
func (e *Engines) models(m *engine.Manifest) giode.Element {
	s := e.s
	if len(m.Models) == 0 {
		return nil
	}
	summary := tr.N("{count} included", len(m.Models))
	if downloadable(m) {
		summary = tr.N("{installed} of {count} installed", len(m.Models), "installed", s.InstalledModels(m))
		if !s.Installed(m.ID) {
			summary = tr.N("{count} to choose from", len(m.Models))
		}
	}
	for i := range m.Models {
		if s.ModelInstalls[store.ModelKey(m, &m.Models[i])].Running() {
			summary = tr.T("Downloading…")
		}
	}
	return components.Clickable("engine-models", m.ID, tr.T("Models of {engine}", "engine", m.Name), func() {
		e.modelsOf, e.modelsOn = m, true
		e.modelsScroll.ScrollTo(0)
	}, func(st giode.PressState) giode.Element {
		bg := giode.Color(theme.Raised())
		if st.Hovered {
			bg = theme.Hover()
		}
		return giode.Row(giode.Styles{
			Width: -1, Align: giode.AlignCenter, Gap: 10, Wrap: giode.WrapNoWrap,
			Background: bg, BorderRadius: 10, Padding: giode.SymmetricInset(12, 9),
		},
			theme.Icon("layers", 15, theme.Primary),
			giode.Text(tr.T("Models"), theme.Label().With(giode.Styles{FontSize: 12.5})),
			giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(summary, theme.Caption())),
			theme.Icon("chevron-right", 14, theme.Muted),
		)
	})
}

func downloadable(m *engine.Manifest) bool {
	for _, mdl := range m.Models {
		if len(mdl.Files) > 0 {
			return true
		}
	}
	return false
}

// modelsView is the models dialog: the engine's models, each with its
// own download.
func (e *Engines) modelsView() giode.Element {
	m := e.modelsOf
	if m == nil {
		return giode.Box(giode.Styles{})
	}
	s := e.s
	sub := tr.T("These models come with the engine.")
	if downloadable(m) {
		sub = tr.T("Download the models you want; each one appears in the studio once it's installed.")
		if !s.Installed(m.ID) {
			sub = tr.T("Install {engine} first, then download the models you want.", "engine", m.Name)
		}
	}
	return giode.Stack(giode.Styles{Width: 600, Gap: 16},
		giode.Row(giode.Styles{Gap: 12, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			components.IconBubble("layers", theme.Primary, 40),
			giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
				giode.Text(tr.T("{engine} models", "engine", m.Name), theme.Heading().With(giode.Styles{FontSize: 18})),
				giode.Text(sub, theme.Secondary()),
			),
		),
		giode.Box(giode.Styles{Width: -1, MaxHeight: 520}, e.modelsScroll),
		giode.Row(giode.Styles{Width: -1, Justify: giode.JustifyEnd}, e.modelsClose),
	)
}

func (e *Engines) modelList() giode.Element {
	m := e.modelsOf
	if m == nil {
		return giode.Box(giode.Styles{})
	}
	installed := e.s.Installed(m.ID)
	rows := make([]giode.Element, 0, len(m.Models))
	for i := range m.Models {
		rows = append(rows, e.modelRow(m, &m.Models[i], installed))
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: 8, Padding: giode.Inset{Right: 8}}, rows...)
}

func (e *Engines) modelRow(m *engine.Manifest, mdl *engine.Model, engineInstalled bool) giode.Element {
	s := e.s
	key := store.ModelKey(m, mdl)
	in := s.ModelInstalls[key]
	p := components.ModelRowProps{Name: tr.T(mdl.Name), Description: tr.T(mdl.Description), Detail: mdl.Size, Recommended: mdl.Recommended}
	if len(mdl.Files) == 0 {
		p.Detail = tr.T("Included")
		p.Installed = engineInstalled
		if engineInstalled {
			p.Detail = tr.T("comes with the engine")
		}
		return components.ModelRow(p)
	}
	switch {
	case in.Running():
		p.Downloading, p.Stage, p.Frac = true, in.Status.Stage, in.Status.Frac
		p.Action = components.Action("model-cancel", key, func() *giode.ButtonElement { return theme.GhostButton("", "x") },
			func() { s.CancelModelInstall(m, mdl) })
	case in != nil && in.Err != "":
		p.Err = in.Err
		p.Copy = components.CopyAction("model-copy", key, "", func() string {
			return s.ErrorReport(tr.T("Couldn't download {model} for {engine}", "model", tr.T(mdl.Name), "engine", m.Name), in.Err, in.Log)
		})
		p.Action = components.Action("model-retry", key, func() *giode.ButtonElement { return theme.SecondaryButton(tr.T("Retry"), "rotate-ccw") },
			func() { s.DismissModelError(m, mdl); s.InstallModel(m, mdl) })
	case s.ModelInstalled(m, mdl):
		p.Installed = true
		p.Action = components.Action("model-remove", key, func() *giode.ButtonElement { return theme.GhostButton("", "trash") },
			func() { s.UninstallModel(m, mdl) })
	case engineInstalled:
		p.Action = components.Action("model-install", key, func() *giode.ButtonElement { return theme.SecondaryButton(tr.T("Get"), "download") },
			func() { s.InstallModel(m, mdl) })
	}
	return components.ModelRow(p)
}
