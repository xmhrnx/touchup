package pages

import (
	"context"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/scroll"

	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/components"
	"github.com/mehran9675/touchup/internal/ui/format"
	"github.com/mehran9675/touchup/internal/ui/store"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// Queue lists upscale jobs: running, waiting and finished.
type Queue struct {
	s      *store.Store
	errors *components.ErrorDetails
	scroll *scroll.Scroll
	clear  *giode.ButtonElement
	studio *giode.ButtonElement
}

// NewQueue creates the page.
func NewQueue(s *store.Store, errs *components.ErrorDetails) *Queue {
	q := &Queue{s: s, errors: errs}
	q.scroll = giode.Scroll(q.list, giode.Styles{ScrollBar: giode.ScrollBar{Width: 4, ThumbColor: theme.Border}})
	q.clear = theme.SecondaryButton(tr.T("Clear finished"), "check").OnClick(s.ClearFinished)
	q.studio = theme.PrimaryButton(tr.T("Open the studio"), "wand-sparkles").OnClick(func() { s.Navigate(store.PageStudio) })
	return q
}

// View lays out the page.
func (q *Queue) View() giode.Element {
	s := q.s
	running, waiting, done := 0, 0, 0
	for _, j := range s.Jobs {
		switch {
		case j.Status == store.JobRunning:
			running++
		case j.Status == store.JobQueued:
			waiting++
		default:
			done++
		}
	}
	subtitle := tr.T("Files upscale one at a time, in order.")
	if len(s.Jobs) > 0 {
		subtitle = tr.T("{running} running · {waiting} waiting · {finished} finished", "running", running, "waiting", waiting, "finished", done)
	}
	q.clear.Disabled(done == 0)
	var body giode.Element = q.scroll
	if len(s.Jobs) == 0 {
		body = components.EmptyState("list-checks", tr.T("Nothing in the queue"),
			tr.T("Open a file in the studio, pick an engine, and press Upscale. Jobs show up here while they run."), q.studio)
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1},
		giode.Box(giode.Styles{Width: -1, Padding: giode.Inset{Top: 28, Right: 32, Bottom: 16, Left: 32}},
			components.PageHeader(tr.T("Queue"), subtitle, q.clear)),
		giode.Box(giode.Styles{Width: -1, FlexGrow: 1}, body),
	)
}

func (q *Queue) list() giode.Element {
	s := q.s
	cards := make([]giode.Element, 0, len(s.Jobs))
	for i := len(s.Jobs) - 1; i >= 0; i-- {
		cards = append(cards, q.card(s.Jobs[i]))
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: 12, Padding: giode.Inset{Right: 32, Bottom: 32, Left: 32}}, cards...)
}

func (q *Queue) card(j *store.Job) giode.Element {
	s := q.s
	p := components.JobCardProps{Kind: j.Kind, Name: j.Name, Label: j.Label}
	button := func(scope, label, icon string, ghost bool, fn func()) giode.Element {
		return components.Action(scope, j.ID, func() *giode.ButtonElement {
			if ghost {
				return theme.GhostButton(label, icon)
			}
			return theme.SecondaryButton(label, icon)
		}, fn)
	}
	switch j.Status {
	case store.JobQueued:
		p.Status, p.Tone = tr.T("Waiting"), theme.Muted
		p.Detail = tr.T("Starts when the jobs before it finish")
		p.Actions = []giode.Element{button("job-cancel", tr.T("Cancel"), "x", true, func() { s.CancelJob(j) })}
	case store.JobRunning:
		p.Status, p.Tone = tr.T("Running"), theme.Primary
		p.Running, p.Frac, p.Stage, p.Line = true, j.Update.Frac, j.Update.Stage, j.Update.Line
		p.Clock = components.RunClock(j.Started, j.Log.Last, j.TimeLeft)
		p.Detail = tr.T("Saving to {path}", "path", j.Output)
		p.Actions = []giode.Element{
			components.CopyAction("job-copy-log", j, tr.T("Copy log"), func() string {
				return s.RunningReport(j.Name, j.Update, j.Started, &j.Log)
			}),
			button("job-cancel", tr.T("Cancel"), "x", true, func() { s.CancelJob(j) }),
		}
	case store.JobDone:
		p.Status, p.Tone = tr.T("Done"), theme.Success
		p.Detail = j.Output + " · " + tr.T("took {time}", "time", format.Elapsed(j.Finished.Sub(j.Started)))
		p.Actions = []giode.Element{
			button("job-open", tr.T("Open"), "external-link", true, func() {
				go s.App.OpenURL(context.Background(), "file://"+j.Output)
			}),
			button("job-reveal", tr.T("Show in folder"), "folder-open", false, func() { s.Reveal(j.Output) }),
		}
	case store.JobFailed:
		p.Status, p.Tone = tr.T("Failed"), theme.Danger
		p.Detail = j.Err
		report := func() string { return s.ErrorReport(tr.T("{file} failed", "file", j.Name), j.Err, j.ErrLog) }
		p.Actions = []giode.Element{
			components.CopyAction("job-copy", j, tr.T("Copy"), report),
			button("job-details", tr.T("Details"), "", true, func() { q.errors.Show(tr.T("{file} failed", "file", j.Name), report()) }),
			button("job-retry", tr.T("Try again"), "rotate-ccw", false, func() { s.RetryJob(j) }),
		}
	case store.JobCanceled:
		p.Status, p.Tone = tr.T("Canceled"), theme.Muted
		p.Detail = tr.T("Stopped before it finished")
		p.Actions = []giode.Element{button("job-retry", tr.T("Run again"), "rotate-ccw", false, func() { s.RetryJob(j) })}
	}
	return components.JobCard(p)
}
