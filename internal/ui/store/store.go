// Package store holds TouchUp's application state and the actions that
// change it. Everything here runs on the UI goroutine; background work
// reports back through App.Post.
package store

import (
	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/settings"
	"github.com/mehran9675/touchup/internal/tr"
)

// Page is a top-level screen.
type Page int

const (
	PageStudio Page = iota
	PageEngines
	PageQueue
	PageSettings
)

// Store is the application state.
type Store struct {
	App    *giode.App
	Toasts *giode.ToasterElement
	// ShowReport shows a long text, such as tuning results, in a dialog;
	// the shell sets it.
	ShowReport func(title, subtitle, text string)

	Registry *engine.Registry
	Settings settings.Settings
	Tools    media.Tools
	Python   string // the Python engines use, once there is one
	// PythonNote says where Python comes from, for Settings.
	PythonNote string

	Page    Page
	Session *Session
	Jobs    []*Job

	// Installs tracks engines being installed, by ID, and
	// ModelInstalls models being downloaded, by ModelKey.
	Installs      map[string]*Install
	ModelInstalls map[string]*Install

	Player  *media.Player
	Playing string // the file playing, or ""

	installed map[string]bool // by engine ID and by ModelKey
	nextID    int
}

// New loads the settings and engines.
func New(toasts *giode.ToasterElement) *Store {
	reg, errs := engine.Load()
	s := &Store{
		Toasts:        toasts,
		Registry:      reg,
		Settings:      settings.Load(),
		Installs:      map[string]*Install{},
		ModelInstalls: map[string]*Install{},
		Player:        &media.Player{},
		installed:     map[string]bool{},
	}
	s.locateTools()
	s.refreshInstalled()
	for _, err := range errs {
		toasts.Show(giode.ToastWarning, tr.T("Skipped an engine manifest: {reason}", "reason", err.Error()))
	}
	return s
}

// Attach connects the store to the running app, for posting results
// back from background work.
func (s *Store) Attach(app *giode.App) {
	s.App = app
	s.Player.OnChange = func(playing string) {
		app.Post(func() { s.Playing = playing })
	}
}

// Navigate switches pages.
func (s *Store) Navigate(p Page) { s.Page = p }

// Installed reports whether engine id is ready to run.
func (s *Store) Installed(id string) bool { return s.installed[id] }

// ModelInstalled reports whether a model of engine m is downloaded.
func (s *Store) ModelInstalled(m *engine.Manifest, mdl *engine.Model) bool {
	return s.installed[ModelKey(m, mdl)]
}

// ModelKey identifies a model of an engine.
func ModelKey(m *engine.Manifest, mdl *engine.Model) string { return m.ID + "/" + mdl.ID }

// InstalledModels counts an engine's downloaded models.
func (s *Store) InstalledModels(m *engine.Manifest) int {
	n := 0
	for i := range m.Models {
		if s.ModelInstalled(m, &m.Models[i]) {
			n++
		}
	}
	return n
}

// refreshInstalled checks which engines and models are installed. It
// touches the disk, so it runs when something changed rather than every
// frame.
func (s *Store) refreshInstalled() {
	for _, m := range s.Registry.All() {
		s.installed[m.ID] = m.Installed()
		for i := range m.Models {
			mdl := &m.Models[i]
			s.installed[ModelKey(m, mdl)] = s.installed[m.ID] && m.ModelInstalled(mdl)
		}
	}
}

func (s *Store) locateTools() {
	st := s.Settings
	s.Tools = media.Locate(st.FFmpeg, st.FFprobe, st.FFplay)
	s.Python, s.PythonNote = engine.PythonStatus(st.Python)
}

func (s *Store) id() int {
	s.nextID++
	return s.nextID
}

// post runs fn on the UI goroutine.
func (s *Store) post(fn func()) { s.App.Post(fn) }

// fail shows an error toast; safe from any goroutine.
func (s *Store) fail(err error) {
	if err != nil {
		s.Toasts.Show(giode.ToastError, err.Error())
	}
}
