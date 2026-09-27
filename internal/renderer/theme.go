package renderer

import "github.com/diamondburned/gotk4/pkg/cairo"

type color struct{ red, green, blue float64 }

func rgb(value uint32) color {
	return color{float64(value>>16&0xff) / 255, float64(value>>8&0xff) / 255, float64(value&0xff) / 255}
}

func (c color) set(cr *cairo.Context) { cr.SetSourceRGB(c.red, c.green, c.blue) }

type palette struct {
	background, idle, pressed, unbound, unknown             color
	primary, secondary, outline, statusError, statusWarning color
	strokeWidth                                             float64
}

func themeFor(options Options) palette {
	var colors palette
	if options.Theme == "light" {
		colors = palette{
			background: rgb(0xf8fafc), idle: rgb(0xe2e8f0), pressed: rgb(0xbfdbfe),
			unbound: rgb(0xf1f5f9), unknown: rgb(0xcbd5e1), primary: rgb(0x111827),
			secondary: rgb(0x374151), outline: rgb(0x475569), statusError: rgb(0x991b1b),
			statusWarning: rgb(0x854d0e), strokeWidth: 2,
		}
	} else {
		colors = palette{
			background: rgb(0x111827), idle: rgb(0x273449), pressed: rgb(0x1d4ed8),
			unbound: rgb(0x1f2937), unknown: rgb(0x374151), primary: rgb(0xf9fafb),
			secondary: rgb(0xd1d5db), outline: rgb(0x9ca3af), statusError: rgb(0xfca5a5),
			statusWarning: rgb(0xfde68a), strokeWidth: 2,
		}
	}
	if options.HighContrast {
		colors.strokeWidth = 3
		if options.Theme == "light" {
			colors.background = rgb(0xffffff)
			colors.primary = rgb(0x000000)
			colors.secondary = colors.primary
			colors.outline = colors.primary
			colors.pressed = rgb(0xbae6fd)
		} else {
			colors.background = rgb(0x000000)
			colors.primary = rgb(0xffffff)
			colors.secondary = colors.primary
			colors.outline = colors.primary
			colors.pressed = rgb(0x005a9c)
		}
	}
	return colors
}
