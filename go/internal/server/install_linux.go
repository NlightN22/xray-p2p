//go:build linux

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

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/installstate"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/openwrt"
)

//go:embed assets/templates/*
var serverTemplates embed.FS

type installState struct {
	InstallOptions
	installDir string
	configDir  string
	logsDir    string
	certDest   string
	keyDest    string
	portValue  int
	selfSigned bool
	stateFile  string
	certSource string
}

// Install deploys server configuration files on Linux/OpenWrt hosts.
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
			return fmt.Errorf("xp2p: server files already present (%s) (use --force to overwrite)", reason)
		}
	}

	logging.Info("xp2p server install starting",
		"install_dir", state.installDir,
		"config_dir", state.configDir,
		"port", state.portValue,
		"host", state.Host,
	)

	if err := os.MkdirAll(state.configDir, 0o755); err != nil {
		return fmt.Errorf("xp2p: create config directory: %w", err)
	}
	logRoot := config.LogRoot()
	if err := os.MkdirAll(logRoot, 0o777); err != nil {
		return fmt.Errorf("xp2p: create log root: %w", err)
	}
	if err := os.Chmod(logRoot, 0o777); err != nil {
		logging.Warn("xp2p: chmod log root failed", "path", logRoot, "err", err)
	}
	if err := os.MkdirAll(state.logsDir, 0o777); err != nil {
		return fmt.Errorf("xp2p: create log directory: %w", err)
	}
	if err := os.Chmod(state.logsDir, 0o777); err != nil {
		logging.Warn("xp2p: chmod log directory failed", "path", state.logsDir, "err", err)
	}
	if _, err := config.EnsureTunSettings("", "server", state.TunEnabled, state.TunName, state.TunMTU, state.TunAddr); err != nil {
		if state.Force && errors.Is(err, config.ErrConfigParse) {
			configPath := config.ConfigPath(layout.ServerConfigFileName)
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
	if state.TunEnabled {
		if err := openwrt.EnsureTunInterface(state.TunName, state.TunAddr); err != nil {
			return err
		}
	}

	if err := deployConfiguration(state); err != nil {
		return err
	}
	if err := installstate.Write(state.stateFile, installstate.KindServer); err != nil {
		return fmt.Errorf("xp2p: write server state: %w", err)
	}
	desired, err := loadServerDesiredConfig(state.installDir)
	if err != nil {
		return err
	}
	if err := saveServerAppliedState(
		filepath.Clean(config.ConfigPath(layout.ServerAppliedStateFileName)),
		desired.Reverse,
		desired.Redirects,
		desired.Forwards,
		state.TunEnabled,
		state.TunName,
		state.TunMTU,
		state.TunAddr,
	); err != nil {
		return err
	}

	logging.Info("xp2p server install completed", "install_dir", state.installDir)
	return nil
}

// Remove deletes server configuration files. When KeepFiles is true only existence is verified.
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

	configDir, err := resolveConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}
	if err := removeNetworkdConfig(opts.TunName); err != nil {
		return err
	}
	if err := openwrt.RemoveTunInterfaceIfManaged(opts.TunName); err != nil {
		return err
	}

	if err := os.RemoveAll(configDir); err != nil {
		return fmt.Errorf("xp2p: remove server config dir: %w", err)
	}

	configPath := filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
	if err := os.Remove(configPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xp2p: remove server config file: %w", err)
		}
	}

	appliedPath := filepath.Clean(config.ConfigPath(layout.ServerAppliedStateFileName))
	if err := os.Remove(appliedPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xp2p: remove server applied state: %w", err)
		}
	}

	serverHeartbeatPath := filepath.Join(installDir, layout.ServerHeartbeatStateFileName)
	if err := os.Remove(serverHeartbeatPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("xp2p: remove server heartbeat state: %w", err)
	}

	statePath := filepath.Join(installDir, installstate.FileNameForKind(installstate.KindServer))
	if err := installstate.Remove(statePath, installstate.KindServer); err != nil {
		if !(opts.IgnoreMissing && (errors.Is(err, os.ErrNotExist) || errors.Is(err, installstate.ErrRoleNotInstalled))) {
			return fmt.Errorf("xp2p: remove server state file: %w", err)
		}
	}
	legacyStatePath := filepath.Join(installDir, layout.StateFileName)
	if err := installstate.Remove(legacyStatePath, installstate.KindServer); err != nil {
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, installstate.ErrRoleNotInstalled) {
			return fmt.Errorf("xp2p: remove legacy server state: %w", err)
		}
	}

	if !opts.KeepFiles {
		if err := removeInstallDirIfUnused(installDir); err != nil {
			return err
		}
	}

	logging.Info("xp2p server configuration removed", "install_dir", installDir, "config_dir", configDir)
	return nil
}

func removeInstallDirIfUnused(installDir string) error {
	clientStatePath := filepath.Join(installDir, installstate.FileNameForKind(installstate.KindClient))
	if installedRole(clientStatePath, installstate.KindClient) {
		return nil
	}
	serverStatePath := filepath.Join(installDir, installstate.FileNameForKind(installstate.KindServer))
	if installedRole(serverStatePath, installstate.KindServer) {
		return nil
	}
	legacyStatePath := filepath.Join(installDir, layout.StateFileName)
	if legacyHasRoles(legacyStatePath) {
		return nil
	}
	if dirHasFiles(filepath.Join(installDir, layout.BinDirName)) {
		return nil
	}
	if err := os.RemoveAll(installDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("xp2p: remove install dir: %w", err)
	}
	return nil
}

func dirHasFiles(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

func removeNetworkdConfig(tunName string) error {
	name := strings.TrimSpace(tunName)
	if name == "" {
		return nil
	}
	path := filepath.Join("/etc/systemd/network", fmt.Sprintf("90-%s.network", name))
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("xp2p: remove networkd config: %w", err)
	}
	return nil
}

func installedRole(path string, kind installstate.Kind) bool {
	if _, err := installstate.Read(path, kind); err == nil {
		return true
	} else if errors.Is(err, os.ErrNotExist) || errors.Is(err, installstate.ErrRoleNotInstalled) {
		return false
	}
	return true
}

func legacyHasRoles(path string) bool {
	roles, err := installstate.Roles(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		return true
	}
	return len(roles) > 0
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
	base, err := buildServerInstallBase(dir, configDir, opts)
	if err != nil {
		return installState{}, err
	}

	logsDir := filepath.Join(config.LogRoot(), "server")

	state := installState{
		InstallOptions: base.installOpts,
		installDir:     base.installDir,
		configDir:      base.configDir,
		logsDir:        logsDir,
		portValue:      base.portVal,
		selfSigned:     base.selfSigned,
		stateFile:      filepath.Join(dir, installstate.FileNameForKind(installstate.KindServer)),
		certSource:     base.certSource,
	}

	state.certDest = filepath.Join(state.configDir, "cert.pem")
	state.keyDest = filepath.Join(state.configDir, "key.pem")

	return state, nil
}

func resolveInstallDir(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "", errors.New("xp2p: install directory is required")
	}
	if !filepath.IsAbs(cleaned) {
		abs, err := filepath.Abs(cleaned)
		if err != nil {
			return "", fmt.Errorf("xp2p: resolve install directory: %w", err)
		}
		cleaned = abs
	}
	return cleaned, nil
}

func resolveConfigDir(base, cfg string) (string, error) {
	cfg = strings.TrimSpace(cfg)
	if cfg == "" {
		cfg = DefaultServerConfigDir
	}
	if filepath.IsAbs(cfg) {
		return cfg, nil
	}
	return filepath.Join(base, cfg), nil
}

func serverArtifactsPresent(state installState) (bool, string, error) {
	_, err := installstate.Read(state.stateFile, installstate.KindServer)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, installstate.ErrRoleNotInstalled) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("xp2p: read server state: %w", err)
	}
	inboundsPath := filepath.Join(state.configDir, "inbounds.json")
	if _, err := os.Stat(inboundsPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("xp2p: stat %s: %w", inboundsPath, err)
	}
	return true, fmt.Sprintf("state file %s", state.stateFile), nil
}

func deployConfiguration(state installState) error {
	var certPath string
	var keyPath string
	if state.certDest != "" {
		if state.selfSigned {
			logging.Info("xp2p server install generating self-signed certificate",
				"host", state.Host,
				"valid_years", 10,
				"destination", state.certDest,
			)
			if err := generateSelfSignedCertificate(state.Host, state.certDest, state.keyDest); err != nil {
				return err
			}
		} else if state.certSource == CertificateSourcePath {
			selfSigned, err := isSelfSignedCertificatePath(state.CertificateFile)
			if err != nil {
				return err
			}
			_ = selfSigned
			if err := copyFile(state.CertificateFile, state.certDest, 0o644); err != nil {
				return fmt.Errorf("xp2p: copy certificate: %w", err)
			}
			if err := copyFile(state.KeyFile, state.keyDest, 0o600); err != nil {
				return fmt.Errorf("xp2p: copy key: %w", err)
			}
			certPath = filepath.ToSlash(state.certDest)
			keyPath = filepath.ToSlash(state.keyDest)
		}
		if certPath == "" && keyPath == "" {
			certPath = filepath.ToSlash(state.certDest)
			keyPath = filepath.ToSlash(state.keyDest)
		}
	}

	xrayCfg, err := ensureServerXrayConfigForce(filepath.Clean(config.ConfigPath(layout.ServerConfigFileName)), state.Force)
	if err != nil {
		return err
	}
	if err := writeServerInboundsConfig(state.configDir, xrayCfg, state.TunEnabled, state.TunName, state.TunMTU, state.portValue, certPath, keyPath, false, nil); err != nil {
		return err
	}
	if err := writeServerLogs(state.configDir, xrayCfg.Logs); err != nil {
		return err
	}
	if err := writeServerOutbounds(state.configDir, xrayCfg.DirectOutbound); err != nil {
		return err
	}
	if err := writeServerRouting(state.configDir, xrayCfg, nil, nil); err != nil {
		return err
	}
	return nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	if strings.EqualFold(src, dst) {
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
	content, err := serverTemplates.ReadFile(name)
	if err != nil {
		return fmt.Errorf("xp2p: load template %s: %w", name, err)
	}
	if err := os.WriteFile(dest, content, perm); err != nil {
		return fmt.Errorf("xp2p: write template %s: %w", dest, err)
	}
	return nil
}

func validateCertificateHost(host string) error {
	if net.ParseIP(host) != nil {
		return nil
	}

	if len(host) > 253 {
		return fmt.Errorf("xp2p: invalid host %q", host)
	}

	// Allow optional trailing dot for FQDN and ignore it for validation.
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return fmt.Errorf("xp2p: invalid host")
	}

	labels := strings.Split(host, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return fmt.Errorf("xp2p: invalid host label %q", label)
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("xp2p: invalid host label %q", label)
		}
		for i := 0; i < len(label); i++ {
			ch := label[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' {
				continue
			}
			return fmt.Errorf("xp2p: invalid host label %q", label)
		}
	}
	return nil
}
