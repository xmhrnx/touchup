package pages

import (
	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/components"
	"github.com/mehran9675/touchup/internal/ui/format"
	"github.com/mehran9675/touchup/internal/ui/store"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// Studio is where a file is opened, previewed and sent to the queue.
type Studio struct {
	s         *store.Store
	inspector *Inspector
	video     *VideoStage
	audio     *AudioStage
	image     *ImageStage
	browse    *giode.ButtonElement
	open      *giode.ButtonElement
	close     *giode.ButtonElement
	retry     *giode.ButtonElement
	copyErr   giode.Element
	// dragging reports whether files are being dragged over the window.
	dragging func() bool
}

// NewStudio creates the studio page.
func NewStudio(s *store.Store, dragging func() bool, errs *components.ErrorDetails, options *components.EngineSettings) *Studio {
	st := &Studio{
		s:         s,
		inspector: NewInspector(s, options),
		video:     NewVideoStage(s, errs),
		audio:     NewAudioStage(s, errs),
		image:     NewImageStage(s),
		browse:    theme.PrimaryButton(tr.T("Choose a file"), "folder-open").OnClick(s.OpenDialog),
		open:      theme.SecondaryButton(tr.T("Open"), "folder-open").OnClick(s.OpenDialog),
		// Closing waits for the frame to end: the inspector and stage are
		// laid out after this button and still need the session.
		close:    theme.GhostButton(tr.T("Close"), "x").OnClick(func() { s.App.Post(s.Close) }),
		retry:    theme.SecondaryButton(tr.T("Choose another file"), "folder-open").OnClick(s.OpenDialog),
		dragging: dragging,
	}
	st.copyErr = theme.CopyButton(tr.T("Copy"), func() string {
		if sess := s.Session; sess != nil {
			return s.ErrorReport(tr.T("{file} can't be opened", "file", sess.Name), sess.Err, "")
		}
		return ""
	})
	return st
}

// View lays out the page.
func (st *Studio) View() giode.Element {
	sess := st.s.Session
	if sess == nil {
		return giode.Box(giode.Styles{Width: -1, Height: -1, Align: giode.AlignCenter, Justify: giode.JustifyCenter, Padding: giode.UniformInset(32)},
			components.DropZone(st.dragging(), st.browse))
	}

	var stage giode.Element
	switch {
	case sess.Loading:
		stage = components.LoadingState(tr.T("Reading {file}…", "file", sess.Name))
	case sess.Err != "":
		stage = components.EmptyState("triangle-alert", tr.T("This file can't be opened"), sess.Err, st.retry, st.copyErr)
	case sess.Kind == media.Video:
		stage = st.video.View()
	case sess.Kind == media.Audio:
		stage = st.audio.View()
	default:
		stage = st.image.View()
	}

	body := []giode.Element{giode.Box(giode.Styles{FlexGrow: 1, Height: -1}, stage)}
	if !sess.Loading && sess.Err == "" {
		body = append(body, st.inspector.View())
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Gap: 20, Padding: giode.Inset{Top: 24, Right: 28, Bottom: 24, Left: 28}},
		st.header(),
		giode.Row(giode.Styles{Width: -1, FlexGrow: 1, Gap: 20, Wrap: giode.WrapNoWrap}, body...),
	)
}

// header shows the file's name and facts.
func (st *Studio) header() giode.Element {
	sess := st.s.Session
	icon, color := components.MediaIcon(sess.Kind)
	info := sess.Info
	chips := []giode.Element{}
	if !sess.Loading && sess.Err == "" {
		switch sess.Kind {
		case media.Image:
			chips = append(chips, components.MetaChip("scan", format.Resolution(info.Width, info.Height)))
		case media.Video:
			chips = append(chips,
				components.MetaChip("scan", format.Resolution(info.Width, info.Height)),
				components.MetaChip("film", format.FPS(info.FPS)),
				components.MetaChip("timer", format.Duration(info.Duration)),
			)
			if info.HasAudio {
				chips = append(chips, components.MetaChip("volume-2", format.Rate(info.SampleRate)))
			}
		case media.Audio:
			chips = append(chips,
				components.MetaChip("audio-waveform", format.Rate(info.SampleRate)),
				components.MetaChip("volume-2", format.Channels(info.Channels)),
				components.MetaChip("timer", format.Duration(info.Duration)),
			)
		}
		if info.Size > 0 {
			chips = append(chips, components.MetaChip("hard-drive", format.Bytes(info.Size)))
		}
	}
	return giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 14, Wrap: giode.WrapNoWrap},
		components.IconBubble(icon, color, 44),
		giode.Stack(giode.Styles{Gap: 6, FlexGrow: 1},
			giode.Text(sess.Name, theme.Heading().With(giode.Styles{FontSize: 18, TextOverflow: giode.TextOverflowEllipsis})),
			giode.Row(giode.Styles{Gap: 6}, chips...),
		),
		st.open, st.close,
	)
}

// runningPreviewReport is what the session's running preview has
// printed so far, to copy when it seems stuck.
func runningPreviewReport(s *store.Store) string {
	sess := s.Session
	if sess == nil {
		return ""
	}
	p := &sess.Preview
	return s.RunningReport(tr.T("The preview of {file}", "file", sess.Name), p.Update, p.Started, &p.Log)
}

// previewReport is the full report of the session's failed preview.
func previewReport(s *store.Store) string {
	sess := s.Session
	if sess == nil {
		return ""
	}
	p := sess.Preview
	return s.ErrorReport(tr.T("The preview of {file} failed", "file", sess.Name), p.Err, p.ErrLog)
}
