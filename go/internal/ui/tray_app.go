package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/ports"
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
	actionErrors   map[string]actionError
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

type actionError struct {
	err   error
	until time.Time
}

const (
	clientRoleName   = "Client"
	serverRoleName   = "Server"
	clientService    = "xp2p-client"
	serverService    = "xp2p-server"
	defaultWaitShort = 5 * time.Second
	defaultWaitLong  = 20 * time.Second
	actionErrorTTL   = 10 * time.Second
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
		actionErrors:   make(map[string]actionError),
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
	logsItem := systray.AddMenuItem("Open logs", "Open xp2p-ui log file")
	go func() {
		for {
			select {
			case <-logsItem.ClickedCh:
				a.openLogs()
			case <-a.shutdownSignal:
				return
			}
		}
	}()
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
	a.runServiceAction(ctx, cancel, name, "starting", ports.ServiceStateStartPending, func(ctx context.Context) error {
		return a.serviceControl.Start(ctx, name)
	})
}

func (a *TrayApp) stopService(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultWaitLong)
	a.runServiceAction(ctx, cancel, name, "stopping", ports.ServiceStateStopPending, func(ctx context.Context) error {
		return a.serviceControl.Stop(ctx, name)
	})
}

func (a *TrayApp) runServiceAction(ctx context.Context, cancel context.CancelFunc, name, action string, pendingState ports.ServiceState, fn func(context.Context) error) {
	a.setServicePending(name, pendingState)

	go func() {
		defer cancel()
		err := fn(ctx)
		if err != nil {
			logging.Error("xp2p-ui service action failed", "service", name, "action", action, "err", err)
			a.setActionError(name, err)
		} else {
			logging.Info("xp2p-ui service action succeeded", "service", name, "action", action)
			a.clearActionError(name)
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

func (a *TrayApp) setActionError(name string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.actionErrors[name] = actionError{
		err:   err,
		until: time.Now().Add(actionErrorTTL),
	}
	item, ok := a.roleItems[name]
	if !ok {
		return
	}
	a.updateRoleMenuLocked(item, roleStatus{
		installed: true,
		state:     ports.ServiceStateUnknown,
		err:       err,
	})
}

func (a *TrayApp) clearActionError(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.actionErrors, name)
}

func (a *TrayApp) fetchRoleStatus(ctx context.Context, serviceName string) roleStatus {
	info, err := a.serviceControl.Status(ctx, serviceName)
	if err == nil {
		return roleStatus{installed: true, state: info.State}
	}
	logging.Error("xp2p-ui service status failed", "service", serviceName, "err", err)
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
	if status.err == nil {
		if pending, ok := a.actionErrors[item.serviceName]; ok {
			if time.Now().After(pending.until) {
				delete(a.actionErrors, item.serviceName)
			} else {
				status.err = pending.err
			}
		}
	}

	label := roleLabel(item.role, status)
	item.item.SetTitle(label)

	if status.err != nil {
		item.startItem.Disable()
		item.stopItem.Disable()
		item.installItem.Enable()
		item.deployItem.Enable()
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

	item.installItem.Enable()
	item.deployItem.Enable()
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
	switch role {
	case clientRoleName:
		a.openScenario("client-deploy")
	case serverRoleName:
		a.openScenario("server-deploy")
	default:
		Notify("xp2p", "Deploy action is unavailable for this service")
	}
}

func (a *TrayApp) installClient() {
	a.openScenario("client-install")
}

func (a *TrayApp) installServer() {
	a.openScenario("server-install")
}

func (a *TrayApp) openScenario(name string) {
	ctx, ok := waitWailsContext(5 * time.Second)
	if !ok {
		Notify("xp2p", "UI runtime is not ready")
		return
	}
	runtime.WindowShow(ctx)
	runtime.EventsEmit(ctx, "xp2p-ui:open", name)
}

func showDialog(ctx context.Context, dialogType runtime.DialogType, title, message string) {
	_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:    dialogType,
		Title:   title,
		Message: message,
	})
}

func (a *TrayApp) openLogs() {
	logPath := config.LogPath("xp2p-ui.log")
	if _, err := os.Stat(logPath); err != nil {
		Notify("xp2p", fmt.Sprintf("Log file not found: %s", logPath))
		return
	}
	if err := exec.Command("explorer.exe", "/select,", logPath).Start(); err != nil {
		Notify("xp2p", fmt.Sprintf("Open log failed: %v", err))
	}
}
