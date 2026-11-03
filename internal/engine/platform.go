package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/mehran9675/touchup/internal/paths"
)

// pick returns the value for this platform from a map keyed by
// "goos/goarch", "goos" or "*".
func pick(m map[string]string) string {
	for _, key := range []string{runtime.GOOS + "/" + runtime.GOARCH, runtime.GOOS, "*"} {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return ""
}

// Supported reports whether the engine can be installed on this
// platform.
func (m *Manifest) Supported() bool {
	if m.Builtin() {
		return builtinSupported(m.ID)
	}
	if m.Install.Type == "archive" && pick(m.Install.URLs) == "" {
		return false
	}
	return pick(m.Executable) != ""
}

// Dir is the engine's install folder.
func (m *Manifest) Dir() string { return paths.Engine(m.ID) }

// venvBin is the scripts folder of the engine's Python environment.
func (m *Manifest) venvBin() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(m.Dir(), "venv", "Scripts")
	}
	return filepath.Join(m.Dir(), "venv", "bin")
}

// ExecutablePath resolves the program to run, or "" when it isn't there.
func (m *Manifest) ExecutablePath() string {
	name := pick(m.Executable)
	if name == "" {
		return ""
	}
	if filepath.IsAbs(name) {
		if exists(name) {
			return name
		}
		return ""
	}
	var dirs []string
	switch m.Install.Type {
	case "pip":
		dirs = []string{m.venvBin()}
	case "archive":
		dirs = []string{m.Dir()}
	}
	for _, dir := range dirs {
		for _, candidate := range withExeSuffix(filepath.Join(dir, name)) {
			if exists(candidate) {
				return candidate
			}
		}
	}
	if m.Install.Type == "none" {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// Installed reports whether the engine is ready to run.
func (m *Manifest) Installed() bool {
	if m.Builtin() {
		return m.Supported()
	}
	if m.Install.Type != "none" && !exists(filepath.Join(m.Dir(), installedMarker)) {
		return false
	}
	return m.ExecutablePath() != ""
}

func withExeSuffix(p string) []string {
	if runtime.GOOS == "windows" && filepath.Ext(p) == "" {
		return []string{p + ".exe", p + ".cmd", p + ".bat", p}
	}
	return []string{p}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
