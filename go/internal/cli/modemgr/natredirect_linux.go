//go:build linux

package modemgr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/firewall"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

// ApplyNatRedirectMode updates nat-redirect rules for the selected mode.
func ApplyNatRedirectMode(mode string) error {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "tun":
		return disableNatRedirect()
	case "proxy":
		return enableNatRedirect()
	default:
		return fmt.Errorf("unsupported mode %q", mode)
	}
}

func enableNatRedirect() error {
	snippet, entryDir := detectDefaultPaths()
	manager := firewall.NewManager(snippet, entryDir)
	entries, err := manager.List()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	plan, err := manager.PlanAdd(entries[0].CIDR, entries[0].Port)
	if err != nil {
		return err
	}
	_, err = manager.ApplyPlan(plan)
	return err
}

func disableNatRedirect() error {
	snippet, entryDir := detectDefaultPaths()
	manager := firewall.NewManager(snippet, entryDir)
	plan, err := manager.PlanRemove("", true)
	if err != nil {
		return err
	}
	_, err = manager.ApplyPlan(plan)
	return err
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
