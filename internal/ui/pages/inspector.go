// Package pages holds TouchUp's screens and the larger panels they're
// made of. Each is created once and laid out every frame.
package pages

import (
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/scroll"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/pipeline"
	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/components"
	"github.com/mehran9675/touchup/internal/ui/format"
	"github.com/mehran9675/touchup/internal/ui/store"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// Inspector is the studio's right-hand panel: engine, model and scale,
// what the output will be, and the preview and upscale actions.
type Inspector struct {
	s       *store.Store
	scroll  *scroll.Scroll
	preview *giode.ButtonElement
	run     *giode.ButtonElement
	browse  *giode.ButtonElement
	models  *giode.ButtonElement
	options *components.EngineSettings
	adjust  *giode.ButtonElement
	tune    *giode.ButtonElement
}

// NewInspector creates the inspector.
func NewInspector(s *store.Store, options *components.EngineSettings) *Inspector {
	in := &Inspector{s: s, options: options}
	in.tune = theme.SecondaryButton(tr.T("Find the fastest"), "gauge").OnClick(s.RunTune)
	in.adjust = theme.SecondaryButton(tr.T("Adjust"), "sliders-horizontal").OnClick(func() {
		if sess := s.Session; sess != nil {
			if sel := s.Selected(sess.Kind); sel.Engine != nil {
				options.Show(sel.Engine)
			}
		}
	})
	wide := giode.Styles{Width: -1, Padding: giode.SymmetricInset(16, 12)}
	in.preview = giode.Button(tr.T("Preview"), theme.SecondaryStyle().With(wide)).OnClick(s.RunPreview).
		Icon(giode.Icon("scan-eye", giode.Styles{Width: 16, Height: 16}))
	in.run = giode.Button(tr.T("Upscale"), theme.PrimaryStyle().With(wide)).OnClick(s.Enqueue).
		Icon(giode.Icon("sparkles", giode.Styles{Width: 16, Height: 16}))
	in.browse = theme.SecondaryButton(tr.T("Browse engines"), "package").OnClick(func() { s.Navigate(store.PageEngines) })
	in.models = theme.SecondaryButton(tr.T("Get models"), "download").OnClick(func() { s.Navigate(store.PageEngines) })
	in.scroll = giode.Scroll(in.content, giode.Styles{ScrollBar: giode.ScrollBar{Width: 4, ThumbColor: theme.Border}})
	return in
}

// View lays out the inspector.
func (in *Inspector) View() giode.Element {
	sess := in.s.Session
	sel := in.s.Selected(sess.Kind)
	canRun := sel.Ready() && !sess.Loading && sess.Err == ""

	verb := map[media.Kind]string{media.Image: tr.T("Upscale image"), media.Video: tr.T("Upscale video"), media.Audio: tr.T("Enhance audio")}[sess.Kind]
	in.run.Label(verb).Disabled(!canRun)
	actions := []giode.Element{}
	switch sess.Kind {
	case media.Video, media.Audio:
		label := tr.T("Preview frame")
		if sess.Kind == media.Audio {
			label = tr.T("Preview section")
		}
		in.preview.Label(label).Disabled(!canRun).Loading(sess.Preview.Running)
		actions = append(actions, in.preview)
	}
	actions = append(actions, in.run)

	return giode.Stack(giode.Styles{
		Width: 340, Height: -1,
		Background: theme.Surface, BorderColor: theme.Faint(), BorderWidth: 1, BorderRadius: 18,
	},
		giode.Box(giode.Styles{FlexGrow: 1, Width: -1}, in.scroll),
		giode.Divider(giode.Styles{Color: theme.Faint()}),
		giode.Stack(giode.Styles{Width: -1, Gap: 10, Padding: giode.UniformInset(18)},
			giode.Stack(giode.Styles{Width: -1, Gap: 8}, actions...),
			giode.Text(in.footnote(), theme.Caption().With(giode.Styles{TextAlign: giode.TextAlignCenter})),
		),
	)
}

func (in *Inspector) footnote() string {
	sess := in.s.Session
	switch sess.Kind {
	case media.Video:
		return tr.T("Preview upscales only the frame under the playhead.")
	case media.Audio:
		return tr.T("Preview enhances only the highlighted section.")
	}
	return tr.T("Images go straight to the queue: no preview needed.")
}

func (in *Inspector) content() giode.Element {
	s := in.s
	sess := s.Session
	kind := sess.Kind
	sel := s.Selected(kind)
	engines := s.EnginesFor(kind)

	sections := []giode.Element{}

	// Engines.
	rows := []giode.Element{components.SectionLabel(tr.T("Engine"))}
	installedAny := false
	for _, m := range engines {
		installed := s.Installed(m.ID)
		installedAny = installedAny || installed
		selected := sel.Engine != nil && sel.Engine.ID == m.ID
		note := ""
		if installed && len(m.Models) > 0 && s.InstalledModels(m) == 0 {
			note = tr.T("No model installed yet")
		}
		rows = append(rows, components.EngineOption(m, selected, installed, note, func() {
			if installed {
				s.Choose(kind, m.ID, "", 0)
			} else {
				s.Navigate(store.PageEngines)
			}
		}))
	}
	if !installedAny {
		rows = append(rows, components.Banner("info", theme.Info, tr.T("No engine installed yet"), noEngineHint(kind)), in.browse)
	}
	sections = append(sections, giode.Stack(giode.Styles{Gap: 8, Width: -1}, rows...))

	if sel.Engine != nil && !sel.Ready() {
		sections = append(sections, giode.Stack(giode.Styles{Gap: 8, Width: -1},
			components.Banner("download", theme.Info, tr.T("Choose a model to download"),
				tr.T("{engine} is installed, but its models are separate downloads. Pick one on the Engines page.", "engine", sel.Engine.Name)),
			in.models))
	}
	if sel.Ready() && sess.Kind == media.Video && slices.Contains(sel.Engine.Tags, "Diffusion") && sess.Info.Frames > longVideoFrames {
		sections = append(sections, components.Banner("timer", theme.Warning, tr.T("This will take a long time"),
			tr.T("{engine} takes a few seconds per frame, and this video has {frames} frames: {time} on a mid-range GPU. Use Find the fastest below to tune it for this GPU first.",
				"engine", sel.Engine.Name, "frames", format.Count(sess.Info.Frames), "time", diffusionTime(sess.Info.Frames))))
	}
	if sel.Ready() {
		// Models: the installed ones.
		var available []*engine.Model
		for i := range sel.Engine.Models {
			if mdl := &sel.Engine.Models[i]; s.ModelInstalled(sel.Engine, mdl) {
				available = append(available, mdl)
			}
		}
		if len(available) > 1 {
			pills := []giode.Element{}
			for _, mdl := range available {
				pills = append(pills, components.Pill("model:"+sel.Engine.ID, mdl.ID, tr.T(mdl.Name),
					sel.Model != nil && sel.Model.ID == mdl.ID, func() { s.Choose(kind, "", mdl.ID, 0) }))
			}
			group := []giode.Element{components.SectionLabel(tr.T("Model")), giode.Row(giode.Styles{Gap: 8}, pills...)}
			if sel.Model != nil && sel.Model.Description != "" {
				group = append(group, giode.Text(tr.T(sel.Model.Description), theme.Caption()))
			}
			sections = append(sections, giode.Stack(giode.Styles{Gap: 10, Width: -1}, group...))
		}
		// Size: a scale, or for pictures a resolution to reach.
		if scales := sel.Engine.ScalesFor(sel.Model); len(scales) > 0 {
			pills := []giode.Element{}
			for _, sc := range scales {
				pills = append(pills, components.Pill("scale:"+sel.Engine.ID, sc, fmt.Sprintf("%dx", sc),
					sel.Target == 0 && sel.Scale == sc, func() { s.Choose(kind, "", "", sc) }))
			}
			group := []giode.Element{components.SectionLabel(tr.T("Size")), giode.Row(giode.Styles{Gap: 8}, pills...)}
			if kind != media.Audio {
				if targets := sel.Targets(sess.Info.Width, sess.Info.Height); len(targets) > 0 {
					res := []giode.Element{}
					for _, t := range targets {
						res = append(res, components.Pill("target", t, pipeline.TargetLabel(t),
							sel.Target == t, func() { s.ChooseTarget(kind, t) }))
					}
					group = append(group, giode.Row(giode.Styles{Gap: 8}, res...))
				}
			}
			sections = append(sections, giode.Stack(giode.Styles{Gap: 10, Width: -1}, group...))
		}
		if len(sel.Engine.Options) > 0 {
			sections = append(sections, in.engineSettings(sel.Engine))
		}
		sections = append(sections, in.output(sel))
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: 26, Padding: giode.UniformInset(18)}, sections...)
}

// engineSettings summarizes the engine's options, with a button to
// change them.
func (in *Inspector) engineSettings(m *engine.Manifest) giode.Element {
	values := in.s.EngineOptions(m)
	var changed []string
	for i := range m.Options {
		if o := &m.Options[i]; values[o.ID] != o.Default {
			changed = append(changed, tr.T(o.Name)+": "+components.OptionLabel(o, values[o.ID]))
		}
	}
	summary := tr.T("Defaults, tuned for most GPUs")
	if len(changed) > 0 {
		summary = strings.Join(changed, " · ")
	}
	group := []giode.Element{
		components.SectionLabel(tr.T("Engine settings")),
		giode.Row(giode.Styles{Width: -1, Gap: 12, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(summary, theme.Caption().With(giode.Styles{MaxLines: 3, TextOverflow: giode.TextOverflowEllipsis}))),
			in.adjust,
		),
	}
	if m.Tune != nil && in.s.Session.Kind == media.Video {
		in.tune.Disabled(!in.s.CanTune())
		// Text first and the button under it: translations run long.
		group = append(group,
			giode.Text(
				tr.T("Time the settings on {seconds} seconds of this video and keep the fastest. Each try takes about as long as upscaling that clip.", "seconds", tr.Decimal(m.Tune.Seconds, 0)),
				theme.Caption()),
			in.tune,
		)
	}
	return giode.Stack(giode.Styles{Gap: 10, Width: -1}, group...)
}

// longVideoFrames is the length, about a minute and a half, past which
// a diffusion engine's slowness gets a warning.
const longVideoFrames = 2000

// diffusionTime roughly says how long a diffusion engine takes over
// frames, at the 2.5 seconds a frame SeedVR2's 3B model managed for
// 1080p on a 12 GB RTX 5070.
func diffusionTime(frames int) string {
	hours := float64(frames) * 2.5 / 3600
	if hours < 2 {
		return tr.T("over an hour")
	}
	return tr.N("about {count} hours", int(math.Round(hours)))
}

// output summarizes the file the upscale will write.
func (in *Inspector) output(sel store.Selection) giode.Element {
	s := in.s
	sess := s.Session
	info := sess.Info
	st := s.Settings
	rows := []giode.Element{components.SectionLabel(tr.T("Output"))}
	line := func(label, value string) giode.Element {
		return giode.Row(giode.Styles{Width: -1, Wrap: giode.WrapNoWrap, Gap: 12},
			giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(label, theme.Secondary())),
			giode.Text(value, theme.Label().With(giode.Styles{TextAlign: giode.TextAlignEnd})),
		)
	}
	switch sess.Kind {
	case media.Image, media.Video:
		if info.Width > 0 {
			sz := sel.Size(info.Width, info.Height)
			rows = append(rows, line(tr.T("Size"), format.Resolution(sz.Width, sz.Height)))
			if sz.Exact && sz.Scale > 1 && !sel.Engine.TakesResolution() {
				rows = append(rows, line(tr.T("Engine runs at"), tr.T("{scale}x, then resized", "scale", sz.Scale)))
			}
		}
		if sess.Kind == media.Image {
			rows = append(rows, line(tr.T("Format"), strings.ToUpper(st.ImageFormat)))
		} else {
			codec := map[string]string{"h264": "H.264", "h265": "H.265", "prores": "ProRes 422 HQ"}[st.VideoCodec]
			rows = append(rows, line(tr.T("Format"), codec), line(tr.T("Frames"), format.Count(info.Frames)))
			if info.HasAudio {
				rows = append(rows, line(tr.T("Audio"), tr.T("Copied from the original")))
			}
		}
	case media.Audio:
		rows = append(rows, line(tr.T("Format"), strings.ToUpper(st.AudioFormat)))
		if info.SampleRate > 0 {
			rows = append(rows, line(tr.T("Source"), format.Rate(info.SampleRate)+" · "+format.Channels(info.Channels)))
		}
	}
	dest := tr.T("Next to the original")
	if st.OutputDir != "" {
		dest = filepath.Base(st.OutputDir)
	}
	rows = append(rows, line(tr.T("Saved to"), dest))
	return giode.Stack(giode.Styles{Gap: 10, Width: -1}, rows...)
}

// noEngineHint says where to get an engine for kind.
func noEngineHint(k media.Kind) string {
	switch k {
	case media.Image:
		return tr.T("Install one from the Engines page to start upscaling images.")
	case media.Video:
		return tr.T("Install one from the Engines page to start upscaling videos.")
	}
	return tr.T("Install one from the Engines page to start enhancing audio.")
}
