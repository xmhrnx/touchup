package pages

import (
	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/components"
	"github.com/mehran9675/touchup/internal/ui/format"
	"github.com/mehran9675/touchup/internal/ui/store"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// VideoStage shows the video frame under the playhead, the timeline to
// pick it, and the before/after comparison once it's previewed.
type VideoStage struct {
	s        *store.Store
	compare  *components.CompareView
	timeline *components.Timeline
	prev     *giode.ButtonElement
	next     *giode.ButtonElement
	cancel   *giode.ButtonElement
	dismiss  *giode.ButtonElement
	details  *giode.ButtonElement
	copy     giode.Element
	copyLog  giode.Element
}

// NewVideoStage creates the stage.
func NewVideoStage(s *store.Store, errs *components.ErrorDetails) *VideoStage {
	v := &VideoStage{s: s, compare: components.NewCompareView()}
	v.timeline = components.NewTimeline(func(t float64, final bool) { s.Seek(t) })
	step := func(dir float64) func() {
		return func() {
			if sess := s.Session; sess != nil && sess.Info.FPS > 0 {
				s.Seek(sess.Time + dir/sess.Info.FPS)
			}
		}
	}
	v.prev = theme.SecondaryButton("", "chevron-right").OnClick(step(-1))
	v.prev.Icon(giode.Box(giode.Styles{Transform: giode.Transform{Rotate: 180}},
		giode.Icon("chevron-right", giode.Styles{Width: 15, Height: 15})))
	v.next = theme.SecondaryButton("", "chevron-right").OnClick(step(1))
	v.cancel = theme.GhostButton(tr.T("Cancel"), "x").OnClick(s.CancelPreview)
	v.dismiss = theme.GhostButton(tr.T("Dismiss"), "").OnClick(s.DismissPreviewError)
	v.details = theme.GhostButton(tr.T("Details"), "").OnClick(func() {
		if sess := s.Session; sess != nil {
			errs.Show(tr.T("The preview failed"), previewReport(s))
		}
	})
	v.copy = theme.CopyButton(tr.T("Copy"), func() string { return previewReport(s) })
	v.copyLog = theme.CopyButton(tr.T("Copy log"), func() string { return runningPreviewReport(s) })
	return v
}

// View lays out the stage.
func (v *VideoStage) View() giode.Element {
	sess := v.s.Session
	p := sess.Preview
	matches := sess.PreviewMatches()

	var viewer giode.Element
	if matches {
		v.compare.Set(p.Frame.Before, p.Frame.After, tr.T("Original"), "TouchUp · "+p.Label)
		viewer = v.compare
	} else {
		hint := tr.T("Pick a moment on the timeline, then preview it")
		if p.Frame != nil {
			hint = tr.T("The playhead moved. Preview this frame to compare again")
		}
		viewer = giode.Box(giode.Styles{Width: -1, Height: -1, BorderRadius: 14, Overflow: giode.OverflowHidden, Background: "#050507"},
			components.Picture(sess.Picture),
			giode.Box(giode.Styles{Position: giode.PositionAbsolute, Left: 14, Bottom: 14},
				giode.Row(giode.Styles{
					Align: giode.AlignCenter, Gap: 8, Wrap: giode.WrapNoWrap,
					Background: "#0B0B10CC", BorderRadius: 999, Padding: giode.SymmetricInset(12, 7),
				},
					theme.Icon("scan-eye", 14, "#D4D4DC"),
					giode.Text(hint, giode.Styles{FontFamily: theme.Sans, FontSize: 12, Color: "#D4D4DC"}),
				)),
		)
		if sess.FrameLoading {
			viewer = withOverlay(viewer, giode.Box(giode.Styles{Position: giode.PositionAbsolute, Top: 14, Right: 14},
				giode.Spinner(giode.Styles{Width: 20, Height: 20, Color: "#FFFFFF"})))
		}
	}
	if p.Running {
		title := tr.T("Previewing {time}", "time", format.Clock(sess.Time))
		if p.Tuning {
			title = tr.T("Finding the fastest settings")
		}
		viewer = withOverlay(viewer, centered(components.ProgressPanel(
			title, p.Update.Stage, p.Update.Frac, p.Update.Line,
			components.RunClock(p.Started, p.Log.Last, p.TimeLeft), v.copyLog, v.cancel)))
	}

	v.timeline.Duration = sess.Info.Duration
	v.timeline.Time = sess.Time
	v.timeline.Film = sess.Film
	v.timeline.Marker = -1
	if p.Frame != nil {
		v.timeline.Marker = p.At
	}

	frame := 0
	if sess.Info.FPS > 0 {
		frame = int(sess.Time*sess.Info.FPS) + 1
	}
	controls := giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 10, Wrap: giode.WrapNoWrap},
		giode.Text(format.Clock(sess.Time), theme.Label().With(giode.Styles{FontSize: 15, FontWeight: giode.FontWeightSemiBold})),
		giode.Text("/ "+format.Clock(sess.Info.Duration), theme.Secondary()),
		giode.Box(giode.Styles{FlexGrow: 1}),
		giode.Text(tr.T("Frame {frame} of {total}", "frame", format.Count(frame), "total", format.Count(sess.Info.Frames)), theme.Caption()),
		v.prev, v.next,
	)

	children := []giode.Element{giode.Box(giode.Styles{Width: -1, FlexGrow: 1, MinHeight: 260}, viewer)}
	if p.Err != "" {
		children = append(children, components.Banner("triangle-alert", theme.Danger, tr.T("The preview failed"), p.Err, v.copy, v.details, v.dismiss))
	}
	children = append(children, controls, v.timeline)
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Gap: 14}, children...)
}

// withOverlay stacks an absolutely positioned overlay over el.
func withOverlay(el, overlay giode.Element) giode.Element {
	return giode.Box(giode.Styles{Width: -1, Height: -1}, el, overlay)
}

// centered fills its parent and centers child, as an overlay.
func centered(child giode.Element) giode.Element {
	return giode.Box(giode.Styles{
		Position: giode.PositionAbsolute, Top: giode.Flush, Left: giode.Flush, Right: giode.Flush, Bottom: giode.Flush,
		Align: giode.AlignCenter, Justify: giode.JustifyCenter, Background: "#00000066",
	}, child)
}
