package servercmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func validateWindowsDeployInstallDir(installDir string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	installDir = strings.TrimSpace(installDir)
	if installDir == "" {
		return nil
	}
	if strings.HasPrefix(installDir, "/") && !strings.HasPrefix(installDir, "//") {
		return fmt.Errorf("invalid install_dir for Windows: %q", installDir)
	}
	return nil
}

type deployRunPlan struct {
	runConfigDir string
	skipRun      bool
	cleanupDir   string
}

func (s *deployServer) buildDeployRunPlan(ctx context.Context, liveConfigDir string) (deployRunPlan, error) {
	status, err := servicecontrol.Default().Status(ctx, servicecontrol.RoleServer)
	if err == nil && status.Active {
		return deployRunPlan{skipRun: true}, nil
	}
	if err != nil && !errors.Is(err, servicecontrol.ErrUnsupported) {
		logging.Warn("xp2p server deploy: service status check failed", "err", err)
	}

	hasLive, err := server.HasRunConfigFiles(liveConfigDir)
	if err != nil {
		return deployRunPlan{}, err
	}
	if hasLive {
		return deployRunPlan{runConfigDir: liveConfigDir}, nil
	}

	configPath, err := config.DesiredConfigPathForRole(apply.RoleServer)
	if err != nil {
		return deployRunPlan{}, err
	}
	extensionsDir, err := config.DesiredExtensionsDirForRole(apply.RoleServer)
	if err != nil {
		return deployRunPlan{}, err
	}
	artifacts, err := server.CompileDesiredArtifacts(configPath, extensionsDir)
	if err != nil {
		return deployRunPlan{}, err
	}
	tmpDir, err := os.MkdirTemp("", "xp2p-server-deploy-")
	if err != nil {
		return deployRunPlan{}, fmt.Errorf("server deploy: create temp dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, layout.XrayConfigFileName), artifacts.XrayJSON, 0o644); err != nil {
		_ = os.RemoveAll(tmpDir)
		return deployRunPlan{}, fmt.Errorf("server deploy: write xray.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, layout.RuntimeMetaFileName), artifacts.RuntimeMetaJSON, 0o644); err != nil {
		_ = os.RemoveAll(tmpDir)
		return deployRunPlan{}, fmt.Errorf("server deploy: write runtime.json: %w", err)
	}
	for name, data := range artifacts.Extra {
		if strings.TrimSpace(name) == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(tmpDir, filepath.Clean(name)), data, 0o644); err != nil {
			_ = os.RemoveAll(tmpDir)
			return deployRunPlan{}, fmt.Errorf("server deploy: write artifact %s: %w", name, err)
		}
	}
	return deployRunPlan{runConfigDir: tmpDir, cleanupDir: tmpDir}, nil
}
