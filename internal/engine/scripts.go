package engine

import (
	"bytes"
	"context"
	"errors"
	"runtime"
	"slices"
	"strings"
	"time"

	"embed"
	"os"
	"path/filepath"

	"github.com/mehran9675/touchup/internal/proc"
	"github.com/mehran9675/touchup/internal/tr"
)

// Runner scripts that let TouchUp drive Python libraries that come
// without a command line of their own.
//
//go:embed scripts/*.py
var scriptsFS embed.FS

// installScripts copies the engine's runner scripts into its folder,
// rewriting only those that differ. Runs call it too, so an engine
// installed by an older TouchUp gets the scripts that ship with this one.
func installScripts(m *Manifest) error {
	for _, name := range m.Install.Scripts {
		data, err := scriptsFS.ReadFile("scripts/" + name)
		if err != nil {
			return err
		}
		target := filepath.Join(m.Dir(), name)
		if current, err := os.ReadFile(target); err == nil && bytes.Equal(current, data) {
			continue
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// UsesTorch reports whether an engine installs PyTorch, so a GPU check
// means something for it.
func (m *Manifest) UsesTorch() bool {
	if m.Install.Type != "pip" {
		return false
	}
	args := slices.Clone(m.Install.Packages)
	for _, step := range m.Install.Pip {
		args = append(args, step.Args...)
	}
	return slices.ContainsFunc(args, func(a string) bool { return strings.HasPrefix(a, "torch") })
}

// CheckGPU runs a short check of the GPU with the engine's PyTorch and
// returns its report.
func CheckGPU(ctx context.Context, m *Manifest) (string, error) {
	py := filepath.Join(m.venvBin(), "python")
	if runtime.GOOS == "windows" {
		py += ".exe"
	}
	if !exists(py) {
		return "", errors.New(tr.T("{engine} is not installed", "engine", m.Name))
	}
	data, err := scriptsFS.ReadFile("scripts/gpu_check.py")
	if err != nil {
		return "", err
	}
	script := filepath.Join(m.Dir(), "touchup_gpu_check.py")
	if err := os.WriteFile(script, data, 0o644); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := proc.Command(ctx, py, script)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
