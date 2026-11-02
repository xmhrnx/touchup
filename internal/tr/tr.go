// Package tr translates TouchUp's text. Messages are their English
// text: a message without a translation in the current language, or one
// from a custom engine's manifest, shows as written. Translations live
// in locales/<language>.json, keyed by the English text; run
// go run ./tools/i18n to list the messages and check the translations.
package tr

import "github.com/mehran9675/giode/i18n"

// T translates msg, filling its {placeholders} from args: name, value
// pairs or a single map.
//
//	tr.T("Saved to {folder}", "folder", dir)
func T(msg string, args ...any) string { return i18n.Intl(msg, args...) }

// N translates a message with plural forms for count, keyed by its
// English plural ("other") form; {count} is count.
//
//	tr.N("{count} engines installed", n)
func N(msg string, count int, args ...any) string {
	return i18n.Intl(msg, append(args, "count", count)...)
}
