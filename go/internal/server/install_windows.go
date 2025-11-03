//go:build windows

package server

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/NlightN22/xray-p2p/go/assets/xray"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

const (
	socksInboundPort    = 51080
	dokodemoInboundPort = 48044
)

//go:embed assets/templates/*
var serverTemplates embed.FS

type installState struct {
	InstallOptions
	installDir string
	binDir     string
	configDir  string
	xrayPath   string
	certDest   string
	keyDest    string
	portValue  int
	selfSigned bool
}

// Install deploys xray-core binaries and configuration files.
func Install(ctx context.Context, opts InstallOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

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

	logging.Info("xp2p server install starting",
		"install_dir", state.installDir,
		"config_dir", state.configDir,
		"port", state.portValue,
		"host", state.Host,
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

	if err := os.RemoveAll(installDir); err != nil {
		if opts.IgnoreMissing && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("xp2p: remove install directory: %w", err)
	}

	logging.Info("xp2p server files removed", "install_dir", installDir)
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

	host := strings.TrimSpace(opts.Host)
	if host == "" {
		return installState{}, errors.New("xp2p: host is required")
	}
	if err := validateCertificateHost(host); err != nil {
		return installState{}, err
	}

	portStr := strings.TrimSpace(opts.Port)
	if portStr == "" {
		portStr = strconv.Itoa(DefaultTrojanPort)
	}
	portVal, err := strconv.Atoi(portStr)
	if err != nil || portVal <= 0 || portVal > 65535 {
		return installState{}, fmt.Errorf("xp2p: invalid port %q", portStr)
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
			ConfigDir:       opts.ConfigDir,
			Port:            portStr,
			CertificateFile: certSource,
			KeyFile:         keySource,
			Host:            host,
			Force:           opts.Force,
		},
		installDir: dir,
		binDir:     filepath.Join(dir, "bin"),
		configDir:  configDir,
		xrayPath:   filepath.Join(dir, "bin", "xray.exe"),
		portValue:  portVal,
	}

	state.certDest = filepath.Join(state.configDir, "cert.pem")
	state.keyDest = filepath.Join(state.configDir, "key.pem")

	if certSource == "" {
		state.selfSigned = true
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
	if err := os.WriteFile(state.xrayPath, xray.WindowsAMD64(), 0o755); err != nil {
		return fmt.Errorf("xp2p: write xray-core binary: %w", err)
	}
	return nil
}

func deployConfiguration(state installState) error {
	var certPath string
	var keyPath string
	if state.certDest != "" {
		if state.selfSigned {
			if err := generateSelfSignedCertificate(state); err != nil {
				return err
			}
		} else {
			mode := os.FileMode(0o644)
			if err := copyFile(state.CertificateFile, state.certDest, mode); err != nil {
				return fmt.Errorf("xp2p: copy certificate: %w", err)
			}

			keySource := state.KeyFile
			if keySource == "" {
				keySource = state.CertificateFile
			}
			if err := copyFile(keySource, state.keyDest, 0o600); err != nil {
				return fmt.Errorf("xp2p: copy key: %w", err)
			}
		}
		certPath = filepath.ToSlash(state.certDest)
		keyPath = filepath.ToSlash(state.keyDest)
	}

	data := struct {
		TrojanPort      int
		SocksPort       int
		DokodemoPort    int
		TLS             bool
		CertificateFile string
		KeyFile         string
	}{
		TrojanPort:      state.portValue,
		SocksPort:       socksInboundPort,
		DokodemoPort:    dokodemoInboundPort,
		TLS:             certPath != "",
		CertificateFile: certPath,
		KeyFile:         keyPath,
	}

	if err := renderTemplateToFile("assets/templates/inbounds.json.tmpl", filepath.Join(state.configDir, "inbounds.json"), data); err != nil {
		return err
	}
	staticFiles := map[string]string{
		"assets/templates/logs.json":      filepath.Join(state.configDir, "logs.json"),
		"assets/templates/outbounds.json": filepath.Join(state.configDir, "outbounds.json"),
		"assets/templates/routing.json":   filepath.Join(state.configDir, "routing.json"),
	}
	for src, dst := range staticFiles {
		if err := writeEmbeddedFile(src, dst, 0o644); err != nil {
			return err
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

func generateSelfSignedCertificate(state installState) error {
	logging.Info("xp2p server install generating self-signed certificate",
		"host", state.Host,
		"valid_years", 10,
		"destination", state.certDest,
	)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("xp2p: generate private key: %w", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return fmt.Errorf("xp2p: generate certificate serial: %w", err)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: state.Host,
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		SignatureAlgorithm:    x509.SHA256WithRSA,
	}

	if ip := net.ParseIP(state.Host); ip != nil {
		template.IPAddresses = []net.IP{ip}
		template.Subject.CommonName = ip.String()
	} else {
		template.DNSNames = []string{state.Host}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("xp2p: create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	if certPEM == nil {
		return errors.New("xp2p: encode certificate: empty result")
	}
	if err := os.WriteFile(state.certDest, certPEM, 0o644); err != nil {
		return fmt.Errorf("xp2p: write certificate: %w", err)
	}

	keyDER := x509.MarshalPKCS1PrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER})
	if keyPEM == nil {
		return errors.New("xp2p: encode private key: empty result")
	}
	if err := os.WriteFile(state.keyDest, keyPEM, 0o600); err != nil {
		return fmt.Errorf("xp2p: write private key: %w", err)
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
	if strings.HasSuffix(host, ".") {
		host = strings.TrimSuffix(host, ".")
	}
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
