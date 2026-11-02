package components

import (
	"bytes"
	"image"
	"image/png"
	"sync"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/styles/properties"

	"github.com/mehran9675/touchup/assets"
	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

var appIcon = sync.OnceValue(func() image.Image {
	img, _ := png.Decode(bytes.NewReader(assets.AppIcon))
	return img
})

// AppIcon shows TouchUp's icon at size dp.
func AppIcon(size int) giode.Element {
	return giode.Image(appIcon(), giode.Styles{Width: properties.Width(size), Height: properties.Height(size)})
}

// BrandMark is the app icon with the TouchUp wordmark.
func BrandMark() giode.Element {
	return giode.Row(giode.Styles{Align: giode.AlignCenter, Gap: 10, Wrap: giode.WrapNoWrap},
		AppIcon(30),
		giode.Stack(giode.Styles{Gap: 0},
			giode.Text("TouchUp", giode.Styles{FontFamily: theme.Sans, FontSize: 16.5, FontWeight: giode.FontWeightBold, Color: theme.Text}),
			giode.Text(tr.T("Local AI upscaling"), theme.Caption().With(giode.Styles{FontSize: 11})),
		),
	)
}
