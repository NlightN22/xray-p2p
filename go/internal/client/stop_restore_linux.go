//go:build linux

package client

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func restoreFullTunnelOnStop(installDir, configDirName string) {
	paths, err := resolveClientPaths(installDir, configDirName)
	if err != nil {
		logging.Warn("full-tunnel restore on stop skipped", "err", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	desiredOS := DesiredOSState{FullTunnelVerbose: true}
	if liveDir, err := config.LiveRoleDir(apply.RoleClient); err == nil {
		if _, statErr := os.Stat(filepath.Join(strings.TrimSpace(liveDir), layout.RuntimeMetaFileName)); statErr == nil {
			if meta, metaErr := loadLiveRuntimeMeta(liveDir); metaErr == nil {
				desired := runtimeDesiredToClientInstallState(meta.Desired)
				desiredOS = DesiredOSState{
					TunEnabled:        meta.TunEnabled,
					TunName:           meta.TunName,
					TunAddr:           meta.TunAddr,
					TunMTU:            meta.TunMTU,
					TunMode:           meta.TunMode,
					DNSServers:        meta.DNSServers,
					FullTunnelVerbose: true,
					FullTunnelTag:     meta.FullTag,
					Install:           desired,
				}
			}
		}
	}

	orchestrator := NewOSStateOrchestrator(paths, newLinuxOSStateDriver(paths, RunOptions{FullTunnelVerbose: true}))
	if err := orchestrator.Rollback(ctx, RollbackReasonServiceStop, desiredOS); err != nil {
		logging.Warn("full-tunnel restore on stop failed", "err", err)
	}
}
