package pipeline

import (
	"context"
	"os"
	"path/filepath"

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
	report(Update{Stage: tr.T("Upscaling"), Frac: 0.05})
	result, err := o.picture(ctx, ws, src, info.Width, info.Height, report, tr.T("Upscaling"), 0.05, 0.95)
	if err != nil {
		return err
	}
	report(Update{Stage: tr.T("Saving"), Frac: 0.96})
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return o.Tools.ConvertImage(ctx, result, out)
}
