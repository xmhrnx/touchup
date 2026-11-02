package components

import (
	"errors"
	"strconv"
	"strings"

	"gioui.org/layout"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/input"
	"github.com/mehran9675/giode/components/scroll"
	"github.com/mehran9675/giode/components/segmented"
	"github.com/mehran9675/giode/components/toggle"
	"github.com/mehran9675/giode/styles/properties"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// CustomEngineForm collects what's needed to run any command-line
// upscaler as an engine. OnSave gets the manifest and returns an error
// to show, or nil to close.
type CustomEngineForm struct {
	OnSave   func(*engine.Manifest) error
	OnCancel func()

	name, exe, source, args, ext, scales *input.Input
	media, install                       *segmented.Control
	batch                                *toggle.Switch
	save, cancel                         *giode.ButtonElement
	scroll                               *scroll.Scroll
	err                                  string
}

var defaultArgs = []string{
	"-i {input} -o {output} -s {scale}",
	"--input {input} --output {output} --scale {scale}",
	"-i {input} -o {output}",
}

var defaultExts = []string{".png", ".mp4", ".wav"}

// NewCustomEngineForm creates the form.
func NewCustomEngineForm() *CustomEngineForm {
	f := &CustomEngineForm{
		name:    theme.Field(tr.T("My Upscaler")),
		exe:     theme.Field(tr.T("my-upscaler, or /full/path/to/my-upscaler")),
		source:  theme.Field("https://…/my-upscaler-linux.zip"),
		args:    theme.Field(defaultArgs[0]),
		ext:     theme.Field(".png"),
		scales:  theme.Field("2 4"),
		media:   theme.Segmented(tr.T("Images & video"), tr.T("Video files"), tr.T("Audio")),
		install: theme.Segmented(tr.T("On this computer"), tr.T("Download archive"), tr.T("Python package")),
		batch:   giode.Input.Switch("").KnobColor("#FFFFFF", "#FFFFFF"),
	}
	f.args.SetText(defaultArgs[0])
	f.ext.SetText(defaultExts[0])
	f.scales.SetText("2 4")
	f.media.OnChange(func(i int) {
		f.args.SetText(defaultArgs[i])
		f.ext.SetText(defaultExts[i])
		if i == 2 {
			f.scales.SetText("")
		} else if f.scales.Text() == "" {
			f.scales.SetText("2 4")
		}
	})
	f.install.OnChange(func(i int) {
		switch i {
		case 1:
			f.source.Placeholder("https://…/my-upscaler-linux.zip")
		case 2:
			f.source.Placeholder("my-upscaler-package==1.0 torch")
		}
	})
	f.scroll = giode.Scroll(f.fields, giode.Styles{ScrollBar: giode.ScrollBar{Width: 4, ThumbColor: theme.Border}}).FitContent(true)
	f.save = theme.PrimaryButton(tr.T("Add engine"), "plus").OnClick(f.submit)
	f.cancel = theme.GhostButton(tr.T("Cancel"), "").OnClick(func() {
		if f.OnCancel != nil {
			f.OnCancel()
		}
	})
	return f
}

// Reset clears the form for a new engine.
func (f *CustomEngineForm) Reset() {
	f.name.SetText("")
	f.exe.SetText("")
	f.source.SetText("")
	f.media.SetSelected(0)
	f.install.SetSelected(0)
	f.args.SetText(defaultArgs[0])
	f.ext.SetText(defaultExts[0])
	f.scales.SetText("2 4")
	f.batch.SetChecked(false)
	f.err = ""
}

func (f *CustomEngineForm) manifest() (*engine.Manifest, error) {
	name := strings.TrimSpace(f.name.Text())
	if name == "" {
		return nil, errors.New(tr.T("give the engine a name"))
	}
	id := engine.Slug(name)
	if len(id) < 2 {
		return nil, errors.New(tr.T("the name needs some letters or digits"))
	}
	exe := strings.TrimSpace(f.exe.Text())
	if exe == "" {
		return nil, errors.New(tr.T("enter the program to run"))
	}
	m := &engine.Manifest{
		ID: "custom-" + id, Name: name, Vendor: tr.T("Custom engine"),
		Executable: map[string]string{"*": exe},
		Args:       engine.SplitArgs(f.args.Text()),
		OutputExt:  strings.TrimSpace(f.ext.Text()),
	}
	if m.OutputExt != "" && !strings.HasPrefix(m.OutputExt, ".") {
		m.OutputExt = "." + m.OutputExt
	}
	switch f.media.Selected() {
	case 0:
		m.Media, m.Input, m.Batch = []string{"image", "video"}, "image", f.batch.Checked()
		m.Description = tr.T("Runs {program} on images, and on video frame by frame.", "program", exe)
	case 1:
		m.Media, m.Input = []string{"video"}, "video"
		m.Description = tr.T("Runs {program} on whole video files.", "program", exe)
	case 2:
		m.Media, m.Input = []string{"audio"}, "audio"
		m.Description = tr.T("Runs {program} on audio files.", "program", exe)
	}
	for _, s := range strings.Fields(strings.ReplaceAll(f.scales.Text(), ",", " ")) {
		v, err := strconv.Atoi(strings.TrimSuffix(strings.ToLower(s), "x"))
		if err != nil || v < 1 || v > 16 {
			return nil, errors.New(tr.T("scales must be numbers like 2 4"))
		}
		m.Scales = append(m.Scales, v)
	}
	source := strings.TrimSpace(f.source.Text())
	switch f.install.Selected() {
	case 0:
		m.Install = engine.InstallSpec{Type: "none"}
	case 1:
		if source == "" {
			return nil, errors.New(tr.T("enter the archive's download link"))
		}
		m.Install = engine.InstallSpec{Type: "archive", URLs: map[string]string{"*": source}}
	case 2:
		if source == "" {
			return nil, errors.New(tr.T("enter the pip packages to install"))
		}
		m.Install = engine.InstallSpec{Type: "pip", Packages: strings.Fields(source)}
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func (f *CustomEngineForm) submit() {
	m, err := f.manifest()
	if err == nil && f.OnSave != nil {
		err = f.OnSave(m)
	}
	if err != nil {
		f.err = err.Error()
		return
	}
	f.Reset()
}

// Layout draws the form: a header, the fields (scrolling when the window
// is short) and the buttons.
func (f *CustomEngineForm) Layout(gtx layout.Context) layout.Dimensions {
	children := []giode.Element{
		giode.Row(giode.Styles{Gap: 12, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			IconBubble("terminal", theme.Primary, 40),
			giode.Stack(giode.Styles{Gap: 2},
				giode.Text(tr.T("Add a custom engine"), theme.Heading().With(giode.Styles{FontSize: 18})),
				giode.Text(tr.T("Any command-line upscaler works: TouchUp fills in the files and runs it."), theme.Secondary()),
			),
		),
		giode.Box(giode.Styles{Width: -1, MaxHeight: properties.MaxHeight(gtx.Metric.PxToDp(gtx.Constraints.Max.Y)) - 190}, f.scroll),
	}
	if f.err != "" {
		children = append(children, Banner("triangle-alert", theme.Danger, tr.T("Can't add this engine"), f.err))
	}
	children = append(children, giode.Row(giode.Styles{Width: -1, Gap: 8, Justify: giode.JustifyEnd},
		f.cancel, f.save))
	return giode.Stack(giode.Styles{Gap: 16, Width: 560}, children...).Layout(gtx)
}

func (f *CustomEngineForm) fields() giode.Element {
	installType := f.install.Selected()
	fields := []giode.Element{
		FormField(tr.T("Name"), "", f.name),
		FormField(tr.T("Upscales"), "", f.media),
		FormField(tr.T("Install"), "", f.install),
	}
	switch installType {
	case 1:
		fields = append(fields, FormField(tr.T("Archive link"), tr.T("A .zip or .tar.gz; a single top folder inside is unwrapped."), f.source))
	case 2:
		fields = append(fields, FormField(tr.T("pip packages"), tr.T("Installed into a private Python environment."), f.source))
	}
	exeHint := tr.T("A name on your PATH, or a full path.")
	if installType != 0 {
		exeHint = tr.T("The program's name inside the installed files.")
	}
	fields = append(fields,
		FormField(tr.T("Program"), exeHint, f.exe),
		FormField(tr.T("Arguments"), tr.T("Placeholders:")+" {input} {output} {inputDir} {outputDir} {scale} {gpu} {engineDir}", f.args),
		giode.Row(giode.Styles{Gap: 12, Width: -1, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{FlexGrow: 1}, FormField(tr.T("Writes"), tr.T("Extension of the result."), f.ext)),
			giode.Box(giode.Styles{FlexGrow: 1}, FormField(tr.T("Scales"), tr.T("Leave empty for a fixed output."), f.scales)),
		),
	)
	if f.media.Selected() == 0 {
		fields = append(fields, SettingRow(tr.T("Accepts folders"),
			tr.T("Pass folders as {input} and {output} to upscale all video frames in one run.", "input", "{input}", "output", "{output}"), f.batch))
	}
	return giode.Stack(giode.Styles{Gap: 14, Width: -1, Padding: giode.Inset{Right: 10}}, fields...)
}
