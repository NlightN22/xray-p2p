package servercmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/netutil"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func (s *deployServer) proceedInstall(ctx context.Context, conn net.Conn, rw *bufio.ReadWriter, results chan<- runSignal, man spec.Manifest) {
	host := strings.TrimSpace(man.Host)
	if host == "" {
		host = strings.TrimSpace(s.Expected.Host)
	}
	if err := netutil.ValidateHost(host); err != nil {
		_ = writeLine(rw, "ERR invalid host")
		notifyFailure(results)
		return
	}

	installDir := strings.TrimSpace(man.InstallDir)
	if err := validateWindowsDeployInstallDir(installDir); err != nil {
		_ = writeLine(rw, "ERR "+err.Error())
		notifyFailure(results)
		return
	}
	if installDir == "" {
		installDir = strings.TrimSpace(s.Cfg.Server.InstallDir)
	}
	configDir := strings.TrimSpace(s.Cfg.Server.ConfigDir)
	if configDir == "" {
		configDir = server.DefaultServerConfigDir
	}
	resolvedConfigDir, err := server.ResolveConfigDir(installDir, configDir)
	if err != nil {
		_ = writeLine(rw, "ERR "+err.Error())
		notifyFailure(results)
		return
	}

	port := strings.TrimSpace(man.TrojanPort)
	if port == "" {
		port = strconv.Itoa(server.DefaultTrojanPort)
	}
	profile := strings.TrimSpace(man.Profile)
	if profile == "" {
		profile = strings.TrimSpace(s.Cfg.Server.Profile)
	}
	userID := strings.TrimSpace(man.TrojanUser)
	if userID == "" {
		userID = fmt.Sprintf("xp2p-%d@local", time.Now().Unix())
	}
	password := strings.TrimSpace(man.TrojanPassword)
	if password == "" {
		secret, err := generateDeployPassword(profile)
		if err != nil {
			_ = writeLine(rw, "ERR generate password failed")
			notifyFailure(results)
			return
		}
		password = secret
	}
	if err := clishared.ValidateRFC3986Unreserved(password); err != nil {
		_ = writeLine(rw, "ERR invalid password")
		notifyFailure(results)
		return
	}
	if err := validateDeployProfileCredential(profile, password); err != nil {
		_ = writeLine(rw, "ERR invalid password")
		notifyFailure(results)
		return
	}

	logs := []string{
		fmt.Sprintf("install_dir=%s", installDir),
		fmt.Sprintf("config_dir=%s", configDir),
		fmt.Sprintf("trojan_port=%s", port),
		fmt.Sprintf("profile=%s", profile),
		fmt.Sprintf("host=%s", host),
	}

	installed, err := clishared.InstallPresent(clishared.InstallRoleServer, installDir, configDir)
	if err != nil {
		_ = writeLine(rw, "ERR "+err.Error())
		notifyFailure(results)
		return
	}
	if installed {
		required := []string{}
		if liveXray, err := config.LiveXrayPath(apply.RoleServer); err != nil {
			_ = writeLine(rw, "ERR "+err.Error())
			notifyFailure(results)
			return
		} else {
			required = append(required, liveXray)
		}
		if liveMeta, err := config.LiveRuntimeMetaPath(apply.RoleServer); err != nil {
			_ = writeLine(rw, "ERR "+err.Error())
			notifyFailure(results)
			return
		} else {
			required = append(required, liveMeta)
		}
		for _, path := range required {
			if _, err := os.Stat(path); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					_ = writeLine(rw, "ERR server install incomplete: missing "+filepath.Base(path))
					notifyFailure(results)
					return
				}
				_ = writeLine(rw, "ERR "+err.Error())
				notifyFailure(results)
				return
			}
		}
	}

	inst := server.InstallOptions{
		InstallDir:            installDir,
		ConfigDir:             configDir,
		Port:                  port,
		Profile:               profile,
		CertificateStore:      strings.TrimSpace(s.Cfg.Server.CertificateStore),
		CertificateFile:       strings.TrimSpace(s.Cfg.Server.CertificateFile),
		KeyFile:               strings.TrimSpace(s.Cfg.Server.KeyFile),
		Host:                  host,
		Force:                 true,
		RelaxedPathValidation: true,
		TunEnabled:            false,
		TunEnabledSet:         true,
		TunName:               s.Cfg.Server.TunName,
		TunMTU:                s.Cfg.Server.TunMTU,
		TunAddr:               s.Cfg.Server.TunAddr,
	}
	if installed {
		logging.Info("xp2p server deploy: installation detected, running in append mode", "config", config.LiveConfigPath(layout.ServerConfigFileName))
		goto installDone
	}
	if err := server.Install(ctx, inst); err != nil {
		if server.IsCertificateValidationError(err) {
			logging.Warn("xp2p server deploy: certificate validation failed, using self-signed", "err", err)
			inst.CertificateStore = ""
			inst.CertificateFile = ""
			inst.KeyFile = ""
			s.Cfg.Server.CertificateStore = ""
			s.Cfg.Server.CertificateFile = ""
			s.Cfg.Server.KeyFile = ""
			_ = os.Unsetenv("XP2P_SERVER_CERTIFICATE")
			_ = os.Unsetenv("XP2P_SERVER_KEY")
			if _, updateErr := config.ClearServerCertificateOverrides(""); updateErr != nil {
				logging.Warn("xp2p server deploy: failed to clear certificate overrides", "err", updateErr)
			} else if req, reqErr := apply.NewRequest(apply.RoleServer); reqErr == nil {
				_ = apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath())
			}
			if retryErr := server.Install(ctx, inst); retryErr == nil {
				goto installDone
			} else {
				err = retryErr
			}
		}
		_ = writeLine(rw, "EXIT 1")
		_ = writeSegment(rw, "ERR-BEGIN", "ERR-END", []string{err.Error()})
		_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
		_ = writeLine(rw, "DONE")
		notifyFailure(results)
		return
	}
installDone:
	if !installed {
		if _, err := config.UpdateServerTrojanPortBestEffort("", port); err != nil {
			_ = writeLine(rw, "ERR "+err.Error())
			notifyFailure(results)
			return
		} else if req, reqErr := apply.NewRequest(apply.RoleServer); reqErr == nil {
			_ = apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath())
		}
	}

	if err := serverUserStageFunc(ctx, server.AddUserOptions{InstallDir: installDir, ConfigDir: configDir, UserID: userID, Password: password, Host: host, Force: true}); err != nil {
		_ = writeLine(rw, "EXIT 1")
		_ = writeSegment(rw, "ERR-BEGIN", "ERR-END", []string{err.Error()})
		_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
		_ = writeLine(rw, "DONE")
		notifyFailure(results)
		return
	}

	link, err := server.GetUserLink(ctx, server.UserLinkOptions{
		InstallDir: installDir,
		ConfigDir:  configDir,
		Host:       host,
		UserID:     userID,
		Pending:    true,
	})
	if err != nil || strings.TrimSpace(link.Link) == "" {
		_ = writeLine(rw, "EXIT 1")
		reason := "failed to build user link"
		if err != nil {
			reason = err.Error()
		}
		_ = writeSegment(rw, "ERR-BEGIN", "ERR-END", []string{reason})
		_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
		_ = writeLine(rw, "DONE")
		notifyFailure(results)
		return
	}

	liveConfigDir, err := config.LiveConfigDir(resolvedConfigDir)
	if err != nil {
		_ = writeLine(rw, "EXIT 1")
		_ = writeSegment(rw, "ERR-BEGIN", "ERR-END", []string{err.Error()})
		_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
		_ = writeLine(rw, "DONE")
		notifyFailure(results)
		return
	}
	plan, err := s.buildDeployRunPlan(ctx, liveConfigDir)
	if err != nil {
		_ = writeLine(rw, "EXIT 1")
		_ = writeSegment(rw, "ERR-BEGIN", "ERR-END", []string{err.Error()})
		_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
		_ = writeLine(rw, "DONE")
		notifyFailure(results)
		return
	}

	_ = writeLine(rw, "EXIT 0")
	_ = writeSegment(rw, "OUT-BEGIN", "OUT-END", logs)
	_ = writeLine(rw, "LINK "+link.Link)
	_ = writeLine(rw, "DONE")
	if results != nil {
		if plan.skipRun {
			logging.Info("xp2p server deploy: service active; skipping xray-core start", "config_dir", configDir)
		} else {
			logging.Info("xp2p server deploy: starting xray-core", "install_dir", installDir, "config_dir", plan.runConfigDir)
		}
		results <- runSignal{
			ok:           true,
			installDir:   installDir,
			configDir:    configDir,
			runConfigDir: plan.runConfigDir,
			cleanupDir:   plan.cleanupDir,
			skipRun:      plan.skipRun,
		}
	}
	s.waitForCompletion(conn, rw, results, installDir, configDir)
}
