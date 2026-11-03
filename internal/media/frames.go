package media

import (
	"context"
	"slices"
	"strconv"
	"strings"
)

// framePattern is the file name pattern of split frames.
const framePattern = "%08d"

// JoinOptions controls how frames are encoded back into a video.
type JoinOptions struct {
	FPS      float64
	FrameExt string // extension of the frames, ".png" by default
	// RawWidth and RawHeight, when set, mean the frames are packed 8-bit
	// RGB of that size instead of image files.
	RawWidth, RawHeight int
	Codec               string // h264, h265 or prores
	Quality             int    // CRF for h264/h265
	// Fast trades file size for speed, for files TouchUp reads again.
	Fast     bool
	AudioSrc string // file whose audio is copied in; empty for none
	// ConvertAudio encodes the audio as AAC instead of copying it.
	ConvertAudio bool
	Duration     float64
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
		args = append(args, "-i", o.AudioSrc, "-map", "0:v:0", "-map", "1:a?")
		if o.ConvertAudio {
			args = append(args, "-c:a", "aac", "-b:a", "256k")
		} else {
			args = append(args, "-c:a", "copy")
		}
	}
	if o.Width > 0 && o.Height > 0 {
		args = append(args, "-vf", scaleFilter(o.Width, o.Height))
	}
	preset := "slow"
	if o.Fast {
		preset = "veryfast"
	}
	switch o.Codec {
	case "h265":
		args = append(args, "-c:v", "libx265", "-crf", strconv.Itoa(o.Quality), "-preset", preset,
			"-pix_fmt", "yuv420p10le", "-tag:v", "hvc1")
	case "prores":
		args = append(args, "-c:v", "prores_ks", "-profile:v", "3", "-pix_fmt", "yuv422p10le")
	default:
		args = append(args, "-c:v", "libx264", "-crf", strconv.Itoa(o.Quality), "-preset", preset,
			"-pix_fmt", "yuv420p")
	}
	return args
}

// OutputExt returns the container extension for a video in codec whose
// audio, in audioCodec, is copied: the codec's own, or Matroska when
// the audio can't go in it. With convertAudio the audio becomes AAC,
// which goes anywhere.
func OutputExt(codec, audioCodec string, convertAudio bool) string {
	ext := VideoExt(codec)
	if convertAudio || audioCodec == "" || slices.Contains(audioFits[ext], audioCodec) ||
		ext == ".mov" && strings.HasPrefix(audioCodec, "pcm_") {
		return ext
	}
	return ".mkv"
}

// audioFits lists the audio codecs each container takes as they are.
var audioFits = map[string][]string{
	".mp4": {"aac", "mp3", "ac3", "eac3", "alac", "opus", "flac"},
	".mov": {"aac", "mp3", "ac3", "eac3", "alac"},
}

// VideoExt returns the container extension for a codec.
func VideoExt(codec string) string {
	if codec == "prores" {
		return ".mov"
	}
	return ".mp4"
}

// Remux writes video's picture with audioSrc's sound to out, copying
// both streams when the container allows it, re-encoding the picture as
// H.264 when it doesn't, and converting the sound to AAC as a last
// resort. With convertAudio the sound is always AAC.
func (t Tools) Remux(ctx context.Context, video, audioSrc, out string, convertAudio bool) error {
	audio := []string{"-c:a", "copy"}
	if convertAudio {
		audio = []string{"-c:a", "aac", "-b:a", "256k"}
	}
	args := append([]string{"-i", video, "-i", audioSrc, "-map", "0:v:0", "-map", "1:a?", "-c:v", "copy"}, audio...)
	err := t.ffmpeg(ctx, 0, nil, append(args, "-shortest", out)...)
	if err == nil || ctx.Err() != nil {
		return err
	}
	if err = t.reencode(ctx, video, audioSrc, out, audio...); err == nil || ctx.Err() != nil || convertAudio {
		return err
	}
	return t.reencode(ctx, video, audioSrc, out, "-c:a", "aac", "-b:a", "256k")
}

// RemuxSized is Remux that also resizes the picture to w×h, which means
// encoding it again.
func (t Tools) RemuxSized(ctx context.Context, video, audioSrc, out string, w, h int, convertAudio bool) error {
	if !convertAudio {
		err := t.reencode(ctx, video, audioSrc, out, "-vf", scaleFilter(w, h), "-c:a", "copy")
		if err == nil || ctx.Err() != nil {
			return err
		}
	}
	return t.reencode(ctx, video, audioSrc, out, "-vf", scaleFilter(w, h), "-c:a", "aac", "-b:a", "256k")
}

func (t Tools) reencode(ctx context.Context, video, audioSrc, out string, extra ...string) error {
	args := []string{"-i", video, "-i", audioSrc, "-map", "0:v:0", "-map", "1:a?",
		"-c:v", "libx264", "-crf", "14", "-preset", "slow", "-pix_fmt", "yuv420p"}
	args = append(append(args, extra...), "-shortest", out)
	return t.ffmpeg(ctx, 0, nil, args...)
}
