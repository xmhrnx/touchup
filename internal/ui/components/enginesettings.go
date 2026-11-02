package components

import (
	"strconv"
	"strings"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/dialog"
	"github.com/mehran9675/giode/components/scroll"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// EngineSettingsStore is what EngineSettings reads and changes.
type EngineSettingsStore interface {
	EngineOptions(m *engine.Manifest) map[string]string
	SetEngineOption(m *engine.Manifest, id, value string)
	ResetEngineOptions(m *engine.Manifest)
}

// EngineSettings is a dialog with an engine's options: a dropdown,
// switch or number field each, with what it does and its default.
// Changes save at once and apply to the next preview or job. One is
// shared by the pages that open it.
type EngineSettings struct {
	store    EngineSettingsStore
	engine   *engine.Manifest
	open     bool
	dialog   *dialog.Dialog
	rows     []giode.Element
	scroll   *scroll.Scroll
	done     *giode.ButtonElement
	defaults *giode.ButtonElement
}

// NewEngineSettings creates the dialog, closed.
func NewEngineSettings(store EngineSettingsStore) *EngineSettings {
	d := &EngineSettings{store: store}
	d.done = theme.PrimaryButton(tr.T("Done"), "").OnClick(func() { d.open = false })
	d.defaults = theme.GhostButton(tr.T("Reset to defaults"), "rotate-ccw").OnClick(func() {
		if d.engine != nil {
			store.ResetEngineOptions(d.engine)
			d.build()
		}
	})
	d.scroll = giode.Scroll(func() giode.Element {
		return giode.Stack(giode.Styles{Width: -1, Gap: 4, Padding: giode.Inset{Right: 10}}, d.rows...)
	}, giode.Styles{ScrollBar: giode.ScrollBar{Width: 4, ThumbColor: theme.Border}}).FitContent(true)
	d.dialog = giode.Dialog(&d.open, d.view, giode.Styles{
		Background: theme.Surface, BorderRadius: 20, Padding: giode.UniformInset(24),
	})
	return d
}

// Show opens the dialog for m.
func (d *EngineSettings) Show(m *engine.Manifest) {
	d.engine, d.open = m, true
	d.build()
}

// Open reports whether the dialog is showing.
func (d *EngineSettings) Open() bool { return d.open }

// Dialog is the element to lay out above the window.
func (d *EngineSettings) Dialog() giode.Element { return d.dialog }

// build makes a control per option, set to its current value.
func (d *EngineSettings) build() {
	m := d.engine
	values := d.store.EngineOptions(m)
	d.rows = nil
	for i := range m.Options {
		o := &m.Options[i]
		desc := tr.T("Default: {value}.", "value", OptionLabel(o, o.Default))
		if o.Description != "" {
			desc = tr.T(o.Description) + " " + desc
		}
		if i > 0 {
			d.rows = append(d.rows, giode.Divider(giode.Styles{Color: theme.Faint()}))
		}
		d.rows = append(d.rows, SettingRow(tr.T(o.Name), desc, d.control(o, values[o.ID])))
	}
}

func (d *EngineSettings) control(o *engine.Option, value string) giode.Element {
	m := d.engine
	set := func(v string) { d.store.SetEngineOption(m, o.ID, v) }
	switch o.Type {
	case engine.OptionToggle:
		sw := giode.Input.Switch("").OnChange(func(on bool) { set(strconv.FormatBool(on)) })
		sw.SetChecked(value == "true")
		return sw
	case engine.OptionNumber:
		f := theme.Field(o.Default)
		f.SetText(value)
		commit := func() {
			v := strings.TrimSpace(f.Text())
			if o.Valid(v) {
				set(v)
			} else {
				f.SetText(d.store.EngineOptions(m)[o.ID]) // put back the saved value
			}
		}
		f.Submit(func(string) { commit() })
		f.OnBlur(commit)
		return giode.Box(giode.Styles{Width: 120}, f)
	}
	labels := make([]string, len(o.Choices))
	selected := 0
	for i, c := range o.Choices {
		labels[i] = tr.T(c.Label)
		if c.Value == value {
			selected = i
		}
	}
	sel := giode.Input.Select(giode.Styles{
		Width: 200, Background: theme.Raised(), Color: theme.Text, BorderColor: theme.Border,
		BorderRadius: 10, FontSize: 13,
	}, labels...).OnChange(func(i int) { set(o.Choices[i].Value) })
	sel.SetSelected(selected)
	return sel
}

func (d *EngineSettings) view() giode.Element {
	if d.engine == nil {
		return giode.Box(giode.Styles{})
	}
	return giode.Stack(giode.Styles{Width: 720, Gap: 16},
		giode.Row(giode.Styles{Gap: 12, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			IconBubble("sliders-horizontal", theme.Primary, 40),
			giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
				giode.Text(tr.T("{engine} settings", "engine", d.engine.Name), theme.Heading().With(giode.Styles{FontSize: 18})),
				giode.Text(tr.T("Changes save as you make them and apply to the next preview or job."), theme.Secondary()),
			),
		),
		giode.Box(giode.Styles{Width: -1, MaxHeight: 480}, d.scroll),
		giode.Row(giode.Styles{Width: -1, Gap: 8, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			d.defaults, giode.Box(giode.Styles{FlexGrow: 1}), d.done),
	)
}
