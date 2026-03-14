package main

import (
	"context"
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
	winopts "github.com/wailsapp/wails/v2/pkg/options/windows"
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

	tray := ui.NewTrayApp(serviceControl, settings, iconSet, ui.TrayOptions{
		LinkInstall:    linkInstall,
		ConfigTransfer: configTransfer,
	})
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
	userDataPath := wailsUserDataPath()
	if userDataPath != "" {
		if err := os.MkdirAll(userDataPath, 0o755); err != nil {
			logging.Error("xp2p-ui create webview data dir failed", "path", userDataPath, "err", err)
		}
	}
	app := application.NewWithOptions(&options.App{
		Title:             "xp2p-ui",
		Width:             640,
		Height:            520,
		MinWidth:          600,
		MinHeight:         480,
		WindowStartState:  options.Normal,
		StartHidden:       true,
		HideWindowOnClose: true,
		OnStartup: func(ctx context.Context) {
			ui.SetWailsContext(ctx)
		},
		AssetServer: &assetserver.Options{
			Assets: frontendAssets,
		},
		Windows: &winopts.Options{
			WebviewUserDataPath: userDataPath,
		},
		Bind: []interface{}{bindings},
	})
	if err := app.Run(); err != nil {
		logging.Error("xp2p-ui wails app failed", "err", err)
	}
}

func wailsUserDataPath() string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "xp2p-ui", "webview2")
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "xp2p-ui", "webview2")
	}
	return ""
}
