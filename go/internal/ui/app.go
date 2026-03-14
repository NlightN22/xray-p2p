package ui

import (
	"context"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
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

type ClientInstallDefaults struct {
	InstallDir           string `json:"installDir"`
	ConfigDir            string `json:"configDir"`
	ServerAddress        string `json:"serverAddress"`
	ServerPort           string `json:"serverPort"`
	User                 string `json:"user"`
	Password             string `json:"password"`
	ServerName           string `json:"serverName"`
	AllowInsecure        bool   `json:"allowInsecure"`
	PinnedPeerCertSHA256 string `json:"pinnedPeerCertSha256"`
	VerifyPeerCertByName string `json:"verifyPeerCertByName"`
	TunEnabled           bool   `json:"tunEnabled"`
	TunName              string `json:"tunName"`
	TunMTU               int    `json:"tunMtu"`
	TunAddr              string `json:"tunAddr"`
}

type ClientInstallRequest struct {
	InstallDir           string `json:"installDir"`
	ConfigDir            string `json:"configDir"`
	ServerAddress        string `json:"serverAddress"`
	ServerPort           string `json:"serverPort"`
	User                 string `json:"user"`
	Password             string `json:"password"`
	ServerName           string `json:"serverName"`
	AllowInsecure        bool   `json:"allowInsecure"`
	PinnedPeerCertSHA256 string `json:"pinnedPeerCertSha256"`
	VerifyPeerCertByName string `json:"verifyPeerCertByName"`
	TunEnabled           bool   `json:"tunEnabled"`
	TunName              string `json:"tunName"`
	TunMTU               int    `json:"tunMtu"`
	TunAddr              string `json:"tunAddr"`
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

func (a *App) GetClientInstallDefaults() (ClientInstallDefaults, error) {
	cfg, err := config.Load(config.Options{})
	if err != nil {
		return ClientInstallDefaults{}, err
	}
	return ClientInstallDefaults{
		InstallDir:           cfg.Client.InstallDir,
		ConfigDir:            cfg.Client.ConfigDir,
		ServerAddress:        cfg.Client.ServerAddress,
		ServerPort:           cfg.Client.ServerPort,
		User:                 cfg.Client.User,
		Password:             cfg.Client.Password,
		ServerName:           cfg.Client.ServerName,
		AllowInsecure:        cfg.Client.AllowInsecure,
		PinnedPeerCertSHA256: "",
		VerifyPeerCertByName: "",
		TunEnabled:           cfg.Client.TunEnabled,
		TunName:              cfg.Client.TunName,
		TunMTU:               cfg.Client.TunMTU,
		TunAddr:              cfg.Client.TunAddr,
	}, nil
}

func (a *App) InstallClient(req ClientInstallRequest) error {
	cfg, err := config.Load(config.Options{})
	if err != nil {
		return err
	}
	installDir := strings.TrimSpace(req.InstallDir)
	if installDir == "" {
		installDir = cfg.Client.InstallDir
	}
	configDir := strings.TrimSpace(req.ConfigDir)
	if configDir == "" {
		configDir = cfg.Client.ConfigDir
	}
	serverAddress := strings.TrimSpace(req.ServerAddress)
	if serverAddress == "" {
		serverAddress = cfg.Client.ServerAddress
	}
	serverPort := strings.TrimSpace(req.ServerPort)
	if serverPort == "" {
		serverPort = cfg.Client.ServerPort
	}
	user := strings.TrimSpace(req.User)
	if user == "" {
		user = cfg.Client.User
	}
	password := strings.TrimSpace(req.Password)
	if password == "" {
		password = cfg.Client.Password
	}
	serverName := strings.TrimSpace(req.ServerName)
	if serverName == "" {
		serverName = cfg.Client.ServerName
	}
	opts := client.InstallOptions{
		InstallDir:           installDir,
		ConfigDir:            configDir,
		ServerAddress:        serverAddress,
		ServerPort:           serverPort,
		User:                 user,
		Password:             password,
		ServerName:           serverName,
		AllowInsecure:        req.AllowInsecure,
		PinnedPeerCertSHA256: req.PinnedPeerCertSHA256,
		VerifyPeerCertByName: req.VerifyPeerCertByName,
		Force:                true,
		TunEnabled:           req.TunEnabled,
		TunEnabledSet:        true,
		TunName:              req.TunName,
		TunMTU:               req.TunMTU,
		TunAddr:              req.TunAddr,
	}
	return client.Install(context.Background(), opts)
}
