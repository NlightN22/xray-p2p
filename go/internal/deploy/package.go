package deploy

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
	"github.com/NlightN22/xray-p2p/go/internal/version"
)

// Embedded templates that are shipped with the binary and copied into client
// deployment packages.
//
//go:embed templates/*
var templatesFS embed.FS

var (
	ErrRemoteHostEmpty = errors.New("xp2p: remote host is required to build a package")

	packageNameSafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
)

// PackageOptions control how a deployment package is built.
type PackageOptions struct {
	RemoteHost string
	OutputDir  string
	Version    string
	InstallDir string
	TrojanUser string
	TrojanPass string
	Timestamp  time.Time
}

// BuildPackage assembles a directory that contains install templates and
// configuration placeholders for a given remote host. It returns an absolute
// path to the created directory.
func BuildPackage(opts PackageOptions) (string, error) {
	host := strings.TrimSpace(opts.RemoteHost)
	if host == "" {
		return "", ErrRemoteHostEmpty
	}

	pkgVersion := strings.TrimSpace(opts.Version)
	if pkgVersion == "" {
		pkgVersion = version.Current()
	}

	outputDir := strings.TrimSpace(opts.OutputDir)
	if outputDir == "" {
		var err error
		outputDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("xp2p: resolve working directory: %w", err)
		}
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("xp2p: ensure output directory %q: %w", outputDir, err)
	}

	timestamp := opts.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	} else {
		timestamp = timestamp.UTC()
	}

	safeHost := sanitizeForName(host)
	if safeHost == "" {
		safeHost = "remote"
	}
	dirName := fmt.Sprintf("xp2p-client-%s-%s-%s", pkgVersion, safeHost, timestamp.Format("20060102-150405"))
	finalPath := filepath.Join(outputDir, dirName)

	if _, err := os.Stat(finalPath); err == nil {
		return "", fmt.Errorf("xp2p: deployment directory %q already exists", finalPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("xp2p: stat deployment directory %q: %w", finalPath, err)
	}

	if err := os.MkdirAll(finalPath, 0o755); err != nil {
		return "", fmt.Errorf("xp2p: create deployment directory %q: %w", finalPath, err)
	}

	if err := copyTemplates(finalPath); err != nil {
		os.RemoveAll(finalPath)
		return "", err
	}

	if err := writePackageConfig(
		finalPath,
		host,
		pkgVersion,
		strings.TrimSpace(opts.InstallDir),
		strings.TrimSpace(opts.TrojanUser),
		strings.TrimSpace(opts.TrojanPass),
		timestamp,
	); err != nil {
		os.RemoveAll(finalPath)
		return "", err
	}

	return finalPath, nil
}

func sanitizeForName(value string) string {
	lowered := strings.ToLower(value)
	return strings.Trim(packageNameSafeChars.ReplaceAllString(lowered, "-"), "-.")
}

func copyTemplates(dest string) error {
	return fs.WalkDir(templatesFS, "templates", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("xp2p: read embedded template %q: %w", path, walkErr)
		}
		target := filepath.Join(dest, path)
		if entry.IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("xp2p: create directory %q: %w", target, err)
			}
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("xp2p: ensure directory %q: %w", filepath.Dir(target), err)
		}

		data, err := templatesFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("xp2p: read template %q: %w", path, err)
		}

		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("xp2p: write template %q: %w", target, err)
		}
		return nil
	})
}

func writePackageConfig(dest, remoteHost, pkgVersion, installDir, trojanUser, trojanPass string, timestamp time.Time) error {
	manifest := spec.Manifest{
		RemoteHost:  remoteHost,
		XP2PVersion: pkgVersion,
		GeneratedAt: timestamp,
		InstallDir:  installDir,
	}
	if strings.TrimSpace(trojanUser) != "" && strings.TrimSpace(trojanPass) != "" {
		manifest.TrojanUser = trojanUser
		manifest.TrojanPassword = trojanPass
	}

	configPath := filepath.Join(dest, "config", "deployment.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("xp2p: ensure config directory %q: %w", filepath.Dir(configPath), err)
	}

	file, err := os.OpenFile(configPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("xp2p: open config %q: %w", configPath, err)
	}
	defer file.Close()

	if err := spec.Write(file, manifest); err != nil {
		return err
	}
	return nil
}
