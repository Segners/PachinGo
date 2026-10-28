package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "PachinGo",
		Width:  1200,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Fond clair aligné sur le logo (marine / cyan).
		BackgroundColour: &options.RGBA{R: 243, G: 246, B: 250, A: 1},
		OnStartup:        app.startup,
		// Windows : l'icône de l'exe vient de build/windows/icon.ico (généré depuis le logo).
		Windows: &windows.Options{
			Theme: windows.Light,
		},
		Mac: &mac.Options{
			About: &mac.AboutInfo{
				Title: "PachinGo",
				Icon:  icon,
			},
		},
		Linux: &linux.Options{
			Icon: icon,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
