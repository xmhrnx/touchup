package store

import (
	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/filters"
	"github.com/mehran9675/touchup/internal/settings"
)

// UpdateSettings changes the settings, saves them and applies them.
func (s *Store) UpdateSettings(change func(*settings.Settings)) {
	change(&s.Settings)
	s.saveSettings()
	s.locateTools()
	s.refreshInstalled()
	if s.Settings.LightMode {
		giode.SetMode(giode.ModeLight)
	} else {
		giode.SetMode(giode.ModeDark)
	}
}

func (s *Store) saveSettings() {
	if err := settings.Save(s.Settings); err != nil {
		s.fail(err)
	}
}

// Filters are the filters set for images and videos.
func (s *Store) Filters() filters.Settings { return s.Settings.Filters }

// SetFilters changes the filters, and with save writes them to disk;
// sliders change them many times a second and save when they let go.
func (s *Store) SetFilters(change func(*filters.Settings), save bool) {
	change(&s.Settings.Filters)
	if save {
		s.saveSettings()
	}
}
