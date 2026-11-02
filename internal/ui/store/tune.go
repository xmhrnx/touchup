package store

import (
	"context"
	"errors"
	"time"

	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/pipeline"
	"github.com/mehran9675/touchup/internal/tr"
)

// CanTune reports whether the selected video engine has settings to
// time on the open file.
func (s *Store) CanTune() bool {
	sess := s.Session
	if sess == nil || sess.Kind != media.Video || sess.Loading || sess.Err != "" || sess.Preview.Running {
		return false
	}
	sel := s.Selected(sess.Kind)
	return sel.Ready() && sel.Engine.Tune != nil
}

// RunTune times the selected engine's settings on a short clip of the
// open video around the playhead and keeps the fastest. It runs in the
// preview's place, with its progress panel and Cancel.
func (s *Store) RunTune() {
	if !s.CanTune() {
		return
	}
	sess := s.Session
	opts, sel := s.options(sess.Kind)
	m := sel.Engine
	s.Player.Stop()
	ctx, cancel := context.WithCancel(sess.ctx)
	p := &sess.Preview
	p.Running, p.Tuning, p.Err, p.ErrLog, p.cancel = true, true, "", "", cancel
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
	at := sess.Time
	go func() {
		res, err := pipeline.Tune(ctx, opts, sess.Path, at, report)
		s.post(func() {
			p.Running, p.Tuning, p.cancel = false, false, nil
			switch {
			case errors.Is(err, context.Canceled):
				return
			case err != nil:
				p.Err, p.ErrLog = tr.T("Timing {engine} failed: {reason}", "engine", m.Name, "reason", errSummary(err)), engine.LogOf(err)
				return
			}
			s.applyTuning(m, res)
		})
	}()
}

// applyTuning saves the settings tuning kept and shows what it found.
func (s *Store) applyTuning(m *engine.Manifest, res pipeline.TuneResult) {
	changed := false
	for id, v := range res.Best {
		if res.Start[id] != v {
			s.SetEngineOption(m, id, v)
			changed = true
		}
	}
	if changed {
		s.Toasts.Show(giode.ToastSuccess, tr.T("Saved the fastest settings for {engine}", "engine", m.Name))
	} else {
		s.Toasts.Show(giode.ToastInfo, tr.T("{engine}'s settings were already the fastest", "engine", m.Name))
	}
	if s.ShowReport != nil {
		s.ShowReport(tr.T("{engine} timed on this computer", "engine", m.Name),
			tr.T("Each setting was tried on a short clip of this video; the fastest that worked is saved."),
			res.Describe(m))
	}
}
