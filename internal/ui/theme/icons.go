package theme

import (
	"github.com/mehran9675/touchup/assets"
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/styles/properties"
)

// LoadIcons registers the embedded Lucide icons by file name.
func LoadIcons() error {
	return giode.RegisterIconsFS(assets.Icons, "icons")
}

// Icon renders a registered icon at size dp in color.
func Icon(name string, size int, color string) giode.Element {
	return giode.Icon(name, giode.Styles{Width: properties.Width(size), Height: properties.Height(size), Color: color})
}
