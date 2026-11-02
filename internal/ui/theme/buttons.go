package theme

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/iconbutton"
	"github.com/mehran9675/giode/styles/properties"

	"github.com/mehran9675/touchup/internal/tr"
)

// PrimaryButton is a filled call to action in the brand gradient.
func PrimaryButton(label, icon string) *giode.ButtonElement {
	return withIcon(giode.Button(label, PrimaryStyle()), icon, 16)
}

// PrimaryStyle is the style of PrimaryButton.
func PrimaryStyle() giode.Styles {
	return giode.Styles{
		BackgroundGradient: BrandGradient(),
		Color:              "#FFFFFF",
		FontFamily:         Sans,
		FontSize:           13.5,
		FontWeight:         giode.FontWeightSemiBold,
		BorderRadius:       10,
		Padding:            giode.SymmetricInset(18, 11),
		BoxShadow:          giode.BoxShadow{Y: 6, Blur: 18, Color: "#7C5CFF55"},
	}
}

// SecondaryButton is an outlined button on a surface.
func SecondaryButton(label, icon string) *giode.ButtonElement {
	return withIcon(giode.Button(label, SecondaryStyle()), icon, 15)
}

// SecondaryStyle is the style of SecondaryButton.
func SecondaryStyle() giode.Styles {
	return giode.Styles{
		Background:   Raised(),
		Color:        Text,
		BorderColor:  Border,
		BorderWidth:  1,
		FontFamily:   Sans,
		FontSize:     13,
		FontWeight:   giode.FontWeightMedium,
		BorderRadius: 10,
		Padding:      giode.SymmetricInset(14, 9),
	}
}

// withIcon puts a size dp icon before b's label.
func withIcon(b *giode.ButtonElement, icon string, size int) *giode.ButtonElement {
	if icon != "" {
		b.Icon(giode.Icon(icon, giode.Styles{Width: properties.Width(size), Height: properties.Height(size)}))
	}
	return b
}

// GhostButton is a quiet text button.
func GhostButton(label, icon string) *giode.ButtonElement {
	b := giode.Button(label, giode.Styles{
		Background:   Text,
		Color:        Muted,
		FontFamily:   Sans,
		FontSize:     13,
		FontWeight:   giode.FontWeightMedium,
		BorderRadius: 8,
		Padding:      giode.SymmetricInset(10, 7),
	}).Variant(giode.VariantGhost)
	if icon != "" {
		b.Icon(giode.Icon(icon, giode.Styles{Width: 15, Height: 15}))
	}
	return b
}

// DangerButton is an outlined destructive action.
func DangerButton(label, icon string) *giode.ButtonElement {
	b := giode.Button(label, giode.Styles{
		BorderColor:  Danger,
		Color:        Danger,
		FontFamily:   Sans,
		FontSize:     13,
		FontWeight:   giode.FontWeightMedium,
		BorderRadius: 10,
		Padding:      giode.SymmetricInset(14, 9),
	}).Variant(giode.VariantOutlined)
	if icon != "" {
		b.Icon(giode.Icon(icon, giode.Styles{Width: 15, Height: 15}))
	}
	return b
}

// IconButton is a small square ghost button around an icon.
func IconButton(icon, label string) *iconbutton.Button {
	return giode.IconButton(giode.Icon(icon, giode.Styles{Width: 16, Height: 16, Color: Muted}),
		giode.Styles{Background: Text, BorderRadius: 8, Padding: giode.UniformInset(7)}).
		Variant(giode.IconButtonGhost).Label(label)
}

// CopyButton is a quiet button that copies text() to the clipboard and
// confirms with a check mark. An empty label shows the icon alone.
func CopyButton(label string, text func() string) giode.Element {
	b := giode.CopyButton(text, giode.Styles{
		Color:        Muted,
		FontSize:     13,
		BorderRadius: 8,
		Padding:      giode.SymmetricInset(10, 7),
	})
	if label == "" {
		b.Labels("", "")
	} else {
		b.Labels(label, tr.T("Copied"))
	}
	return b
}
