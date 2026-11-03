package engine

import (
	"runtime"
	"slices"
	"testing"
)

func TestCatalogIsValid(t *testing.T) {
	t.Setenv("TOUCHUP_HOME", t.TempDir())
	reg, errs := Load()
	for _, err := range errs {
		t.Error(err)
	}
	all := reg.All()
	if len(all) == 0 {
		t.Fatal("the catalog is empty")
	}
	for _, m := range all {
		if m.Custom {
			continue
		}
		if err := m.Validate(); err != nil {
			t.Errorf("%s: %v", m.ID, err)
		}
		// RTX Video goes through Direct3D, so it's Windows-only.
		if !m.Supported() && !(m.ID == "rtxvideo" && runtime.GOOS != "windows") {
			t.Errorf("%s has no build for this platform", m.ID)
		}
	}
}

func TestCommand(t *testing.T) {
	m := &Manifest{
		ID: "x", Args: []string{"-i", "{input}", "-o", "{output}", "-s", "{scale}", "{modelArgs}", "--dir={outputDir}"},
		GPUArgs: []string{"-g", "{gpu}"},
	}
	model := &Model{ID: "m", Args: []string{"-n", "{model}-x{scale}"}}
	j := Job{Model: model, Scale: 4, GPU: -1, Input: "in.png", Output: "out.png", OutputDir: "/out"}
	want := []string{"-i", "in.png", "-o", "out.png", "-s", "4", "-n", "m-x4", "--dir=/out"}
	if got := m.Command(j); !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	j.GPU = 1
	if got := m.Command(j); !slices.Equal(got[len(got)-2:], []string{"-g", "1"}) {
		t.Fatalf("GPU args missing: %q", got)
	}
}

func TestValidateRejectsIncomplete(t *testing.T) {
	m := &Manifest{ID: "A", Media: []string{"image"}, Input: "audio", Install: InstallSpec{Type: "pip"}}
	if err := m.Validate(); err == nil {
		t.Fatal("an incomplete manifest passed validation")
	}
}

func TestPreferredScale(t *testing.T) {
	m := &Manifest{Scales: []int{2, 3, 4}, DefaultScale: 3}
	if got := m.PreferredScale(nil); got != 3 {
		t.Errorf("got %d, want the default 3", got)
	}
	if got := m.PreferredScale(&Model{Scales: []int{2}}); got != 2 {
		t.Errorf("got %d, want the model's only scale 2", got)
	}
	if got := (&Manifest{}).PreferredScale(nil); got != 1 {
		t.Errorf("got %d, want 1 for engines without scales", got)
	}
}
