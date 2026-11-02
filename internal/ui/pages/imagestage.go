package pages

import (
	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/components"
	"github.com/mehran9675/touchup/internal/ui/store"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// ImageStage shows the image as it is. Images go straight to the queue:
// upscaling one is as quick as previewing it.
type ImageStage struct {
	s *store.Store
}

// NewImageStage creates the stage.
func NewImageStage(s *store.Store) *ImageStage { return &ImageStage{s: s} }

// View lays out the stage.
func (i *ImageStage) View() giode.Element {
	sess := i.s.Session
	return giode.Box(giode.Styles{Width: -1, Height: -1, BorderRadius: 14, Overflow: giode.OverflowHidden, Background: "#050507"},
		components.Picture(sess.Picture),
		giode.Box(giode.Styles{Position: giode.PositionAbsolute, Left: 14, Bottom: 14},
			giode.Row(giode.Styles{
				Align: giode.AlignCenter, Gap: 8, Wrap: giode.WrapNoWrap,
				Background: "#0B0B10CC", BorderRadius: 999, Padding: giode.SymmetricInset(12, 7),
			},
				theme.Icon("image", 14, "#D4D4DC"),
				giode.Text(tr.T("Original"), giode.Styles{FontFamily: theme.Sans, FontSize: 12, Color: "#D4D4DC"}),
			)),
	)
}
