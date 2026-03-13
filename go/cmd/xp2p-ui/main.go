package main

import (
	"embed"
	_ "embed"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/platform/windows"
	"github.com/NlightN22/xray-p2p/go/internal/ui"
	"github.com/NlightN22/xray-p2p/go/internal/usecase"
	"github.com/wailsapp/wails/v2/pkg/application"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed assets/Base.png
var iconBase []byte

//go:embed assets/Enabling.png
var iconEnabling []byte

//go:embed assets/Enabled.png
var iconEnabled []byte

//go:embed frontend/dist/*
var frontendAssets embed.FS

func main() {
	logFile, closeFn := initLogging()
	if closeFn != nil {
		defer closeFn()
	}
	logging.Info("xp2p-ui starting", "log_file", logFile)

	iconSet, err := ui.BuildIconSet(iconBase, iconEnabling, iconEnabled, 64)
	if err != nil {
		logging.Error("xp2p-ui icon build failed", "err", err)
	}

	serviceManager := windows.NewServiceManager()
	linkInstaller := windows.NewLinkInstaller()
	pingClient := windows.NewPingClient()
	serviceControl := usecase.NewServiceControl(serviceManager, []string{"xp2p-server", "xp2p-client"})
	configTransfer := usecase.NewConfigTransfer()
	linkInstall := usecase.NewLinkInstall(linkInstaller)
	pingUsecase := usecase.NewPing(pingClient)

	appBindings := ui.NewApp(ui.AppOptions{
		ServiceControl: serviceControl,
		ConfigTransfer: configTransfer,
		LinkInstall:    linkInstall,
		Ping:           pingUsecase,
	})

	go startWailsApp(appBindings)

	settings := ui.LoadSettings()

	tray := ui.NewTrayApp(serviceControl, settings, iconSet)
	tray.Run()
}

func initLogging() (string, func()) {
	logPath := filepath.Clean(config.LogPath("xp2p-ui.log"))
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		logging.Error("xp2p-ui create log dir failed", "path", logPath, "err", err)
		return "", nil
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		logging.Error("xp2p-ui open log file failed", "path", logPath, "err", err)
		return logPath, nil
	}
	logging.Configure(logging.Options{
		Output: file,
	})
	return logPath, func() { _ = file.Close() }
}

func startWailsApp(bindings *ui.App) {
	app := application.NewWithOptions(&options.App{
		Title:             "xp2p-ui",
		StartHidden:       true,
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Assets: frontendAssets,
		},
		Bind: []interface{}{bindings},
	})
	if err := app.Run(); err != nil {
		logging.Error("xp2p-ui wails app failed", "err", err)
	}
}
