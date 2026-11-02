// Package format turns numbers into the short strings the UI shows, in
// the user's language.
package format

import (
	"fmt"
	"math"
	"time"

	"github.com/mehran9675/touchup/internal/tr"
)

// Clock formats seconds as m:ss.cc, or h:mm:ss for long media, as
// timecode is written everywhere.
func Clock(sec float64) string {
	sec = max(0, sec)
	if sec >= 3600 {
		h := int(sec) / 3600
		return fmt.Sprintf("%d:%02d:%02d", h, int(sec)%3600/60, int(sec)%60)
	}
	m := int(sec) / 60
	s := sec - float64(m*60)
	return fmt.Sprintf("%d:%05.2f", m, s)
}

// Duration formats seconds compactly: "42s", "3m 05s", "1h 02m".
func Duration(sec float64) string {
	d := time.Duration(sec * float64(time.Second)).Round(time.Second)
	switch {
	case d < time.Minute:
		return tr.T("{s}s", "s", int(d.Seconds()))
	case d < time.Hour:
		return tr.T("{m}m {s}s", "m", int(d.Minutes()), "s", fmt.Sprintf("%02d", int(d.Seconds())%60))
	}
	return tr.T("{h}h {m}m", "h", int(d.Hours()), "m", fmt.Sprintf("%02d", int(d.Minutes())%60))
}

// Bytes formats a size: "12.4 MB".
func Bytes(n int64) string {
	if n < 1024 {
		return tr.T("{n} B", "n", n)
	}
	v := float64(n)
	i := -1
	for v >= 1024 && i < 3 {
		v /= 1024
		i++
	}
	n1 := tr.Decimal(v, 1)
	switch i {
	case 0:
		return tr.T("{n} KB", "n", n1)
	case 1:
		return tr.T("{n} MB", "n", n1)
	case 2:
		return tr.T("{n} GB", "n", n1)
	}
	return tr.T("{n} TB", "n", n1)
}

// Rate formats a sample rate: "44.1 kHz".
func Rate(hz int) string {
	if hz <= 0 {
		return "—"
	}
	k := float64(hz) / 1000
	if k == math.Trunc(k) {
		return tr.T("{n} kHz", "n", int(k))
	}
	return tr.T("{n} kHz", "n", tr.Decimal(k, 1))
}

// Resolution formats a size: "1920 × 1080".
func Resolution(w, h int) string { return fmt.Sprintf("%d × %d", w, h) }

// FPS formats a frame rate: "23.98 fps", "30 fps".
func FPS(f float64) string {
	if f == math.Trunc(f) {
		return tr.T("{n} fps", "n", int(f))
	}
	return tr.T("{n} fps", "n", tr.Decimal(f, 2))
}

// Channels names a channel count.
func Channels(n int) string {
	switch n {
	case 1:
		return tr.T("Mono")
	case 2:
		return tr.T("Stereo")
	case 6:
		return "5.1"
	case 8:
		return "7.1"
	}
	return tr.T("{n} channels", "n", n)
}

// Elapsed formats how long something took or has been running.
func Elapsed(d time.Duration) string { return Duration(d.Seconds()) }

// Remaining formats an estimate of the time left, to the minute: "1 min",
// "45 min", "3h 20m".
func Remaining(d time.Duration) string {
	m := max(1, int(d.Round(time.Minute).Minutes()))
	if m < 60 {
		return tr.T("{m} min", "m", m)
	}
	return tr.T("{h}h {m}m", "h", m/60, "m", fmt.Sprintf("%02d", m%60))
}

// Count formats a count with the language's thousands separators:
// "35,787".
func Count(n int) string { return tr.Int(n) }

// Percent formats a fraction as a percentage with digits decimals:
// "2.4%".
func Percent(f float64, digits int) string {
	return tr.T("{n}%", "n", tr.Decimal(f*100, digits))
}
