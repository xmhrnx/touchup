// Package engine describes, installs and runs upscaling engines.
//
// An engine is any command-line program that turns an input file (or a
// folder of them) into a better one. A JSON manifest says how to install
// it (download an archive, create a Python environment, or point at a
// program already on disk), how to call it, and which models it offers.
// TouchUp ships manifests for well-known open-source engines, and users
// add their own the same way.
package engine

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Manifest describes an engine.
type Manifest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Vendor      string   `json:"vendor,omitempty"`
	Description string   `json:"description,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	License     string   `json:"license,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	// Requirements is shown to users before they install: "Vulkan GPU",
	// "Python 3.10, NVIDIA GPU recommended".
	Requirements string `json:"requirements,omitempty"`

	// Media lists what the engine can upscale: "image", "video", "audio".
	Media []string `json:"media"`
	// Input is what the command itself consumes: "image", "video" or
	// "audio". An image engine listed for video runs on extracted frames.
	Input string `json:"input"`
	// Batch means the command accepts folders for {input} and {output},
	// which makes frame-by-frame video much faster.
	Batch bool `json:"batch,omitempty"`
	// Serve means the command can stay running between jobs, so models
	// load once: started with TOUCHUP_SERVE=1 in its environment, it
	// runs the job in its arguments, prints a line "TOUCHUP-SERVE-DONE
	// <exit code>", then reads more jobs from standard input, a JSON
	// array of arguments a line, until it closes. See serve.go.
	Serve bool `json:"serve,omitempty"`

	Install InstallSpec `json:"install"`
	// Executable is the program to run, by GOOS ("linux", "windows",
	// "darwin") or "*". A relative name is looked up in the engine's
	// folder (or its Python environment), then on PATH.
	Executable map[string]string `json:"executable"`
	// Args are the command's arguments, with placeholders replaced:
	// {input}, {output}, {inputDir}, {outputDir}, {scale}, {gpu},
	// {engineDir}, {model}, {modelDir}, {modelPath} (the model's first
	// file), {inWidth}, {inHeight}, {outWidth}, {outHeight} and {outShort}
	// (the output's shorter side). An argument that is exactly
	// "{modelArgs}" expands to the selected model's arguments.
	Args []string `json:"args"`
	// GPUArgs are appended when the user picked a GPU.
	GPUArgs []string `json:"gpuArgs,omitempty"`
	// OutputExt is the extension the engine writes: ".png", ".wav".
	OutputExt string `json:"outputExt"`
	// ImageOutputExt is what a video engine writes when it's given an
	// image. Defaults to ".png".
	ImageOutputExt string `json:"imageOutputExt,omitempty"`
	// InputRate is the sample rate an audio engine needs its input in;
	// 0 keeps the file's own.
	InputRate int `json:"inputRate,omitempty"`

	// Scales offered to the user; empty for engines with a fixed output
	// (most audio engines).
	Scales       []int   `json:"scales,omitempty"`
	DefaultScale int     `json:"defaultScale,omitempty"`
	Models       []Model `json:"models,omitempty"`

	// Progress is a regular expression whose first group is a
	// percentage in the engine's output. Defaults to "N%".
	Progress string `json:"progress,omitempty"`
	// ProgressSteps replaces Progress for engines that count parts
	// instead of printing percentages: regular expressions, outermost
	// first, whose two groups are a part's number and the total, such as
	// "Chunk (\\d+)/(\\d+)" and then "batch (\\d+)/(\\d+)".
	ProgressSteps []string `json:"progressSteps,omitempty"`

	// Options are settings the user can change, shown on the engine's
	// card.
	Options []Option `json:"options,omitempty"`
	// Tune lists the options worth timing to find the fastest settings
	// for the user's GPU.
	Tune *Tuning `json:"tune,omitempty"`

	// Featured engines are listed first: the strongest of the catalog.
	Featured bool `json:"featured,omitempty"`

	// Custom marks engines the user added; set by the registry.
	Custom bool `json:"-"`
}

// Model is one set of weights or one mode of an engine.
type Model struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Args        []string `json:"args,omitempty"`
	// Scales overrides the engine's scales for this model.
	Scales []int `json:"scales,omitempty"`
	// Files are the weights to download, into the engine's models
	// folder. A model without files comes with the engine.
	Files []ModelFile `json:"files,omitempty"`
	// Size is a display hint for the download: "64 MB", "≈ 4 GB".
	Size string `json:"size,omitempty"`
	// Recommended marks the model to suggest first.
	Recommended bool `json:"recommended,omitempty"`
}

// ModelFile is one file of a model's weights.
type ModelFile struct {
	URL string `json:"url"`
	// Name is the file's path inside the models folder; files with the
	// same name are shared between models.
	Name   string `json:"name"`
	SHA256 string `json:"sha256,omitempty"`
}

// InstallSpec says how to install an engine.
type InstallSpec struct {
	// Type is "archive" (download and unpack), "pip" (a Python virtual
	// environment), "none" (Executable already exists) or, for TouchUp's
	// own engines only, "builtin" (written in Go; see builtin.go).
	Type string `json:"type"`
	// URLs of archives by "goos/goarch", "goos" or "*".
	URLs map[string]string `json:"urls,omitempty"`
	// Packages are pip requirement specifiers, installed in one step.
	Packages []string `json:"packages,omitempty"`
	// Pip are install steps run in order after Packages, for packages
	// that need their own index (PyTorch's CUDA builds on Windows).
	Pip []PipStep `json:"pip,omitempty"`
	// Source is an archive of the engine's code, unpacked into the
	// engine's folder before the Python packages are installed, so a
	// step can install its requirements file.
	Source *Source `json:"source,omitempty"`
	// Scripts are runner scripts TouchUp ships, copied into the engine's
	// folder.
	Scripts []string `json:"scripts,omitempty"`
	// Size is a display hint: "45 MB", "2.5 GB with models".
	Size string `json:"size,omitempty"`
}

// PipStep is one pip install command.
type PipStep struct {
	// Args are pip install arguments; {engineDir} is replaced.
	Args []string `json:"args"`
	// IndexURL replaces PyPI for this step, by "goos" or "*".
	IndexURL map[string]string `json:"indexURL,omitempty"`
	// OS limits the step to these systems ("windows", "linux",
	// "darwin"); empty means all.
	OS []string `json:"os,omitempty"`
	// Optional steps install extras the engine works without, such as a
	// faster attention kernel: when one fails, the install goes on.
	Optional bool `json:"optional,omitempty"`
}

// Source is an archive of an engine's code.
type Source struct {
	URL string `json:"url"`
	// Dir is the folder inside the engine's folder it's unpacked into.
	Dir string `json:"dir"`
}

// Handles reports whether the engine upscales kind ("image", "video",
// "audio").
func (m *Manifest) Handles(kind string) bool { return slices.Contains(m.Media, kind) }

// Model returns the model with id, or the first model, or nil when the
// engine has none.
func (m *Manifest) Model(id string) *Model {
	for i := range m.Models {
		if m.Models[i].ID == id {
			return &m.Models[i]
		}
	}
	if len(m.Models) > 0 {
		return &m.Models[0]
	}
	return nil
}

// ScalesFor returns the scales offered with model.
func (m *Manifest) ScalesFor(model *Model) []int {
	if model != nil && len(model.Scales) > 0 {
		return model.Scales
	}
	return m.Scales
}

// ScaleFor returns the smallest of model's scales that enlarges by at
// least factor, or the largest when none does; 1 when the engine has no
// scales.
func (m *Manifest) ScaleFor(model *Model, factor float64) int {
	scales := slices.Sorted(slices.Values(m.ScalesFor(model)))
	if len(scales) == 0 {
		return 1
	}
	for _, s := range scales {
		if float64(s) >= factor-1e-9 {
			return s
		}
	}
	return scales[len(scales)-1]
}

// TakesResolution reports whether the engine is told the output size
// itself ({outWidth}, {outHeight} or {outShort}) rather than a scale, so
// it can make any size.
func (m *Manifest) TakesResolution() bool {
	args := slices.Clone(m.Args)
	for _, mdl := range m.Models {
		args = append(args, mdl.Args...)
	}
	for _, a := range args {
		if strings.Contains(a, "{outWidth}") || strings.Contains(a, "{outHeight}") || strings.Contains(a, "{outShort}") {
			return true
		}
	}
	return false
}

// PreferredScale returns the default scale for model.
func (m *Manifest) PreferredScale(model *Model) int {
	scales := m.ScalesFor(model)
	if len(scales) == 0 {
		return 1
	}
	if slices.Contains(scales, m.DefaultScale) {
		return m.DefaultScale
	}
	return scales[len(scales)-1]
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,63}$`)

// Validate checks that a manifest is complete enough to install and run.
func (m *Manifest) Validate() error {
	var problems []string
	if !idPattern.MatchString(m.ID) {
		problems = append(problems, "id must be 2-64 lowercase letters, digits, dots, dashes or underscores")
	}
	if strings.TrimSpace(m.Name) == "" {
		problems = append(problems, "name is required")
	}
	if len(m.Media) == 0 {
		problems = append(problems, "media must list image, video or audio")
	}
	for _, k := range m.Media {
		if k != "image" && k != "video" && k != "audio" {
			problems = append(problems, fmt.Sprintf("unknown media %q", k))
		}
	}
	switch m.Input {
	case "image", "video", "audio":
	default:
		problems = append(problems, "input must be image, video or audio")
	}
	if m.Input == "image" && m.Handles("audio") || m.Input == "audio" && (m.Handles("image") || m.Handles("video")) {
		problems = append(problems, "input doesn't fit the media it handles")
	}
	switch m.Install.Type {
	case "archive":
		if len(m.Install.URLs) == 0 {
			problems = append(problems, "an archive install needs urls")
		}
	case "pip":
		if len(m.Install.Packages) == 0 && len(m.Install.Pip) == 0 {
			problems = append(problems, "a pip install needs packages")
		}
	case "none", "builtin":
	default:
		problems = append(problems, "install.type must be archive, pip or none")
	}
	if len(m.Executable) == 0 && !m.Builtin() {
		problems = append(problems, "executable is required")
	}
	if !slices.ContainsFunc(m.Args, func(a string) bool { return strings.Contains(a, "{input") }) {
		problems = append(problems, "args must use {input} or {inputDir}")
	}
	if !strings.HasPrefix(m.OutputExt, ".") {
		problems = append(problems, `outputExt must start with a dot, like ".png"`)
	}
	for _, mdl := range m.Models {
		for _, f := range mdl.Files {
			if f.URL == "" || f.Name == "" || strings.Contains(f.Name, "..") {
				problems = append(problems, fmt.Sprintf("model %q has a file without a URL or a safe name", mdl.ID))
			}
		}
	}
	if m.Progress != "" {
		if _, err := regexp.Compile(m.Progress); err != nil {
			problems = append(problems, "progress is not a valid regular expression")
		}
	}
	problems = append(problems, m.validateOptions()...)
	problems = append(problems, m.validateTuning()...)
	for _, s := range m.ProgressSteps {
		if re, err := regexp.Compile(s); err != nil || re.NumSubexp() < 2 {
			problems = append(problems, fmt.Sprintf("progress step %q needs two groups: the part and the total", s))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
