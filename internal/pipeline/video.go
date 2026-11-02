package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/tr"
)

// Video upscales the video at in and writes it to out, keeping its
// audio. Image engines run on every frame; video engines on the file.
func Video(ctx context.Context, o Options, in, out string, info media.Info, report Reporter) error {
	if err := o.check(); err != nil {
		return err
	}
	ws, err := newWorkspace("video", o.Settings.KeepWork)
	if err != nil {
		return err
	}
	defer ws.close()
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if o.Engine.Input == "video" {
		return wholeVideo(ctx, o, ws, in, out, info, report)
	}
	return frameVideo(ctx, o, ws, in, out, info, report)
}

// Frames of a video go through the engine a segment at a time, read
// from one FFmpeg decode and encoded as they're done, so a long video
// never has more than a segment's frames on disk.

// segmentBytes is about how much disk a segment's upscaled frames may
// take; segments are between minSegment and maxSegment frames. Small
// segments keep the work folder to a few hundred megabytes, which the
// system mostly holds in memory, since each file lives only seconds.
// Engines that stay running between jobs (Manifest.Serve) lose nothing
// to them; others start again for each segment.
var segmentBytes int64 = 512 << 20

const minSegment, maxSegment = 24, 96

// contextFrames are the last frames of a segment that engines taking a
// folder see again at the start of the next, so ones that follow motion
// (FlashVSR) carry it over; their results are dropped.
const contextFrames = 8

// segmentFrames is how many frames of w×h results fit in segmentBytes,
// as PNGs at light compression.
func segmentFrames(w, h int) int {
	per := max(1, int64(w)*int64(h)*3*3/4)
	return int(min(maxSegment, max(minSegment, segmentBytes/per)))
}

// frameVideo upscales the video frame by frame, a segment at a time, and
// encodes the results with the original audio as they come.
func frameVideo(ctx context.Context, o Options, ws *workspace, in, out string, info media.Info, report Reporter) error {
	reader, err := o.Tools.ReadFrames(ctx, in)
	if err != nil {
		return err
	}
	defer reader.Close()
	sz := o.size(info.Width, info.Height)
	join := media.JoinOptions{
		FPS: info.FPS, FrameExt: o.Engine.OutputExt, Codec: o.Settings.VideoCodec, Quality: o.Settings.VideoQuality,
		AudioSrc: in,
	}
	if sz.Exact {
		join.Width, join.Height = sz.Width, sz.Height
	}
	writer, err := o.Tools.WriteFrames(ctx, out, join)
	if err != nil {
		return err
	}
	finished := false
	defer func() {
		if !finished {
			writer.Abort()
			os.Remove(out)
		}
	}()

	total := info.Frames
	if total <= 0 {
		total = int(math.Round(info.Duration * info.FPS))
	}
	total = max(1, total)
	frac := func(done float64) float64 { return 0.02 + 0.95*min(1, done/float64(total)) }
	seg := segmentFrames(sz.Width, sz.Height)
	keep := 0
	if o.Engine.Batch {
		keep = contextFrames
	}
	stage := tr.T("Upscaling frames")
	var carry [][]byte // the last frames read, for the next segment
	done := 0
	for ended := false; !ended; {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, d := range []string{ws.in, ws.out} {
			if err := emptyDir(d); err != nil {
				return err
			}
		}
		var names []string
		add := func(data []byte) error {
			name := fmt.Sprintf("%08d.png", len(names)+1)
			names = append(names, name)
			return os.WriteFile(filepath.Join(ws.in, name), data, 0o644)
		}
		for _, data := range carry {
			if err := add(data); err != nil {
				return err
			}
		}
		skip := len(carry)
		for len(names)-skip < seg {
			data, err := reader.Next()
			if err == io.EOF {
				ended = true
				break
			}
			if err != nil {
				return err
			}
			if err := add(data); err != nil {
				return err
			}
			if carry = append(carry, data); len(carry) > keep {
				carry = carry[len(carry)-keep:]
			}
		}
		fresh := names[skip:]
		if len(fresh) == 0 {
			break
		}
		from, to := frac(float64(done)), frac(float64(done+len(fresh)))
		if err := o.upscaleFrames(ctx, ws, names, fresh, info, report, stage, from, to); err != nil {
			return err
		}
		for _, name := range fresh {
			result := filepath.Join(ws.out, strings.TrimSuffix(name, ".png")+o.Engine.OutputExt)
			data, err := os.ReadFile(result)
			if err != nil {
				return fmt.Errorf("%s: %w", tr.T("the engine skipped a frame"), err)
			}
			if err := writer.Write(data); err != nil {
				return err
			}
		}
		done += len(fresh)
		report(Update{Stage: stage, Frac: to})
	}
	if done == 0 {
		return errors.New(tr.T("no frames could be read from the video"))
	}
	report(Update{Stage: tr.T("Encoding"), Frac: 0.98})
	finished = true
	return writer.Close()
}

// upscaleFrames runs the engine on the frames names in ws.in, writing
// the results of fresh, the ones not seen before, into ws.out under the
// same names: in one run for engines that take a folder, otherwise
// frame by frame.
func (o Options) upscaleFrames(ctx context.Context, ws *workspace, names, fresh []string, info media.Info, report Reporter, stage string, from, to float64) error {
	if o.Engine.Batch {
		// One run over the folder; follow progress by counting results.
		stop := watchFolder(ws.out, o.Engine.OutputExt, len(names), func(f float64) {
			report(Update{Stage: stage, Frac: from + (to-from)*f})
		})
		j := o.sized(o.job(ws.in, ws.out, ws.in, ws.out, report, stage, from, to), info.Width, info.Height)
		j.Progress = nil // per-frame percentages would jump around
		_, err := o.Engine.Run(ctx, j)
		stop()
		return err
	}
	// Engines that name their own output write into frameDir; the result
	// is moved in line with the other frames.
	frameDir := ws.path("frame-out")
	if err := os.MkdirAll(frameDir, 0o755); err != nil {
		return err
	}
	for i, name := range fresh {
		if err := ctx.Err(); err != nil {
			return err
		}
		frame := filepath.Join(ws.in, name)
		target := filepath.Join(ws.out, strings.TrimSuffix(name, ".png")+o.Engine.OutputExt)
		j := o.sized(o.job(frame, target, ws.in, frameDir, report, stage, from, to), info.Width, info.Height)
		j.Progress = nil
		result, err := o.Engine.Run(ctx, j)
		if err != nil {
			return err
		}
		if result != target {
			if err := os.Rename(result, target); err != nil {
				return err
			}
		}
		report(Update{Stage: stage, Frac: from + (to-from)*float64(i+1)/float64(len(fresh))})
	}
	return nil
}

// emptyDir deletes what's in dir.
func emptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// wholeVideo hands the file to a video engine, then copies the source's
// audio into the result.
func wholeVideo(ctx context.Context, o Options, ws *workspace, in, out string, info media.Info, report Reporter) error {
	src := filepath.Join(ws.in, "source"+filepath.Ext(in))
	if err := os.Link(in, src); err != nil {
		if err := copyInto(in, src); err != nil {
			return err
		}
	}
	result, err := o.Engine.Run(ctx, o.sized(o.job(src, filepath.Join(ws.out, "source"+o.Engine.OutputExt),
		ws.in, ws.out, report, tr.T("Upscaling"), 0.02, 0.92), info.Width, info.Height))
	if err != nil {
		return err
	}
	report(Update{Stage: tr.T("Adding audio"), Frac: 0.93})
	// An engine told a target size can still land a few pixels off it,
	// or one with a fixed scale far from it: then it's resized, which
	// means encoding the picture again.
	if sz := o.size(info.Width, info.Height); sz.Exact {
		if got, err := o.Tools.Probe(ctx, result); err == nil && (got.Width != sz.Width || got.Height != sz.Height) {
			report(Update{Stage: tr.T("Resizing to {size}", "size", fmt.Sprintf("%d × %d", sz.Width, sz.Height)), Frac: 0.93})
			return o.Tools.RemuxSized(ctx, result, in, out, sz.Width, sz.Height)
		}
	}
	return o.Tools.Remux(ctx, result, in, out)
}

// watchFolder reports the share of total files with extension ext that
// have appeared in dir, until the returned stop is called: progress for
// engines given a folder, whose own counts restart with every file.
func watchFolder(dir, ext string, total int, progress func(float64)) (stop func()) {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(400 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				found, _ := filepath.Glob(filepath.Join(dir, "*"+ext))
				progress(float64(len(found)) / float64(max(1, total)))
			}
		}
	}()
	return func() { close(done) }
}
