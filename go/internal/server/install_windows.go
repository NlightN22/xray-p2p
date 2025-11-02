//go:build windows

package server

import (
	"bufio"
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	windowsapi "golang.org/x/sys/windows"
	winsvc "golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	windowsServiceName        = "xp2p-xray"
	windowsServiceDisplayName = "XP2P XRAY core"
	serviceStartTimeout       = 30 * time.Second
	serviceStopTimeout        = 30 * time.Second
)

//go:embed assets/bin/win-amd64/xray.exe
var xrayCoreWindowsAMD64 []byte

//go:embed assets/templates/*
var serverTemplates embed.FS

type installState struct {
	InstallOptions
	installDir string
	binDir     string
	configDir  string
	xrayPath   string
	configPath string
	certDest   string
	keyDest    string
	portValue  int
}

// Install deploys xray-core, configuration templates, and a Windows service.
func Install(ctx context.Context, opts InstallOptions) error {
	state, err := normalizeInstallOptions(opts)
	if err != nil {
		return err
	}

	if !state.Force {
		if exists, err := pathExists(state.installDir); err != nil {
			return err
		} else if exists {
			return fmt.Errorf("xp2p: installation directory %q already exists (use --force to overwrite)", state.installDir)
		}
	}

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

	if err := configureService(ctx, state); err != nil {
		return err
	}

	return nil
}

// Remove stops the Windows service and removes installed files.
func Remove(ctx context.Context, opts RemoveOptions) error {
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}

	if err := removeWindowsService(ctx, opts.IgnoreMissing); err != nil {
		return err
	}

	if opts.KeepFiles {
		return nil
	}

	if err := os.RemoveAll(installDir); err != nil {
		if opts.IgnoreMissing && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("xp2p: remove install directory: %w", err)
	}

	return nil
}

func normalizeInstallOptions(opts InstallOptions) (installState, error) {
	dir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return installState{}, err
	}

	portStr := strings.TrimSpace(opts.Port)
	if portStr == "" {
		portStr = DefaultPort
	}
	portVal, err := strconv.Atoi(portStr)
	if err != nil || portVal <= 0 || portVal > 65535 {
		return installState{}, fmt.Errorf("xp2p: invalid port %q", portStr)
	}

	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	if mode == "" {
		mode = "auto"
	}
	if mode != "auto" && mode != "manual" {
		return installState{}, fmt.Errorf("xp2p: unsupported server mode %q (want auto or manual)", opts.Mode)
	}

	certSource := strings.TrimSpace(opts.CertificateFile)
	keySource := strings.TrimSpace(opts.KeyFile)

	if certSource != "" {
		if err := ensureFileExists(certSource); err != nil {
			return installState{}, fmt.Errorf("xp2p: certificate: %w", err)
		}
		if keySource != "" {
			if err := ensureFileExists(keySource); err != nil {
				return installState{}, fmt.Errorf("xp2p: key: %w", err)
			}
		}
	}

	if certSource == "" && keySource != "" {
		return installState{}, errors.New("xp2p: key file provided without certificate file")
	}

	state := installState{
		InstallOptions: InstallOptions{
			InstallDir:      dir,
			Port:            portStr,
			Mode:            mode,
			CertificateFile: certSource,
			KeyFile:         keySource,
			Force:           opts.Force,
			StartService:    opts.StartService,
		},
		installDir: dir,
		binDir:     filepath.Join(dir, "bin"),
		configDir:  filepath.Join(dir, "config"),
		xrayPath:   filepath.Join(dir, "bin", "xray.exe"),
		configPath: filepath.Join(dir, "config", "xray-server.json"),
		portValue:  portVal,
	}

	if certSource != "" {
		state.certDest = filepath.Join(state.configDir, "cert.pem")
		state.keyDest = filepath.Join(state.configDir, "key.pem")
	}

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

	// Prevent UNC roots without subdirectories.
	if strings.HasPrefix(clean, `\\`) {
		parts := strings.Split(clean[2:], `\`)
		if len(parts) < 3 {
			return false
		}
	}

	return true
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

func ensureFileExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	return nil
}

func writeBinary(state installState) error {
	if !state.Force {
		if exists, err := pathExists(state.xrayPath); err != nil {
			return err
		} else if exists {
			return fmt.Errorf("xp2p: xray-core already present at %s (use --force to overwrite)", state.xrayPath)
		}
	}
	if err := os.WriteFile(state.xrayPath, xrayCoreWindowsAMD64, 0o755); err != nil {
		return fmt.Errorf("xp2p: write xray-core binary: %w", err)
	}
	return nil
}

func deployConfiguration(state installState) error {
	var certPath string
	var keyPath string
	if state.certDest != "" {
		mode := os.FileMode(0o644)
		if err := copyFile(state.CertificateFile, state.certDest, mode); err != nil {
			return fmt.Errorf("xp2p: copy certificate: %w", err)
		}
		certPath = filepath.ToSlash(state.certDest)

		keySource := state.KeyFile
		if keySource == "" {
			keySource = state.CertificateFile
		}
		if err := copyFile(keySource, state.keyDest, 0o600); err != nil {
			return fmt.Errorf("xp2p: copy key: %w", err)
		}
		keyPath = filepath.ToSlash(state.keyDest)
	}

	data := struct {
		Port            int
		TLS             bool
		CertificateFile string
		KeyFile         string
	}{
		Port:            state.portValue,
		TLS:             certPath != "",
		CertificateFile: certPath,
		KeyFile:         keyPath,
	}

	return renderTemplateToFile("assets/templates/xray-server.json.tmpl", state.configPath, data)
}

func copyFile(src, dst string, perm os.FileMode) error {
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return err
	}

	if strings.EqualFold(srcAbs, dstAbs) {
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return nil
}

func renderTemplateToFile(name, dest string, data any) error {
	content, err := serverTemplates.ReadFile(name)
	if err != nil {
		return fmt.Errorf("xp2p: load template %s: %w", name, err)
	}
	tmpl, err := template.New(filepath.Base(name)).Parse(string(content))
	if err != nil {
		return fmt.Errorf("xp2p: parse template %s: %w", name, err)
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

func configureService(ctx context.Context, state installState) error {
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("xp2p: connect service manager: %w", err)
	}
	defer manager.Disconnect()

	if err := ensureServiceAbsent(ctx, manager, state.Force); err != nil {
		return err
	}

	startType := mgr.StartAutomatic
	if state.Mode == "manual" {
		startType = mgr.StartManual
	}

	service, err := manager.CreateService(
		windowsServiceName,
		state.xrayPath,
		mgr.Config{
			DisplayName: windowsServiceDisplayName,
			Description: "XRAY core service managed by xp2p",
			StartType:   uint32(startType),
		},
		"-config",
		state.configPath,
	)
	if err != nil {
		return fmt.Errorf("xp2p: create service: %w", err)
	}
	defer service.Close()

	if !state.StartService {
		return nil
	}

	if err := startWindowsService(ctx, service); err != nil {
		return err
	}

	return nil
}

func ensureServiceAbsent(ctx context.Context, manager *mgr.Mgr, force bool) error {
	service, err := manager.OpenService(windowsServiceName)
	if err != nil {
		if errors.Is(err, windowsapi.ERROR_SERVICE_DOES_NOT_EXIST) {
			return nil
		}
		return fmt.Errorf("xp2p: open existing service: %w", err)
	}
	defer service.Close()

	if !force {
		return fmt.Errorf("xp2p: service %s already exists (use --force to overwrite)", windowsServiceName)
	}

	if err := stopWindowsService(ctx, service); err != nil {
		return err
	}

	if err := service.Delete(); err != nil {
		return fmt.Errorf("xp2p: delete existing service: %w", err)
	}

	return nil
}

func removeWindowsService(ctx context.Context, ignoreMissing bool) error {
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("xp2p: connect service manager: %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(windowsServiceName)
	if err != nil {
		if errors.Is(err, windowsapi.ERROR_SERVICE_DOES_NOT_EXIST) {
			if ignoreMissing {
				return nil
			}
			return fmt.Errorf("xp2p: service %s not found", windowsServiceName)
		}
		return fmt.Errorf("xp2p: open service: %w", err)
	}
	defer service.Close()

	if err := stopWindowsService(ctx, service); err != nil {
		return err
	}

	if err := service.Delete(); err != nil {
		return fmt.Errorf("xp2p: delete service: %w", err)
	}

	return nil
}

func stopWindowsService(ctx context.Context, service *mgr.Service) error {
	status, err := service.Control(winsvc.Stop)
	if err != nil {
		if errors.Is(err, windowsapi.ERROR_SERVICE_NOT_ACTIVE) {
			return nil
		}
		// If the service is already stopping, ignore.
		if errors.Is(err, windowsapi.ERROR_INVALID_SERVICE_CONTROL) {
			return nil
		}
		return fmt.Errorf("xp2p: stop service: %w", err)
	}

	// Service may already be stopped.
	if status.State == winsvc.Stopped {
		return nil
	}

	if err := waitForServiceState(ctx, service, winsvc.Stopped, serviceStopTimeout); err != nil {
		return err
	}
	return nil
}

func startWindowsService(ctx context.Context, service *mgr.Service) error {
	if err := service.Start(); err != nil {
		return fmt.Errorf("xp2p: start service: %w", err)
	}
	if err := waitForServiceState(ctx, service, winsvc.Running, serviceStartTimeout); err != nil {
		return err
	}
	return nil
}

func waitForServiceState(ctx context.Context, service *mgr.Service, target winsvc.State, timeout time.Duration) error {
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadlineCtx.Done():
			return fmt.Errorf("xp2p: service %s did not reach state %v: %w", windowsServiceName, target, deadlineCtx.Err())
		case <-ticker.C:
			status, err := service.Query()
			if err != nil {
				return fmt.Errorf("xp2p: query service status: %w", err)
			}
			switch status.State {
			case target:
				return nil
			case winsvc.Stopped:
				if target != winsvc.Stopped {
					return fmt.Errorf("xp2p: service %s stopped unexpectedly", windowsServiceName)
				}
				return nil
			}
		}
	}
}
