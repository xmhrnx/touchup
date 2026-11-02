package pages

import (
	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/components"
	"github.com/mehran9675/touchup/internal/ui/format"
	"github.com/mehran9675/touchup/internal/ui/store"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// sectionLengths are the preview lengths offered, in seconds.
var sectionLengths = []float64{5, 10, 20, 30}

// AudioStage shows the waveform to pick a section, and once that's
// previewed, A/B playback and spectrograms of before and after.
type AudioStage struct {
	s        *store.Store
	waveform *components.Waveform
	spectra  *components.CompareView
	cancel   *giode.ButtonElement
	dismiss  *giode.ButtonElement
	details  *giode.ButtonElement
	copy     giode.Element
	copyLog  giode.Element
}

// NewAudioStage creates the stage.
func NewAudioStage(s *store.Store, errs *components.ErrorDetails) *AudioStage {
	a := &AudioStage{s: s, spectra: components.NewCompareView()}
	a.waveform = components.NewWaveform(func(start, length float64) { s.SetSection(start, length) })
	a.cancel = theme.GhostButton(tr.T("Cancel"), "x").OnClick(s.CancelPreview)
	a.dismiss = theme.GhostButton(tr.T("Dismiss"), "").OnClick(s.DismissPreviewError)
	a.details = theme.GhostButton(tr.T("Details"), "").OnClick(func() {
		if sess := s.Session; sess != nil {
			errs.Show(tr.T("The preview failed"), previewReport(s))
		}
	})
	a.copy = theme.CopyButton(tr.T("Copy"), func() string { return previewReport(s) })
	a.copyLog = theme.CopyButton(tr.T("Copy log"), func() string { return runningPreviewReport(s) })
	return a
}

// View lays out the stage.
func (a *AudioStage) View() giode.Element {
	s := a.s
	sess := s.Session
	p := sess.Preview

	a.waveform.Peaks = sess.Peaks
	a.waveform.Duration = sess.Info.Duration
	a.waveform.Start, a.waveform.Length = sess.SelStart, sess.SelLen

	pills := []giode.Element{}
	for _, l := range sectionLengths {
		if l > sess.Info.Duration && l != sectionLengths[0] {
			continue
		}
		pills = append(pills, components.Pill("section-length", l, format.Duration(l),
			abs(sess.SelLen-l) < 0.01, func() {
				center := sess.SelStart + sess.SelLen/2
				s.SetSection(center-l/2, l)
			}))
	}

	picker := components.Card(giode.Styles{Gap: 16},
		giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 12, Wrap: giode.WrapNoWrap},
			giode.Stack(giode.Styles{Gap: 3, FlexGrow: 1},
				giode.Text(tr.T("Choose a section to preview"), theme.Heading()),
				giode.Text(tr.T("Drag the highlight to move it, or its edges to resize it."), theme.Caption()),
			),
			giode.Row(giode.Styles{Gap: 6, Wrap: giode.WrapNoWrap}, pills...),
		),
		a.waveform,
		giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Gap: 8},
			giode.Text("0:00", theme.Caption()),
			giode.Box(giode.Styles{FlexGrow: 1}),
			components.MetaChip("timer", format.Clock(sess.SelStart)+" → "+format.Clock(sess.SelStart+sess.SelLen)),
			giode.Box(giode.Styles{FlexGrow: 1}),
			giode.Text(format.Clock(sess.Info.Duration), theme.Caption()),
		),
	)

	children := []giode.Element{picker}
	if p.Err != "" {
		children = append(children, components.Banner("triangle-alert", theme.Danger, tr.T("The preview failed"), p.Err, a.copy, a.details, a.dismiss))
	}
	var result giode.Element
	switch {
	case p.Running:
		result = giode.Box(giode.Styles{Width: -1, Height: -1, Align: giode.AlignCenter, Justify: giode.JustifyCenter},
			components.ProgressPanel(tr.T("Previewing {time} of audio", "time", format.Duration(sess.SelLen)),
				p.Update.Stage, p.Update.Frac, p.Update.Line,
				components.RunClock(p.Started, p.Log.Last, p.TimeLeft), a.copyLog, a.cancel))
	case sess.PreviewMatches():
		result = a.result()
	case p.Audio != nil:
		result = components.EmptyState("refresh-cw", tr.T("The section changed"),
			tr.T("Preview again to hear the new section, or move it back to compare the last one."))
	default:
		result = components.EmptyState("audio-waveform", tr.T("Hear the difference first"),
			tr.T("Preview a few seconds with the engine you picked before you enhance the whole file."))
	}
	children = append(children, giode.Box(giode.Styles{Width: -1, FlexGrow: 1, MinHeight: 220}, result))
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Gap: 16}, children...)
}

// result shows A/B playback and the spectrograms.
func (a *AudioStage) result() giode.Element {
	s := a.s
	p := s.Session.Preview
	res := p.Audio
	a.spectra.Set(res.BeforeSpec, res.AfterSpec,
		tr.T("Original")+" · "+format.Rate(res.BeforeRate), tr.T("Enhanced")+" · "+format.Rate(res.AfterRate))
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Gap: 14},
		giode.Row(giode.Styles{Width: -1, Gap: 12, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{FlexGrow: 1}, components.PlayButton("before", tr.T("Original"), format.Rate(res.BeforeRate)+" · "+tr.T("as it is now"),
				s.Playing == res.Before, false, func() { s.Play(res.Before) })),
			giode.Box(giode.Styles{FlexGrow: 1}, components.PlayButton("after", tr.T("Enhanced"), format.Rate(res.AfterRate)+" · "+p.Label,
				s.Playing == res.After, true, func() { s.Play(res.After) })),
		),
		giode.Box(giode.Styles{Width: -1, FlexGrow: 1, MinHeight: 180}, a.spectra),
		giode.Text(tr.T("Spectrograms: frequency rises upward. Restored highs show as new detail at the top."),
			theme.Caption().With(giode.Styles{TextAlign: giode.TextAlignCenter, Width: -1})),
	)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
