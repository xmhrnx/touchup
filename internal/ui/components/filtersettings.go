package components

import (
	"fmt"
	"strings"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/dialog"
	"github.com/mehran9675/giode/components/dropdown"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/mehran9675/giode/components/slider"
	"github.com/mehran9675/giode/components/toggle"

	"github.com/mehran9675/touchup/internal/filters"
	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// FilterStore is what FilterSettings reads and changes.
type FilterStore interface {
	Filters() filters.Settings
	// SetFilters changes the filters; save writes them to disk, which
	// waits for the end of a drag.
	SetFilters(change func(*filters.Settings), save bool)
}

// FilterSettings is a dialog with TouchUp's own filters: clean-up for
// the source and the look of the result, each a slider or switch.
type FilterSettings struct {
	store    FilterStore
	open     bool
	dialog   *dialog.Dialog
	scroll   *scroll.Scroll
	done     *giode.ButtonElement
	reset    *giode.ButtonElement
	deint    *toggle.Switch
	field    *dropdown.Select
	grain    *dropdown.Select
	sliders  map[string]*slider.Slider
	controls []filterControl
}

// filterControl is a slider for one of the filters' amounts.
type filterControl struct {
	id, name, desc string
	bipolar        bool // -100 to 100 rather than 0 to 100
	field          func(*filters.Settings) *int
}

// grainSizes are the grain sizes offered, in tenths of a pixel.
var grainSizes = []int{10, 15, 25}

// NewFilterSettings creates the dialog, closed.
func NewFilterSettings(store FilterStore) *FilterSettings {
	d := &FilterSettings{store: store, sliders: map[string]*slider.Slider{}}
	d.controls = []filterControl{
		{"deblock", tr.T("Deblock"), tr.T("Smooths the square blocks heavy compression leaves."), false, func(s *filters.Settings) *int { return &s.Deblock }},
		{"denoise", tr.T("Denoise"), tr.T("Smooths grain and sensor noise while keeping edges."), false, func(s *filters.Settings) *int { return &s.Denoise }},
		{"deband", tr.T("Deband"), tr.T("Smooths the steps of banding in skies and gradients."), false, func(s *filters.Settings) *int { return &s.Deband }},
		{"sharpen", tr.T("Sharpen"), tr.T("Brings out detail without halos, and less where it's noisy."), false, func(s *filters.Settings) *int { return &s.Sharpen }},
		{"brightness", tr.T("Brightness"), "", true, func(s *filters.Settings) *int { return &s.Brightness }},
		{"contrast", tr.T("Contrast"), "", true, func(s *filters.Settings) *int { return &s.Contrast }},
		{"saturation", tr.T("Saturation"), tr.T("All the way down is black and white."), true, func(s *filters.Settings) *int { return &s.Saturation }},
		{"warmth", tr.T("Warmth"), tr.T("Toward orange, or toward blue."), true, func(s *filters.Settings) *int { return &s.Warmth }},
		{"grain", tr.T("Film grain"), tr.T("Fine monochrome grain, strongest in the midtones, different every frame."), false, func(s *filters.Settings) *int { return &s.Grain }},
	}
	for _, c := range d.controls {
		c := c
		sl := giode.Input.Slider(giode.Styles{Color: theme.Primary, Background: theme.Raised(), BorderColor: "#FFFFFF"})
		sl.OnChange(func(v float32) {
			n := int(v*100 + 0.5)
			if c.bipolar {
				n = int(v*200+0.5) - 100
			}
			d.store.SetFilters(func(s *filters.Settings) { *c.field(s) = n }, false)
		})
		d.sliders[c.id] = sl
	}
	d.deint = giode.Input.Switch("").OnChange(func(on bool) {
		store.SetFilters(func(s *filters.Settings) { s.Deinterlace = on }, true)
	})
	selectStyle := giode.Styles{Width: 200, Background: theme.Raised(), Color: theme.Text, BorderColor: theme.Border, BorderRadius: 10, FontSize: 13}
	d.field = giode.Input.Select(selectStyle, tr.T("Top field first"), tr.T("Bottom field first")).OnChange(func(i int) {
		store.SetFilters(func(s *filters.Settings) { s.BottomFieldFirst = i == 1 }, true)
	})
	d.grain = giode.Input.Select(selectStyle, tr.T("Fine"), tr.T("Medium"), tr.T("Coarse")).OnChange(func(i int) {
		store.SetFilters(func(s *filters.Settings) { s.GrainSize = grainSizes[i] }, true)
	})
	d.done = theme.PrimaryButton(tr.T("Done"), "").OnClick(func() {
		d.open = false
		store.SetFilters(func(*filters.Settings) {}, true)
	})
	d.reset = theme.GhostButton(tr.T("Turn all off"), "rotate-ccw").OnClick(func() {
		store.SetFilters(func(s *filters.Settings) { *s = filters.Settings{} }, true)
		d.sync()
	})
	d.scroll = giode.Scroll(d.rows, giode.Styles{ScrollBar: giode.ScrollBar{Width: 4, ThumbColor: theme.Border}}).FitContent(true)
	d.dialog = giode.Dialog(&d.open, d.view, giode.Styles{
		Background: theme.Surface, BorderRadius: 20, Padding: giode.UniformInset(24),
	})
	return d
}

// Show opens the dialog.
func (d *FilterSettings) Show() {
	d.open = true
	d.sync()
}

// Open reports whether the dialog is showing.
func (d *FilterSettings) Open() bool { return d.open }

// Dialog is the element to lay out above the window.
func (d *FilterSettings) Dialog() giode.Element { return d.dialog }

// sync sets the controls to the saved values.
func (d *FilterSettings) sync() {
	s := d.store.Filters()
	for _, c := range d.controls {
		v := float32(*c.field(&s)) / 100
		if c.bipolar {
			v = (v + 1) / 2
		}
		d.sliders[c.id].SetValue(v)
	}
	d.deint.SetChecked(s.Deinterlace)
	field := 0
	if s.BottomFieldFirst {
		field = 1
	}
	d.field.SetSelected(field)
	grain := 1
	for i, g := range grainSizes {
		if g == s.GrainSize {
			grain = i
		}
	}
	d.grain.SetSelected(grain)
}

func (d *FilterSettings) rows() giode.Element {
	s := d.store.Filters()
	slider := func(id string) giode.Element {
		for _, c := range d.controls {
			if c.id != id {
				continue
			}
			v := *c.field(&s)
			value := fmt.Sprint(v)
			if c.bipolar && v > 0 {
				value = "+" + value
			}
			if v == 0 {
				value = tr.T("Off")
			}
			return SettingRow(c.name+" · "+value, c.desc, giode.Box(giode.Styles{Width: 240}, d.sliders[id]))
		}
		return nil
	}
	divider := giode.Divider(giode.Styles{Color: theme.Faint()})
	return giode.Stack(giode.Styles{Width: -1, Gap: 4, Padding: giode.Inset{Right: 10}},
		SectionLabel(tr.T("Clean up · before upscaling")),
		SettingRow(tr.T("Deinterlace"), tr.T("For TV recordings and old camcorder video: rebuilds every other line along edges, so moving things lose their combing."), d.deint),
		SettingRow(tr.T("Field order"), tr.T("Which lines are kept. If motion stutters back and forth, try the other."), d.field),
		divider, slider("deblock"), slider("denoise"), slider("deband"),
		giode.Box(giode.Styles{Height: 12}),
		SectionLabel(tr.T("Look · after upscaling")),
		slider("sharpen"), slider("brightness"), slider("contrast"), slider("saturation"), slider("warmth"),
		divider, slider("grain"),
		SettingRow(tr.T("Grain size"), "", d.grain),
	)
}

func (d *FilterSettings) view() giode.Element {
	return giode.Stack(giode.Styles{Width: 720, Gap: 16},
		giode.Row(giode.Styles{Gap: 12, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			IconBubble("wand-sparkles", theme.Primary, 40),
			giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
				giode.Text(tr.T("Filters"), theme.Heading().With(giode.Styles{FontSize: 18})),
				giode.Text(tr.T("Run in TouchUp itself, with any engine or on their own with Classic at 1x. They apply to the next preview or job."), theme.Secondary()),
			),
		),
		giode.Box(giode.Styles{Width: -1, MaxHeight: 500}, d.scroll),
		giode.Row(giode.Styles{Width: -1, Gap: 8, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			d.reset, giode.Box(giode.Styles{FlexGrow: 1}), d.done),
	)
}

// FilterSummary lists the filters that are on, or says none are.
func FilterSummary(s filters.Settings) string {
	var on []string
	if s.Deinterlace {
		on = append(on, tr.T("Deinterlace"))
	}
	amount := func(name string, v int, bipolar bool) {
		if v == 0 {
			return
		}
		if bipolar && v > 0 {
			on = append(on, fmt.Sprintf("%s +%d", name, v))
			return
		}
		on = append(on, fmt.Sprintf("%s %d", name, v))
	}
	amount(tr.T("Deblock"), s.Deblock, false)
	amount(tr.T("Denoise"), s.Denoise, false)
	amount(tr.T("Deband"), s.Deband, false)
	amount(tr.T("Sharpen"), s.Sharpen, false)
	amount(tr.T("Brightness"), s.Brightness, true)
	amount(tr.T("Contrast"), s.Contrast, true)
	amount(tr.T("Saturation"), s.Saturation, true)
	amount(tr.T("Warmth"), s.Warmth, true)
	amount(tr.T("Film grain"), s.Grain, false)
	if len(on) == 0 {
		return tr.T("None")
	}
	return strings.Join(on, " · ")
}
