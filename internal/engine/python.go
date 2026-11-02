package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mehran9675/touchup/internal/paths"
	"github.com/mehran9675/touchup/internal/proc"
	"github.com/mehran9675/touchup/internal/tr"
)

// TouchUp runs Python engines on its own copy of Python, a standalone
// build from python-build-standalone, rather than whatever the system
// has: on Windows "python" is often only the Microsoft Store's shortcut,
// Linux distributions leave out venv, and the newest Python usually has
// no PyTorch builds yet. A Python set in Settings is used instead when
// it passes the same checks.
const (
	pythonVersion = "3.12.15"
	pythonRelease = "20261009"
)

// pythonBuilds are the standalone builds and their SHA-256 sums, by
// "goos/goarch".
var pythonBuilds = map[string]struct{ target, sha256 string }{
	"windows/amd64": {"x86_64-pc-windows-msvc", "194c8f0d9e0de6f1c4576e5dfa67819a48c764be87608b74b8ad43387a57b93b"},
	"windows/arm64": {"aarch64-pc-windows-msvc", "4da354e7366d4437fceea714eab1bbdca1eb17bba4491a3d20b9c6af94b44f68"},
	"linux/amd64":   {"x86_64-unknown-linux-gnu", "5864001bf969db329fadc2b654c4878c6027d170c6bd77e28f88666260bcaf63"},
	"linux/arm64":   {"aarch64-unknown-linux-gnu", "b342ccfb577fda349d0237c98b24848234ca767c10f034628a5ff9fc858d662f"},
	"darwin/amd64":  {"x86_64-apple-darwin", "4c84ef5abe4bc145646175fce9b881936d60126b9a3f9a24c6308e2d035ca5c6"},
	"darwin/arm64":  {"aarch64-apple-darwin", "83dcb04777c798bd95f3687acf9ae84b09860272ee6bdfb05be8bcf9447f75b8"},
}

// Python versions engines are known to install on: PyTorch supports
// these.
const minPython, maxPython = 9, 13

// ManagedPython is where TouchUp keeps its own Python.
func ManagedPython() string {
	dir := filepath.Join(paths.Data(), "python", pythonVersion)
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, "python.exe")
	}
	return filepath.Join(dir, "bin", "python3")
}

// PythonStatus describes the Python engines will use, for Settings.
func PythonStatus(configured string) (path, note string) {
	if configured != "" {
		return configured, "Set in Settings"
	}
	if exists(ManagedPython()) {
		return ManagedPython(), "TouchUp's own Python " + pythonVersion
	}
	return "", tr.T("TouchUp downloads its own Python {version} the first time you install a Python engine.", "version", pythonVersion)
}

// EnsurePython returns a Python that can create environments for
// engines: the configured one after checking it, or TouchUp's own,
// downloaded the first time.
func EnsurePython(ctx context.Context, configured string, report Report) (string, error) {
	if configured != "" {
		if err := checkPython(ctx, configured); err != nil {
			return "", fmt.Errorf("%s: %w. %s", tr.T("the Python set in Settings can't be used"), err, tr.T("Clear it to let TouchUp use its own."))
		}
		return configured, nil
	}
	py := ManagedPython()
	if exists(py) && checkPython(ctx, py) == nil {
		return py, nil
	}
	build, ok := pythonBuilds[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return "", errors.New(tr.T("there's no standalone Python for this system; install Python 3.12 and set its path in Settings"))
	}
	name := fmt.Sprintf("cpython-%s+%s-%s-install_only.tar.gz", pythonVersion, pythonRelease, build.target)
	url := "https://github.com/astral-sh/python-build-standalone/releases/download/" + pythonRelease + "/" + name
	tmp, err := os.CreateTemp("", "touchup-python-*.tar.gz")
	if err != nil {
		return "", err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if err := download(ctx, url, tmp.Name(), func(frac float64) {
		report(Status{Stage: tr.T("Downloading Python"), Frac: frac})
	}); err != nil {
		return "", fmt.Errorf("%s: %w", tr.T("downloading Python"), err)
	}
	report(Status{Stage: tr.T("Verifying Python"), Frac: -1})
	if err := verify(tmp.Name(), build.sha256); err != nil {
		return "", fmt.Errorf("%s: %w", tr.T("downloading Python"), err)
	}
	report(Status{Stage: tr.T("Unpacking Python"), Frac: -1})
	dir := filepath.Dir(py)
	if runtime.GOOS != "windows" {
		dir = filepath.Dir(dir) // bin/python3
	}
	os.RemoveAll(dir)
	if err := extract(tmp.Name(), dir); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("%s: %w", tr.T("unpacking Python"), err)
	}
	if err := checkPython(ctx, py); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("%s: %w", tr.T("the downloaded Python doesn't run"), err)
	}
	return py, nil
}

// checkPython runs py to make sure it's a real Python of a version
// PyTorch supports, with venv and pip's bootstrapper.
func checkPython(ctx context.Context, py string) error {
	if strings.Contains(strings.ToLower(py), `\windowsapps\`) {
		return errors.New(tr.T("it's the Microsoft Store shortcut, not an installed Python"))
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := proc.Command(ctx, py, "-c",
		"import sys, venv, ensurepip; print('%d.%d' % sys.version_info[:2])")
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		switch {
		case strings.Contains(msg, "ensurepip") || strings.Contains(msg, "venv"):
			return errors.New(tr.T("it lacks venv (on Debian and Ubuntu, install python3-venv)"))
		case msg != "":
			lines := strings.Split(msg, "\n")
			return errors.New(strings.TrimSpace(lines[len(lines)-1]))
		}
		return err
	}
	version := strings.TrimSpace(out.String())
	major, minor, _ := strings.Cut(version, ".")
	m, _ := strconv.Atoi(minor)
	if major != "3" || m < minPython || m > maxPython {
		return errors.New(tr.T("it's Python {version}; engines need 3.{min} to 3.{max}", "version", version, "min", minPython, "max", maxPython))
	}
	return nil
}
