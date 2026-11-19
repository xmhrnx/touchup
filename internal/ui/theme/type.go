package theme

import "github.com/xmhrnx/giode"

// Text styles. Each returns fresh Styles, so callers can set more fields.

// Display is the large page title.
func Display() giode.Styles {
	return giode.Styles{FontFamily: Sans, FontSize: 26, FontWeight: giode.FontWeightBold, Color: Text, LineHeight: 32}
}

// Heading titles a card or a section.
func Heading() giode.Styles {
	return giode.Styles{FontFamily: Sans, FontSize: 16, FontWeight: giode.FontWeightSemiBold, Color: Text}
}

// Body is running text.
func Body() giode.Styles {
	return giode.Styles{FontFamily: Sans, FontSize: 13.5, Color: Text, LineHeight: 20}
}

// Secondary is supporting text.
func Secondary() giode.Styles {
	return giode.Styles{FontFamily: Sans, FontSize: 13, Color: Muted, LineHeight: 19}
}

// Caption is small print.
func Caption() giode.Styles {
	return giode.Styles{FontFamily: Sans, FontSize: 11.5, Color: Muted, LineHeight: 16}
}

// Overline labels a group: small capitals, tracked out by weight.
func Overline() giode.Styles {
	return giode.Styles{FontFamily: Sans, FontSize: 11, FontWeight: giode.FontWeightSemiBold, Color: Muted,
		TextTransform: giode.TextTransformUppercase}
}

// Label is the text of controls.
func Label() giode.Styles {
	return giode.Styles{FontFamily: Sans, FontSize: 13, FontWeight: giode.FontWeightMedium, Color: Text}
}

// Code is monospaced text: paths, commands.
func Code() giode.Styles {
	return giode.Styles{FontFamily: Mono, FontSize: 12, Color: Muted, LineHeight: 18}
}

// T is shorthand for a Text element.
func T(s string, st giode.Styles) giode.Element { return giode.Text(s, st) }
