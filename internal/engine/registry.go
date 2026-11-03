package engine

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/mehran9675/touchup/internal/paths"
)

//go:embed catalog/*.json
var catalogFS embed.FS

// Registry is the set of known engines: the built-in catalog and the
// engines the user added. It is safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	engines []*Manifest
}

// Load reads the catalog and the user's custom manifests. Broken custom
// manifests are skipped and returned as errors.
func Load() (*Registry, []error) {
	r := &Registry{}
	var errs []error
	entries, _ := catalogFS.ReadDir("catalog")
	for _, e := range entries {
		data, _ := catalogFS.ReadFile("catalog/" + e.Name())
		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			errs = append(errs, fmt.Errorf("catalog %s: %w", e.Name(), err))
			continue
		}
		r.engines = append(r.engines, &m)
	}
	custom, _ := filepath.Glob(filepath.Join(paths.CustomManifests(), "*.json"))
	for _, file := range custom {
		m, err := ReadManifest(file)
		if err == nil && m.Builtin() {
			err = fmt.Errorf("%s: install.type must be archive, pip or none", filepath.Base(file))
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		m.Custom = true
		r.put(m)
	}
	r.sort()
	return r, errs
}

// ReadManifest reads and validates a manifest file.
func ReadManifest(file string) (*Manifest, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return ParseManifest(data)
}

// ParseManifest decodes and validates a manifest.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("not a valid manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// All returns every engine, built-in first, alphabetically.
func (r *Registry) All() []*Manifest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Clone(r.engines)
}

// For returns the engines that upscale kind.
func (r *Registry) For(kind string) []*Manifest {
	var out []*Manifest
	for _, m := range r.All() {
		if m.Handles(kind) {
			out = append(out, m)
		}
	}
	return out
}

// Get returns the engine with id, or nil.
func (r *Registry) Get(id string) *Manifest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.engines {
		if m.ID == id {
			return m
		}
	}
	return nil
}

// AddCustom saves a user manifest and adds it, replacing a custom engine
// with the same ID. Built-in IDs can't be replaced.
func (r *Registry) AddCustom(m *Manifest) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if existing := r.Get(m.ID); existing != nil && !existing.Custom {
		return fmt.Errorf("%q is a built-in engine; choose another ID", m.ID)
	}
	if err := os.MkdirAll(paths.CustomManifests(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(paths.CustomManifests(), m.ID+".json"), data, 0o644); err != nil {
		return err
	}
	m.Custom = true
	r.mu.Lock()
	r.put(m)
	r.sort()
	r.mu.Unlock()
	return nil
}

// RemoveCustom deletes a user manifest. Uninstall the engine first.
func (r *Registry) RemoveCustom(id string) error {
	m := r.Get(id)
	if m == nil || !m.Custom {
		return fmt.Errorf("%q is not a custom engine", id)
	}
	if err := os.Remove(filepath.Join(paths.CustomManifests(), id+".json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	r.mu.Lock()
	r.engines = slices.DeleteFunc(r.engines, func(e *Manifest) bool { return e.ID == id })
	r.mu.Unlock()
	return nil
}

// put adds or replaces m; the caller holds the lock or owns r.
func (r *Registry) put(m *Manifest) {
	for i, e := range r.engines {
		if e.ID == m.ID {
			r.engines[i] = m
			return
		}
	}
	r.engines = append(r.engines, m)
}

func (r *Registry) sort() {
	slices.SortStableFunc(r.engines, func(a, b *Manifest) int {
		if a.Custom != b.Custom {
			if a.Custom {
				return 1
			}
			return -1
		}
		if a.Featured != b.Featured {
			if a.Featured {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
}
