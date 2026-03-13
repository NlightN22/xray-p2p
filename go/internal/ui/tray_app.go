package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/ports"
	"github.com/NlightN22/xray-p2p/go/internal/usecase"
	"github.com/getlantern/systray"
)

type TrayApp struct {
	serviceControl *usecase.ServiceControl
	settings       Settings
	icons          IconSet

	mu             sync.Mutex
	serviceItems   map[string]*serviceMenuItem
	currentStates  map[string]ports.ServiceState
	shutdownSignal chan struct{}
}

type serviceMenuItem struct {
	name        string
	displayName string
	item        *systray.MenuItem
}

func NewTrayApp(serviceControl *usecase.ServiceControl, settings Settings, icons IconSet) *TrayApp {
	return &TrayApp{
		serviceControl: serviceControl,
		settings:       settings,
		icons:          icons,
		serviceItems:   make(map[string]*serviceMenuItem),
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

	header := systray.AddMenuItem("xp2p-ui", "xp2p-ui")
	header.Disable()

	servicesMenu := systray.AddMenuItem("Services", "Service control")
	a.buildServiceMenu(servicesMenu)
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

func (a *TrayApp) buildServiceMenu(parent *systray.MenuItem) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	services, err := a.serviceControl.List(ctx)
	if err != nil {
		logging.Error("xp2p-ui list services failed", "err", err)
		item := parent.AddSubMenuItem("Service list failed", "")
		item.Disable()
		return
	}

	if len(services) == 0 {
		item := parent.AddSubMenuItem("No services installed", "")
		item.Disable()
		return
	}

	for _, info := range services {
		name := info.Name
		display := info.DisplayName
		label := serviceLabel(name, display, info.State)
		item := parent.AddSubMenuItemCheckbox(label, "", info.State == ports.ServiceStateRunning)
		svcItem := &serviceMenuItem{
			name:        name,
			displayName: display,
			item:        item,
		}
		a.serviceItems[name] = svcItem
		a.currentStates[name] = info.State
		go a.watchServiceItem(svcItem)
	}
}

func (a *TrayApp) watchServiceItem(item *serviceMenuItem) {
	for {
		select {
		case <-item.item.ClickedCh:
			a.toggleService(item.name)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	infos, err := a.serviceControl.List(ctx)
	if err != nil {
		logging.Error("xp2p-ui status refresh failed", "err", err)
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for _, info := range infos {
		item, ok := a.serviceItems[info.Name]
		if !ok {
			continue
		}
		a.currentStates[info.Name] = info.State
		label := serviceLabel(info.Name, item.displayName, info.State)
		item.item.SetTitle(label)
		if info.State == ports.ServiceStateRunning {
			item.item.Check()
			item.item.Enable()
		} else {
			item.item.Uncheck()
			if isPendingState(info.State) {
				item.item.Disable()
			} else {
				item.item.Enable()
			}
		}
	}

	a.updateTrayIcon()
}

func (a *TrayApp) toggleService(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	info, err := a.serviceControl.Status(ctx, name)
	if err != nil {
		Notify("xp2p", fmt.Sprintf("Service %s status failed: %v", name, err))
		logging.Error("xp2p-ui status failed", "service", name, "err", err)
		return
	}

	if info.State == ports.ServiceStateRunning {
		a.runServiceAction(name, "stopping", ports.ServiceStateStopPending, func() error {
			return a.serviceControl.Stop(ctx, name)
		})
		return
	}

	a.runServiceAction(name, "starting", ports.ServiceStateStartPending, func() error {
		return a.serviceControl.Start(ctx, name)
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

	item, ok := a.serviceItems[name]
	if !ok {
		return
	}
	item.item.Disable()
	item.item.SetTitle(serviceLabel(name, item.displayName, pendingState))
	a.updateTrayIconLocked(pendingState)
}

func (a *TrayApp) updateTrayIcon() {
	overall := ports.ServiceStateStopped
	for _, state := range a.currentStates {
		if isPendingState(state) {
			overall = ports.ServiceStateStartPending
			break
		}
		if state == ports.ServiceStateRunning {
			overall = ports.ServiceStateRunning
		}
	}
	a.updateTrayIconLocked(overall)
}

func (a *TrayApp) updateTrayIconLocked(state ports.ServiceState) {
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

func serviceLabel(name, display string, state ports.ServiceState) string {
	base := display
	if base == "" {
		base = name
	}
	switch state {
	case ports.ServiceStateStartPending:
		return base + " (starting)"
	case ports.ServiceStateStopPending:
		return base + " (stopping)"
	default:
		return base
	}
}

func isPendingState(state ports.ServiceState) bool {
	switch state {
	case ports.ServiceStateStartPending, ports.ServiceStateStopPending, ports.ServiceStateContinuePending, ports.ServiceStatePausePending:
		return true
	default:
		return false
	}
}
