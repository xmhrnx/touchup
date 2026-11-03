package media

import "testing"

func TestOutputExt(t *testing.T) {
	for _, c := range []struct {
		codec, audio string
		convert      bool
		want         string
	}{
		{"h264", "aac", false, ".mp4"},
		{"h264", "pcm_s16le", false, ".mkv"},
		{"h264", "pcm_s16le", true, ".mp4"},
		{"h265", "vorbis", false, ".mkv"},
		{"h264", "", false, ".mp4"},
		{"prores", "pcm_s24le", false, ".mov"},
		{"prores", "opus", false, ".mkv"},
	} {
		if got := OutputExt(c.codec, c.audio, c.convert); got != c.want {
			t.Errorf("OutputExt(%q, %q, %v) = %q, want %q", c.codec, c.audio, c.convert, got, c.want)
		}
	}
}
