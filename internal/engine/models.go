package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mehran9675/touchup/internal/tr"
)

// ModelsDir is where an engine's downloaded weights live.
func (m *Manifest) ModelsDir() string { return filepath.Join(m.Dir(), "models") }

// ModelPath is the path of the model's first file, or "" for a model
// that comes with the engine.
func (m *Manifest) ModelPath(mdl *Model) string {
	if mdl == nil || len(mdl.Files) == 0 {
		return ""
	}
	return filepath.Join(m.ModelsDir(), filepath.FromSlash(mdl.Files[0].Name))
}

// ModelInstalled reports whether a model's files are all downloaded.
// Models without files come with the engine.
func (m *Manifest) ModelInstalled(mdl *Model) bool {
	for _, f := range mdl.Files {
		if !exists(m.modelFile(f)) {
			return false
		}
	}
	return true
}

func (m *Manifest) modelFile(f ModelFile) string {
	return filepath.Join(m.ModelsDir(), filepath.FromSlash(f.Name))
}

// InstallModel downloads a model's missing files, checking their SHA-256
// sums when the manifest has them. Files shared with another model are
// downloaded once.
func InstallModel(ctx context.Context, m *Manifest, mdl *Model, report Report) error {
	var missing []ModelFile
	for _, f := range mdl.Files {
		if !exists(m.modelFile(f)) {
			missing = append(missing, f)
		}
	}
	for i, f := range missing {
		target := m.modelFile(f)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		part := target + ".part"
		stage := "Downloading"
		if len(missing) > 1 {
			stage = fmt.Sprintf("Downloading %d of %d", i+1, len(missing))
		}
		err := download(ctx, f.URL, part, func(frac float64) {
			if frac >= 0 {
				frac = (float64(i) + frac) / float64(len(missing))
			}
			report(Status{Stage: stage, Frac: frac})
		})
		if err == nil && f.SHA256 != "" {
			report(Status{Stage: tr.T("Verifying"), Frac: -1})
			err = verify(part, f.SHA256)
		}
		if err != nil {
			os.Remove(part)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("%s: %w", filepath.Base(f.Name), err)
		}
		if err := os.Rename(part, target); err != nil {
			return err
		}
	}
	return nil
}

// UninstallModel deletes a model's files, keeping those another
// installed model still uses.
func UninstallModel(m *Manifest, mdl *Model) error {
	shared := map[string]bool{}
	for i := range m.Models {
		other := &m.Models[i]
		if other.ID == mdl.ID || !m.ModelInstalled(other) {
			continue
		}
		for _, f := range other.Files {
			shared[f.Name] = true
		}
	}
	for _, f := range mdl.Files {
		if shared[f.Name] {
			continue
		}
		if err := os.Remove(m.modelFile(f)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// verify checks a file's SHA-256 sum.
func verify(file, want string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return errors.New(tr.T("the download is corrupt (checksum {got}…, expected {want}…)", "got", got[:12], "want", want[:12]))
	}
	return nil
}
