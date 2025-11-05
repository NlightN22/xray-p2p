package clientcmd

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

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

	if opts.runtime.remoteHost != "gateway.internal" {
		t.Fatalf("remoteHost mismatch: got %q", opts.runtime.remoteHost)
	}
	if opts.runtime.serverHost != "edge.example.test" {
		t.Fatalf("serverHost mismatch: got %q", opts.runtime.serverHost)
	}
	if opts.manifest.trojanPort != "58443" {
		t.Fatalf("serverPort mismatch: got %q", opts.manifest.trojanPort)
	}
	if opts.manifest.trojanUser != "user@example.test" {
		t.Fatalf("trojanUser mismatch: got %q", opts.manifest.trojanUser)
	}
	if opts.manifest.trojanPassword != "hunter2" {
		t.Fatalf("trojanPassword mismatch: got %q", opts.manifest.trojanPassword)
	}
	if opts.manifest.installDir != `C:\remote` {
		t.Fatalf("remoteInstallDir mismatch: got %q", opts.manifest.installDir)
	}
	if opts.runtime.remoteConfigDir != "cfg-server" {
		t.Fatalf("remoteConfigDir mismatch: got %q", opts.runtime.remoteConfigDir)
	}
	if opts.runtime.localInstallDir != filepath.Clean(`C:\local`) {
		t.Fatalf("localInstallDir mismatch: got %q", opts.runtime.localInstallDir)
	}
	if opts.runtime.localConfigDir != "cfg-client" {
		t.Fatalf("localConfigDir mismatch: got %q", opts.runtime.localConfigDir)
	}
	if opts.runtime.packageOnly {
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

func TestParseDeployFlagsAllowsEmptyUser(t *testing.T) {
	cfg := config.Config{}
	opts, err := parseDeployFlags(cfg, []string{"--remote-host", "gateway.internal"})
	if err != nil {
		t.Fatalf("parseDeployFlags: %v", err)
	}
	if opts.manifest.trojanUser != "" {
		t.Fatalf("trojanUser expected empty, got %q", opts.manifest.trojanUser)
	}
}

func TestParseDeployFlagsPackageOnly(t *testing.T) {
	cfg := config.Config{}

	opts, err := parseDeployFlags(cfg, []string{"--remote-host", "gateway.internal", "--package-only"})
	if err != nil {
		t.Fatalf("parseDeployFlags: %v", err)
	}
	if !opts.runtime.packageOnly {
		t.Fatalf("packageOnly expected true")
	}
	if opts.manifest.trojanUser != "client@example.invalid" {
		t.Fatalf("trojanUser mismatch: %q", opts.manifest.trojanUser)
	}
	if opts.manifest.trojanPassword != "placeholder-secret" {
		t.Fatalf("trojanPassword mismatch: %q", opts.manifest.trojanPassword)
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
	if opts.manifest.trojanPort != "8445" {
		t.Fatalf("trojan port mismatch: got %q", opts.manifest.trojanPort)
	}
}

func TestRunClientDeployPackageOnly(t *testing.T) {
	ctx := context.Background()
	cfg := config.Config{
		Client: config.ClientConfig{
			User:     "user@example.test",
			Password: "secret",
		},
	}

	var (
		ensureCalled  bool
		packageCalled bool
	)
	restore := multiRestore(
		stubEnsureSSHPrerequisites(t, func() (sshPrerequisites, error) {
			ensureCalled = true
			return sshPrerequisites{
				sshPath: "ssh",
				scpPath: "scp",
			}, nil
		}),
		stubBuildDeploymentPackage(t, func(o deployOptions) (string, error) {
			packageCalled = true
			if o.manifest.remoteHost != "gateway.internal" {
				t.Fatalf("package remoteHost: %q", o.manifest.remoteHost)
			}
			if !o.runtime.packageOnly {
				t.Fatalf("expected packageOnly in package builder options")
			}
			return `C:\package.zip`, nil
		}),
		stubRunRemoteDeployment(t, func(context.Context, deployOptions) error {
			t.Fatalf("runRemoteDeployment should not be called in package-only mode")
			return nil
		}),
	)
	defer restore()

	code := runClientDeploy(ctx, cfg, []string{"--remote-host", "gateway.internal", "--package-only"})
	if code != 0 {
		t.Fatalf("expected zero exit code in package-only mode, got %d", code)
	}
	if !ensureCalled {
		t.Fatalf("expected ensureSSHPrerequisites to be called")
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

	restore := multiRestore(
		stubEnsureSSHPrerequisites(t, func() (sshPrerequisites, error) {
			return sshPrerequisites{sshPath: "ssh", scpPath: "scp"}, nil
		}),
		stubBuildDeploymentPackage(t, func(o deployOptions) (string, error) {
			if o.manifest.remoteHost != "gateway.internal" {
				t.Fatalf("package remoteHost: %q", o.manifest.remoteHost)
			}
			return "", errors.New("packaging failed")
		}),
		stubRunRemoteDeployment(t, func(context.Context, deployOptions) error {
			t.Fatalf("runRemoteDeployment should not be called on package failure")
			return nil
		}),
	)
	defer restore()

	code := runClientDeploy(ctx, cfg, []string{"--remote-host", "gateway.internal"})
	if code == 0 {
		t.Fatalf("expected non-zero exit code on package failure")
	}
}

func TestRunClientDeployPrerequisitesFailure(t *testing.T) {
	ctx := context.Background()
	cfg := config.Config{}

	restore := stubEnsureSSHPrerequisites(t, func() (sshPrerequisites, error) {
		return sshPrerequisites{}, errors.New("ssh missing")
	})
	defer restore()

	code := runClientDeploy(ctx, cfg, []string{"--remote-host", "gateway.internal"})
	if code == 0 {
		t.Fatalf("expected non-zero exit code when prerequisites fail")
	}
}

func TestRunClientDeployRemoteFailure(t *testing.T) {
	ctx := context.Background()
	cfg := config.Config{
		Client: config.ClientConfig{
			User:     "user@example.test",
			Password: "secret",
		},
	}

	restore := multiRestore(
		stubEnsureSSHPrerequisites(t, func() (sshPrerequisites, error) {
			return sshPrerequisites{sshPath: "ssh", scpPath: "scp"}, nil
		}),
		stubBuildDeploymentPackage(t, func(o deployOptions) (string, error) {
			return `C:\package.zip`, nil
		}),
		stubRunRemoteDeployment(t, func(context.Context, deployOptions) error {
			return errors.New("remote failure")
		}),
	)
	defer restore()

	code := runClientDeploy(ctx, cfg, []string{"--remote-host", "gateway.internal"})
	if code == 0 {
		t.Fatalf("expected non-zero exit code when remote deployment fails")
	}
}

func TestRunRemoteDeploymentWindows(t *testing.T) {
	ctx := context.Background()
	opts := deployOptions{
		packagePath: `C:\local\pkg`,
		runtime: runtimeOptions{
			remoteHost: "gateway.internal",
			sshUser:    "admin",
			sshPort:    "22",
			sshBinary:  "ssh",
			scpBinary:  "scp",
		},
	}

	var (
		callIndex      int
		scpCalled      bool
		remoteDest     string
		expectedParent = "~"
	)

	restore := multiRestore(
		stubSSHCommand(t, func(command string) (string, string, error) {
			callIndex++
			switch callIndex {
			case 1:
				script, ok := decodePSEncodedCommand(command)
				if !ok {
					t.Fatalf("unable to decode encoded command: %q", command)
				}
				if !strings.Contains(script, "install.ps1") {
					t.Fatalf("expected install script path, got %q", script)
				}
				if !strings.Contains(script, "pkg") {
					t.Fatalf("expected package directory in script, got %q", script)
				}
				return "[INFO] success", "", nil
			default:
				t.Fatalf("unexpected ssh command: %q", command)
				return "", "", errors.New("unexpected command")
			}
		}),
		stubSCPCopy(t, func(localPath, remotePath string, recursive bool) error {
			scpCalled = true
			if localPath != opts.packagePath {
				t.Fatalf("unexpected local path: %q", localPath)
			}
			if !recursive {
				t.Fatalf("scp should be recursive")
			}
			remoteDest = remotePath
			if remotePath != expectedParent {
				t.Fatalf("unexpected remote destination: %q", remotePath)
			}
			return nil
		}),
	)
	defer restore()

	if err := runRemoteDeployment(ctx, opts); err != nil {
		t.Fatalf("runRemoteDeployment failed: %v", err)
	}
	if !scpCalled {
		t.Fatalf("scpCopyFunc was not called")
	}
	if remoteDest == "" {
		t.Fatalf("remote destination not captured")
	}
	if callIndex != 1 {
		t.Fatalf("unexpected number of ssh commands: %d", callIndex)
	}
}

func TestRunRemoteDeploymentUnsupportedOS(t *testing.T) {
	ctx := context.Background()
	opts := deployOptions{
		packagePath: `C:\local\pkg`,
		runtime: runtimeOptions{
			remoteHost: "gateway.internal",
			sshBinary:  "ssh",
			scpBinary:  "scp",
		},
	}

	var scpCalled bool
	restore := multiRestore(
		stubSSHCommand(t, func(command string) (string, string, error) {
			if strings.Contains(command, "EncodedCommand") {
				return "", "powershell not found", errors.New("command failed")
			}
			return "", "", errors.New("unexpected command")
		}),
		stubSCPCopy(t, func(string, string, bool) error {
			scpCalled = true
			return nil
		}),
	)
	defer restore()

	if err := runRemoteDeployment(ctx, opts); err == nil {
		t.Fatalf("expected error for unsupported operating system")
	}
	if !scpCalled {
		t.Fatalf("expected scp to be called before failure")
	}
}

func stubPromptString(t *testing.T, fn func(string) (string, error)) func() {
	t.Helper()
	prev := promptStringFunc
	promptStringFunc = fn
	return func() { promptStringFunc = prev }
}

func stubEnsureSSHPrerequisites(t *testing.T, fn func() (sshPrerequisites, error)) func() {
	t.Helper()
	prev := ensureSSHPrerequisitesFunc
	ensureSSHPrerequisitesFunc = fn
	return func() { ensureSSHPrerequisitesFunc = prev }
}

func stubBuildDeploymentPackage(t *testing.T, fn func(deployOptions) (string, error)) func() {
	t.Helper()
	prev := buildDeploymentPackageFunc
	buildDeploymentPackageFunc = fn
	return func() { buildDeploymentPackageFunc = prev }
}

func stubRunRemoteDeployment(t *testing.T, fn func(context.Context, deployOptions) error) func() {
	t.Helper()
	prev := runRemoteDeploymentFunc
	runRemoteDeploymentFunc = fn
	return func() { runRemoteDeploymentFunc = prev }
}

func stubSSHCommand(t *testing.T, fn func(string) (string, string, error)) func() {
	t.Helper()
	prev := sshCommandFunc
	sshCommandFunc = func(ctx context.Context, binary string, target sshTarget, command string) (string, string, error) {
		return fn(command)
	}
	return func() { sshCommandFunc = prev }
}

func stubSCPCopy(t *testing.T, fn func(localPath, remotePath string, recursive bool) error) func() {
	t.Helper()
	prev := scpCopyFunc
	scpCopyFunc = func(ctx context.Context, binary string, target sshTarget, localPath, remotePath string, recursive bool) error {
		return fn(localPath, remotePath, recursive)
	}
	return func() { scpCopyFunc = prev }
}

func decodePSEncodedCommand(command string) (string, bool) {
	const token = "-EncodedCommand "
	idx := strings.Index(command, token)
	if idx == -1 {
		return "", false
	}
	rest := strings.TrimSpace(command[idx+len(token):])
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return "", false
	}
	encoded := parts[0]
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", false
	}
	if len(data)%2 != 0 {
		return "", false
	}
	u16 := make([]uint16, len(data)/2)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(data[2*i:])
	}
	return string(utf16.Decode(u16)), true
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
