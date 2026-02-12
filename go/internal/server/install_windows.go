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

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/installstate"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
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
	certDest   string
	keyDest    string
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
			return fmt.Errorf("xp2p: server files already present (%s) (use --force to overwrite)", reason)
		}
	}

	logging.Info("xp2p server install starting",
		"install_dir", state.installDir,
		"config_dir", state.configDir,
		"port", state.portValue,
		"host", state.Host,
	)

	if err := os.MkdirAll(state.binDir, 0o755); err != nil {
		return fmt.Errorf("xp2p: create bin directory: %w", err)
	}
	if err := os.MkdirAll(state.logsDir, 0o755); err != nil {
		return fmt.Errorf("xp2p: create logs directory: %w", err)
	}
	if err := os.MkdirAll(state.configDir, 0o755); err != nil {
		return fmt.Errorf("xp2p: create config directory: %w", err)
	}

	if _, err := config.EnsureTunSettings("", "server", state.TunEnabled, state.TunName, state.TunMTU, state.TunAddr); err != nil {
		return err
	}

	if err := ensureXrayBinaryPresent(state.xrayPath); err != nil {
		return err
	}
	if err := deployConfiguration(state); err != nil {
		return err
	}
	if err := installstate.Write(state.stateFile, installstate.KindServer); err != nil {
		return fmt.Errorf("xp2p: write server state: %w", err)
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

	configDir, err := resolveConfigDir(installDir, opts.ConfigDir)
	if err != nil {
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

	statePaths := []string{
		filepath.Join(installDir, installstate.FileNameForKind(installstate.KindServer)),
		filepath.Join(installDir, layout.StateFileName),
	}
	var lastErr error
	for idx, statePath := range statePaths {
		if err := installstate.Remove(statePath, installstate.KindServer); err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, installstate.ErrRoleNotInstalled) {
				lastErr = err
				if idx == len(statePaths)-1 && !opts.IgnoreMissing {
					return fmt.Errorf("xp2p: remove server state file: %w", err)
				}
				if opts.IgnoreMissing {
					continue
				}
				continue
			}
			return fmt.Errorf("xp2p: remove server state file: %w", err)
		} else {
			lastErr = nil
			break
		}
	}
	if lastErr != nil && !opts.IgnoreMissing {
		return fmt.Errorf("xp2p: remove server state file: %w", lastErr)
	}

	logging.Info("xp2p server configuration removed", "install_dir", installDir, "config_dir", configDir)
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
	base, err := buildServerInstallBase(dir, configDir, opts)
	if err != nil {
		return installState{}, err
	}

	state := installState{
		InstallOptions: base.installOpts,
		installDir:     base.installDir,
		binDir:         filepath.Join(dir, layout.BinDirName),
		logsDir:        filepath.Join(dir, layout.LogsDirName),
		configDir:      base.configDir,
		xrayPath:       filepath.Join(dir, layout.BinDirName, "xray.exe"),
		portValue:      base.portVal,
		selfSigned:     base.selfSigned,
		certSource:     base.certSource,
	}

	state.certDest = filepath.Join(state.configDir, "cert.pem")
	state.keyDest = filepath.Join(state.configDir, "key.pem")
	state.stateFile = filepath.Join(state.installDir, installstate.FileNameForKind(installstate.KindServer))

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

func deployConfiguration(state installState) error {
	var certPath string
	var keyPath string
	allowInsecure := false
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
			allowInsecure = true
		} else if state.certSource == CertificateSourcePath {
			selfSigned, err := isSelfSignedCertificatePath(state.CertificateFile)
			if err != nil {
				return err
			}
			allowInsecure = selfSigned
			certPath = filepath.ToSlash(state.CertificateFile)
			keyPath = filepath.ToSlash(state.KeyFile)
		}
		if certPath == "" && keyPath == "" {
			certPath = filepath.ToSlash(state.certDest)
			keyPath = filepath.ToSlash(state.keyDest)
		}
	}

	xrayCfg, err := ensureServerXrayConfig(filepath.Clean(config.ConfigPath(layout.ServerConfigFileName)))
	if err != nil {
		return err
	}
	if allowInsecure && !xrayCfg.Inbounds.Trojan.AllowInsecure {
		xrayCfg.Inbounds.Trojan.AllowInsecure = true
		if err := xrayconfig.SaveServerConfig(filepath.Clean(config.ConfigPath(layout.ServerConfigFileName)), config.AuditLogPath(), xrayCfg); err != nil {
			return err
		}
	}

	if err := writeServerInboundsConfig(state.configDir, xrayCfg, state.TunEnabled, state.TunName, state.TunMTU, state.portValue, certPath, keyPath, allowInsecure, nil); err != nil {
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

func ensureXrayBinaryPresent(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xp2p: xray binary is missing at %s (copy xray.exe into this directory before running install)", path)
		}
		return fmt.Errorf("xp2p: inspect xray binary at %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("xp2p: expected file at %s, found directory", path)
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
