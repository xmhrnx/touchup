package tr

import (
	"testing"

	"github.com/mehran9675/giode/i18n"

	"github.com/mehran9675/touchup/assets"
)

func TestTranslate(t *testing.T) {
	i18n.Add("en", map[string]any{"{count} files": map[string]string{"one": "{count} file", "other": "{count} files"}})
	i18n.Add("de", map[string]any{
		"Saved to {folder}": "Gespeichert in {folder}",
		"{count} files":     map[string]string{"one": "{count} Datei", "other": "{count} Dateien"},
	})
	i18n.Add("pl", map[string]any{"{count} files": map[string]string{"one": "{count} plik", "few": "{count} pliki", "many": "{count} plików", "other": "{count} plików"}})
	defer Use("en")

	Use("en")
	if got := T("Saved to {folder}", "folder", "out"); got != "Saved to out" {
		t.Errorf("English without a file: %q", got)
	}
	if got := N("{count} files", 1); got != "1 file" {
		t.Errorf("English plural: %q", got)
	}
	Use("de")
	if got := T("Saved to {folder}", "folder", "out"); got != "Gespeichert in out" {
		t.Errorf("German: %q", got)
	}
	if got := T("Not translated"); got != "Not translated" {
		t.Errorf("a missing message: %q", got)
	}
	if got := Decimal(2.45, 1) + " " + Int(35787); got != "2,5 35.787" && got != "2,4 35.787" {
		t.Errorf("German numbers: %q", got)
	}
	Use("pl")
	for n, want := range map[int]string{1: "1 plik", 3: "3 pliki", 5: "5 plików", 22: "22 pliki"} {
		if got := N("{count} files", n); got != want {
			t.Errorf("Polish %d: %q, want %q", n, got, want)
		}
	}
	if got := Int(35787); got != "35 787" {
		t.Errorf("Polish thousands: %q", got)
	}
}

func TestResolve(t *testing.T) {
	for in, want := range map[string]string{"de": "de", "no": "nb", "nn-NO": "nb", "nb": "nb"} {
		if got := Resolve(in); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every translation TouchUp ships must load; the app would start in
// English otherwise. go run ./tools/i18n checks what's in them.
func TestLocalesLoad(t *testing.T) {
	if err := i18n.Load(assets.Locales, "locales"); err != nil {
		t.Fatal(err)
	}
	for _, l := range Languages {
		if l.Code == "en" {
			continue
		}
		if _, err := assets.Locales.Open("locales/" + l.Code + ".json"); err != nil {
			t.Errorf("%s (%s) has no translation: %v", l.Name, l.Code, err)
		}
	}
}
