package store

import (
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/mehran9675/giode"
)

// ErrorReport is the whole of an error as plain text to copy into a bug
// report: what failed, why, the computer and setup it ran on, and
// everything the engine or installer printed that TouchUp kept. The
// title and summary are in the user's language; the rest stays in
// English, for whoever reads the report.
func (s *Store) ErrorReport(title, summary, log string) string {
	var b strings.Builder
	b.WriteString(title + "\n")
	if summary != "" {
		b.WriteString(summary + "\n")
	}
	version := giode.AppVersion
	if version == "" {
		version = "development build"
	}
	if rev := revision(); rev != "" {
		version += " (" + rev + ")"
	}
	b.WriteString("\nTouchUp " + version + " on " + runtime.GOOS + "/" + runtime.GOARCH + "\n")
	gpu := "automatic"
	if s.Settings.GPU >= 0 {
		gpu = strconv.Itoa(s.Settings.GPU)
	}
	b.WriteString("GPU setting: " + gpu + "\n")
	if s.Python != "" {
		b.WriteString("Python: " + s.Python + "\n")
	}
	if log != "" {
		b.WriteString("\nOutput:\n" + log + "\n")
	}
	return b.String()
}

// revision is the commit TouchUp was built from, marked "+changes" when
// the build had uncommitted ones, so reports tell builds apart; empty
// when Go didn't record it.
func revision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	var rev, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if rev != "" && modified == "true" {
		rev += "+changes"
	}
	return rev
}
