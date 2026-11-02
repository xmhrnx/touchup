package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mehran9675/touchup/internal/proc"
	"github.com/mehran9675/touchup/internal/tr"
)

// Job is one run of an engine.
type Job struct {
	Model *Model
	Scale int
	GPU   int // -1 lets the engine choose

	// Input is a file, or a folder for batch runs. Output is the file
	// the engine should write, or a folder for batch runs.
	Input, Output string
	// InputDir and OutputDir are the folders holding Input and Output.
	InputDir, OutputDir string
	// InWidth and InHeight are the input's size in pixels, for engines
	// that take a target resolution instead of a scale. OutWidth and
	// OutHeight are the size wanted, when it isn't the input's times
	// Scale.
	InWidth, InHeight   int
	OutWidth, OutHeight int
	// OutputExt overrides the extension of the file the engine writes.
	OutputExt string
	// PathDirs are put in front of PATH for the engine, so engines that
	// call FFmpeg themselves find the one TouchUp uses.
	PathDirs []string
	// Options are the values of the engine's options, from
	// OptionValues; missing ones take their defaults.
	Options map[string]string
	// StrictGPUMemory keeps Python engines' PyTorch within the GPU's
	// free memory, so running out fails at once instead of spilling into
	// system memory, which Windows drivers do and which is many times
	// slower.
	StrictGPUMemory bool

	// Progress receives the engine's own percentage, when it prints one.
	Progress func(frac float64)
	// Log receives each line the engine prints.
	Log func(line string)
}

var defaultProgress = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)

// Run runs the engine and returns the file it produced: Output when the
// engine wrote it, otherwise the newest file with the engine's output
// extension in OutputDir (for engines that pick their own names). Batch
// runs return OutputDir.
func (m *Manifest) Run(ctx context.Context, j Job) (string, error) {
	exe := m.ExecutablePath()
	if exe == "" {
		return "", errors.New(tr.T("{engine} is not installed", "engine", m.Name))
	}
	if j.Model != nil && !m.ModelInstalled(j.Model) {
		return "", errors.New(tr.T("the {model} model of {engine} is not installed", "model", tr.T(j.Model.Name), "engine", m.Name))
	}
	if err := installScripts(m); err != nil {
		return "", fmt.Errorf("updating %s's scripts: %w", m.Name, err)
	}
	args := m.Command(j)
	env := append(os.Environ(), "PYTHONUNBUFFERED=1", "PYTHONIOENCODING=utf-8")
	if len(j.PathDirs) > 0 {
		dirs := append(slices.Clone(j.PathDirs), os.Getenv("PATH"))
		env = append(env, "PATH="+strings.Join(dirs, string(os.PathListSeparator)))
	}
	if m.Install.Type == "pip" {
		hooks, err := pythonHooks(j.StrictGPUMemory)
		if err != nil {
			return "", fmt.Errorf("preparing %s: %w", m.Name, err)
		}
		env = append(env, hooks...)
	}
	// Another engine's server would hold on to GPU memory this one needs.
	stopServers(m.ID)

	tail := &tailBuffer{max: 400}
	handle := lineHandler(m, j, tail)
	started := time.Now()
	served := false
	var err error
	if m.Serve {
		served, err = m.runServed(ctx, j, exe, args, env, m.workingDir(exe), handle)
	}
	if !served {
		cmd := proc.Command(ctx, exe, args...)
		cmd.Dir, cmd.Env = m.workingDir(exe), env
		pr, pw := io.Pipe()
		cmd.Stdout, cmd.Stderr = pw, pw
		done := make(chan struct{})
		go func() {
			defer close(done)
			proc.Lines(pr, handle)
		}()
		err = cmd.Run()
		pw.Close()
		<-done
	}
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", &RunError{Engine: m.Name, Summary: summarize(tail.lines), Log: tail.String()}
	}
	if _, err := os.Stat(j.Output); err == nil {
		return j.Output, nil
	}
	ext := m.OutputExt
	if j.OutputExt != "" {
		ext = j.OutputExt
	}
	if found := newestOutput(j.OutputDir, ext, started); found != "" {
		return found, nil
	}
	return "", &RunError{Engine: m.Name, Summary: tr.T("finished without writing a {ext} file", "ext", ext), Log: tail.String()}
}

// lineHandler returns what's done with each line an engine prints: it's
// kept for error reports, logged, and read for progress.
func lineHandler(m *Manifest, j Job, tail *tailBuffer) func(string) {
	prog := newProgress(m)
	return func(raw string) {
		line := strings.TrimSpace(raw)
		if line == "" {
			return
		}
		tail.add(raw)
		if j.Log != nil {
			j.Log(line)
		}
		if j.Progress == nil {
			return
		}
		if f, ok := prog.line(line); ok {
			j.Progress(f)
		}
	}
}

// Command returns the engine's arguments with the placeholders filled in.
func (m *Manifest) Command(j Job) []string {
	vars := map[string]string{
		"{input}":     j.Input,
		"{output}":    j.Output,
		"{inputDir}":  j.InputDir,
		"{outputDir}": j.OutputDir,
		"{scale}":     strconv.Itoa(max(1, j.Scale)),
		"{gpu}":       strconv.Itoa(j.GPU),
		"{engineDir}": m.Dir(),
		"{model}":     "",
		"{modelDir}":  m.ModelsDir(),
		"{modelPath}": m.ModelPath(j.Model),
		"{inWidth}":   strconv.Itoa(j.InWidth),
		"{inHeight}":  strconv.Itoa(j.InHeight),
	}
	outW, outH := j.InWidth*max(1, j.Scale), j.InHeight*max(1, j.Scale)
	if j.OutWidth > 0 && j.OutHeight > 0 {
		outW, outH = j.OutWidth, j.OutHeight
	}
	vars["{outWidth}"], vars["{outHeight}"] = strconv.Itoa(outW), strconv.Itoa(outH)
	vars["{outShort}"] = strconv.Itoa(min(outW, outH))
	if j.Model != nil {
		vars["{model}"] = j.Model.ID
	}
	expand := func(a string) string {
		for k, v := range vars {
			a = strings.ReplaceAll(a, k, v)
		}
		return a
	}
	var out []string
	for _, a := range m.Args {
		if a == "{modelArgs}" {
			if j.Model != nil {
				for _, ma := range j.Model.Args {
					out = append(out, expand(ma))
				}
			}
			continue
		}
		out = append(out, expand(a))
	}
	out = append(out, m.optionArgs(j.Options, expand)...)
	if j.GPU >= 0 {
		for _, a := range m.GPUArgs {
			out = append(out, expand(a))
		}
	}
	return out
}

// workingDir is where the engine runs: its install folder, where ncnn
// engines look for their models, or the executable's folder.
func (m *Manifest) workingDir(exe string) string {
	if m.Install.Type == "archive" {
		return m.Dir()
	}
	return filepath.Dir(exe)
}

// newestOutput finds the newest file with ext written under dir since
// started.
func newestOutput(dir, ext string, started time.Time) string {
	var best string
	var bestTime time.Time
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(p), ext) {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.ModTime().Before(started.Add(-2*time.Second)) {
			return nil
		}
		if best == "" || info.ModTime().After(bestTime) {
			best, bestTime = p, info.ModTime()
		}
		return nil
	})
	return best
}

// RunError is an engine or install step that failed: a one-line Summary
// to show, and the end of what it printed, for a details view.
type RunError struct {
	Engine  string
	Summary string
	Log     string
}

func (e *RunError) Error() string {
	return tr.T("{engine} failed: {reason}", "engine", e.Engine, "reason", e.Summary)
}

// exceptionLine matches the last line of a Python traceback, such as
// "torch.OutOfMemoryError: Allocation on device 0 ...", capturing it
// without the module path.
var exceptionLine = regexp.MustCompile(`^(?:[A-Za-z_]\w*\.)*([A-Z]\w*(?:Error|Exception|Interrupt)\b.*)$`)

// summarize picks the line that says why a run failed, in order: the
// last "Error:" line (TouchUp's runner scripts end with one), the last
// Python exception, the last line flagged "[ERROR]", else the last line
// that isn't part of a traceback's code listing. Engines often print a
// sign-off after failing, so the last line alone can't be trusted.
func summarize(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		if s, ok := strings.CutPrefix(strings.TrimSpace(lines[i]), "Error: "); ok {
			return s
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if m := exceptionLine.FindStringSubmatch(strings.TrimSpace(lines[i])); m != nil {
			return m[1]
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if _, s, ok := strings.Cut(lines[i], "[ERROR]"); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l == "" || strings.Trim(l, "^~ ") == "" || strings.HasPrefix(l, "File \"") {
			continue
		}
		return l
	}
	return tr.T("it exited without saying why")
}

// LogOf returns the engine output behind err, if it's a RunError.
func LogOf(err error) string {
	var re *RunError
	if errors.As(err, &re) {
		return re.Log
	}
	return ""
}
