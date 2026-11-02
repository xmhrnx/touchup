package tr

import (
	"strings"

	"github.com/mehran9675/giode/i18n"
)

// Language is a language TouchUp is translated into.
type Language struct {
	Code string // as the locale files are named: "de", "pt"
	Name string // in the language itself: "Deutsch"
}

// Languages are TouchUp's languages, by their own names' order.
var Languages = []Language{
	{"bg", "Български"},
	{"cs", "Čeština"},
	{"da", "Dansk"},
	{"de", "Deutsch"},
	{"et", "Eesti"},
	{"el", "Ελληνικά"},
	{"en", "English"},
	{"es", "Español"},
	{"fr", "Français"},
	{"ga", "Gaeilge"},
	{"hr", "Hrvatski"},
	{"it", "Italiano"},
	{"lv", "Latviešu"},
	{"lt", "Lietuvių"},
	{"hu", "Magyar"},
	{"mt", "Malti"},
	{"nl", "Nederlands"},
	{"nb", "Norsk bokmål"},
	{"pl", "Polski"},
	{"pt", "Português"},
	{"ro", "Română"},
	{"sk", "Slovenčina"},
	{"sl", "Slovenščina"},
	{"fi", "Suomi"},
	{"sv", "Svenska"},
	{"uk", "Українська"},
}

// Resolve returns the locale to use for a language setting: code, or
// the system's locale for "".
func Resolve(code string) string {
	if code == "" {
		code = i18n.SystemLocale()
	}
	// Norwegian systems may say "no" or "nn"; the translation is Bokmål.
	if l := strings.SplitN(code, "-", 2)[0]; l == "no" || l == "nn" {
		code = "nb"
	}
	return code
}

// Use switches the UI to code's language, or to the system's for "".
func Use(code string) { i18n.SetLocale(Resolve(code)) }
