package deploy

import (
	"archive/zip"
	"embed"
	"errors"
	"fmt"
	"io"
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
	Timestamp  time.Time
}

// BuildPackage assembles a zip archive that contains install templates and
// configuration placeholders for a given remote host. It returns an absolute
// path to the created archive.
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
	archiveName := fmt.Sprintf("xp2p-client-%s-%s-%s.zip", pkgVersion, safeHost, timestamp.Format("20060102-150405"))
	archivePath := filepath.Join(outputDir, archiveName)

	stageDir, err := os.MkdirTemp("", "xp2p-package-*")
	if err != nil {
		return "", fmt.Errorf("xp2p: create staging directory: %w", err)
	}
	defer os.RemoveAll(stageDir)

	if err := copyTemplates(stageDir); err != nil {
		return "", err
	}

	if err := writePackageConfig(stageDir, host, pkgVersion, timestamp); err != nil {
		return "", err
	}

	if err := writeZipArchive(stageDir, archivePath, timestamp); err != nil {
		return "", err
	}

	return archivePath, nil
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

func writePackageConfig(dest, remoteHost, pkgVersion string, timestamp time.Time) error {
	manifest := spec.Manifest{
		RemoteHost:  remoteHost,
		XP2PVersion: pkgVersion,
		GeneratedAt: timestamp,
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

func writeZipArchive(stageDir, archivePath string, timestamp time.Time) error {
	if _, err := os.Stat(archivePath); err == nil {
		return fmt.Errorf("xp2p: archive %q already exists", archivePath)
	}

	file, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("xp2p: create archive %q: %w", archivePath, err)
	}
	defer func() {
		if err != nil {
			file.Close()
			os.Remove(archivePath)
		}
	}()

	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	zipWriter := zip.NewWriter(file)
	defer func() {
		if closeErr := zipWriter.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	err = filepath.WalkDir(stageDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("xp2p: walk stage directory %q: %w", path, walkErr)
		}
		if path == stageDir {
			return nil
		}

		relPath, err := filepath.Rel(stageDir, path)
		if err != nil {
			return fmt.Errorf("xp2p: compute archive entry for %q: %w", path, err)
		}
		archiveEntry := filepath.ToSlash(relPath)

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("xp2p: stat staged file %q: %w", path, err)
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("xp2p: build archive header for %q: %w", path, err)
		}
		header.Name = archiveEntry
		header.Method = zip.Deflate
		header.Modified = timestamp

		if entry.IsDir() {
			if !strings.HasSuffix(header.Name, "/") {
				header.Name += "/"
			}
			if _, err := zipWriter.CreateHeader(header); err != nil {
				return fmt.Errorf("xp2p: write archive directory %q: %w", header.Name, err)
			}
			return nil
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("xp2p: write archive entry %q: %w", header.Name, err)
		}

		if err := copyFileToWriter(path, writer); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	return err
}

func copyFileToWriter(path string, dst io.Writer) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("xp2p: open staged file %q: %w", path, err)
	}
	defer file.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return fmt.Errorf("xp2p: copy staged file %q: %w", path, err)
	}
	return nil
}
