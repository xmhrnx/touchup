package shell

import (
	"os"
	"runtime"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/titlebar"

	"github.com/mehran9675/touchup/internal/ui/components"
	"github.com/mehran9675/touchup/internal/ui/store"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// CustomFrame reports whether TouchUp draws its own title bar. It does on
// Windows, where a frameless window keeps its resize borders and snapping.
// Elsewhere Gio can't resize a frameless window from its edges, so the
// system frame stays unless TOUCHUP_CUSTOM_FRAME=1 asks for it.
func CustomFrame() bool {
	switch os.Getenv("TOUCHUP_CUSTOM_FRAME") {
	case "1", "true":
		return true
	case "0", "false":
		return false
	}
	return runtime.GOOS == "windows"
}

// newTitleBar builds the title bar of a frameless window.
func newTitleBar(s *store.Store) *titlebar.TitleBar {
	bar := giode.TitleBar(s.App, "TouchUp", giode.Styles{
		Background: theme.Surface, Color: theme.Text, Height: 38,
		FontSize: 12.5, FontWeight: giode.FontWeightMedium,
		BorderColor: theme.Faint(), BorderWidth: 1,
	}).Icon(giode.Box(giode.Styles{Padding: giode.Inset{Left: 4}}, components.AppIcon(16)))
	if runtime.GOOS == "darwin" {
		bar.Variant(giode.TitleBarMac)
	}
	return bar
}

// title names the window after the open file.
func title(s *store.Store) string {
	if s.Session != nil {
		return s.Session.Name + "  —  TouchUp"
	}
	return "TouchUp"
}
