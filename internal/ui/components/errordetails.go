package components

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/dialog"
	"github.com/mehran9675/giode/components/scroll"

	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// ErrorDetails is a dialog with the whole of an error: a report with what
// failed, the setup it ran on and what the engine or installer printed,
// selectable and with a copy button. One is shared by every page.
type ErrorDetails struct {
	title, text string
	subtitle    string
	icon        string
	tone        giode.Color
	open        bool
	dialog      *dialog.Dialog
	copy        giode.Element
	close       *giode.ButtonElement
	log         *scroll.Scroll
}

// NewErrorDetails creates the dialog, closed.
func NewErrorDetails() *ErrorDetails {
	d := &ErrorDetails{}
	d.close = theme.SecondaryButton(tr.T("Close"), "").OnClick(func() { d.open = false })
	d.copy = theme.CopyButton(tr.T("Copy all"), func() string { return d.text })
	// The reason is at the end of a log, so it opens scrolled to the end.
	d.log = giode.Scroll(func() giode.Element {
		return giode.Box(giode.Styles{Width: -1, Padding: giode.UniformInset(14)},
			giode.Text(d.text, theme.Code().With(giode.Styles{Color: theme.Text})).Selectable("error-details"))
	}, giode.Styles{ScrollBar: giode.ScrollBar{Width: 4, ThumbColor: theme.Border}}).
		FitContent(true).Anchor(giode.AnchorBottom)
	d.dialog = giode.Dialog(&d.open, d.view, giode.Styles{
		Background: theme.Surface, BorderRadius: 20, Padding: giode.UniformInset(24),
	})
	return d
}

// Show opens the dialog with a title and an error's full report.
func (d *ErrorDetails) Show(title, report string) {
	d.ShowReport(title, tr.T("The whole error, with what it printed. Copy it to report the problem."), report, "triangle-alert", theme.Danger)
}

// ShowReport opens the dialog for any long text, such as a diagnostic
// report, with its own subtitle, icon and tone.
func (d *ErrorDetails) ShowReport(title, subtitle, text, icon string, tone giode.Color) {
	d.title, d.subtitle, d.text, d.icon, d.tone, d.open = title, subtitle, text, icon, tone, true
	d.log.ScrollToBottom()
}

// Open reports whether the dialog is showing.
func (d *ErrorDetails) Open() bool { return d.open }

// Dialog is the element to lay out above the window.
func (d *ErrorDetails) Dialog() giode.Element { return d.dialog }

func (d *ErrorDetails) view() giode.Element {
	return giode.Stack(giode.Styles{Width: 680, Gap: 16},
		giode.Row(giode.Styles{Gap: 12, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			IconBubble(d.icon, d.tone, 40),
			giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
				giode.Text(d.title, theme.Heading().With(giode.Styles{FontSize: 18})),
				giode.Text(d.subtitle, theme.Secondary()),
			),
		),
		giode.Box(giode.Styles{
			Width: -1, MaxHeight: 420, Background: theme.Sunken(), BorderRadius: 12, Overflow: giode.OverflowHidden,
		}, d.log),
		giode.Row(giode.Styles{Width: -1, Gap: 8, Justify: giode.JustifyEnd}, d.copy, d.close),
	)
}
