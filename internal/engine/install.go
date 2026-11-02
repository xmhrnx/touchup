package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mehran9675/touchup/internal/tr"
)

// installedMarker is written into an engine's folder once its install
// finished, so half-finished installs aren't mistaken for working ones.
const installedMarker = ".touchup-installed"

// Status reports install progress.
type Status struct {
	Stage string  // "Downloading", "Unpacking", "Installing packages"...
	Frac  float64 // 0..1, or -1 when unknown
	Line  string  // the latest line of tool output, if any
}

// Report receives install progress, from a background goroutine.
type Report func(Status)

// Install installs the engine. python is the interpreter used to create
// Python environments; empty finds python3 or python on PATH.
func Install(ctx context.Context, m *Manifest, python string, report Report) error {
	if !m.Supported() {
		return errors.New(tr.T("this engine has no build for your system"))
	}
	if m.Install.Type == "none" {
		if m.ExecutablePath() == "" {
			return errors.New(tr.T("{program} was not found; check the executable path", "program", pick(m.Executable)))
		}
		return nil
	}
	dir := m.Dir()
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var err error
	switch m.Install.Type {
	case "archive":
		err = installArchive(ctx, m, report)
	case "pip":
		err = installPip(ctx, m, python, report)
	default:
		err = fmt.Errorf("unknown install type %q", m.Install.Type)
	}
	if err != nil {
		os.RemoveAll(dir)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	if m.ExecutablePath() == "" {
		os.RemoveAll(dir)
		return errors.New(tr.T("installed, but {program} wasn't found in it", "program", pick(m.Executable)))
	}
	return os.WriteFile(filepath.Join(dir, installedMarker), []byte(time.Now().Format(time.RFC3339)), 0o644)
}

// Uninstall removes the engine's folder.
func Uninstall(m *Manifest) error {
	if m.Install.Type == "none" {
		return nil
	}
	return os.RemoveAll(m.Dir())
}

func installArchive(ctx context.Context, m *Manifest, report Report) error {
	url := pick(m.Install.URLs)
	tmp, err := os.CreateTemp("", "touchup-engine-*"+archiveExt(url))
	if err != nil {
		return err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if err := download(ctx, url, tmp.Name(), func(frac float64) {
		report(Status{Stage: tr.T("Downloading"), Frac: frac})
	}); err != nil {
		return err
	}
	report(Status{Stage: tr.T("Unpacking"), Frac: -1})
	if err := extract(tmp.Name(), m.Dir()); err != nil {
		return err
	}
	if exe := m.ExecutablePath(); exe != "" {
		os.Chmod(exe, 0o755)
	}
	return nil
}
