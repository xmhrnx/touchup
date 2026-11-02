package tr

import (
	"strconv"
	"strings"

	"github.com/mehran9675/giode/i18n"
)

// separators are the decimal and thousands separators of languages
// that don't write numbers as English does.
var separators = map[string][2]string{
	// a decimal comma and a point between thousands
	"de": {",", "."}, "nl": {",", "."}, "it": {",", "."}, "es": {",", "."}, "da": {",", "."},
	"el": {",", "."}, "ro": {",", "."}, "sl": {",", "."}, "hr": {",", "."},
	// a decimal comma and a no-break space between thousands
	"fr": {",", " "}, "cs": {",", " "}, "sk": {",", " "}, "pl": {",", " "},
	"fi": {",", " "}, "sv": {",", " "}, "nb": {",", " "}, "no": {",", " "},
	"et": {",", " "}, "lv": {",", " "}, "lt": {",", " "}, "bg": {",", " "},
	"uk": {",", " "}, "hu": {",", " "}, "pt": {",", " "},
}

func seps() (decimal, group string) {
	l := i18n.Locale()
	if i := strings.IndexByte(l, '-'); i >= 0 {
		l = l[:i]
	}
	if s, ok := separators[l]; ok {
		return s[0], s[1]
	}
	return ".", ","
}

// Decimal writes f with digits decimals the way the language does:
// "2.4" in English, "2,4" in German.
func Decimal(f float64, digits int) string {
	s := strconv.FormatFloat(f, 'f', digits, 64)
	if d, _ := seps(); d != "." {
		s = strings.Replace(s, ".", d, 1)
	}
	return s
}

// Int writes n with the language's thousands separator: "35,787" in
// English, "35.787" in German.
func Int(n int) string {
	s := strconv.Itoa(n)
	_, g := seps()
	for i := len(s) - 3; i > 0 && s[i-1] != '-'; i -= 3 {
		s = s[:i] + g + s[i:]
	}
	return s
}
