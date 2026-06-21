package main

import (
	"embed"

	"github.com/0x3639/go-syrius/app"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	a := app.New()
	if err := wails.Run(&options.App{
		Title:       "syrius",
		Width:       1100,
		Height:      720,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   a.OnStartup,
		OnShutdown:  a.OnShutdown,
		Bind:        a.Bindings(),
	}); err != nil {
		panic(err)
	}
}
