package platform

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// BuildMainWindowOptions creates the baseline main window options and applies platform-specific window behaviour.
func BuildMainWindowOptions(useNativeTitleBar bool, width, height int) application.WebviewWindowOptions {
	options := application.WebviewWindowOptions{
		Name:             "main",
		Title:            "InvestGo",
		URL:              "/",
		Width:            width,
		Height:           height,
		MinWidth:         1024,
		MinHeight:        700,
		InitialPosition:  application.WindowCentered,
		Frameless:        true,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		Windows: application.WindowsWindow{
			Theme:                             application.SystemDefault,
			BackdropType:                      application.Auto,
			DisableIcon:                       true,
			DisableFramelessWindowDecorations: false,
			EventMapping:                      events.DefaultWindowEventMapping(),
		},
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTranslucent,
		},
	}

	if !useNativeTitleBar {
		options.Mac.TitleBar = application.MacTitleBarHiddenInsetUnified
	}

	return options
}
