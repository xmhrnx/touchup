package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// frameVideo splits the video into frames, upscales them and encodes
// them back with the original audio.
func frameVideo(ctx context.Context, o Options, ws *workspace, in, out string, info media.Info, report Reporter) error {
	if err := o.Tools.SplitFrames(ctx, in, ws.in, info.Duration, func(f float64) {
		report(Update{Stage: tr.T("Extracting frames"), Frac: 0.08 * f})
	}); err != nil {
		return err
	}
	frames, _ := filepath.Glob(filepath.Join(ws.in, "*.png"))
	if len(frames) == 0 {
		return errors.New(tr.T("no frames could be read from the video"))
	}
	total := float64(len(frames))
	const from, to = 0.08, 0.9
	stage := tr.T("Upscaling frames")

	if o.Engine.Batch {
		// One run over the folder; follow progress by counting results.
		stop := watchFolder(ws.out, o.Engine.OutputExt, len(frames), func(f float64) {
			report(Update{Stage: stage, Frac: from + (to-from)*f})
		})
		j := o.sized(o.job(ws.in, ws.out, ws.in, ws.out, report, stage, from, to), info.Width, info.Height)
		j.Progress = nil // per-frame percentages would jump around
		_, err := o.Engine.Run(ctx, j)
		stop()
		if err != nil {
			return err
		}
	} else {
		// Engines that name their own output write into frameDir; the
		// result is moved in line with the other frames.
		frameDir := ws.path("frame-out")
		if err := os.MkdirAll(frameDir, 0o755); err != nil {
			return err
		}
		for i, frame := range frames {
			if err := ctx.Err(); err != nil {
				return err
			}
			name := filepath.Base(frame)
			target := filepath.Join(ws.out, name[:len(name)-len(filepath.Ext(name))]+o.Engine.OutputExt)
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
			os.Remove(frame) // keep disk use in check on long videos
			report(Update{Stage: stage, Frac: from + (to-from)*float64(i+1)/total})
		}
	}
	join := media.JoinOptions{
		FPS: info.FPS, FrameExt: o.Engine.OutputExt, Codec: o.Settings.VideoCodec, Quality: o.Settings.VideoQuality,
		AudioSrc: in, Duration: info.Duration,
	}
	if sz := o.size(info.Width, info.Height); sz.Exact {
		join.Width, join.Height = sz.Width, sz.Height
	}
	return o.Tools.JoinFrames(ctx, ws.out, out, join, func(f float64) {
		report(Update{Stage: tr.T("Encoding"), Frac: to + (1-to)*f})
	})
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
