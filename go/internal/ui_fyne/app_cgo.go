//go:build windows && cgo

package uifyne

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/getlantern/systray"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/usecase"
)

func Run(opts Options) error {
	uiApp := app.NewWithID("xp2p-ui-fyne")
	window := uiApp.NewWindow("xp2p UI (Fyne)")
	statusLabel := widget.NewLabel("Ready.")
	hideButton := widget.NewButton("Hide window", func() {
		window.Hide()
	})
	window.SetContent(container.NewVBox(
		widget.NewLabel("xp2p UI (Fyne)"),
		statusLabel,
		hideButton,
	))
	window.Resize(fyne.NewSize(640, 520))
	window.Hide()

	go uiApp.Run()

	systray.Run(func() {
		systray.SetTooltip("xp2p")
		clientItem := systray.AddMenuItem("Client", "")
		clientStart := clientItem.AddSubMenuItem("Start", "")
		clientStop := clientItem.AddSubMenuItem("Stop", "")
		clientInstall := clientItem.AddSubMenuItem("Install", "")
		clientDeploy := clientItem.AddSubMenuItem("Deploy", "")

		serverItem := systray.AddMenuItem("Server", "")
		serverStart := serverItem.AddSubMenuItem("Start", "")
		serverStop := serverItem.AddSubMenuItem("Stop", "")
		serverInstall := serverItem.AddSubMenuItem("Install", "")
		serverDeploy := serverItem.AddSubMenuItem("Deploy", "")

		logsItem := systray.AddMenuItem("Open logs", "Open xp2p-ui log file")
		systray.AddSeparator()
		quitItem := systray.AddMenuItem("Quit", "Quit xp2p-ui")

		go watchMenu(clientStart, func() { runServiceAction(opts.ServiceControl, "xp2p-client", true, uiApp, window, statusLabel) })
		go watchMenu(clientStop, func() { runServiceAction(opts.ServiceControl, "xp2p-client", false, uiApp, window, statusLabel) })
		go watchMenu(clientInstall, func() { showWindow(uiApp, window, statusLabel, "Client install requested.") })
		go watchMenu(clientDeploy, func() { showWindow(uiApp, window, statusLabel, "Client deploy requested.") })

		go watchMenu(serverStart, func() { runServiceAction(opts.ServiceControl, "xp2p-server", true, uiApp, window, statusLabel) })
		go watchMenu(serverStop, func() { runServiceAction(opts.ServiceControl, "xp2p-server", false, uiApp, window, statusLabel) })
		go watchMenu(serverInstall, func() { showWindow(uiApp, window, statusLabel, "Server install requested.") })
		go watchMenu(serverDeploy, func() { showWindow(uiApp, window, statusLabel, "Server deploy requested.") })

		go watchMenu(logsItem, func() { openLogs(uiApp, window, statusLabel) })
		go watchMenu(quitItem, func() {
			systray.Quit()
			uiApp.Quit()
		})
	}, func() {
		uiApp.Quit()
	})

	return nil
}

func watchMenu(item *systray.MenuItem, action func()) {
	for range item.ClickedCh {
		action()
	}
}

func showWindow(app fyne.App, window fyne.Window, label *widget.Label, message string) {
	runOnMain(app, func() {
		label.SetText(message)
		window.Show()
		window.RequestFocus()
	})
}

func runServiceAction(ctrl *usecase.ServiceControl, name string, start bool, app fyne.App, window fyne.Window, label *widget.Label) {
	if ctrl == nil {
		showWindow(app, window, label, "Service control is unavailable.")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var err error
	if start {
		err = ctrl.Start(ctx, name)
	} else {
		err = ctrl.Stop(ctx, name)
	}
	if err != nil {
		logging.Error("xp2p-ui fyne service action failed", "service", name, "start", start, "err", err)
		showWindow(app, window, label, fmt.Sprintf("Service action failed: %v", err))
		return
	}
	action := "stopped"
	if start {
		action = "started"
	}
	showWindow(app, window, label, fmt.Sprintf("%s %s.", name, action))
}

func openLogs(app fyne.App, window fyne.Window, label *widget.Label) {
	logPath := config.LogPath("xp2p-ui.log")
	if _, err := os.Stat(logPath); err != nil {
		showWindow(app, window, label, fmt.Sprintf("Log file not found: %s", logPath))
		return
	}
	if err := exec.Command("explorer.exe", "/select,", logPath).Start(); err != nil {
		showWindow(app, window, label, fmt.Sprintf("Open log failed: %v", err))
		return
	}
	showWindow(app, window, label, "Log file opened.")
}

func runOnMain(app fyne.App, fn func()) {
	if app == nil {
		fn()
		return
	}
	app.Driver().RunOnMain(fn)
}
