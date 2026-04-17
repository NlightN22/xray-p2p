//go:build windows

package server

import (
	"bufio"
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/extensions"
	"github.com/NlightN22/xray-p2p/go/internal/installstate"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/service/control"
)

//go:embed assets/templates/*
var serverTemplates embed.FS

type installState struct {
	InstallOptions
	installDir string
	binDir     string
	logsDir    string
	configDir  string
	xrayPath   string
	portValue  int
	selfSigned bool
	stateFile  string
	certSource string
}

// Install deploys server configuration files.
func Install(ctx context.Context, opts InstallOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	state, err := normalizeInstallOptions(opts)
	if err != nil {
		return err
	}

	if !state.Force {
		if occupied, reason, err := serverArtifactsPresent(state); err != nil {
			return err
		} else if occupied {
			return fmt.Errorf("server files already present (%s) (use --force to overwrite)", reason)
		}
	}

	logging.Info("xp2p server install starting",
		"install_dir", state.installDir,
		"config_dir", state.configDir,
		"port", state.portValue,
		"host", state.Host,
	)

	if err := os.MkdirAll(state.binDir, 0o755); err != nil {
		return fmt.Errorf("create bin directory: %w", err)
	}
	if err := os.MkdirAll(state.logsDir, 0o755); err != nil {
		return fmt.Errorf("create logs directory: %w", err)
	}
	if err := os.MkdirAll(state.configDir, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	if _, err := config.EnsureTunSettings("", "server", state.TunEnabled, state.TunName, state.TunMTU, state.TunAddr); err != nil {
		if state.Force && errors.Is(err, config.ErrConfigParse) {
			configPath := config.PendingConfigPath(layout.ServerConfigFileName)
			if removeErr := os.Remove(configPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return removeErr
			}
			if _, retryErr := config.EnsureTunSettings("", "server", state.TunEnabled, state.TunName, state.TunMTU, state.TunAddr); retryErr != nil {
				return retryErr
			}
		} else {
			return err
		}
	}

	if err := ensureXrayBinaryPresent(state.xrayPath); err != nil {
		return err
	}
	if err := deployDesiredConfiguration(state); err != nil {
		return err
	}
	if err := installstate.Write(state.stateFile, installstate.KindServer); err != nil {
		return fmt.Errorf("write server state: %w", err)
	}
	req, err := apply.NewRequest(apply.RoleServer)
	if err != nil {
		return err
	}
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath()); err != nil {
		return err
	}

	logging.Info("xp2p server install completed", "install_dir", state.installDir)
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
		logging.Info("xp2p server remove skipping files", "install_dir", installDir)
		return nil
	}

	if err := ensureServiceInactive(ctx, control.RoleServer, "xp2p server service stop"); err != nil {
		return err
	}
	if err := apply.RemoveRoleMarkers(config.ApplyRequestPath(), config.ApplyErrorPath(), apply.RoleServer); err != nil {
		return err
	}

	configDir, err := config.DesiredExtensionsDirForRole(apply.RoleServer)
	if err != nil {
		return err
	}
	liveDir, err := config.LiveRoleDir(apply.RoleServer)
	if err != nil {
		return err
	}
	lkgDir, err := config.LkgRoleDir(apply.RoleServer)
	if err != nil {
		return err
	}

	if err := os.RemoveAll(configDir); err != nil {
		return fmt.Errorf("remove server config dir: %w", err)
	}
	if err := os.RemoveAll(liveDir); err != nil {
		return fmt.Errorf("remove server live dir: %w", err)
	}
	if err := os.RemoveAll(lkgDir); err != nil {
		return fmt.Errorf("remove server lkg dir: %w", err)
	}

	configPath := filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
	if err := os.Remove(configPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove server config file: %w", err)
		}
	}

	appliedPath := filepath.Clean(config.ConfigPath(layout.ServerAppliedStateFileName))
	if err := os.Remove(appliedPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove server applied state: %w", err)
		}
	}

	statePaths := []string{
		filepath.Clean(config.ConfigPath(layout.ServerStateFileName)),
		filepath.Clean(config.ConfigPath(layout.StateFileName)),
	}
	var lastErr error
	for idx, statePath := range statePaths {
		if err := installstate.Remove(statePath, installstate.KindServer); err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, installstate.ErrRoleNotInstalled) {
				lastErr = err
				if idx == len(statePaths)-1 && !opts.IgnoreMissing {
					return fmt.Errorf("remove server state file: %w", err)
				}
				if opts.IgnoreMissing {
					continue
				}
				continue
			}
			return fmt.Errorf("remove server state file: %w", err)
		} else {
			lastErr = nil
			break
		}
	}
	if lastErr != nil && !opts.IgnoreMissing {
		return fmt.Errorf("remove server state file: %w", lastErr)
	}

	logging.Info("xp2p server configuration removed", "install_dir", installDir, "config_dir", configDir)
	return nil
}

func normalizeInstallOptions(opts InstallOptions) (installState, error) {
	dir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return installState{}, err
	}

	configDir, err := config.DesiredExtensionsDirForRole(apply.RoleServer)
	if err != nil {
		return installState{}, err
	}
	base, err := buildServerInstallBase(dir, configDir, opts)
	if err != nil {
		return installState{}, err
	}

	state := installState{
		InstallOptions: base.installOpts,
		installDir:     base.installDir,
		binDir:         filepath.Join(dir, layout.BinDirName),
		logsDir:        config.LogRoot(),
		configDir:      base.configDir,
		xrayPath:       filepath.Join(dir, layout.BinDirName, "xray.exe"),
		portValue:      base.portVal,
		selfSigned:     base.selfSigned,
		certSource:     base.certSource,
	}

	state.stateFile = filepath.Clean(config.ConfigPath(layout.ServerStateFileName))

	return state, nil
}

func resolveInstallDir(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "", errors.New("install directory is required")
	}
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("resolve install directory: %w", err)
	}

	if !isSafeInstallDir(abs) {
		return "", fmt.Errorf("install directory %q is not allowed", abs)
	}

	return abs, nil
}

func ResolveConfigDir(base, cfg string) (string, error) {
	cfg = strings.TrimSpace(cfg)
	if cfg == "" {
		cfg = DefaultServerConfigDir
	}
	if filepath.IsAbs(cfg) {
		return cfg, nil
	}
	return filepath.Join(config.ConfigRoot(), cfg), nil
}

func serverArtifactsPresent(state installState) (bool, string, error) {
	if _, err := installstate.Read(state.stateFile, installstate.KindServer); err == nil {
		return true, fmt.Sprintf("state file %s", state.stateFile), nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, installstate.ErrRoleNotInstalled) {
		return false, "", fmt.Errorf("read server state: %w", err)
	}

	desiredPath := filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
	if _, err := os.Stat(desiredPath); err == nil {
		return true, fmt.Sprintf("desired config %s", desiredPath), nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, "", fmt.Errorf("stat %s: %w", desiredPath, err)
	}

	liveXray, err := config.LiveXrayPath(apply.RoleServer)
	if err != nil {
		return false, "", err
	}
	if _, err := os.Stat(liveXray); err == nil {
		return true, fmt.Sprintf("live artifact %s", liveXray), nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, "", fmt.Errorf("stat %s: %w", liveXray, err)
	}
	return false, "", nil
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

func deployDesiredConfiguration(state installState) error {
	if _, err := ensureServerXrayConfigForce(pendingConfigPath(), state.Force); err != nil {
		return err
	}
	if err := extensions.EnsureTemplates(state.configDir); err != nil {
		return err
	}
	if state.selfSigned || state.certSource == CertificateSourcePath {
		if err := os.MkdirAll(defaultTLSDir(), 0o755); err != nil {
			return fmt.Errorf("create tls dir: %w", err)
		}
		if state.selfSigned {
			logging.Info("xp2p server install generating self-signed certificate",
				"host", state.Host,
				"valid_years", 10,
				"destination", defaultCertPath(),
			)
			if err := generateSelfSignedCertificate(state.Host, defaultCertPath(), defaultKeyPath()); err != nil {
				return err
			}
		} else {
			if err := copyFile(state.CertificateFile, defaultCertPath(), 0o644); err != nil {
				return fmt.Errorf("copy certificate: %w", err)
			}
			if err := copyFile(state.KeyFile, defaultKeyPath(), 0o600); err != nil {
				return fmt.Errorf("copy key: %w", err)
			}
		}
	}
	return nil
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
		return fmt.Errorf("load template %s: %w", name, err)
	}
	tmpl, err := templateFromBytes(name, content)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create config %s: %w", dest, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	if err := tmpl.Execute(writer, data); err != nil {
		return fmt.Errorf("render template %s: %w", name, err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush config %s: %w", dest, err)
	}

	return nil
}

func templateFromBytes(name string, content []byte) (*template.Template, error) {
	tmpl, err := template.New(filepath.Base(name)).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}
	return tmpl, nil
}

func writeEmbeddedFile(name, dest string, perm os.FileMode) error {
	content, err := serverTemplates.ReadFile(name)
	if err != nil {
		return fmt.Errorf("load template %s: %w", name, err)
	}
	if err := os.WriteFile(dest, content, perm); err != nil {
		return fmt.Errorf("write template %s: %w", dest, err)
	}
	return nil
}

func ensureXrayBinaryPresent(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xray binary is missing at %s (copy xray.exe into this directory before running install)", path)
		}
		return fmt.Errorf("inspect xray binary at %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("expected file at %s, found directory", path)
	}
	return nil
}

func validateCertificateHost(host string) error {
	if net.ParseIP(host) != nil {
		return nil
	}

	if len(host) > 253 {
		return fmt.Errorf("invalid host %q", host)
	}

	// Allow optional trailing dot for FQDN and ignore it for validation.
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return fmt.Errorf("invalid host")
	}

	labels := strings.Split(host, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return fmt.Errorf("invalid host label %q", label)
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("invalid host label %q", label)
		}
		for i := 0; i < len(label); i++ {
			ch := label[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' {
				continue
			}
			return fmt.Errorf("invalid host label %q", label)
		}
	}
	return nil
}
