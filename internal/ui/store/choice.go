package store

import (
	"errors"
	"fmt"
	"slices"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/pipeline"
	"github.com/mehran9675/touchup/internal/settings"
	"github.com/mehran9675/touchup/internal/tr"
)

// Selection is the engine, model and size chosen for a kind of media,
// resolved against what's installed and the open file.
type Selection struct {
	Engine *engine.Manifest // nil when no engine for the kind is installed
	Model  *engine.Model    // nil when the engine has no installed model
	Scale  int
	Target int // a resolution's short side, which replaces Scale; or 0
}

// Size plans the result for a w×h input.
func (sel Selection) Size(w, h int) pipeline.Size {
	return pipeline.PlanSize(sel.Engine, sel.Model, sel.Scale, sel.Target, w, h)
}

// SizeLabel names the chosen size: "4x", "1080p" or "4K"; empty for
// engines without one.
func (sel Selection) SizeLabel() string {
	switch {
	case sel.Target > 0:
		return pipeline.TargetLabel(sel.Target)
	case sel.Scale > 1:
		return fmt.Sprintf("%dx", sel.Scale)
	}
	return ""
}

// Targets lists the resolutions sel can make from a w×h input: those
// larger than it, and for engines with fixed scales only those within
// their largest scale, so the last step is never a big plain resize.
func (sel Selection) Targets(w, h int) []int {
	short := min(w, h)
	scales := sel.Engine.ScalesFor(sel.Model)
	if short <= 0 || len(scales) == 0 {
		return nil
	}
	var out []int
	for _, t := range pipeline.Targets {
		if t <= short {
			continue
		}
		if !sel.Engine.TakesResolution() && t > short*slices.Max(scales) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// Ready reports whether the selection can run: an installed engine, and
// an installed model when the engine has models.
func (sel Selection) Ready() bool {
	return sel.Engine != nil && (sel.Model != nil || len(sel.Engine.Models) == 0)
}

// problem explains why the selection can't run, or returns nil.
func (sel Selection) problem(kind media.Kind) error {
	switch {
	case sel.Engine == nil:
		switch kind {
		case media.Image:
			return errors.New(tr.T("install an engine for images first"))
		case media.Video:
			return errors.New(tr.T("install an engine for video first"))
		}
		return errors.New(tr.T("install an engine for audio first"))
	case !sel.Ready():
		return errors.New(tr.T("install a model for {engine} on the Engines page first", "engine", sel.Engine.Name))
	}
	return nil
}

// Label describes the selection: "Real-ESRGAN · General · 4x".
func (sel Selection) Label() string {
	if sel.Engine == nil {
		return ""
	}
	l := sel.Engine.Name
	if sel.Model != nil {
		l += " · " + tr.T(sel.Model.Name)
	}
	if size := sel.SizeLabel(); size != "" {
		l += " · " + size
	}
	return l
}

// EnginesFor lists the engines that handle kind, installed ones first.
func (s *Store) EnginesFor(kind media.Kind) []*engine.Manifest {
	list := s.Registry.For(kind.String())
	slices.SortStableFunc(list, func(a, b *engine.Manifest) int {
		ia, ib := s.Installed(a.ID), s.Installed(b.ID)
		switch {
		case ia && !ib:
			return -1
		case ib && !ia:
			return 1
		}
		return 0
	})
	return list
}

// Selected returns the selection for kind: the remembered one when its
// engine is still installed, otherwise the first installed engine.
func (s *Store) Selected(kind media.Kind) Selection {
	c := s.Settings.Choices[kind.String()]
	m := s.Registry.Get(c.Engine)
	if m == nil || !s.Installed(m.ID) || !m.Handles(kind.String()) {
		m = nil
		for _, e := range s.EnginesFor(kind) {
			if s.Installed(e.ID) {
				m = e
				break
			}
		}
		if m == nil {
			return Selection{}
		}
		c = settings.Choice{Engine: m.ID, Target: c.Target}
	}
	model := s.installedModel(m, c.Model)
	scale := c.Scale
	if scales := m.ScalesFor(model); !slices.Contains(scales, scale) {
		scale = m.PreferredScale(model)
	}
	if len(m.ScalesFor(model)) == 0 {
		scale = 1
	}
	sel := Selection{Engine: m, Model: model, Scale: scale}
	// A target holds only for files it enlarges.
	if sess := s.Session; c.Target > 0 && sess != nil && sess.Kind == kind &&
		slices.Contains(sel.Targets(sess.Info.Width, sess.Info.Height), c.Target) {
		sel.Target = c.Target
	}
	return sel
}

// installedModel returns the model with id when it's downloaded,
// otherwise the first downloaded one, recommended ones first, or nil.
func (s *Store) installedModel(m *engine.Manifest, id string) *engine.Model {
	var first, recommended *engine.Model
	for i := range m.Models {
		mdl := &m.Models[i]
		if !s.ModelInstalled(m, mdl) {
			continue
		}
		if mdl.ID == id {
			return mdl
		}
		if first == nil {
			first = mdl
		}
		if recommended == nil && mdl.Recommended {
			recommended = mdl
		}
	}
	if recommended != nil {
		return recommended
	}
	return first
}

// Choose remembers an engine, model and scale for kind. Empty values
// keep the current ones; a new engine resets the model and scale, and
// a scale replaces a chosen resolution.
func (s *Store) Choose(kind media.Kind, engineID, modelID string, scale int) {
	cur := s.Selected(kind)
	c := settings.Choice{}
	if cur.Engine != nil {
		c.Engine = cur.Engine.ID
		if cur.Model != nil {
			c.Model = cur.Model.ID
		}
		c.Scale, c.Target = cur.Scale, s.Settings.Choices[kind.String()].Target
	}
	if engineID != "" && engineID != c.Engine {
		c = settings.Choice{Engine: engineID, Target: c.Target} // a resolution suits any engine
	}
	if modelID != "" {
		c.Model = modelID
	}
	if scale > 0 {
		c.Scale, c.Target = scale, 0
	}
	s.setChoice(kind, c)
}

// ChooseTarget picks a resolution for kind instead of a scale.
func (s *Store) ChooseTarget(kind media.Kind, target int) {
	c := s.Settings.Choices[kind.String()]
	c.Target = target
	s.setChoice(kind, c)
}

func (s *Store) setChoice(kind media.Kind, c settings.Choice) {
	if s.Settings.Choices == nil {
		s.Settings.Choices = map[string]settings.Choice{}
	}
	s.Settings.Choices[kind.String()] = c
	s.saveSettings()
}

// options builds pipeline options for kind's selection.
func (s *Store) options(kind media.Kind) (pipeline.Options, Selection) {
	sel := s.Selected(kind)
	return pipeline.Options{
		Engine: sel.Engine, Model: sel.Model, Scale: sel.Scale, Target: sel.Target,
		EngineOptions: s.EngineOptions(sel.Engine),
		Settings:      s.Settings, Tools: s.Tools,
	}, sel
}
