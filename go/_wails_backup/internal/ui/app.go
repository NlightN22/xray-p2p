package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	clientcmd "github.com/NlightN22/xray-p2p/go/internal/cli/client"
	servercmd "github.com/NlightN22/xray-p2p/go/internal/cli/server"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/ports"
	"github.com/NlightN22/xray-p2p/go/internal/server"
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

type ClientDeployDefaults struct {
	Host       string `json:"host"`
	DeployPort string `json:"deployPort"`
	InstallDir string `json:"installDir"`
	User       string `json:"user"`
	Password   string `json:"password"`
	TrojanPort string `json:"trojanPort"`
}

type ClientDeployRequest struct {
	Host       string `json:"host"`
	DeployPort string `json:"deployPort"`
	InstallDir string `json:"installDir"`
	User       string `json:"user"`
	Password   string `json:"password"`
	TrojanPort string `json:"trojanPort"`
}

type ServerInstallDefaults struct {
	InstallDir string `json:"installDir"`
	ConfigDir  string `json:"configDir"`
	Port       string `json:"port"`
	CertStore  string `json:"certStore"`
	CertFile   string `json:"certFile"`
	KeyFile    string `json:"keyFile"`
	Host       string `json:"host"`
}

type ServerInstallRequest struct {
	InstallDir string `json:"installDir"`
	ConfigDir  string `json:"configDir"`
	Port       string `json:"port"`
	CertStore  string `json:"certStore"`
	CertFile   string `json:"certFile"`
	KeyFile    string `json:"keyFile"`
	Host       string `json:"host"`
}

type ServerDeployDefaults struct {
	Listen   string `json:"listen"`
	DiagPort string `json:"diagPort"`
	Timeout  string `json:"timeout"`
}

type ServerDeployRequest struct {
	Listen   string `json:"listen"`
	Link     string `json:"link"`
	DiagPort string `json:"diagPort"`
	Timeout  string `json:"timeout"`
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

func (a *App) GetClientDeployDefaults() (ClientDeployDefaults, error) {
	cfg, err := config.Load(config.Options{})
	if err != nil {
		return ClientDeployDefaults{}, err
	}
	return ClientDeployDefaults{
		Host:       cfg.Server.Host,
		DeployPort: "62025",
		InstallDir: cfg.Server.InstallDir,
		User:       cfg.Client.User,
		Password:   cfg.Client.Password,
		TrojanPort: defaultTrojanPort(cfg),
	}, nil
}

func (a *App) GetServerInstallDefaults() (ServerInstallDefaults, error) {
	cfg, err := config.Load(config.Options{})
	if err != nil {
		return ServerInstallDefaults{}, err
	}
	return ServerInstallDefaults{
		InstallDir: cfg.Server.InstallDir,
		ConfigDir:  cfg.Server.ConfigDir,
		Port:       cfg.Server.TrojanPort,
		CertStore:  cfg.Server.CertificateStore,
		CertFile:   cfg.Server.CertificateFile,
		KeyFile:    cfg.Server.KeyFile,
		Host:       cfg.Server.Host,
	}, nil
}

func (a *App) GetServerDeployDefaults() (ServerDeployDefaults, error) {
	cfg, err := config.Load(config.Options{})
	if err != nil {
		return ServerDeployDefaults{}, err
	}
	return ServerDeployDefaults{
		Listen:   ":62025",
		DiagPort: cfg.Server.Port,
		Timeout:  "10m",
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

func (a *App) DeployClient(req ClientDeployRequest) error {
	cfg, err := config.Load(config.Options{})
	if err != nil {
		return err
	}
	opts := clientcmd.DeployOptions{
		Host:       strings.TrimSpace(req.Host),
		DeployPort: strings.TrimSpace(req.DeployPort),
		InstallDir: strings.TrimSpace(req.InstallDir),
		User:       strings.TrimSpace(req.User),
		Password:   strings.TrimSpace(req.Password),
		TrojanPort: strings.TrimSpace(req.TrojanPort),
	}
	return clientcmd.Deploy(context.Background(), cfg, opts)
}

func (a *App) InstallServer(req ServerInstallRequest) error {
	cfg, err := config.Load(config.Options{})
	if err != nil {
		return err
	}
	opts := servercmd.InstallOptions{
		Path:      strings.TrimSpace(req.InstallDir),
		ConfigDir: strings.TrimSpace(req.ConfigDir),
		Port:      strings.TrimSpace(req.Port),
		CertStore: strings.TrimSpace(req.CertStore),
		CertFile:  strings.TrimSpace(req.CertFile),
		KeyFile:   strings.TrimSpace(req.KeyFile),
		Host:      strings.TrimSpace(req.Host),
	}
	return servercmd.Install(context.Background(), cfg, opts)
}

func (a *App) DeployServer(req ServerDeployRequest) error {
	cfg, err := config.Load(config.Options{})
	if err != nil {
		return err
	}
	timeout, err := parseDurationOrZero(req.Timeout)
	if err != nil {
		return err
	}
	opts := servercmd.DeployOptions{
		Listen:   strings.TrimSpace(req.Listen),
		Link:     strings.TrimSpace(req.Link),
		DiagPort: strings.TrimSpace(req.DiagPort),
		Timeout:  timeout,
	}
	return servercmd.Deploy(context.Background(), cfg, opts)
}

func parseDurationOrZero(raw string) (time.Duration, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout %q: %w", value, err)
	}
	return duration, nil
}

func defaultTrojanPort(cfg config.Config) string {
	if value := strings.TrimSpace(cfg.Client.ServerPort); value != "" {
		return value
	}
	if value := strings.TrimSpace(cfg.Server.TrojanPort); value != "" {
		return value
	}
	return fmt.Sprintf("%d", server.DefaultTrojanPort)
}
