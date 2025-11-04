package clientcmd

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestParseDeployFlagsUsesDefaults(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir: `C:\remote`,
			ConfigDir:  "cfg-server",
			Host:       "edge.example.test",
			Port:       "62022",
		},
		Client: config.ClientConfig{
			InstallDir: `C:\local`,
			ConfigDir:  "cfg-client",
			User:       "user@example.test",
			Password:   "hunter2",
		},
	}

	opts, err := parseDeployFlags(cfg, []string{"--remote-host", "gateway.internal"})
	if err != nil {
		t.Fatalf("parseDeployFlags: %v", err)
	}

	if opts.remoteHost != "gateway.internal" {
		t.Fatalf("remoteHost mismatch: got %q", opts.remoteHost)
	}
	if opts.serverHost != "edge.example.test" {
		t.Fatalf("serverHost mismatch: got %q", opts.serverHost)
	}
	if opts.serverPort != "58443" {
		t.Fatalf("serverPort mismatch: got %q", opts.serverPort)
	}
	if opts.trojanUser != "user@example.test" {
		t.Fatalf("trojanUser mismatch: got %q", opts.trojanUser)
	}
	if opts.trojanPassword != "hunter2" {
		t.Fatalf("trojanPassword mismatch: got %q", opts.trojanPassword)
	}
	if opts.remoteInstallDir != `C:\remote` {
		t.Fatalf("remoteInstallDir mismatch: got %q", opts.remoteInstallDir)
	}
	if opts.remoteConfigDir != "cfg-server" {
		t.Fatalf("remoteConfigDir mismatch: got %q", opts.remoteConfigDir)
	}
	if opts.localInstallDir != filepath.Clean(`C:\local`) {
		t.Fatalf("localInstallDir mismatch: got %q", opts.localInstallDir)
	}
	if opts.localConfigDir != "cfg-client" {
		t.Fatalf("localConfigDir mismatch: got %q", opts.localConfigDir)
	}
	if opts.packageOnly {
		t.Fatalf("packageOnly expected false")
	}
}

func TestParseDeployFlagsRejectsInvalidHost(t *testing.T) {
	cfg := config.Config{
		Client: config.ClientConfig{
			User:     "user@example.test",
			Password: "secret",
		},
	}
	if _, err := parseDeployFlags(cfg, []string{"--remote-host"}); err == nil {
		t.Fatalf("expected error when --remote-host has no value")
	}
	if _, err := parseDeployFlags(cfg, []string{"--remote-host", "--package-only"}); err == nil {
		t.Fatalf("expected error when --remote-host value looks like a flag")
	}
	if _, err := parseDeployFlags(cfg, []string{"--remote-host", "bad host"}); err == nil {
		t.Fatalf("expected error when --remote-host is invalid")
	}
}
func TestParseDeployFlagsPromptsForUser(t *testing.T) {
	cfg := config.Config{}
	restore := stubPromptString(t, func(string) (string, error) {
		return "prompt@example.test", nil
	})
	defer restore()

	opts, err := parseDeployFlags(cfg, []string{"--remote-host", "gateway.internal"})
	if err != nil {
		t.Fatalf("parseDeployFlags: %v", err)
	}
	if opts.trojanUser != "prompt@example.test" {
		t.Fatalf("trojanUser mismatch: got %q", opts.trojanUser)
	}
}

func TestParseDeployFlagsPackageOnly(t *testing.T) {
	cfg := config.Config{}

	opts, err := parseDeployFlags(cfg, []string{"--remote-host", "gateway.internal", "--package-only"})
	if err != nil {
		t.Fatalf("parseDeployFlags: %v", err)
	}
	if !opts.packageOnly {
		t.Fatalf("packageOnly expected true")
	}
	if opts.trojanUser != "client@example.invalid" {
		t.Fatalf("trojanUser mismatch: %q", opts.trojanUser)
	}
	if opts.trojanPassword != "placeholder-secret" {
		t.Fatalf("trojanPassword mismatch: %q", opts.trojanPassword)
	}
}

func TestParseDeployFlagsOverridesTrojanPort(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{
			Port: "62022",
		},
		Client: config.ClientConfig{
			User:     "user@example.test",
			Password: "secret",
		},
	}

	opts, err := parseDeployFlags(cfg, []string{"--remote-host", "gateway.internal", "--trojan-port", "8445"})
	if err != nil {
		t.Fatalf("parseDeployFlags: %v", err)
	}
	if opts.serverPort != "8445" {
		t.Fatalf("trojan port mismatch: got %q", opts.serverPort)
	}
}

func TestRunClientDeploySuccessfulFlow(t *testing.T) {
	ctx := context.Background()
	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir: `C:\remote`,
			ConfigDir:  "cfg-server",
			Host:       "edge.example.test",
		},
		Client: config.ClientConfig{
			InstallDir: `C:\local`,
			ConfigDir:  "cfg-client",
			User:       "user@example.test",
			Password:   "secret",
		},
	}

	var packageCalled bool
	restore := multiRestore(
		stubBuildDeploymentPackage(t, func(o deployOptions) (string, error) {
			packageCalled = true
			if o.remoteHost != "gateway.internal" {
				t.Fatalf("package remoteHost: %q", o.remoteHost)
			}
			return `C:\package.zip`, nil
		}),
		stubLookPath(t, func(string) (string, error) { return `C:\Windows\System32\ssh.exe`, nil }),
		stubExecutable(t, func() (string, error) { return `C:\xp2p.exe`, nil }),
		stubSleep(t, func(time.Duration) {}),
	)
	defer restore()

	var (
		gotEnsureTarget sshTarget
		gotPrepareOpts  deployOptions
		gotInstallOpts  deployOptions
		ensureCalled    bool
		prepareCalled   bool
		installCalled   bool
		startRemote     bool
		startLocal      bool
		pingCalled      bool
		released        bool
	)

	restore = multiRestore(
		restore,
		stubEnsureRemoteBinary(t, func(_ context.Context, target sshTarget, _ string, installDir string) error {
			ensureCalled = true
			gotEnsureTarget = target
			if installDir != `C:\remote` {
				t.Fatalf("ensureRemoteBinary installDir: %q", installDir)
			}
			return nil
		}),
		stubPrepareRemoteServer(t, func(_ context.Context, target sshTarget, opts deployOptions) (string, error) {
			prepareCalled = true
			if target != gotEnsureTarget {
				t.Fatalf("prepare target mismatch")
			}
			gotPrepareOpts = opts
			return "trojan://secret@edge.example.test:58443?security=tls&sni=edge.example.test#user@example.test", nil
		}),
		stubInstallLocalClient(t, func(_ context.Context, opts deployOptions, link string) error {
			installCalled = true
			gotInstallOpts = opts
			if link == "" {
				t.Fatalf("installLocalClient link empty")
			}
			return nil
		}),
		stubStartRemoteServer(t, func(context.Context, sshTarget, deployOptions) error {
			startRemote = true
			return nil
		}),
		stubStartLocalClient(t, func(deployOptions) (*exec.Cmd, error) {
			startLocal = true
			return &exec.Cmd{}, nil
		}),
		stubRunPingCheck(t, func(context.Context, deployOptions) error {
			pingCalled = true
			return nil
		}),
		stubReleaseHandle(t, func(*exec.Cmd) {
			released = true
		}),
	)
	defer restore()

	code := runClientDeploy(ctx, cfg, []string{"--remote-host", "gateway.internal"})
	if code != 0 {
		t.Fatalf("runClientDeploy exit code: %d", code)
	}
	if !packageCalled {
		t.Fatalf("expected package builder to be called")
	}

	if !ensureCalled || !prepareCalled || !installCalled || !startRemote || !startLocal || !pingCalled || !released {
		t.Fatalf("deployment steps missing: ensure=%t prepare=%t install=%t startRemote=%t startLocal=%t ping=%t released=%t",
			ensureCalled, prepareCalled, installCalled, startRemote, startLocal, pingCalled, released)
	}

	if gotEnsureTarget.host != "gateway.internal" {
		t.Fatalf("ensure target host: %q", gotEnsureTarget.host)
	}
	if gotPrepareOpts.serverHost != "edge.example.test" {
		t.Fatalf("prepare opts serverHost: %q", gotPrepareOpts.serverHost)
	}
	if gotPrepareOpts.packagePath != `C:\package.zip` {
		t.Fatalf("prepare opts packagePath: %q", gotPrepareOpts.packagePath)
	}
	if gotInstallOpts.localInstallDir != filepath.Clean(`C:\local`) {
		t.Fatalf("install opts localInstallDir: %q", gotInstallOpts.localInstallDir)
	}
	if gotInstallOpts.packagePath != `C:\package.zip` {
		t.Fatalf("install opts packagePath: %q", gotInstallOpts.packagePath)
	}
}

func TestRunClientDeployStopsOnFailure(t *testing.T) {
	ctx := context.Background()
	cfg := config.Config{
		Server: config.ServerConfig{
			InstallDir: `C:\remote`,
			ConfigDir:  "cfg-server",
			Host:       "edge.example.test",
		},
		Client: config.ClientConfig{
			InstallDir: `C:\local`,
			ConfigDir:  "cfg-client",
			User:       "user@example.test",
			Password:   "secret",
		},
	}

	restore := multiRestore(
		stubBuildDeploymentPackage(t, func(o deployOptions) (string, error) {
			if o.remoteHost != "gateway.internal" {
				t.Fatalf("package remoteHost: %q", o.remoteHost)
			}
			return `C:\package.zip`, nil
		}),
		stubLookPath(t, func(string) (string, error) { return `C:\Windows\System32\ssh.exe`, nil }),
		stubExecutable(t, func() (string, error) { return `C:\xp2p.exe`, nil }),
		stubEnsureRemoteBinary(t, func(context.Context, sshTarget, string, string) error {
			return errors.New("upload failed")
		}),
	)
	defer restore()

	code := runClientDeploy(ctx, cfg, []string{"--remote-host", "gateway.internal"})
	if code == 0 {
		t.Fatalf("expected non-zero exit code on failure")
	}
}

func TestRunClientDeployPackageOnlySkipsRemote(t *testing.T) {
	ctx := context.Background()
	cfg := config.Config{}

	var packageCalled bool
	restore := multiRestore(
		stubBuildDeploymentPackage(t, func(o deployOptions) (string, error) {
			packageCalled = true
			if o.remoteHost != "gateway.internal" {
				t.Fatalf("package remoteHost: %q", o.remoteHost)
			}
			if !o.packageOnly {
				t.Fatalf("expected packageOnly in package builder options")
			}
			return `C:\package.zip`, nil
		}),
		stubEnsureRemoteBinary(t, func(context.Context, sshTarget, string, string) error {
			t.Fatalf("ensureRemoteBinary should not be called in package-only mode")
			return nil
		}),
		stubPrepareRemoteServer(t, func(context.Context, sshTarget, deployOptions) (string, error) {
			t.Fatalf("prepareRemoteServer should not be called in package-only mode")
			return "", nil
		}),
		stubInstallLocalClient(t, func(context.Context, deployOptions, string) error {
			t.Fatalf("installLocalClient should not be called in package-only mode")
			return nil
		}),
		stubStartRemoteServer(t, func(context.Context, sshTarget, deployOptions) error {
			t.Fatalf("startRemoteServer should not be called in package-only mode")
			return nil
		}),
		stubStartLocalClient(t, func(deployOptions) (*exec.Cmd, error) {
			t.Fatalf("startLocalClient should not be called in package-only mode")
			return nil, nil
		}),
	)
	defer restore()

	code := runClientDeploy(ctx, cfg, []string{"--remote-host", "gateway.internal", "--package-only"})
	if code != 0 {
		t.Fatalf("expected zero exit code in package-only mode, got %d", code)
	}
	if !packageCalled {
		t.Fatalf("expected package builder to be called")
	}
}

func TestRunClientDeployPackageBuildFailure(t *testing.T) {
	ctx := context.Background()
	cfg := config.Config{
		Client: config.ClientConfig{
			User:     "user@example.test",
			Password: "secret",
		},
	}

	restore := stubBuildDeploymentPackage(t, func(o deployOptions) (string, error) {
		if o.remoteHost != "gateway.internal" {
			t.Fatalf("package remoteHost: %q", o.remoteHost)
		}
		return "", errors.New("packaging failed")
	})
	defer restore()

	code := runClientDeploy(ctx, cfg, []string{"--remote-host", "gateway.internal"})
	if code == 0 {
		t.Fatalf("expected non-zero exit code on package failure")
	}
}

func stubLookPath(t *testing.T, fn func(string) (string, error)) func() {
	t.Helper()
	prev := lookPathFunc
	lookPathFunc = fn
	return func() { lookPathFunc = prev }
}

func stubExecutable(t *testing.T, fn func() (string, error)) func() {
	t.Helper()
	prev := executablePathFunc
	executablePathFunc = fn
	return func() { executablePathFunc = prev }
}

func stubSleep(t *testing.T, fn func(time.Duration)) func() {
	t.Helper()
	prev := sleepFunc
	sleepFunc = fn
	return func() { sleepFunc = prev }
}

func stubEnsureRemoteBinary(t *testing.T, fn func(context.Context, sshTarget, string, string) error) func() {
	t.Helper()
	prev := ensureRemoteBinaryFunc
	ensureRemoteBinaryFunc = fn
	return func() { ensureRemoteBinaryFunc = prev }
}

func stubPrepareRemoteServer(t *testing.T, fn func(context.Context, sshTarget, deployOptions) (string, error)) func() {
	t.Helper()
	prev := prepareRemoteServerFunc
	prepareRemoteServerFunc = fn
	return func() { prepareRemoteServerFunc = prev }
}

func stubInstallLocalClient(t *testing.T, fn func(context.Context, deployOptions, string) error) func() {
	t.Helper()
	prev := installLocalClientFunc
	installLocalClientFunc = fn
	return func() { installLocalClientFunc = prev }
}

func stubStartRemoteServer(t *testing.T, fn func(context.Context, sshTarget, deployOptions) error) func() {
	t.Helper()
	prev := startRemoteServerFunc
	startRemoteServerFunc = fn
	return func() { startRemoteServerFunc = prev }
}

func stubStartLocalClient(t *testing.T, fn func(deployOptions) (*exec.Cmd, error)) func() {
	t.Helper()
	prev := startLocalClientFunc
	startLocalClientFunc = fn
	return func() { startLocalClientFunc = prev }
}

func stubRunPingCheck(t *testing.T, fn func(context.Context, deployOptions) error) func() {
	t.Helper()
	prev := runPingCheckFunc
	runPingCheckFunc = fn
	return func() { runPingCheckFunc = prev }
}

func stubReleaseHandle(t *testing.T, fn func(*exec.Cmd)) func() {
	t.Helper()
	prev := releaseProcessHandleFunc
	releaseProcessHandleFunc = fn
	return func() { releaseProcessHandleFunc = prev }
}

func stubPromptString(t *testing.T, fn func(string) (string, error)) func() {
	t.Helper()
	prev := promptStringFunc
	promptStringFunc = fn
	return func() { promptStringFunc = prev }
}

func stubBuildDeploymentPackage(t *testing.T, fn func(deployOptions) (string, error)) func() {
	t.Helper()
	prev := buildDeploymentPackageFunc
	buildDeploymentPackageFunc = fn
	return func() { buildDeploymentPackageFunc = prev }
}

func multiRestore(restores ...func()) func() {
	return func() {
		for i := len(restores) - 1; i >= 0; i-- {
			if restores[i] != nil {
				restores[i]()
			}
		}
	}
}
