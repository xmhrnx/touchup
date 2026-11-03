package pipeline

import (
	"context"
	"os"
	"path/filepath"

	"github.com/mehran9675/touchup/internal/filters"
	"github.com/mehran9675/touchup/internal/tr"
)

// Image upscales the image at in and writes it to out.
func Image(ctx context.Context, o Options, in, out string, report Reporter) error {
	if err := o.check(); err != nil {
		return err
	}
	ws, err := newWorkspace("image", o.Settings.KeepWork)
	if err != nil {
		return err
	}
	defer ws.close()

	report(Update{Stage: tr.T("Preparing"), Frac: 0.02})
	src := filepath.Join(ws.in, "source.png")
	if err := o.Tools.ConvertImage(ctx, in, src); err != nil {
		return err
	}
	info, err := o.Tools.Probe(ctx, src)
	if err != nil {
		return err
	}
	fl := o.Settings.Filters
	if fl.HasCleanup() {
		report(Update{Stage: tr.T("Cleaning up"), Frac: 0.04})
		if err := filterFile(src, src, func(m *filters.Image) { fl.Cleanup(m, 0) }); err != nil {
			return err
		}
	}
	report(Update{Stage: tr.T("Upscaling"), Frac: 0.05})
	result, err := o.picture(ctx, ws, src, info.Width, info.Height, report, tr.T("Upscaling"), 0.05, 0.9)
	if err != nil {
		return err
	}
	if fl.HasLook() {
		report(Update{Stage: tr.T("Applying filters"), Frac: 0.92})
		looked := filepath.Join(ws.out, "filtered.png")
		if err := filterFile(result, looked, func(m *filters.Image) { fl.Look(m, 0) }); err != nil {
			return err
		}
		result = looked
	}
	report(Update{Stage: tr.T("Saving"), Frac: 0.96})
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return o.Tools.ConvertImage(ctx, result, out)
}
