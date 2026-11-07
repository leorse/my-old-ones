package main

import (
	"embed"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"moo/internal/api"
	"moo/internal/logging"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	cfg, err := os.UserConfigDir()
	if err != nil {
		cfg = os.TempDir()
	}
	cfgDir := filepath.Join(cfg, "Moo")
	closeLog := logging.Setup(filepath.Join(cfgDir, "logs"))
	defer closeLog()

	app := api.New(cfgDir)

	err = wails.Run(&options.App{
		Title:     "My Olds One",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup:        app.Startup,
		OnShutdown:       app.Shutdown,
		Bind:             []interface{}{app},
	})
	if err != nil {
		slog.Error("wails", "err", err)
		println("Error:", err.Error())
	}
}
