package servercmd

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func ensureServerAssets(ctx context.Context, cfg config.Config, installDir, configDirName, configDirPath string, autoInstall, quiet bool) error {
	present, err := serverAssetsPresent(installDir, configDirPath)
	if err != nil {
		return err
	}
	if present {
		return nil
	}
	if handled, err := skipInstallForSystemBinary(installDir); handled {
		return err
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

func serverAssetsPresent(installDir, configDirPath string) (bool, error) {
	binaryName := "xray.exe"
	if runtime.GOOS != "windows" {
		binaryName = "xray"
	}
	binPath := filepath.Join(installDir, layout.BinDirName, binaryName)
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

func skipInstallForSystemBinary(installDir string) (bool, error) {
	if runtime.GOOS != "linux" {
		return false, nil
	}
	if filepath.Clean(installDir) != layout.UnixConfigRoot {
		return false, nil
	}

	binPath := filepath.Join(layout.UnixConfigRoot, layout.BinDirName, "xray")
	info, err := os.Stat(binPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, fmt.Errorf("xp2p: xray binary not found at %s (install the system package or set XP2P_XRAY_BIN)", binPath)
		}
		return true, fmt.Errorf("xp2p: inspect xray binary at %s: %w", binPath, err)
	}
	if info.IsDir() {
		return true, fmt.Errorf("xp2p: expected xray binary file at %s", binPath)
	}
	return true, nil
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
	host := clishared.FirstNonEmpty(explicit, fallback)
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
		Host:       host,
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

	announceCredential("Generated trojan credential", result)
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

func firstNonEmpty(values ...string) string {
	return clishared.FirstNonEmpty(values...)
}
