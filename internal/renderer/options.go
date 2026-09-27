package renderer

import "github.com/sh4869221b/azerlay/internal/config"

type Options struct {
	Scale           float64
	Opacity         float64
	Mode            string
	ShowProfileName bool
	ShowStatus      bool
	ShowUnbound     bool
	ShowAmbiguous   bool
	Theme           string
	FontScale       float64
	HighContrast    bool
}

func NewOptions(overlay config.Overlay, appearance config.Appearance, showAmbiguous bool) Options {
	return Options{
		Scale:           overlay.Scale,
		Opacity:         overlay.Opacity,
		Mode:            overlay.Mode,
		ShowProfileName: overlay.ShowProfileName,
		ShowStatus:      overlay.ShowStatus,
		ShowUnbound:     overlay.ShowUnbound,
		ShowAmbiguous:   showAmbiguous,
		Theme:           appearance.Theme,
		FontScale:       appearance.FontScale,
		HighContrast:    appearance.HighContrast,
	}
}
