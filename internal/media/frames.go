package media

import (
	"context"
	"strconv"
)

// framePattern is the file name pattern of split frames.
const framePattern = "%08d"

// JoinOptions controls how frames are encoded back into a video.
type JoinOptions struct {
	FPS      float64
	FrameExt string // extension of the frames, ".png" by default
	Codec    string // h264, h265 or prores
	Quality  int    // CRF for h264/h265
	AudioSrc string // file whose audio is copied in; empty for none
	Duration float64
	// Width and Height resize the frames, when both are set.
	Width, Height int
}

// scaleFilter resizes to w×h with Lanczos filtering.
func scaleFilter(w, h int) string {
	return "scale=" + strconv.Itoa(w) + ":" + strconv.Itoa(h) + ":flags=lanczos"
}

// encodeArgs are the arguments after the frames' input: the audio, the
// size and the codec.
func encodeArgs(o JoinOptions) []string {
	var args []string
	if o.AudioSrc != "" {
		args = append(args, "-i", o.AudioSrc, "-map", "0:v:0", "-map", "1:a?", "-c:a", "copy")
	}
	if o.Width > 0 && o.Height > 0 {
		args = append(args, "-vf", scaleFilter(o.Width, o.Height))
	}
	switch o.Codec {
	case "h265":
		args = append(args, "-c:v", "libx265", "-crf", strconv.Itoa(o.Quality), "-preset", "slow",
			"-pix_fmt", "yuv420p10le", "-tag:v", "hvc1")
	case "prores":
		args = append(args, "-c:v", "prores_ks", "-profile:v", "3", "-pix_fmt", "yuv422p10le")
	default:
		args = append(args, "-c:v", "libx264", "-crf", strconv.Itoa(o.Quality), "-preset", "slow",
			"-pix_fmt", "yuv420p")
	}
	return args
}

// VideoExt returns the container extension for a codec.
func VideoExt(codec string) string {
	if codec == "prores" {
		return ".mov"
	}
	return ".mp4"
}

// Remux writes video's picture with audioSrc's sound to out, copying
// both streams when the container allows it and re-encoding the picture
// as H.264 when it doesn't.
func (t Tools) Remux(ctx context.Context, video, audioSrc, out string) error {
	err := t.ffmpeg(ctx, 0, nil, "-i", video, "-i", audioSrc, "-map", "0:v:0", "-map", "1:a?",
		"-c", "copy", "-shortest", out)
	if err == nil || ctx.Err() != nil {
		return err
	}
	return t.reencode(ctx, video, audioSrc, out, "-c:a", "aac", "-b:a", "256k")
}

// RemuxSized is Remux that also resizes the picture to w×h, which means
// encoding it again; the sound is still copied.
func (t Tools) RemuxSized(ctx context.Context, video, audioSrc, out string, w, h int) error {
	err := t.reencode(ctx, video, audioSrc, out, "-vf", scaleFilter(w, h), "-c:a", "copy")
	if err == nil || ctx.Err() != nil {
		return err
	}
	return t.reencode(ctx, video, audioSrc, out, "-vf", scaleFilter(w, h), "-c:a", "aac", "-b:a", "256k")
}

func (t Tools) reencode(ctx context.Context, video, audioSrc, out string, extra ...string) error {
	args := []string{"-i", video, "-i", audioSrc, "-map", "0:v:0", "-map", "1:a?",
		"-c:v", "libx264", "-crf", "14", "-preset", "slow", "-pix_fmt", "yuv420p"}
	args = append(append(args, extra...), "-shortest", out)
	return t.ffmpeg(ctx, 0, nil, args...)
}
