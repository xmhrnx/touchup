package components

import "github.com/mehran9675/touchup/internal/tr"

// MediaName names a kind of media, "image", "video" or "audio", for a
// tag or a label.
func MediaName(kind string) string {
	switch kind {
	case "image":
		return tr.T("Image")
	case "video":
		return tr.T("Video")
	case "audio":
		return tr.T("Audio")
	}
	return kind
}
