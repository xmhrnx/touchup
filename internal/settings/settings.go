// Package settings holds the user's preferences and saves them as JSON.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mehran9675/touchup/internal/paths"
)

// Settings are the user's preferences. Empty tool paths mean "find it on
// PATH".
type Settings struct {
	FFmpeg  string `json:"ffmpeg,omitempty"`
	FFprobe string `json:"ffprobe,omitempty"`
	FFplay  string `json:"ffplay,omitempty"`
	Python  string `json:"python,omitempty"`

	// OutputDir is where results are written; empty writes them next
	// to the source file.
	OutputDir string `json:"outputDir,omitempty"`
	// GPU is the device index passed to engines; -1 lets them choose.
	GPU int `json:"gpu"`
	// AllowSystemMemory lets engines whose GPU runs out of memory carry
	// on in system memory, as Windows drivers allow, many times slower.
	// Off, they fail with "out of memory" instead.
	AllowSystemMemory bool `json:"allowSystemMemory"`

	ImageFormat  string `json:"imageFormat"`  // png, jpg, webp
	AudioFormat  string `json:"audioFormat"`  // wav, flac, mp3
	VideoCodec   string `json:"videoCodec"`   // h264, h265, prores
	VideoQuality int    `json:"videoQuality"` // CRF, lower is better
	KeepWork     bool   `json:"keepWork"`     // keep frames and clips after a job
	LightMode    bool   `json:"lightMode"`
	// Language is the UI's language ("de"), or empty for the system's.
	Language string `json:"language,omitempty"`

	// Choices remembers the engine, model and scale last used for each
	// kind of media.
	Choices map[string]Choice `json:"choices,omitempty"`
	// EngineOptions are the user's values for engines' options, by
	// engine ID and option ID. Options not here use their defaults.
	EngineOptions map[string]map[string]string `json:"engineOptions,omitempty"`
}

// Choice is an engine, model and size picked for a kind of media: a
// scale, or a target resolution (the short side in pixels) instead.
type Choice struct {
	Engine string `json:"engine"`
	Model  string `json:"model,omitempty"`
	Scale  int    `json:"scale,omitempty"`
	Target int    `json:"target,omitempty"`
}

// Default returns the settings a fresh install starts with.
func Default() Settings {
	return Settings{
		GPU:          -1,
		ImageFormat:  "png",
		AudioFormat:  "wav",
		VideoCodec:   "h264",
		VideoQuality: 16,
	}
}

// Load reads the settings file, falling back to the defaults for a
// missing or unreadable file.
func Load() Settings {
	s := Default()
	data, err := os.ReadFile(paths.Settings())
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s)
	return s
}

// Save writes the settings file.
func Save(s Settings) error {
	if err := os.MkdirAll(filepath.Dir(paths.Settings()), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(paths.Settings(), data, 0o644)
}
