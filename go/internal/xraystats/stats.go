package xraystats

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

const (
	FormatHuman = "human"
	FormatBytes = "bytes"
)

var execCommand = exec.CommandContext

// TrafficStats stores accumulated Xray user traffic counters.
type TrafficStats struct {
	UploadBytes   uint64
	DownloadBytes uint64
}

// TotalBytes returns upload + download with saturation.
func (s TrafficStats) TotalBytes() uint64 {
	if math.MaxUint64-s.UploadBytes < s.DownloadBytes {
		return math.MaxUint64
	}
	return s.UploadBytes + s.DownloadBytes
}

// QueryOptions controls Xray stats retrieval.
type QueryOptions struct {
	APIAddress string
	XrayBin    string
	InstallDir string
	Timeout    time.Duration
}

// QueryUserStats retrieves and parses Xray user stats.
func QueryUserStats(ctx context.Context, opts QueryOptions) (map[string]TrafficStats, error) {
	apiAddress := strings.TrimSpace(opts.APIAddress)
	if apiAddress == "" {
		return nil, errors.New("xray API address is empty")
	}
	xrayBin, err := ResolveBinary(opts.InstallDir, opts.XrayBin)
	if err != nil {
		return nil, err
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := execCommand(queryCtx, xrayBin, "api", "statsquery", "--server="+apiAddress)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("xray statsquery failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	stats, err := ParseUserStats(output)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// ResolveBinary resolves the xray binary path.
func ResolveBinary(installDir, override string) (string, error) {
	if path := strings.TrimSpace(override); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv("XP2P_XRAY_BIN")); path != "" {
		return path, nil
	}
	name := "xray"
	if runtime.GOOS == "windows" {
		name = "xray.exe"
	}
	if dir := strings.TrimSpace(installDir); dir != "" {
		path := filepath.Join(dir, layout.BinDirName, name)
		if stat, err := os.Stat(path); err == nil {
			if stat.IsDir() {
				return "", fmt.Errorf("%s is a directory, expected xray binary", path)
			}
			return path, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect xray binary at %s: %w", path, err)
		}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("xray binary not found (set --xray-bin or XP2P_XRAY_BIN): %w", err)
	}
	return path, nil
}

// APIListenFromXrayConfig reads api.listen from a compiled Xray config.
func APIListenFromXrayConfig(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var doc struct {
		API struct {
			Listen string `json:"listen"`
		} `json:"api"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("parse xray config %s: %w", path, err)
	}
	return strings.TrimSpace(doc.API.Listen), nil
}

// ParseUserStats parses Xray StatsService.QueryStats JSON output.
func ParseUserStats(data []byte) (map[string]TrafficStats, error) {
	var response struct {
		Stats []struct {
			Name  string `json:"name"`
			Value any    `json:"value"`
		} `json:"stat"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(data), &response); err != nil {
		return nil, fmt.Errorf("parse xray stats JSON: %w", err)
	}
	result := make(map[string]TrafficStats)
	for _, item := range response.Stats {
		user, direction, ok := parseStatName(item.Name)
		if !ok {
			continue
		}
		value, err := statValue(item.Value)
		if err != nil {
			return nil, fmt.Errorf("parse stat %q value: %w", item.Name, err)
		}
		key := NormalizeUser(user)
		stats := result[key]
		switch direction {
		case "uplink":
			stats.UploadBytes = value
		case "downlink":
			stats.DownloadBytes = value
		}
		result[key] = stats
	}
	return result, nil
}

// NormalizeUser returns a stable stats lookup key.
func NormalizeUser(user string) string {
	return strings.ToLower(strings.TrimSpace(user))
}

// NormalizeFormat validates and normalizes a display format.
func NormalizeFormat(format string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(format))
	if normalized == "" {
		return FormatHuman, nil
	}
	switch normalized {
	case FormatHuman, FormatBytes:
		return normalized, nil
	default:
		return "", fmt.Errorf("invalid xray stats format %q (use human or bytes)", format)
	}
}

func parseStatName(name string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(name), ">>>")
	if len(parts) != 4 {
		return "", "", false
	}
	if parts[0] != "user" || parts[2] != "traffic" {
		return "", "", false
	}
	direction := strings.TrimSpace(parts[3])
	if direction != "uplink" && direction != "downlink" {
		return "", "", false
	}
	user := strings.TrimSpace(parts[1])
	if user == "" {
		return "", "", false
	}
	return user, direction, true
}

func statValue(value any) (uint64, error) {
	switch v := value.(type) {
	case nil:
		return 0, nil
	case float64:
		if v < 0 {
			return 0, errors.New("negative value")
		}
		return uint64(v), nil
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, nil
		}
		return strconv.ParseUint(strings.TrimSpace(v), 10, 64)
	default:
		return 0, fmt.Errorf("unsupported type %T", value)
	}
}

// FormatTraffic formats traffic stats for table output.
func FormatTraffic(stats TrafficStats, format string) (upload, download, total string) {
	if strings.EqualFold(strings.TrimSpace(format), FormatBytes) {
		return strconv.FormatUint(stats.UploadBytes, 10),
			strconv.FormatUint(stats.DownloadBytes, 10),
			strconv.FormatUint(stats.TotalBytes(), 10)
	}
	return FormatByteCount(stats.UploadBytes), FormatByteCount(stats.DownloadBytes), FormatByteCount(stats.TotalBytes())
}

// FormatByteCount formats bytes with IEC units.
func FormatByteCount(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	size := float64(value)
	idx := -1
	for size >= unit && idx < len(units)-1 {
		size /= unit
		idx++
	}
	if size >= 10 {
		return fmt.Sprintf("%.0f %s", size, units[idx])
	}
	return fmt.Sprintf("%.1f %s", size, units[idx])
}
