package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// blueGreyTheme is the Pomodoro GUI's look: a blue accent over a
// neutral grey surface, in both the light and dark variant. Fonts,
// icons and sizes are left to Fyne's default theme — only the
// palette changes.
type blueGreyTheme struct{}

var _ fyne.Theme = (*blueGreyTheme)(nil)

// NewTheme returns the app's custom blue/grey theme.
func NewTheme() fyne.Theme {
	return &blueGreyTheme{}
}

func (blueGreyTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if variant == theme.VariantDark {
		switch name {
		case theme.ColorNameBackground:
			return color.NRGBA{R: 0x1c, G: 0x22, B: 0x2b, A: 0xff} // slate-900
		case theme.ColorNameForeground:
			return color.NRGBA{R: 0xe2, G: 0xe8, B: 0xf0, A: 0xff} // slate-200
		case theme.ColorNameButton, theme.ColorNameInputBackground:
			return color.NRGBA{R: 0x2a, G: 0x33, B: 0x40, A: 0xff} // slate-800
		case theme.ColorNameInputBorder, theme.ColorNameSeparator:
			return color.NRGBA{R: 0x3d, G: 0x49, B: 0x59, A: 0xff} // slate-700
		case theme.ColorNameDisabled:
			return color.NRGBA{R: 0x5a, G: 0x66, B: 0x76, A: 0xff}
		case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
			return color.NRGBA{R: 0x24, G: 0x2b, B: 0x35, A: 0xff}
		case theme.ColorNameHover:
			return color.NRGBA{R: 0x33, G: 0x60, B: 0x9c, A: 0x33}
		case theme.ColorNameSelection:
			return color.NRGBA{R: 0x3b, G: 0x82, B: 0xf6, A: 0x55}
		}
	} else {
		switch name {
		case theme.ColorNameBackground:
			return color.NRGBA{R: 0xf1, G: 0xf4, B: 0xf8, A: 0xff} // slate-100
		case theme.ColorNameForeground:
			return color.NRGBA{R: 0x1e, G: 0x29, B: 0x3b, A: 0xff} // slate-800
		case theme.ColorNameButton, theme.ColorNameInputBackground:
			return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
		case theme.ColorNameInputBorder, theme.ColorNameSeparator:
			return color.NRGBA{R: 0xcb, G: 0xd5, B: 0xe1, A: 0xff} // slate-300
		case theme.ColorNameDisabled:
			return color.NRGBA{R: 0x94, G: 0xa3, B: 0xb8, A: 0xff} // slate-400
		case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
			return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
		case theme.ColorNameHover:
			return color.NRGBA{R: 0x21, G: 0x67, B: 0xb5, A: 0x22}
		case theme.ColorNameSelection:
			return color.NRGBA{R: 0x21, G: 0x67, B: 0xb5, A: 0x44}
		}
	}

	// Shared across both variants: a consistent blue accent.
	switch name {
	case theme.ColorNamePrimary, theme.ColorNameFocus:
		return color.NRGBA{R: 0x21, G: 0x67, B: 0xb5, A: 0xff} // blue-700
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0x1a, G: 0x52, B: 0x8f, A: 0xff} // blue-800
	case theme.ColorNameForegroundOnPrimary:
		return color.White
	case theme.ColorNameHyperlink:
		return color.NRGBA{R: 0x3b, G: 0x82, B: 0xf6, A: 0xff} // blue-500
	}

	return theme.DefaultTheme().Color(name, variant)
}

func (blueGreyTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (blueGreyTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (blueGreyTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
