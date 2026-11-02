// Command touchup is a desktop app for upscaling images, video and audio
// with AI engines that run locally.
package main

import (
	"image"
	"log"
	"net/url"
	"os"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/i18n"

	"github.com/mehran9675/touchup/assets"
	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/pipeline"
	"github.com/mehran9675/touchup/internal/settings"
	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/shell"
	"github.com/mehran9675/touchup/internal/ui/store"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

func main() {
	if err := theme.LoadFonts(); err != nil {
		log.Fatal(err)
	}
	if err := theme.LoadIcons(); err != nil {
		log.Fatal(err)
	}
	pipeline.ClearWork()

	// Translations first: the store can show messages as it starts.
	intl := &giode.IntlOptions{FS: assets.Locales, Dir: "locales", Fallback: "en"}
	if err := i18n.Load(intl.FS, intl.Dir); err != nil {
		log.Print(err)
	}
	i18n.SetFallback(intl.Fallback)
	tr.Use(settings.Load().Language)

	toasts := giode.Toaster().Placement(giode.ToastBottomRight).Max(4)
	s := store.New(toasts)
	mode := giode.ModeDark
	if s.Settings.LightMode {
		mode = giode.ModeLight
	}
	theme.Install(mode)

	app := giode.New(giode.Config{
		Title:      "TouchUp",
		Size:       image.Pt(1360, 860),
		MinSize:    image.Pt(1080, 700),
		Resizable:  true,
		Frameless:  shell.CustomFrame(),
		Center:     true,
		Background: theme.Background,
		Icon:       assets.AppIcon,
		Locale:     tr.Resolve(s.Settings.Language),
		Intl:       intl,
		OnFileDrop: func(paths []string) {
			if len(paths) > 0 {
				s.Open(paths[0])
			}
		},
		SingleInstanceLock: &giode.SingleInstanceLock{
			UniqueID: "com.mehran9675.touchup",
			// A file opened with TouchUp while it runs comes here.
			OnLaunchArgs: func(args []string) {
				if len(args) > 0 {
					s.Open(args[len(args)-1])
				}
			},
		},
		// macOS hands files opened with the app over as file:// URLs.
		OnOpenURL: func(u *url.URL) {
			if u.Scheme == "file" {
				s.Open(u.Path)
			}
		},
		OnBeforeClose: s.Shutdown,
		OnShutdown: func() {
			engine.StopServers() // they'd hold the GPU and the work folder
			pipeline.ClearWork()
		},
	})
	s.Attach(app)
	if len(os.Args) > 1 {
		s.Open(os.Args[len(os.Args)-1])
	}

	dragging := func() bool {
		_, on := app.FileDrag()
		return on
	}
	// Components resolve some colors and labels when they're built, so
	// the UI is rebuilt when the color mode or the language changes. Its
	// state lives in the store.
	var root *shell.Shell
	var builtMode giode.Mode
	var builtLocale string
	if err := app.Run(func() giode.Element {
		if root == nil || giode.CurrentMode() != builtMode || i18n.Locale() != builtLocale {
			root, builtMode, builtLocale = shell.New(s, dragging), giode.CurrentMode(), i18n.Locale()
		}
		return root.View()
	}); err != nil {
		log.Fatal(err)
	}
}
