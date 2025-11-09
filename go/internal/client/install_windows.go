//go:build windows

package client

import (
	"bufio"
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/NlightN22/xray-p2p/go/assets/xray"
	"github.com/NlightN22/xray-p2p/go/internal/installstate"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

//go:embed assets/templates/*
var clientTemplates embed.FS

type installState struct {
	InstallOptions
	installDir   string
	binDir       string
	configDir    string
	xrayPath     string
	serverPort   int
	serverName   string
	serverRemote string
	stateFile    string
}

// Install deploys xray-core binaries and client configuration files.
func Install(ctx context.Context, opts InstallOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	state, err := normalizeInstallOptions(opts)
	if err != nil {
		return err
	}

	if !state.Force {
		if exists, reason, err := clientInstallationPresent(state); err != nil {
			return err
		} else if exists {
			return fmt.Errorf("xp2p: client already installed (%s) (use --force to overwrite)", reason)
		}
	}

	logging.Info("xp2p client install starting",
		"install_dir", state.installDir,
		"config_dir", state.configDir,
		"server_address", state.serverRemote,
		"server_port", state.serverPort,
		"allow_insecure", state.AllowInsecure,
		"xray_version", xray.Version,
	)

	if err := os.MkdirAll(state.binDir, 0o755); err != nil {
		return fmt.Errorf("xp2p: create bin directory: %w", err)
	}
	if err := os.MkdirAll(state.configDir, 0o755); err != nil {
		return fmt.Errorf("xp2p: create config directory: %w", err)
	}

	if err := writeBinary(state); err != nil {
		return err
	}
	if err := deployConfiguration(state); err != nil {
		return err
	}
	if err := installstate.Write(state.stateFile, installstate.KindClient); err != nil {
		return fmt.Errorf("xp2p: write client state: %w", err)
	}

	logging.Info("xp2p client install completed", "install_dir", state.installDir)
	return nil
}

// Remove deletes installation files. When KeepFiles is true only existence is verified.
func Remove(ctx context.Context, opts RemoveOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	if opts.KeepFiles {
		logging.Info("xp2p client remove skipping files", "install_dir", installDir)
		return nil
	}

	if err := os.RemoveAll(installDir); err != nil {
		if opts.IgnoreMissing && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("xp2p: remove install directory: %w", err)
	}

	logging.Info("xp2p client files removed", "install_dir", installDir)
	return nil
}

func normalizeInstallOptions(opts InstallOptions) (installState, error) {
	dir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return installState{}, err
	}

	configDir, err := resolveConfigDir(dir, opts.ConfigDir)
	if err != nil {
		return installState{}, err
	}

	address := strings.TrimSpace(opts.ServerAddress)
	if address == "" {
		return installState{}, errors.New("xp2p: client server address is required")
	}

	portStr := strings.TrimSpace(opts.ServerPort)
	if portStr == "" {
		portStr = "8443"
	}
	portVal, err := strconv.Atoi(portStr)
	if err != nil || portVal <= 0 || portVal > 65535 {
		return installState{}, fmt.Errorf("xp2p: invalid client server port %q", portStr)
	}

	password := strings.TrimSpace(opts.Password)
	if password == "" {
		return installState{}, errors.New("xp2p: client password is required")
	}

	user := strings.TrimSpace(opts.User)
	if user == "" {
		return installState{}, errors.New("xp2p: client user email is required")
	}

	serverName := strings.TrimSpace(opts.ServerName)
	if serverName == "" {
		serverName = address
	}

	state := installState{
		InstallOptions: InstallOptions{
			InstallDir:    dir,
			ConfigDir:     opts.ConfigDir,
			ServerAddress: address,
			ServerPort:    portStr,
			User:          user,
			Password:      password,
			ServerName:    serverName,
			AllowInsecure: opts.AllowInsecure,
			Force:         opts.Force,
		},
		installDir:   dir,
		binDir:       filepath.Join(dir, "bin"),
		configDir:    configDir,
		xrayPath:     filepath.Join(dir, "bin", "xray.exe"),
		serverPort:   portVal,
		serverName:   serverName,
		serverRemote: address,
	}

	state.xrayPath = filepath.Join(state.binDir, "xray.exe")
	state.stateFile = filepath.Join(state.configDir, installstate.FileName)

	return state, nil
}

func resolveInstallDir(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "", errors.New("xp2p: install directory is required")
	}
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("xp2p: resolve install directory: %w", err)
	}

	if !isSafeInstallDir(abs) {
		return "", fmt.Errorf("xp2p: install directory %q is not allowed", abs)
	}

	return abs, nil
}

func resolveConfigDir(base, cfg string) (string, error) {
	cfg = strings.TrimSpace(cfg)
	if cfg == "" {
		cfg = DefaultClientConfigDir
	}
	if filepath.IsAbs(cfg) {
		return cfg, nil
	}
	return filepath.Join(base, cfg), nil
}

func isSafeInstallDir(path string) bool {
	clean := filepath.Clean(path)
	if clean == "." || clean == string(filepath.Separator) {
		return false
	}

	volume := filepath.VolumeName(clean)
	if volume != "" {
		root := volume + string(filepath.Separator)
		if strings.EqualFold(clean, root) {
			return false
		}
	}

	if strings.HasPrefix(clean, `\\`) {
		parts := strings.Split(clean[2:], `\`)
		if len(parts) < 3 {
			return false
		}
	}

	return true
}

func clientInstallationPresent(state installState) (bool, string, error) {
	marker, err := installstate.Read(state.stateFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("xp2p: read client state: %w", err)
	}
	if marker.Kind != installstate.KindClient {
		return false, "", fmt.Errorf("xp2p: unexpected install state kind %q in %s", marker.Kind, state.stateFile)
	}
	return true, fmt.Sprintf("state file %s", state.stateFile), nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func writeBinary(state installState) error {
	if !state.Force {
		if exists, err := pathExists(state.xrayPath); err != nil {
			return err
		} else if exists {
			return fmt.Errorf("xp2p: xray-core already present at %s (use --force to overwrite)", state.xrayPath)
		}
	}
	if err := os.WriteFile(state.xrayPath, xray.WindowsAMD64(), 0o755); err != nil {
		return fmt.Errorf("xp2p: write xray-core binary: %w", err)
	}
	return nil
}

func deployConfiguration(state installState) error {
	if err := writeEmbeddedFile("assets/templates/inbounds.json", filepath.Join(state.configDir, "inbounds.json"), 0o644); err != nil {
		return err
	}

	if err := renderTemplateToFile(
		"assets/templates/outbounds.json.tmpl",
		filepath.Join(state.configDir, "outbounds.json"),
		struct {
			ServerAddress string
			ServerPort    int
			Password      string
			AllowInsecure bool
			ServerName    string
			Email         string
		}{
			ServerAddress: state.serverRemote,
			ServerPort:    state.serverPort,
			Password:      state.Password,
			AllowInsecure: state.AllowInsecure,
			ServerName:    state.serverName,
			Email:         state.User,
		},
	); err != nil {
		return err
	}

	staticFiles := map[string]string{
		"assets/templates/logs.json":    filepath.Join(state.configDir, "logs.json"),
		"assets/templates/routing.json": filepath.Join(state.configDir, "routing.json"),
	}
	for src, dst := range staticFiles {
		if err := writeEmbeddedFile(src, dst, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func renderTemplateToFile(name, dest string, data any) error {
	content, err := clientTemplates.ReadFile(name)
	if err != nil {
		return fmt.Errorf("xp2p: load template %s: %w", name, err)
	}
	tmpl, err := templateFromBytes(name, content)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("xp2p: create config %s: %w", dest, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	if err := tmpl.Execute(writer, data); err != nil {
		return fmt.Errorf("xp2p: render template %s: %w", name, err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("xp2p: flush config %s: %w", dest, err)
	}

	return nil
}

func templateFromBytes(name string, content []byte) (*template.Template, error) {
	tmpl, err := template.New(filepath.Base(name)).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("xp2p: parse template %s: %w", name, err)
	}
	return tmpl, nil
}

func writeEmbeddedFile(name, dest string, perm os.FileMode) error {
	content, err := clientTemplates.ReadFile(name)
	if err != nil {
		return fmt.Errorf("xp2p: load template %s: %w", name, err)
	}
	if err := os.WriteFile(dest, content, perm); err != nil {
		return fmt.Errorf("xp2p: write template %s: %w", dest, err)
	}
	return nil
}
