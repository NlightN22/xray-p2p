//go:build linux

package natredirect

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/firewall"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func promptYes() bool {
	fmt.Print(promptYesMessage)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}

func promptSelectPort(ports []int) (int, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Select port number: ")
		raw, _ := reader.ReadString('\n')
		raw = strings.TrimSpace(raw)
		val, err := strconv.Atoi(raw)
		if err == nil {
			for _, p := range ports {
				if p == val {
					return val, nil
				}
			}
		}
		fmt.Println("Invalid selection.")
	}
}

func printPlan(plan firewall.Plan) {
	if len(plan.Snippet) > 0 {
		fmt.Printf("Planned nftables snippet (%s):\n%s\n", plan.SnippetPath, plan.Snippet)
	}
	if len(plan.IPTables) > 0 {
		fmt.Println("Planned iptables commands:")
		for _, line := range plan.IPTables {
			fmt.Println(line)
		}
	}
	if plan.EntryPath != "" {
		fmt.Printf("Entry file would be written to %s\n", plan.EntryPath)
	}
}

func removeTarget(opts removeOptions) string {
	if opts.all {
		return "all"
	}
	return opts.cidr
}

func fallback(value, def string) string {
	trim := strings.TrimSpace(value)
	if trim == "" {
		return def
	}
	return trim
}

type exitError struct {
	code int
}

func (e exitError) Error() string {
	return fmt.Sprintf("exit %d", e.code)
}

func (e exitError) ExitCode() int {
	return e.code
}

func detectDefaultPaths() (string, string) {
	candidates := []string{
		"/etc/nftables.d",
		filepath.Join(layout.UnixConfigRoot, "nftables"),
	}
	for _, base := range candidates {
		dir := strings.TrimSpace(base)
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		return filepath.Join(dir, "xray-transparent.nft"), filepath.Join(dir, "xray-transparent.d")
	}
	if commandExists("fw4") {
		return "/etc/nftables.d/xray-transparent.nft", "/etc/nftables.d/xray-transparent.d"
	}
	return "/etc/nftables.d/xray-transparent.nft", "/etc/nftables.d/xray-transparent.d"
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func autodetectPorts(inboundsFlag string, quiet bool) ([]int, error) {
	seen := map[int]struct{}{}
	var ports []int
	candidates := []string{strings.TrimSpace(inboundsFlag)}
	if strings.TrimSpace(inboundsFlag) == "" {
		candidates = []string{
			defaultInbounds,
			layout.UnixConfigRoot + "/" + layout.ServerConfigDir + "/inbounds.json",
		}
	}
	for _, path := range candidates {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		paths := []string{trimmed}
		if pending := pendingInboundsPath(trimmed); pending != trimmed {
			paths = append([]string{pending}, paths...)
		}
		for _, candidate := range paths {
			info, err := os.Stat(candidate)
			if err != nil || info.IsDir() {
				continue
			}
			detected, err := firewall.DetectDokodemoPorts(candidate, true)
			if err != nil {
				continue
			}
			for _, p := range detected {
				if _, ok := seen[p]; ok {
					continue
				}
				seen[p] = struct{}{}
				ports = append(ports, p)
			}
		}
	}
	if len(ports) == 0 {
		return nil, fmt.Errorf("nat-redirect add: no dokodemo-door ports found")
	}
	if len(ports) == 1 {
		return ports, nil
	}
	if quiet {
		return ports, nil
	}
	fmt.Printf("Detected dokodemo-door ports: %v\n", ports)
	selected, err := promptSelectPort(ports)
	if err != nil {
		return nil, err
	}
	return []int{selected}, nil
}

func ensureProxyMode(cfg config.Config, inboundsPath string) error {
	path := strings.TrimSpace(inboundsPath)
	if path != "" {
		candidate := pendingInboundsPath(path)
		checked := path
		if candidate != "" && candidate != path {
			if _, err := os.Stat(candidate); err == nil {
				checked = candidate
			}
		}
		if hasTun, err := inboundsHasTun(checked); err == nil && hasTun {
			return fmt.Errorf("xp2p: nat-redirect is available only in proxy mode (disable tun to proceed)")
		}
	}
	clientTun, serverTun, err := resolveTunState(cfg)
	if err != nil {
		return err
	}
	if clientTun && serverTun {
		return fmt.Errorf("xp2p: nat-redirect is available only in proxy mode (set tun_enabled=false)")
	}
	return nil
}

func resolveTunState(cfg config.Config) (bool, bool, error) {
	clientTun := cfg.Client.TunEnabled
	serverTun := cfg.Server.TunEnabled

	clientPending := filepath.Clean(config.PendingConfigPath(layout.ClientConfigFileName))
	if clientPending != "" {
		if _, err := os.Stat(clientPending); err == nil {
			pendingCfg, err := config.Load(config.Options{Path: clientPending})
			if err != nil {
				return false, false, err
			}
			clientTun = pendingCfg.Client.TunEnabled
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, false, err
		}
	}

	serverPending := filepath.Clean(config.PendingConfigPath(layout.ServerConfigFileName))
	if serverPending != "" {
		if _, err := os.Stat(serverPending); err == nil {
			pendingCfg, err := config.Load(config.Options{Path: serverPending})
			if err != nil {
				return false, false, err
			}
			serverTun = pendingCfg.Server.TunEnabled
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, false, err
		}
	}

	return clientTun, serverTun, nil
}

func pendingInboundsPath(path string) string {
	cleaned := filepath.Clean(path)
	pendingRoot := filepath.Clean(config.PendingRoot())
	if strings.HasPrefix(cleaned, pendingRoot+string(os.PathSeparator)) || cleaned == pendingRoot {
		return cleaned
	}
	dir := filepath.Dir(cleaned)
	base := filepath.Base(cleaned)
	if strings.HasSuffix(dir, string(os.PathSeparator)+layout.ClientConfigDir) || strings.HasSuffix(dir, string(os.PathSeparator)+layout.ServerConfigDir) {
		pendingDir, err := config.PendingConfigDir(dir)
		if err != nil {
			return cleaned
		}
		return filepath.Join(pendingDir, base)
	}
	return cleaned
}

func inboundsHasTun(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if info.IsDir() {
		return false, fmt.Errorf("xp2p: inbounds path %s is a directory", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var doc struct {
		Inbounds []struct {
			Protocol string `json:"protocol"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return false, err
	}
	for _, inbound := range doc.Inbounds {
		if strings.EqualFold(strings.TrimSpace(inbound.Protocol), "tun") {
			return true, nil
		}
	}
	return false, nil
}
