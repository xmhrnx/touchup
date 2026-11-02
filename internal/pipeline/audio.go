package pipeline

import (
	"context"
	"os"
	"path/filepath"

	"github.com/mehran9675/touchup/internal/tr"
)

// Audio upscales the audio at in and writes it to out.
func Audio(ctx context.Context, o Options, in, out string, duration float64, report Reporter) error {
	if err := o.check(); err != nil {
		return err
	}
	ws, err := newWorkspace("audio", o.Settings.KeepWork)
	if err != nil {
		return err
	}
	defer ws.close()

	src := filepath.Join(ws.in, "source.wav")
	if err := o.Tools.AudioClip(ctx, in, 0, 0, o.Engine.InputRate, src, func(f float64) {
		report(Update{Stage: tr.T("Decoding"), Frac: 0.05 * f})
	}); err != nil {
		return err
	}
	report(Update{Stage: tr.T("Enhancing"), Frac: 0.05})
	result, err := o.Engine.Run(ctx, o.job(src, filepath.Join(ws.out, "source"+o.Engine.OutputExt),
		ws.in, ws.out, report, tr.T("Enhancing"), 0.05, 0.95))
	if err != nil {
		return err
	}
	report(Update{Stage: tr.T("Saving"), Frac: 0.96})
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return o.Tools.ConvertAudio(ctx, result, out)
}
