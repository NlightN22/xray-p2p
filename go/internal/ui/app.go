package ui

import (
	"context"

	"github.com/NlightN22/xray-p2p/go/internal/ports"
	"github.com/NlightN22/xray-p2p/go/internal/usecase"
)

type App struct {
	serviceControl *usecase.ServiceControl
	configTransfer *usecase.ConfigTransfer
	linkInstall    *usecase.LinkInstall
	ping           *usecase.Ping
}

type AppOptions struct {
	ServiceControl *usecase.ServiceControl
	ConfigTransfer *usecase.ConfigTransfer
	LinkInstall    *usecase.LinkInstall
	Ping           *usecase.Ping
}

func NewApp(opts AppOptions) *App {
	return &App{
		serviceControl: opts.ServiceControl,
		configTransfer: opts.ConfigTransfer,
		linkInstall:    opts.LinkInstall,
		ping:           opts.Ping,
	}
}

func (a *App) ListServices() ([]ports.ServiceInfo, error) {
	return a.serviceControl.List(context.Background())
}

func (a *App) StartService(name string) error {
	return a.serviceControl.Start(context.Background(), name)
}

func (a *App) StopService(name string) error {
	return a.serviceControl.Stop(context.Background(), name)
}

func (a *App) ServiceStatus(name string) (ports.ServiceInfo, error) {
	return a.serviceControl.Status(context.Background(), name)
}

func (a *App) ExportConfig(root, outputPath string) error {
	return a.configTransfer.Export(context.Background(), root, outputPath)
}

func (a *App) ImportConfig(root, inputPath string) error {
	return a.configTransfer.Import(context.Background(), root, inputPath)
}

func (a *App) InstallFromLink(link string) error {
	return a.linkInstall.Install(context.Background(), link)
}

func (a *App) Ping(target string) (ports.PingResult, error) {
	return a.ping.Run(context.Background(), target)
}
