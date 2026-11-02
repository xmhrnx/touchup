package components

import (
	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// DropZone invites the user to drop or browse for a file. active
// highlights it while files are dragged over the window.
func DropZone(active bool, browse giode.Element) giode.Element {
	border, glow := theme.Faint(), giode.Color("#00000000")
	if active {
		border, glow = theme.Primary, "#7C5CFF40"
	}
	kind := func(icon, title, formats string, color giode.Color) giode.Element {
		return giode.Row(giode.Styles{
			Align: giode.AlignCenter, Gap: 12, Wrap: giode.WrapNoWrap,
			Background: theme.Raised(), BorderRadius: 12, Padding: giode.SymmetricInset(14, 12), Width: 210,
		},
			IconBubble(icon, color, 34),
			giode.Stack(giode.Styles{Gap: 2},
				giode.Text(title, theme.Label()),
				giode.Text(formats, theme.Caption()),
			),
		)
	}
	return giode.Stack(giode.Styles{
		Width: -1, MaxWidth: 760, Align: giode.AlignCenter, Gap: 18,
		Background: theme.Surface, BorderColor: border, BorderWidth: 2, BorderRadius: 24,
		Padding:   giode.SymmetricInset(40, 48),
		BoxShadow: giode.BoxShadow{Y: 18, Blur: 60, Spread: 4, Color: glow},
	},
		giode.Box(giode.Styles{
			Width: 76, Height: 76, BorderRadius: 22, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
			BackgroundGradient: theme.BrandGradient(),
			BoxShadow:          giode.BoxShadow{Y: 10, Blur: 30, Color: "#7C5CFF66"},
		}, theme.Icon("cloud-upload", 34, "#FFFFFF")),
		giode.Text(tr.T("Drop a photo, video or song"), theme.Display().With(giode.Styles{FontSize: 24})),
		giode.Box(giode.Styles{MaxWidth: 520},
			giode.Text(tr.T("Everything runs on this computer with the engines you install. Nothing is uploaded."),
				theme.Secondary().With(giode.Styles{TextAlign: giode.TextAlignCenter}))),
		browse,
		giode.Row(giode.Styles{Gap: 12, Justify: giode.JustifyCenter, Margin: giode.Inset{Top: 10}},
			kind("image", tr.T("Images"), "PNG · JPEG · WebP", theme.Info),
			kind("film", tr.T("Video"), "MP4 · MOV · MKV", theme.Accent),
			kind("audio-lines", tr.T("Audio"), "WAV · FLAC · MP3", theme.Success),
		),
	)
}
