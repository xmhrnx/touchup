package store

import (
	"context"
	"errors"
	"io"

	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/tr"
)

// Install is an engine install in progress or failed.
type Install struct {
	Status engine.Status
	Err    string
	Log    string // what the installer printed, when it failed
	cancel context.CancelFunc
}

// Running reports whether the install is still going.
func (i *Install) Running() bool { return i != nil && i.cancel != nil }

// InstallEngine downloads and sets up an engine in the background.
func (s *Store) InstallEngine(m *engine.Manifest) {
	if s.Installs[m.ID].Running() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	in := &Install{cancel: cancel, Status: engine.Status{Stage: tr.T("Starting"), Frac: -1}}
	s.Installs[m.ID] = in
	python := s.Settings.Python
	go func() {
		err := engine.Install(ctx, m, python, func(st engine.Status) {
			s.post(func() { in.Status = st })
		})
		s.post(func() {
			in.cancel = nil
			switch {
			case errors.Is(err, context.Canceled):
				delete(s.Installs, m.ID)
			case err != nil:
				in.Err, in.Log = err.Error(), engine.LogOf(err)
				s.Toasts.Show(giode.ToastError, tr.T("Couldn't install {engine}", "engine", m.Name))
			default:
				delete(s.Installs, m.ID)
				s.Toasts.Show(giode.ToastSuccess, tr.T("{engine} is installed", "engine", m.Name))
			}
			s.locateTools() // Python may have been downloaded
			s.refreshInstalled()
		})
	}()
}

// CancelInstall stops an install.
func (s *Store) CancelInstall(id string) {
	if in := s.Installs[id]; in.Running() {
		in.cancel()
	}
}

// DismissInstallError forgets a failed install.
func (s *Store) DismissInstallError(id string) { delete(s.Installs, id) }

// UninstallEngine removes an engine's files.
func (s *Store) UninstallEngine(m *engine.Manifest) {
	if err := engine.Uninstall(m); err != nil {
		s.fail(err)
	}
	s.refreshInstalled()
	s.Toasts.Show(giode.ToastInfo, tr.T("{engine} was removed", "engine", m.Name))
}

// AddCustomEngine saves a user's engine.
func (s *Store) AddCustomEngine(m *engine.Manifest) error {
	if err := s.Registry.AddCustom(m); err != nil {
		return err
	}
	s.refreshInstalled()
	s.Toasts.Show(giode.ToastSuccess, tr.T("{engine} was added", "engine", m.Name))
	return nil
}

// RemoveCustomEngine uninstalls a user's engine and forgets it.
func (s *Store) RemoveCustomEngine(m *engine.Manifest) {
	engine.Uninstall(m)
	if err := s.Registry.RemoveCustom(m.ID); err != nil {
		s.fail(err)
		return
	}
	s.refreshInstalled()
	s.Toasts.Show(giode.ToastInfo, tr.T("{engine} was removed", "engine", m.Name))
}

// ImportManifest asks for a manifest file and adds its engine.
func (s *Store) ImportManifest() {
	go func() {
		f, err := s.App.OpenFile(context.Background(), giode.FileFilter{Extensions: []string{"json"}})
		if errors.Is(err, giode.ErrCanceled) {
			return
		}
		if err != nil {
			s.fail(err)
			return
		}
		data, err := io.ReadAll(io.LimitReader(f, 1<<20))
		f.Close()
		if err != nil {
			s.fail(err)
			return
		}
		m, err := engine.ParseManifest(data)
		if err != nil {
			s.fail(err)
			return
		}
		s.post(func() { s.fail(s.AddCustomEngine(m)) })
	}()
}

// InstallModel downloads a model in the background.
func (s *Store) InstallModel(m *engine.Manifest, mdl *engine.Model) {
	key := ModelKey(m, mdl)
	if s.ModelInstalls[key].Running() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	in := &Install{cancel: cancel, Status: engine.Status{Stage: tr.T("Starting"), Frac: -1}}
	s.ModelInstalls[key] = in
	go func() {
		err := engine.InstallModel(ctx, m, mdl, func(st engine.Status) {
			s.post(func() { in.Status = st })
		})
		s.post(func() {
			in.cancel = nil
			switch {
			case errors.Is(err, context.Canceled):
				delete(s.ModelInstalls, key)
			case err != nil:
				in.Err, in.Log = err.Error(), engine.LogOf(err)
				s.Toasts.Show(giode.ToastError, tr.T("Couldn't download {model}", "model", tr.T(mdl.Name)))
			default:
				delete(s.ModelInstalls, key)
				s.Toasts.Show(giode.ToastSuccess, tr.T("{model} is ready to use", "model", tr.T(mdl.Name)))
			}
			s.refreshInstalled()
		})
	}()
}

// CancelModelInstall stops a model download.
func (s *Store) CancelModelInstall(m *engine.Manifest, mdl *engine.Model) {
	if in := s.ModelInstalls[ModelKey(m, mdl)]; in.Running() {
		in.cancel()
	}
}

// DismissModelError forgets a failed model download.
func (s *Store) DismissModelError(m *engine.Manifest, mdl *engine.Model) {
	delete(s.ModelInstalls, ModelKey(m, mdl))
}

// UninstallModel deletes a model's files.
func (s *Store) UninstallModel(m *engine.Manifest, mdl *engine.Model) {
	if err := engine.UninstallModel(m, mdl); err != nil {
		s.fail(err)
	}
	s.refreshInstalled()
	s.Toasts.Show(giode.ToastInfo, tr.T("{model} was removed", "model", tr.T(mdl.Name)))
}

// CheckGPU runs an engine's GPU check in the background and calls done
// with its report on the UI goroutine.
func (s *Store) CheckGPU(m *engine.Manifest, done func(report string, err error)) {
	go func() {
		report, err := engine.CheckGPU(context.Background(), m)
		s.post(func() { done(report, err) })
	}()
}
