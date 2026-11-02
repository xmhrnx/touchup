package components

import (
	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/tr"
)

// OptionLabel is how an option's value reads in the user's language: a
// choice's label, "On" or "Off", or the number.
func OptionLabel(o *engine.Option, v string) string {
	switch o.Type {
	case engine.OptionToggle:
		if v == "true" {
			return tr.T("On")
		}
		return tr.T("Off")
	case engine.OptionChoiceType:
		return tr.T(o.Label(v))
	}
	return v
}
