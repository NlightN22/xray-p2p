package xrayassets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

func Sync(ctx context.Context, cfg Config, opts Options) (returnErr error) {
	assetDir := strings.TrimSpace(opts.AssetDir)
	if assetDir == "" {
		return fmt.Errorf("xray asset preflight failed: asset directory is required")
	}
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		return fmt.Errorf("xray asset preflight failed: create asset directory %s: %w", assetDir, err)
	}
	files, err := normalizeFiles(cfg, opts.XrayConfigPath)
	if err != nil {
		return fmt.Errorf("xray asset preflight failed: %w", err)
	}
	client := opts.HTTPClient
	if client == nil {
		owned := ownedhttp.NewClient(ownedhttp.ClientOptions{Timeout: 30 * time.Second})
		defer func() {
			returnErr = errors.Join(returnErr, shutdownAssetClient(owned))
		}()
		client = owned
	}
	for _, file := range files {
		if err := syncFile(ctx, client, file, assetDir); err != nil {
			return err
		}
	}
	return nil
}

func normalizeFiles(cfg Config, xrayConfigPath string) ([]File, error) {
	byName := make(map[string]File, len(cfg.Files))
	for _, file := range cfg.Files {
		file.Name = strings.TrimSpace(file.Name)
		file.URL = strings.TrimSpace(file.URL)
		if err := ValidateName(file.Name); err != nil {
			return nil, err
		}
		byName[strings.ToLower(file.Name)] = file
	}
	required, err := RequiredFromXrayConfig(xrayConfigPath)
	if err != nil {
		return nil, err
	}
	for _, name := range required {
		if err := ValidateName(name); err != nil {
			return nil, err
		}
		key := strings.ToLower(name)
		if _, ok := byName[key]; !ok {
			byName[key] = File{Name: name}
		}
	}
	out := make([]File, 0, len(byName))
	for _, file := range byName {
		out = append(out, file)
	}
	return out, nil
}

func syncFile(ctx context.Context, client ownedhttp.Doer, file File, assetDir string) error {
	path, err := safePath(assetDir, file.Name)
	if err != nil {
		return fmt.Errorf("xray asset preflight failed: %w", err)
	}
	info, statErr := os.Stat(path)
	if statErr != nil && !os.IsNotExist(statErr) {
		return fmt.Errorf("xray asset preflight failed: inspect required asset %s in %s: %w", file.Name, assetDir, statErr)
	}
	if os.IsNotExist(statErr) {
		if file.URL == "" {
			return fmt.Errorf("xray asset preflight failed: required asset %s is missing in %s; configure xray_assets.files url or place the file manually", file.Name, assetDir)
		}
		if err := download(ctx, client, file.URL, path); err != nil {
			return fmt.Errorf("xray asset preflight failed: required asset %s is missing in %s and download failed: %w", file.Name, assetDir, err)
		}
		logging.Info("xray asset downloaded", "file", file.Name, "url", file.URL)
		return nil
	}
	if info.IsDir() {
		return fmt.Errorf("xray asset preflight failed: required asset %s is a directory in %s", file.Name, assetDir)
	}
	if file.StaleAfter <= 0 || time.Since(info.ModTime()) <= file.StaleAfter {
		return nil
	}
	if file.URL == "" {
		return nil
	}
	if err := download(ctx, client, file.URL, path); err != nil {
		logging.Warn("xray asset refresh failed; using existing file", "file", file.Name, "age", time.Since(info.ModTime()).Round(time.Second).String(), "err", err)
		return nil
	}
	logging.Info("xray asset refreshed", "file", file.Name, "url", file.URL)
	return nil
}

func download(ctx context.Context, client ownedhttp.Doer, url, target string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	defer ownedhttp.DrainAndClose(resp, maxDownloadSize+1)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("download %s: unexpected HTTP status %s", url, resp.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary asset file: %w", err)
	}
	tmpPath := tmp.Name()
	removeTmp := true
	defer func() {
		if removeTmp {
			_ = os.Remove(tmpPath)
		}
	}()
	limited := io.LimitReader(resp.Body, maxDownloadSize+1)
	written, copyErr := io.Copy(tmp, limited)
	closeErr := tmp.Close()
	if copyErr != nil {
		return fmt.Errorf("download %s: %w", url, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("write temporary asset file: %w", closeErr)
	}
	if written > maxDownloadSize {
		return fmt.Errorf("download %s: asset exceeds %d bytes", url, maxDownloadSize)
	}
	if err := replaceFile(tmpPath, target); err != nil {
		return fmt.Errorf("replace asset file: %w", err)
	}
	removeTmp = false
	return nil
}

func shutdownAssetClient(client ownedhttp.OwnedClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown xray asset HTTP client: %w", err)
	}
	return nil
}
