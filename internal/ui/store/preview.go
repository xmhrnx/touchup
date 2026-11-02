package store

import (
	"context"
	"errors"
	"time"

	"github.com/mehran9675/touchup/internal/engine"

	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/pipeline"
	"github.com/mehran9675/touchup/internal/tr"
)

// RunPreview upscales the selected video frame or audio section.
func (s *Store) RunPreview() {
	sess := s.Session
	if sess == nil || sess.Loading || sess.Preview.Running {
		return
	}
	opts, sel := s.options(sess.Kind)
	if err := sel.problem(sess.Kind); err != nil {
		s.fail(err)
		return
	}
	s.Player.Stop()
	ctx, cancel := context.WithCancel(sess.ctx)
	p := &sess.Preview
	p.Running, p.Err, p.ErrLog, p.cancel = true, "", "", cancel
	p.Update, p.Started = pipeline.Update{Stage: tr.T("Starting"), Frac: 0}, time.Now()
	p.Log.reset()
	p.Pace.reset()
	report := func(u pipeline.Update) {
		s.post(func() {
			if u.Line != "" {
				p.Log.add(u.Line)
			}
			if u.Frac >= 0 && u.Frac > p.Update.Frac {
				p.Pace.add(u.Frac)
			}
			if u.Frac < 0 {
				u.Frac = p.Update.Frac
			}
			if u.Line == "" {
				u.Line = p.Update.Line
			}
			p.Update = u
		})
	}
	label := sel.Label()

	switch sess.Kind {
	case media.Video:
		at := sess.Time
		go func() {
			res, err := pipeline.PreviewFrame(ctx, opts, sess.Path, at, report)
			s.post(func() {
				p.Running, p.cancel = false, nil
				if err != nil {
					if !errors.Is(err, context.Canceled) {
						p.Err, p.ErrLog = errSummary(err), engine.LogOf(err)
					}
					return
				}
				p.Frame, p.At, p.Label = &res, at, label
			})
		}()
	case media.Audio:
		start, length := sess.SelStart, sess.SelLen
		go func() {
			res, err := pipeline.PreviewAudio(ctx, opts, sess.Path, start, length, report)
			s.post(func() {
				p.Running, p.cancel = false, nil
				if err != nil {
					if !errors.Is(err, context.Canceled) {
						p.Err, p.ErrLog = errSummary(err), engine.LogOf(err)
					}
					return
				}
				p.Audio.Discard()
				p.Audio, p.At, p.Length, p.Label = res, start, length, label
			})
		}()
	default:
		cancel()
		p.Running = false
	}
}

// CancelPreview stops a running preview.
func (s *Store) CancelPreview() {
	if sess := s.Session; sess != nil && sess.Preview.cancel != nil {
		sess.Preview.cancel()
	}
}

// RunningReport is a running preview's report so far, to copy when it
// seems stuck.
func (s *Store) RunningReport(title string, u pipeline.Update, started time.Time, log *LiveLog) string {
	status := u.Stage + " · running for " + time.Since(started).Round(time.Second).String()
	if !log.Last.IsZero() {
		status += " · last output " + time.Since(log.Last).Round(time.Second).String() + " ago"
	}
	return s.ErrorReport(title+" (still running)", status, log.String())
}

// DismissPreviewError clears a failed preview's message.
func (s *Store) DismissPreviewError() {
	if sess := s.Session; sess != nil {
		sess.Preview.Err = ""
	}
}

// Play plays path, or stops it when it's already playing.
func (s *Store) Play(path string) {
	if s.Playing == path {
		s.Player.Stop()
		return
	}
	if err := s.Player.Play(s.Tools, path); err != nil {
		s.fail(err)
	}
}

// errSummary is the short form of an error for a banner or a card: an
// engine's own reason, without the "Engine failed:" prefix the title
// already says.
func errSummary(err error) string {
	var re *engine.RunError
	if errors.As(err, &re) {
		return re.Summary
	}
	return err.Error()
}
