package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/ports"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/usecase"
	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type TrayApp struct {
	serviceControl *usecase.ServiceControl
	linkInstall    *usecase.LinkInstall
	configTransfer *usecase.ConfigTransfer
	settings       Settings
	icons          IconSet

	mu             sync.Mutex
	roleItems      map[string]*roleMenuItem
	currentStates  map[string]ports.ServiceState
	shutdownSignal chan struct{}
}

type TrayOptions struct {
	LinkInstall    *usecase.LinkInstall
	ConfigTransfer *usecase.ConfigTransfer
}

type roleMenuItem struct {
	role        string
	serviceName string
	item        *systray.MenuItem
	startItem   *systray.MenuItem
	stopItem    *systray.MenuItem
	installItem *systray.MenuItem
	deployItem  *systray.MenuItem
}

type roleStatus struct {
	installed bool
	state     ports.ServiceState
	err       error
}

const (
	clientRoleName   = "Client"
	serverRoleName   = "Server"
	clientService    = "xp2p-client"
	serverService    = "xp2p-server"
	defaultWaitShort = 5 * time.Second
	defaultWaitLong  = 20 * time.Second
)

func NewTrayApp(serviceControl *usecase.ServiceControl, settings Settings, icons IconSet, opts TrayOptions) *TrayApp {
	return &TrayApp{
		serviceControl: serviceControl,
		linkInstall:    opts.LinkInstall,
		configTransfer: opts.ConfigTransfer,
		settings:       settings,
		icons:          icons,
		roleItems:      make(map[string]*roleMenuItem),
		currentStates:  make(map[string]ports.ServiceState),
		shutdownSignal: make(chan struct{}),
	}
}

func (a *TrayApp) Run() {
	systray.Run(a.onReady, a.onExit)
}

func (a *TrayApp) onReady() {
	if len(a.icons.Base) > 0 {
		systray.SetIcon(a.icons.Base)
	}
	systray.SetTooltip("xp2p")

	a.buildRoleMenu(clientRoleName, clientService)
	a.buildRoleMenu(serverRoleName, serverService)
	systray.AddSeparator()

	quitItem := systray.AddMenuItem("Quit", "Quit xp2p-ui")
	go func() {
		for {
			select {
			case <-quitItem.ClickedCh:
				systray.Quit()
				return
			case <-a.shutdownSignal:
				return
			}
		}
	}()

	if a.settings.AutoStart {
		if err := EnsureAutoStart(true); err != nil {
			logging.Warn("xp2p-ui autostart failed", "err", err)
		}
	}

	a.refreshStatuses()
	go a.pollStatuses()
}

func (a *TrayApp) onExit() {
	close(a.shutdownSignal)
}

func (a *TrayApp) buildRoleMenu(role, serviceName string) {
	item := systray.AddMenuItem(role+": loading", "")
	startItem := item.AddSubMenuItem("Start", "")
	stopItem := item.AddSubMenuItem("Stop", "")
	installItem := item.AddSubMenuItem("Install", "")
	deployItem := item.AddSubMenuItem("Deploy", "")

	roleItem := &roleMenuItem{
		role:        role,
		serviceName: serviceName,
		item:        item,
		startItem:   startItem,
		stopItem:    stopItem,
		installItem: installItem,
		deployItem:  deployItem,
	}
	a.roleItems[serviceName] = roleItem
	go a.watchRoleItem(roleItem)
}

func (a *TrayApp) watchRoleItem(item *roleMenuItem) {
	for {
		select {
		case <-item.startItem.ClickedCh:
			a.startService(item.serviceName)
		case <-item.stopItem.ClickedCh:
			a.stopService(item.serviceName)
		case <-item.installItem.ClickedCh:
			a.installRole(item.role, item.serviceName)
		case <-item.deployItem.ClickedCh:
			a.deployRole(item.role)
		case <-a.shutdownSignal:
			return
		}
	}
}

func (a *TrayApp) pollStatuses() {
	ticker := time.NewTicker(a.settings.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.refreshStatuses()
		case <-a.shutdownSignal:
			return
		}
	}
}

func (a *TrayApp) refreshStatuses() {
	ctx, cancel := context.WithTimeout(context.Background(), defaultWaitShort)
	defer cancel()

	a.mu.Lock()
	defer a.mu.Unlock()

	for _, role := range a.roleItems {
		status := a.fetchRoleStatus(ctx, role.serviceName)
		a.updateRoleMenuLocked(role, status)
	}

	a.updateTrayIconLocked()
}

func (a *TrayApp) startService(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultWaitLong)
	defer cancel()
	a.runServiceAction(name, "starting", ports.ServiceStateStartPending, func() error {
		return a.serviceControl.Start(ctx, name)
	})
}

func (a *TrayApp) stopService(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultWaitLong)
	defer cancel()
	a.runServiceAction(name, "stopping", ports.ServiceStateStopPending, func() error {
		return a.serviceControl.Stop(ctx, name)
	})
}

func (a *TrayApp) runServiceAction(name, action string, pendingState ports.ServiceState, fn func() error) {
	a.setServicePending(name, pendingState)

	go func() {
		err := fn()
		if err != nil {
			Notify("xp2p", fmt.Sprintf("Service %s %s failed: %v", name, action, err))
			logging.Error("xp2p-ui service action failed", "service", name, "action", action, "err", err)
		} else {
			Notify("xp2p", fmt.Sprintf("Service %s %s succeeded", name, action))
			logging.Info("xp2p-ui service action succeeded", "service", name, "action", action)
		}
		a.refreshStatuses()
	}()
}

func (a *TrayApp) setServicePending(name string, pendingState ports.ServiceState) {
	a.mu.Lock()
	defer a.mu.Unlock()

	item, ok := a.roleItems[name]
	if !ok {
		return
	}
	status := roleStatus{
		installed: true,
		state:     pendingState,
	}
	a.updateRoleMenuLocked(item, status)
}

func (a *TrayApp) updateTrayIcon() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.updateTrayIconLocked()
}

func (a *TrayApp) updateTrayIconLocked() {
	overall := ports.ServiceStateStopped
	if len(a.currentStates) == 0 {
		a.updateTrayIconLockedState(ports.ServiceStateStopped)
		return
	}
	for _, state := range a.currentStates {
		if isPendingState(state) {
			overall = ports.ServiceStateStartPending
			break
		}
		if state == ports.ServiceStateRunning {
			overall = ports.ServiceStateRunning
		}
	}
	a.updateTrayIconLockedState(overall)
}

func (a *TrayApp) updateTrayIconLockedState(state ports.ServiceState) {
	switch {
	case isPendingState(state):
		a.setTrayIcon(a.icons.Enabling)
	case state == ports.ServiceStateRunning:
		a.setTrayIcon(a.icons.Enabled)
	default:
		a.setTrayIcon(a.icons.Base)
	}
}

func (a *TrayApp) setTrayIcon(icon []byte) {
	if len(icon) == 0 {
		return
	}
	systray.SetIcon(icon)
}

func (a *TrayApp) fetchRoleStatus(ctx context.Context, serviceName string) roleStatus {
	info, err := a.serviceControl.Status(ctx, serviceName)
	if err == nil {
		return roleStatus{installed: true, state: info.State}
	}
	if isServiceMissingError(err) {
		return roleStatus{installed: false, state: ports.ServiceStateStopped}
	}
	return roleStatus{installed: false, state: ports.ServiceStateUnknown, err: err}
}

func isPendingState(state ports.ServiceState) bool {
	switch state {
	case ports.ServiceStateStartPending, ports.ServiceStateStopPending, ports.ServiceStateContinuePending, ports.ServiceStatePausePending:
		return true
	default:
		return false
	}
}

func (a *TrayApp) updateRoleMenuLocked(item *roleMenuItem, status roleStatus) {
	label := roleLabel(item.role, status)
	item.item.SetTitle(label)

	if status.err != nil {
		item.startItem.Disable()
		item.stopItem.Disable()
		item.installItem.Disable()
		item.deployItem.Disable()
		delete(a.currentStates, item.serviceName)
		return
	}

	if !status.installed {
		item.startItem.Disable()
		item.stopItem.Disable()
		item.installItem.Enable()
		item.deployItem.Enable()
		delete(a.currentStates, item.serviceName)
		return
	}

	item.installItem.Disable()
	item.deployItem.Disable()
	switch status.state {
	case ports.ServiceStateRunning:
		item.startItem.Disable()
		item.stopItem.Enable()
	case ports.ServiceStateStopped:
		item.startItem.Enable()
		item.stopItem.Disable()
	default:
		item.startItem.Disable()
		item.stopItem.Disable()
	}

	a.currentStates[item.serviceName] = status.state
}

func roleLabel(role string, status roleStatus) string {
	if status.err != nil {
		return role + ": " + classifyStatusError(status.err)
	}
	if !status.installed {
		return role + ": Not installed"
	}
	return role + ": " + serviceStateLabel(status.state)
}

func serviceStateLabel(state ports.ServiceState) string {
	switch state {
	case ports.ServiceStateRunning:
		return "Running"
	case ports.ServiceStateStopped:
		return "Stopped"
	case ports.ServiceStateStartPending:
		return "Starting"
	case ports.ServiceStateStopPending:
		return "Stopping"
	case ports.ServiceStatePausePending:
		return "Pausing"
	case ports.ServiceStatePaused:
		return "Paused"
	case ports.ServiceStateContinuePending:
		return "Resuming"
	default:
		return "Unknown"
	}
}

func classifyStatusError(err error) string {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "access is denied") {
		return "Access denied"
	}
	return "Error"
}

func isServiceMissingError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not installed") || strings.Contains(msg, "does not exist")
}

func (a *TrayApp) installRole(role, serviceName string) {
	switch serviceName {
	case clientService:
		a.installClient()
	case serverService:
		a.installServer()
	default:
		Notify("xp2p", "Install action is unavailable for this service")
	}
}

func (a *TrayApp) deployRole(role string) {
	if a.configTransfer == nil {
		Notify("xp2p", "Config deploy is unavailable")
		return
	}
	ctx, ok := ensureWailsContext()
	if !ok {
		Notify("xp2p", "UI runtime is not ready")
		return
	}
	runtime.WindowShow(ctx)
	path, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
		Title: "Select xp2p config bundle",
		Filters: []runtime.FileFilter{
			{DisplayName: "xp2p bundles", Pattern: "*.zip;*.tar.gz;*.tgz"},
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
	if err != nil {
		showDialog(ctx, runtime.ErrorDialog, "xp2p", fmt.Sprintf("Config deploy failed: %v", err))
		return
	}
	if strings.TrimSpace(path) == "" {
		return
	}
	root := config.ConfigRoot()
	if err := a.configTransfer.Import(ctx, root, path); err != nil {
		showDialog(ctx, runtime.ErrorDialog, "xp2p", fmt.Sprintf("Config deploy failed: %v", err))
		return
	}
	Notify("xp2p", fmt.Sprintf("%s config deploy completed", role))
	a.refreshStatuses()
}

func (a *TrayApp) installClient() {
	ctx, ok := ensureWailsContext()
	if !ok {
		Notify("xp2p", "UI runtime is not ready")
		return
	}
	runtime.WindowShow(ctx)
	linkText := ""
	if text, err := runtime.ClipboardGetText(ctx); err == nil {
		linkText = strings.TrimSpace(text)
	}
	if linkText != "" && a.linkInstall != nil {
		if err := a.linkInstall.Install(ctx, linkText); err != nil {
			showDialog(ctx, runtime.ErrorDialog, "xp2p", fmt.Sprintf("Client install failed: %v", err))
			return
		}
		Notify("xp2p", "Client install completed")
		a.refreshStatuses()
		return
	}

	cfg, err := config.Load(config.Options{})
	if err != nil {
		showDialog(ctx, runtime.ErrorDialog, "xp2p", fmt.Sprintf("Client install failed: %v", err))
		return
	}

	opts := client.InstallOptions{
		InstallDir:    cfg.Client.InstallDir,
		ConfigDir:     cfg.Client.ConfigDir,
		ServerAddress: cfg.Client.ServerAddress,
		ServerPort:    cfg.Client.ServerPort,
		User:          cfg.Client.User,
		Password:      cfg.Client.Password,
		ServerName:    cfg.Client.ServerName,
		AllowInsecure: cfg.Client.AllowInsecure,
		Force:         true,
		TunEnabled:    cfg.Client.TunEnabled,
		TunEnabledSet: true,
		TunName:       cfg.Client.TunName,
		TunMTU:        cfg.Client.TunMTU,
		TunAddr:       cfg.Client.TunAddr,
	}
	if err := client.Install(ctx, opts); err != nil {
		showDialog(ctx, runtime.ErrorDialog, "xp2p", fmt.Sprintf("Client install failed: %v", err))
		return
	}
	Notify("xp2p", "Client install completed")
	a.refreshStatuses()
}

func (a *TrayApp) installServer() {
	ctx, ok := ensureWailsContext()
	if !ok {
		Notify("xp2p", "UI runtime is not ready")
		return
	}
	runtime.WindowShow(ctx)

	cfg, err := config.Load(config.Options{})
	if err != nil {
		showDialog(ctx, runtime.ErrorDialog, "xp2p", fmt.Sprintf("Server install failed: %v", err))
		return
	}

	opts := server.InstallOptions{
		InstallDir:       cfg.Server.InstallDir,
		ConfigDir:        cfg.Server.ConfigDir,
		Port:             cfg.Server.Port,
		CertificateStore: cfg.Server.CertificateStore,
		CertificateFile:  cfg.Server.CertificateFile,
		KeyFile:          cfg.Server.KeyFile,
		Host:             cfg.Server.Host,
		Force:            true,
		TunEnabled:       cfg.Server.TunEnabled,
		TunEnabledSet:    true,
		TunName:          cfg.Server.TunName,
		TunMTU:           cfg.Server.TunMTU,
		TunAddr:          cfg.Server.TunAddr,
	}
	if err := server.Install(ctx, opts); err != nil {
		if errors.Is(err, server.ErrUnsupported) {
			showDialog(ctx, runtime.ErrorDialog, "xp2p", "Server install is not supported on this platform")
			return
		}
		showDialog(ctx, runtime.ErrorDialog, "xp2p", fmt.Sprintf("Server install failed: %v", err))
		return
	}
	Notify("xp2p", "Server install completed")
	a.refreshStatuses()
}

func showDialog(ctx context.Context, dialogType runtime.DialogType, title, message string) {
	_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:    dialogType,
		Title:   title,
		Message: message,
	})
}
