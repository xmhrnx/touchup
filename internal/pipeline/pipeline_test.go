package pipeline

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/filters"
	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/settings"
)

// These tests run the real pipeline with stand-in engines: shell scripts
// that scale with FFmpeg. They need FFmpeg and a POSIX shell.

func setup(t *testing.T) media.Tools {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stand-in engines are shell scripts")
	}
	tools := media.Locate("", "", "")
	if !tools.Ready() {
		t.Skip("FFmpeg is not installed")
	}
	t.Setenv("TOUCHUP_HOME", t.TempDir())
	t.Setenv("TMPDIR", t.TempDir())
	return tools
}

// script writes an executable shell script and returns its path.
func script(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "engine.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\nset -e\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func ffmpeg(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("ffmpeg", append([]string{"-loglevel", "error", "-y"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v: %s", err, out)
	}
}

func options(t *testing.T, tools media.Tools, m *engine.Manifest) Options {
	t.Helper()
	m.Install = engine.InstallSpec{Type: "none"}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	return Options{Engine: m, Scale: 2, Settings: settings.Default(), Tools: tools}
}

func noReport(Update) {}

func TestImage(t *testing.T) {
	tools := setup(t)
	exe := script(t, `ffmpeg -loglevel error -y -i "$1" -vf "scale=iw*$3:ih*$3" "$2"`+"\n")
	o := options(t, tools, &engine.Manifest{
		ID: "scale", Name: "Scale", Media: []string{"image", "video"}, Input: "image",
		Executable: map[string]string{"*": exe}, Args: []string{"{input}", "{output}", "{scale}"}, OutputExt: ".png",
	})
	in := filepath.Join(t.TempDir(), "in.png")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=64x48", "-frames:v", "1", in)
	out := filepath.Join(t.TempDir(), "out.jpg")
	if err := Image(context.Background(), o, in, out, noReport); err != nil {
		t.Fatal(err)
	}
	info, err := tools.Probe(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 128 || info.Height != 96 {
		t.Fatalf("got %dx%d, want 128x96", info.Width, info.Height)
	}
}

func TestVideoBatch(t *testing.T) {
	tools := setup(t)
	// A batch engine: folders in, folders out, same file names.
	exe := script(t, `for f in "$1"/*.png; do
  ffmpeg -loglevel error -y -i "$f" -vf "scale=iw*$3:ih*$3" "$2/$(basename "$f")"
done
`)
	o := options(t, tools, &engine.Manifest{
		ID: "batch", Name: "Batch", Media: []string{"image", "video"}, Input: "image", Batch: true,
		Executable: map[string]string{"*": exe}, Args: []string{"{input}", "{output}", "{scale}"}, OutputExt: ".png",
	})
	in := filepath.Join(t.TempDir(), "in.mp4")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=64x48:r=10", "-f", "lavfi", "-i", "sine", "-t", "1",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", in)
	info, err := tools.Probe(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.mp4")
	if err := Video(context.Background(), o, in, out, info, noReport); err != nil {
		t.Fatal(err)
	}
	got, err := tools.Probe(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != 128 || got.Height != 96 {
		t.Errorf("got %dx%d, want 128x96", got.Width, got.Height)
	}
	if !got.HasAudio {
		t.Error("the audio was dropped")
	}
	if got.Frames != info.Frames {
		t.Errorf("got %d frames, want %d", got.Frames, info.Frames)
	}
}

func TestPreviews(t *testing.T) {
	tools := setup(t)
	img := script(t, `ffmpeg -loglevel error -y -i "$1" -vf "scale=iw*$3:ih*$3" "$2"`+"\n")
	o := options(t, tools, &engine.Manifest{
		ID: "scale", Name: "Scale", Media: []string{"image", "video"}, Input: "image",
		Executable: map[string]string{"*": img}, Args: []string{"{input}", "{output}", "{scale}"}, OutputExt: ".png",
	})
	video := filepath.Join(t.TempDir(), "in.mp4")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=64x48:r=10", "-t", "2", "-c:v", "libx264", "-pix_fmt", "yuv420p", video)
	p, err := PreviewFrame(context.Background(), o, video, 1.2, noReport)
	if err != nil {
		t.Fatal(err)
	}
	if p.Before.Bounds().Dx() != 64 || p.After.Bounds().Dx() != 128 {
		t.Errorf("frame preview sizes %v and %v", p.Before.Bounds(), p.After.Bounds())
	}

	// A video engine that also takes images, like SeedVR2, previews the
	// frame alone instead of a clip around it.
	vid := script(t, `case "$1" in *.png) ;; *) echo "got $1, not the frame"; exit 3 ;; esac
ffmpeg -loglevel error -y -i "$1" -vf "scale=iw*$3:ih*$3" "$2"`+"\n")
	vo := options(t, tools, &engine.Manifest{
		ID: "vid", Name: "Video", Media: []string{"video", "image"}, Input: "video", ImageOutputExt: ".png",
		Executable: map[string]string{"*": vid}, Args: []string{"{input}", "{output}", "{scale}"}, OutputExt: ".mp4",
	})
	vp, err := PreviewFrame(context.Background(), vo, video, 1.2, noReport)
	if err != nil {
		t.Fatal(err)
	}
	if vp.After.Bounds().Dx() != 128 {
		t.Errorf("video engine frame preview is %v wide, want 128", vp.After.Bounds())
	}

	// An audio engine that writes its own file name into {outputDir}.
	snd := script(t, `ffmpeg -loglevel error -y -i "$1" -ar 48000 "$2/enhanced_$(basename "$1")"`+"\n")
	ao := options(t, tools, &engine.Manifest{
		ID: "rate", Name: "Rate", Media: []string{"audio"}, Input: "audio",
		Executable: map[string]string{"*": snd}, Args: []string{"{input}", "{outputDir}"}, OutputExt: ".wav",
	})
	audio := filepath.Join(t.TempDir(), "in.wav")
	ffmpeg(t, "-f", "lavfi", "-i", "sine=sample_rate=22050", "-t", "6", audio)
	ap, err := PreviewAudio(context.Background(), ao, audio, 1, 3, noReport)
	if err != nil {
		t.Fatal(err)
	}
	defer ap.Discard()
	if ap.BeforeRate != 22050 || ap.AfterRate != 48000 {
		t.Errorf("rates %d and %d, want 22050 and 48000", ap.BeforeRate, ap.AfterRate)
	}
	if ap.BeforeSpec == nil || ap.AfterSpec == nil {
		t.Error("spectrograms missing")
	}
}

func TestOutputPathAvoidsOverwriting(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "song.flac")
	first := OutputPath(in, settings.Default(), "", ".wav", nil)
	if filepath.Base(first) != "song (TouchUp).wav" {
		t.Fatalf("got %q", first)
	}
	os.WriteFile(first, nil, 0o644)
	if second := OutputPath(in, settings.Default(), "", ".wav", nil); filepath.Base(second) != "song (TouchUp 2).wav" {
		t.Fatalf("got %q", second)
	}
	taken := func(p string) bool { return filepath.Base(p) == "song (TouchUp 2).wav" }
	if third := OutputPath(in, settings.Default(), "", ".wav", taken); filepath.Base(third) != "song (TouchUp 3).wav" {
		t.Fatalf("got %q", third)
	}
}

func TestPlanSize(t *testing.T) {
	m := &engine.Manifest{Scales: []int{2, 3, 4}}
	for _, c := range []struct {
		w, h, scale, target int
		want                Size
	}{
		{640, 360, 2, 0, Size{Scale: 2, Width: 1280, Height: 720}},
		{640, 360, 2, 1080, Size{Scale: 3, Width: 1920, Height: 1080, Exact: true}},
		{1920, 1080, 4, 2160, Size{Scale: 2, Width: 3840, Height: 2160, Exact: true}},
		{1080, 1920, 4, 2160, Size{Scale: 2, Width: 2160, Height: 3840, Exact: true}},
		// Beyond the largest scale: the engine's best, then a resize.
		{300, 200, 2, 1080, Size{Scale: 4, Width: 1620, Height: 1080, Exact: true}},
		{64, 48, 2, 100, Size{Scale: 3, Width: 134, Height: 100, Exact: true}},
	} {
		if got := PlanSize(m, nil, c.scale, c.target, c.w, c.h); got != c.want {
			t.Errorf("%dx%d to %d: got %+v, want %+v", c.w, c.h, c.target, got, c.want)
		}
	}
}

func TestTargets(t *testing.T) {
	tools := setup(t)
	scale := script(t, `ffmpeg -loglevel error -y -i "$1" -vf "scale=iw*$3:ih*$3" "$2"`+"\n")
	img := options(t, tools, &engine.Manifest{
		ID: "scale", Name: "Scale", Media: []string{"image", "video"}, Input: "image", Scales: []int{2, 3, 4},
		Executable: map[string]string{"*": scale}, Args: []string{"{input}", "{output}", "{scale}"}, OutputExt: ".png",
	})
	img.Target = 100
	check := func(name, path string, w, h int) {
		t.Helper()
		info, err := tools.Probe(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Width != w || info.Height != h {
			t.Errorf("%s: got %dx%d, want %dx%d", name, info.Width, info.Height, w, h)
		}
	}

	in := filepath.Join(t.TempDir(), "in.png")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=64x48", "-frames:v", "1", in)
	out := filepath.Join(t.TempDir(), "out.png")
	if err := Image(context.Background(), img, in, out, noReport); err != nil {
		t.Fatal(err)
	}
	check("image", out, 134, 100)

	video := filepath.Join(t.TempDir(), "in.mp4")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=64x48:r=10", "-t", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p", video)
	info, err := tools.Probe(context.Background(), video)
	if err != nil {
		t.Fatal(err)
	}
	vout := filepath.Join(t.TempDir(), "frames.mp4")
	if err := Video(context.Background(), img, video, vout, info, noReport); err != nil {
		t.Fatal(err)
	}
	check("video by frames", vout, 134, 100)

	// A video engine told the size, landing a little off it, as SeedVR2
	// can: the result is resized.
	whole := script(t, `ffmpeg -loglevel error -y -i "$1" -vf "scale=-2:$(($3 + 2))" -c:v libx264 -pix_fmt yuv420p "$2"`+"\n")
	vid := options(t, tools, &engine.Manifest{
		ID: "whole", Name: "Whole", Media: []string{"video"}, Input: "video", Scales: []int{2, 3, 4},
		Executable: map[string]string{"*": whole}, Args: []string{"{input}", "{output}", "{outShort}"}, OutputExt: ".mp4",
	})
	vid.Target = 100
	wout := filepath.Join(t.TempDir(), "whole.mp4")
	if err := Video(context.Background(), vid, video, wout, info, noReport); err != nil {
		t.Fatal(err)
	}
	check("whole video", wout, 134, 100)
}

func TestTiledPicture(t *testing.T) {
	tools := setup(t)
	parts := filepath.Join(t.TempDir(), "parts.txt")
	exe := script(t, `echo "$1" >> "`+parts+`"
ffmpeg -loglevel error -y -i "$1" -vf "scale=iw*$3:ih*$3:flags=bicubic" "$2"`+"\n")
	m := &engine.Manifest{
		ID: "parts", Name: "Parts", Media: []string{"image"}, Input: "image", Scales: []int{2},
		Executable: map[string]string{"*": exe}, Args: []string{"{input}", "{output}", "{scale}"}, OutputExt: ".png",
		Options: []engine.Option{{ID: engine.OptionTileImage, Name: "Split", Type: engine.OptionNumber, Default: "0"}},
	}
	o := options(t, tools, m)
	o.EngineOptions = map[string]string{engine.OptionTileImage: "160"}

	in := filepath.Join(t.TempDir(), "in.png")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=200x130", "-frames:v", "1", in)
	out := filepath.Join(t.TempDir(), "out.png")
	if err := Image(context.Background(), o, in, out, noReport); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(parts)
	if n := len(strings.Fields(string(data))); n != 6 {
		t.Errorf("upscaled in %d parts, want 3 × 2", n)
	}
	got, err := readPNG(out)
	if err != nil {
		t.Fatal(err)
	}
	if b := got.Bounds(); b.Dx() != 400 || b.Dy() != 260 {
		t.Fatalf("got %v, want 400x260", b)
	}

	// Joined parts match the picture upscaled in one go, but for the
	// borders the parts had that the whole didn't.
	whole := filepath.Join(t.TempDir(), "whole.png")
	ffmpeg(t, "-i", in, "-vf", "scale=400:260:flags=bicubic", whole)
	ref, err := readPNG(whole)
	if err != nil {
		t.Fatal(err)
	}
	var sum float64
	for i := range got.Pix {
		sum += math.Abs(float64(got.Pix[i]) - float64(ref.Pix[i]))
	}
	mean := sum / float64(len(got.Pix))
	t.Logf("mean difference %.3f", mean)
	if mean > 1.5 {
		t.Errorf("joined parts differ from the whole by %.2f on average", mean)
	}
}

func TestTiledPictureBatch(t *testing.T) {
	tools := setup(t)
	runs := filepath.Join(t.TempDir(), "runs.txt")
	// A batch engine, like SeedVR2 given a folder: it starts once and
	// writes each part under the part's own name.
	exe := script(t, `echo run >> "`+runs+`"
for f in "$1"/*.png; do
  ffmpeg -loglevel error -y -i "$f" -vf "scale=iw*$3:ih*$3:flags=bicubic" "$2/$(basename "$f")"
done
`)
	m := &engine.Manifest{
		ID: "parts", Name: "Parts", Media: []string{"image"}, Input: "image", Scales: []int{2}, Batch: true,
		Executable: map[string]string{"*": exe}, Args: []string{"{input}", "{output}", "{scale}"}, OutputExt: ".png",
		Options: []engine.Option{{ID: engine.OptionTileImage, Name: "Split", Type: engine.OptionNumber, Default: "0"}},
	}
	o := options(t, tools, m)
	o.EngineOptions = map[string]string{engine.OptionTileImage: "160"}

	in := filepath.Join(t.TempDir(), "in.png")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=200x130", "-frames:v", "1", in)
	out := filepath.Join(t.TempDir(), "out.png")
	if err := Image(context.Background(), o, in, out, noReport); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(runs)
	if n := len(strings.Fields(string(data))); n != 1 {
		t.Errorf("the engine started %d times, want once for all parts", n)
	}
	got, err := readPNG(out)
	if err != nil {
		t.Fatal(err)
	}
	whole := filepath.Join(t.TempDir(), "whole.png")
	ffmpeg(t, "-i", in, "-vf", "scale=400:260:flags=bicubic", whole)
	ref, err := readPNG(whole)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != ref.Bounds() {
		t.Fatalf("got %v, want %v", got.Bounds(), ref.Bounds())
	}
	var sum float64
	for i := range got.Pix {
		sum += math.Abs(float64(got.Pix[i]) - float64(ref.Pix[i]))
	}
	if mean := sum / float64(len(got.Pix)); mean > 1.5 {
		t.Errorf("joined parts differ from the whole by %.2f on average", mean)
	}
}

func TestVideoSegments(t *testing.T) {
	tools := setup(t)
	defer func(b int64) { segmentBytes = b }(segmentBytes)
	segmentBytes = 1 // the smallest segments
	counts := filepath.Join(t.TempDir(), "counts.txt")
	// A batch engine that notes how many frames each run was given.
	exe := script(t, `ls "$1" | wc -l >> "`+counts+`"
for f in "$1"/*.png; do
  ffmpeg -loglevel error -y -i "$f" -vf "scale=iw*$3:ih*$3" "$2/$(basename "$f")"
done
`)
	o := options(t, tools, &engine.Manifest{
		ID: "batch", Name: "Batch", Media: []string{"image", "video"}, Input: "image", Batch: true,
		Executable: map[string]string{"*": exe}, Args: []string{"{input}", "{output}", "{scale}"}, OutputExt: ".png",
	})
	in := filepath.Join(t.TempDir(), "in.mp4")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=64x48:r=25", "-f", "lavfi", "-i", "sine", "-t", "2",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", in)
	info, err := tools.Probe(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.mp4")
	if err := Video(context.Background(), o, in, out, info, noReport); err != nil {
		t.Fatal(err)
	}
	got, err := tools.Probe(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if got.Frames != info.Frames || got.Width != 128 || !got.HasAudio {
		t.Errorf("got %d frames of %dx%d (audio %v), want %d of 128x96 with audio", got.Frames, got.Width, got.Height, got.HasAudio, info.Frames)
	}
	data, _ := os.ReadFile(counts)
	runs := strings.Fields(string(data))
	if len(runs) != 3 { // 50 frames, 24 new a run
		t.Errorf("%d runs, want 3: %v", len(runs), runs)
	}
	for i, r := range runs {
		n, _ := strconv.Atoi(r)
		if want := minSegment + contextFrames; n > want || (i > 0 && n <= contextFrames) {
			t.Errorf("run %d had %d frames on disk, want at most %d", i+1, n, want)
		}
	}
}

func classic(t *testing.T, tools media.Tools) Options {
	t.Helper()
	m := &engine.Manifest{ID: "classic", Name: "Classic", Media: []string{"image", "video"}, Input: "image", Batch: true,
		Install: engine.InstallSpec{Type: "builtin"}, Args: []string{"{input}", "{output}", "{outWidth}", "{outHeight}"},
		OutputExt: ".png", Scales: []int{1, 2, 3, 4}}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	return Options{Engine: m, Scale: 2, Settings: settings.Default(), Tools: tools}
}

func TestClassicVideoWithFilters(t *testing.T) {
	tools := setup(t)
	o := classic(t, tools)
	o.Settings.Filters = filters.Settings{Denoise: 40, Sharpen: 30, Saturation: -100}
	in := filepath.Join(t.TempDir(), "in.mp4")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=64x48:r=25", "-f", "lavfi", "-i", "sine", "-t", "1",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", in)
	info, err := tools.Probe(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.mp4")
	if err := Video(context.Background(), o, in, out, info, noReport); err != nil {
		t.Fatal(err)
	}
	got, err := tools.Probe(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != 128 || got.Height != 96 || got.Frames != info.Frames || !got.HasAudio {
		t.Errorf("got %d frames of %dx%d (audio %v), want %d of 128x96 with audio", got.Frames, got.Width, got.Height, got.HasAudio, info.Frames)
	}
	// Saturation -100 makes it black and white.
	frame, err := tools.FrameAt(context.Background(), out, 0.5, 0)
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := frame.At(40, 40).RGBA()
	if d := max(r, g, b) - min(r, g, b); d > 0x0800 {
		t.Errorf("color left in a black and white result: %x %x %x", r, g, b)
	}
}

func TestEngineFramesWithFilters(t *testing.T) {
	tools := setup(t)
	exe := script(t, `for f in "$1"/*.png; do
  ffmpeg -loglevel error -y -i "$f" -vf "scale=iw*$3:ih*$3" "$2/$(basename "$f")"
done
`)
	o := options(t, tools, &engine.Manifest{
		ID: "batch", Name: "Batch", Media: []string{"image", "video"}, Input: "image", Batch: true,
		Executable: map[string]string{"*": exe}, Args: []string{"{input}", "{output}", "{scale}"}, OutputExt: ".png",
	})
	o.Settings.Filters = filters.Settings{Deblock: 50, Grain: 30}
	o.Settings.ConvertAudio = true
	in := filepath.Join(t.TempDir(), "in.mp4")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=64x48:r=25", "-f", "lavfi", "-i", "sine", "-t", "1",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "mp3", in)
	info, err := tools.Probe(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.mp4")
	if err := Video(context.Background(), o, in, out, info, noReport); err != nil {
		t.Fatal(err)
	}
	got, err := tools.Probe(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != 128 || got.Frames != info.Frames || got.AudioCodec != "aac" {
		t.Errorf("got %d frames of %dx%d, audio %q; want %d of 128x96, AAC", got.Frames, got.Width, got.Height, got.AudioCodec, info.Frames)
	}
}

func TestClassicImage(t *testing.T) {
	tools := setup(t)
	o := classic(t, tools)
	o.Scale = 3
	o.Settings.Filters = filters.Settings{Brightness: 20}
	in := filepath.Join(t.TempDir(), "in.png")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=50x40", "-frames:v", "1", in)
	out := filepath.Join(t.TempDir(), "out.png")
	if err := Image(context.Background(), o, in, out, noReport); err != nil {
		t.Fatal(err)
	}
	got, err := readPNG(out)
	if err != nil {
		t.Fatal(err)
	}
	if b := got.Bounds(); b.Dx() != 150 || b.Dy() != 120 {
		t.Errorf("got %v, want 150x120", b)
	}
}

func TestWholeVideoWithFilters(t *testing.T) {
	tools := setup(t)
	// A video engine: upscales the file it's given.
	exe := script(t, `ffmpeg -loglevel error -y -i "$1" -vf "scale=iw*$3:ih*$3" -an "$2"`+"\n")
	o := options(t, tools, &engine.Manifest{
		ID: "whole", Name: "Whole", Media: []string{"video"}, Input: "video",
		Executable: map[string]string{"*": exe}, Args: []string{"{input}", "{output}", "{scale}"}, OutputExt: ".mp4",
	})
	o.Settings.Filters = filters.Settings{Deinterlace: true, Contrast: 20}
	in := filepath.Join(t.TempDir(), "in.mp4")
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=s=64x48:r=25", "-f", "lavfi", "-i", "sine", "-t", "1",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", in)
	info, err := tools.Probe(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.mp4")
	if err := Video(context.Background(), o, in, out, info, noReport); err != nil {
		t.Fatal(err)
	}
	got, err := tools.Probe(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != 128 || got.Frames != info.Frames || !got.HasAudio {
		t.Errorf("got %d frames of %dx%d (audio %v), want %d of 128x96 with audio", got.Frames, got.Width, got.Height, got.HasAudio, info.Frames)
	}
}
