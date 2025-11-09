package servercmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

var (
	serverInstallFunc    = server.Install
	serverRemoveFunc     = server.Remove
	serverRunFunc        = server.Run
	serverUserAddFunc    = server.AddUser
	serverUserRemoveFunc = server.RemoveUser
	detectPublicHostFunc = netutil.DetectPublicHost
	serverSetCertFunc    = server.SetCertificate
	serverUserLinkFunc   = server.GetUserLink
	serverUserListFunc   = server.ListUsers
)

var promptYesNoFunc = promptYesNo

type serverInstallCommandOptions struct {
	Path      string
	ConfigDir string
	Port      string
	Cert      string
	Key       string
	Host      string
	Force     bool
}

type serverRemoveCommandOptions struct {
	Path          string
	KeepFiles     bool
	IgnoreMissing bool
}

type serverRunCommandOptions struct {
	Path        string
	ConfigDir   string
	AutoInstall bool
	Quiet       bool
	XrayLogFile string
}

func runServerInstall(ctx context.Context, cfg config.Config, opts serverInstallCommandOptions) int {
	portValue := resolveInstallPort(cfg, opts.Port)
	if err := validatePortValue(portValue); err != nil {
		logging.Error("xp2p server install: invalid port", "port", portValue, "err", err)
		return 1
	}

	hostValue, autoDetected, err := determineInstallHost(ctx, opts.Host, cfg.Server.Host)
	if err != nil {
		logging.Error("xp2p server install: failed to resolve public host", "err", err)
		return 1
	}
	if autoDetected {
		logging.Info("xp2p server install: detected public host", "host", hostValue)
	}

	installOpts := server.InstallOptions{
		InstallDir:      firstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:       firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
		Port:            portValue,
		CertificateFile: firstNonEmpty(opts.Cert, cfg.Server.CertificateFile),
		KeyFile:         firstNonEmpty(opts.Key, cfg.Server.KeyFile),
		Host:            hostValue,
		Force:           opts.Force,
	}

	if err := serverInstallFunc(ctx, installOpts); err != nil {
		logging.Error("xp2p server install failed", "err", err)
		return 1
	}

	logging.Info("xp2p server installed", "install_dir", installOpts.InstallDir, "config_dir", installOpts.ConfigDir)

	if strings.TrimSpace(cfg.Client.User) == "" && strings.TrimSpace(cfg.Client.Password) == "" {
		if err := generateDefaultServerCredential(ctx, installOpts, hostValue); err != nil {
			logging.Warn("xp2p server install: failed to generate trojan credential", "err", err)
		}
	}

	return 0
}
func runServerRemove(ctx context.Context, cfg config.Config, opts serverRemoveCommandOptions) int {
	removeOpts := server.RemoveOptions{
		InstallDir:    firstNonEmpty(opts.Path, cfg.Server.InstallDir),
		KeepFiles:     opts.KeepFiles,
		IgnoreMissing: opts.IgnoreMissing,
	}

	if err := serverRemoveFunc(ctx, removeOpts); err != nil {
		logging.Error("xp2p server remove failed", "err", err)
		return 1
	}

	logging.Info("xp2p server removed", "install_dir", removeOpts.InstallDir)
	return 0
}
func runServerRun(ctx context.Context, cfg config.Config, opts serverRunCommandOptions) int {
	installDir := firstNonEmpty(opts.Path, cfg.Server.InstallDir)
	if installDir == "" {
		logging.Error("xp2p server run: installation directory is required")
		return 1
	}

	configDirName := firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir)
	configDirPath, err := resolveConfigDirPath(installDir, configDirName)
	if err != nil {
		logging.Error("xp2p server run: resolve config directory", "err", err)
		return 1
	}

	if err := ensureServerAssets(ctx, cfg, installDir, configDirName, configDirPath, opts.AutoInstall, opts.Quiet); err != nil {
		logging.Error("xp2p server run: prerequisites failed", "err", err)
		return 1
	}

	cancelDiagnostics := startDiagnostics(ctx, cfg.Server.Port)
	if cancelDiagnostics != nil {
		defer cancelDiagnostics()
	}

	runOpts := server.RunOptions{
		InstallDir:   installDir,
		ConfigDir:    configDirName,
		ErrorLogPath: strings.TrimSpace(opts.XrayLogFile),
	}

	if err := serverRunFunc(ctx, runOpts); err != nil {
		logging.Error("xp2p server run failed", "err", err)
		return 1
	}

	return 0
}
func ensureServerAssets(ctx context.Context, cfg config.Config, installDir, configDirName, configDirPath string, autoInstall, quiet bool) error {
	present, err := serverAssetsPresent(installDir, configDirPath)
	if err != nil {
		return err
	}
	if present {
		return nil
	}

	if autoInstall {
		return performInstall(ctx, cfg, installDir, configDirName)
	}

	if quiet {
		return errors.New("installation not found and --quiet supplied (use --auto-install)")
	}

	ok, err := promptYesNoFunc(fmt.Sprintf("Install xray-core into %s?", installDir))
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("installation required to run server")
	}

	return performInstall(ctx, cfg, installDir, configDirName)
}

func performInstall(ctx context.Context, cfg config.Config, installDir, configDirName string) error {
	hostValue, autoDetected, err := determineInstallHost(ctx, "", cfg.Server.Host)
	if err != nil {
		return fmt.Errorf("xp2p server install: detect host: %w", err)
	}
	if autoDetected {
		logging.Info("xp2p server install: detected public host", "host", hostValue)
	}

	opts := server.InstallOptions{
		InstallDir: installDir,
		ConfigDir:  configDirName,
		Port:       resolveInstallPort(cfg, ""),
		Host:       hostValue,
	}
	if cfg.Server.CertificateFile != "" {
		opts.CertificateFile = cfg.Server.CertificateFile
	}
	if cfg.Server.KeyFile != "" {
		opts.KeyFile = cfg.Server.KeyFile
	}
	return serverInstallFunc(ctx, opts)
}

func resolveInstallPort(cfg config.Config, flagPort string) string {
	portValue := strings.TrimSpace(flagPort)
	if portValue != "" {
		return portValue
	}

	cfgPort := strings.TrimSpace(cfg.Server.Port)
	if cfgPort != "" && cfgPort != server.DefaultPort {
		return cfgPort
	}

	return strconv.Itoa(server.DefaultTrojanPort)
}

func serverAssetsPresent(installDir, configDirPath string) (bool, error) {
	binPath := filepath.Join(installDir, "bin", "xray.exe")
	if info, err := os.Stat(binPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("xp2p: stat %s: %w", binPath, err)
	} else if info.IsDir() {
		return false, fmt.Errorf("xp2p: expected file at %s", binPath)
	}

	configInfo, err := os.Stat(configDirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("xp2p: stat %s: %w", configDirPath, err)
	}
	if !configInfo.IsDir() {
		return false, fmt.Errorf("xp2p: %s is not a directory", configDirPath)
	}

	requiredFiles := []string{"inbounds.json", "logs.json", "outbounds.json", "routing.json"}
	for _, name := range requiredFiles {
		path := filepath.Join(configDirPath, name)
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return false, nil
			}
			return false, fmt.Errorf("xp2p: stat %s: %w", path, err)
		}
	}
	return true, nil
}

func promptYesNo(question string) (bool, error) {
	fmt.Printf("%s [Y/n]: ", question)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	if answer == "" || answer == "y" || answer == "yes" {
		return true, nil
	}
	if answer == "n" || answer == "no" {
		return false, nil
	}
	fmt.Println("Please answer 'y' or 'n'.")
	return promptYesNo(question)
}

func resolveConfigDirPath(installDir, configDir string) (string, error) {
	cfgDir := strings.TrimSpace(configDir)
	if cfgDir == "" {
		cfgDir = server.DefaultServerConfigDir
	}
	if filepath.IsAbs(cfgDir) {
		return cfgDir, nil
	}
	return filepath.Join(installDir, cfgDir), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func validatePortValue(port string) error {
	value := strings.TrimSpace(port)
	if value == "" {
		return fmt.Errorf("port is empty")
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid port %q: %w", value, err)
	}
	if n <= 0 || n > 65535 {
		return fmt.Errorf("invalid port %q: must be within 1-65535", value)
	}
	return nil
}

func determineInstallHost(ctx context.Context, explicit, fallback string) (string, bool, error) {
	host := firstNonEmpty(explicit, fallback)
	host = strings.TrimSpace(host)
	if host != "" {
		if err := netutil.ValidateHost(host); err != nil {
			return "", false, fmt.Errorf("invalid host %q: %w", host, err)
		}
		return host, false, nil
	}
	value, err := detectPublicHostFunc(ctx)
	if err != nil {
		return "", false, err
	}
	value = strings.TrimSpace(value)
	if err := netutil.ValidateHost(value); err != nil {
		return "", false, fmt.Errorf("invalid host %q: %w", value, err)
	}
	return value, true, nil
}

type credentialResult struct {
	details server.UserLink
	linkErr error
}

func provisionCredential(ctx context.Context, installOpts server.InstallOptions, host, userID, password string) (credentialResult, error) {
	user := strings.TrimSpace(userID)
	pass := strings.TrimSpace(password)
	if user == "" || pass == "" {
		return credentialResult{}, errors.New("xp2p: trojan credential requires user and password")
	}

	addOpts := server.AddUserOptions{
		InstallDir: installOpts.InstallDir,
		ConfigDir:  installOpts.ConfigDir,
		UserID:     user,
		Password:   pass,
	}
	if err := serverUserAddFunc(ctx, addOpts); err != nil {
		return credentialResult{}, err
	}

	link, err := serverUserLinkFunc(ctx, server.UserLinkOptions{
		InstallDir: installOpts.InstallDir,
		ConfigDir:  installOpts.ConfigDir,
		Host:       host,
		UserID:     user,
	})
	if err != nil {
		return credentialResult{
			details: server.UserLink{
				UserID:   user,
				Password: pass,
			},
			linkErr: err,
		}, nil
	}

	if strings.TrimSpace(link.UserID) == "" {
		link.UserID = user
	}
	if strings.TrimSpace(link.Password) == "" {
		link.Password = pass
	}

	return credentialResult{details: link}, nil
}

func announceCredential(prefix string, result credentialResult) {
	fmt.Printf("%s:\n  user: %s\n  password: %s\n", prefix, result.details.UserID, result.details.Password)
	if result.linkErr == nil && strings.TrimSpace(result.details.Link) != "" {
		fmt.Printf("  link: %s\n", result.details.Link)
	} else if result.linkErr != nil {
		fmt.Printf("  link: unavailable (%v)\n", result.linkErr)
	}
}

func generateDefaultServerCredential(ctx context.Context, installOpts server.InstallOptions, host string) error {
	userID, err := generateDefaultUserID()
	if err != nil {
		return err
	}
	password, err := generateRandomSecret(18)
	if err != nil {
		return err
	}

	result, err := provisionCredential(ctx, installOpts, host, userID, password)
	if err != nil {
		return err
	}

	if result.linkErr != nil {
		logging.Warn("xp2p server install: unable to build trojan link", "err", result.linkErr)
	}

	announceCredential("Generated trojan credential", result)
	logging.Info("xp2p server install: trojan credential ready", "source", "generated", "user", result.details.UserID)
	return nil
}

func generateRandomSecret(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func generateDefaultUserID() (string, error) {
	token, err := randomToken(5)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("client-%s@xp2p.local", token), nil
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)), nil
}
