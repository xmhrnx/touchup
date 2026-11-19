package components

import (
	"strings"
	"unicode"

	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// outputLine shows the latest line an engine printed, or nothing. Leading
// symbols are dropped: engines decorate lines with emoji the UI font
// doesn't have.
func outputLine(line string) giode.Element {
	line = strings.TrimLeftFunc(line, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '[' && r != '('
	})
	if line == "" {
		return giode.Box(giode.Styles{})
	}
	return giode.Text(line, theme.Code().With(giode.Styles{MaxLines: 1, TextOverflow: giode.TextOverflowEllipsis}))
}
