package root

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	clientcmd "github.com/NlightN22/xray-p2p/go/internal/cli/client"
	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	servercmd "github.com/NlightN22/xray-p2p/go/internal/cli/server"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

var stage5Paths = []string{
	"xp2p client mode",
	"xp2p client remove",
	"xp2p client service restart",
	"xp2p client service start",
	"xp2p client service status",
	"xp2p client service stop",
	"xp2p server cert set",
	"xp2p server mode",
	"xp2p server remove",
	"xp2p server service restart",
	"xp2p server service start",
	"xp2p server service status",
	"xp2p server service stop",
}

func registerStage5ContractCases(registry map[string]contractCase) {
	registry["xp2p client remove"] = stage5ClientRemoveCase()
	registry["xp2p server cert set"] = stage5ServerCertSetCase()
	registry["xp2p server remove"] = stage5ServerRemoveCase()
	for _, role := range []string{"client", "server"} {
		for _, action := range []string{"start", "stop", "restart"} {
			path := fmt.Sprintf("xp2p %s service %s", role, action)
			registry[path] = stage5ServiceActionCase(role, action)
		}
	}
}

func stage5ClientRemoveCase() contractCase {
	args := []string{"client", "remove", "--all"}
	return stage5MutationCase(
		args,
		append(append([]string{}, args...), "--quiet"),
		"client remove",
		func(t *testing.T, mode string) {
			stage5BaseSetup(t)
			writeStage5InstallConfig(t)
			restore := clientcmd.SetRemoveForTesting(func(context.Context, client.RemoveOptions) error {
				if mode == "error" {
					return errors.New("stage 5 client removal failure")
				}
				return nil
			})
			t.Cleanup(restore)
		},
		"client removed",
	)
}

func stage5ServerRemoveCase() contractCase {
	args := []string{"server", "remove"}
	return stage5MutationCase(
		args,
		append(append([]string{}, args...), "--quiet"),
		"server remove",
		func(t *testing.T, mode string) {
			stage5BaseSetup(t)
			writeStage5InstallConfig(t)
			restore := servercmd.SetRemoveForTesting(func(context.Context, server.RemoveOptions) error {
				if mode == "error" {
					return errors.New("stage 5 server removal failure")
				}
				return nil
			})
			t.Cleanup(restore)
		},
		"server removed",
	)
}

func stage5ServerCertSetCase() contractCase {
	args := []string{"server", "cert", "set", "--host", "edge.example"}
	return stage5MutationCase(
		args,
		append(append([]string{}, args...), "--quiet"),
		"server cert set",
		func(t *testing.T, mode string) {
			stage5BaseSetup(t)
			writeStage5InstallConfig(t)
			restore := servercmd.SetCertificateForTesting(func(context.Context, server.CertificateOptions) error {
				if mode == "error" {
					return errors.New("stage 5 certificate failure")
				}
				return nil
			})
			t.Cleanup(restore)
		},
		"server cert set completed",
	)
}

func stage5ServiceActionCase(role, action string) contractCase {
	args := []string{role, "service", action}
	return stage5MutationCase(
		args,
		args,
		fmt.Sprintf("%s service %s", role, action),
		func(t *testing.T, mode string) {
			stage5BaseSetup(t)
			controller := &stage5ServiceController{}
			if mode == "error" {
				controller.failAction = action
			}
			restore := servicecontrol.SetDefaultForTesting(controller)
			t.Cleanup(restore)
		},
		fmt.Sprintf("%s service %s", role, servicePastTense(action)),
	)
}

func stage5MutationCase(
	args []string,
	human []string,
	operation string,
	setup func(*testing.T, string),
	humanText string,
) contractCase {
	assertResult := func(t *testing.T, result map[string]any) {
		if result["status"] != "completed" || result["operation"] != operation {
			t.Fatalf("unexpected mutation result: %#v", result)
		}
	}
	return contractCase{
		coverage:     contractCovered,
		success:      args,
		empty:        args,
		failure:      args,
		setup:        setup,
		assertResult: assertResult,
		assertEmpty:  assertResult,
		emptyResult: "an idempotent or already-satisfied action returns the same typed " +
			"completed mutation result",
		credentialPolicy: "service and interactive mutation results omit credentials",
		edgeCases:        []string{"closed stdin", "warning isolation", "ANSI-free streams"},
		assertEdgeCases:  assertReadOnlyEdgeCases,
		platform:         "windows,linux",
		human:            human,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			if !strings.Contains(output+diagnostics, humanText) {
				t.Fatalf("human output is missing %q: stdout=%q stderr=%q", humanText, output, diagnostics)
			}
		},
	}
}

func stage5BaseSetup(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
}

func writeStage5InstallConfig(t *testing.T) {
	t.Helper()
	root := os.Getenv("XP2P_CONFIG_ROOT")
	content := fmt.Sprintf(
		"[client]\ninstall_dir = %q\nconfig_dir = \"client\"\n\n"+
			"[server]\ninstall_dir = %q\nconfig_dir = \"server\"\n",
		filepath.Join(root, "client-install"),
		filepath.Join(root, "server-install"),
	)
	if err := os.WriteFile(filepath.Join(root, layout.ClientConfigFileName), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, layout.ServerConfigFileName), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func servicePastTense(action string) string {
	switch action {
	case "start":
		return "started"
	case "stop":
		return "stopped"
	default:
		return "restarted"
	}
}

func TestStage5LeavesCovered(t *testing.T) {
	baseline := buildLegacyPendingBaseline()
	expected := make(map[string]bool)
	for path, scenario := range baseline {
		if scenario.coverage != contractStage5 {
			continue
		}
		expected[path] = true
		covered, ok := contractCaseRegistry[path]
		if !ok || covered.coverage != contractCovered {
			t.Errorf("stage 5 leaf is not covered: %s", path)
		}
	}
	for _, path := range stage5Paths {
		if !expected[path] {
			t.Errorf("stale stage 5 descriptor: %s", path)
		}
	}
}

func TestStage5ServiceActionsUseControllerBoundary(t *testing.T) {
	for _, role := range []string{"client", "server"} {
		for _, action := range []string{"start", "stop", "restart", "status"} {
			role, action := role, action
			path := fmt.Sprintf("xp2p %s service %s", role, action)
			t.Run(path, func(t *testing.T) {
				controller := &stage5ServiceController{
					active:     action == "status",
					diagnostic: "\x1b[33mplatform warning О©\x1b[0m",
				}
				restore := servicecontrol.SetDefaultForTesting(controller)
				t.Cleanup(restore)
				execution := executeContractCase([]string{role, "service", action}, false)
				if action == "status" {
					assertStage5StatusSuccess(t, path, execution)
				} else {
					assertStage5MutationSuccess(t, path, execution)
				}
				controller.assertAction(t, servicecontrol.Role(role), action)
			})
		}
	}
}

func TestStage5ServiceFailures(t *testing.T) {
	for _, role := range []string{"client", "server"} {
		for _, action := range []string{"start", "stop", "restart", "status"} {
			role, action := role, action
			path := fmt.Sprintf("xp2p %s service %s", role, action)
			t.Run(path+"/not-installed", func(t *testing.T) {
				controller := &stage5ServiceController{failAction: "not-installed"}
				restore := servicecontrol.SetDefaultForTesting(controller)
				t.Cleanup(restore)
				execution := executeContractCase([]string{role, "service", action}, false)
				assertStage5Error(t, path, execution)
				controller.assertBoundaryCall(t, servicecontrol.Role(role), action)
			})
		}
		path := fmt.Sprintf("xp2p %s service restart", role)
		t.Run(path+"/start-error-after-stop", func(t *testing.T) {
			controller := &stage5ServiceController{failAction: "restart-start"}
			restore := servicecontrol.SetDefaultForTesting(controller)
			t.Cleanup(restore)
			execution := executeContractCase([]string{role, "service", "restart"}, false)
			assertStage5Error(t, path, execution)
			controller.assertRestartStartFailure(t, servicecontrol.Role(role))
		})
	}
}

func TestStage5PromptPathsUseClosedStdin(t *testing.T) {
	t.Run("client remove", func(t *testing.T) {
		stage5ClientRemoveCase().setup(t, "success")
		execution := executeContractCaseWithClosedStdin(t, []string{"client", "remove", "--all"})
		assertStage5MutationSuccess(t, "xp2p client remove", execution)
	})
	t.Run("server remove", func(t *testing.T) {
		stage5ServerRemoveCase().setup(t, "success")
		execution := executeContractCaseWithClosedStdin(t, []string{"server", "remove"})
		assertStage5MutationSuccess(t, "xp2p server remove", execution)
	})
	t.Run("server cert replacement", func(t *testing.T) {
		stage5BaseSetup(t)
		calls := 0
		restore := servercmd.SetCertificateForTesting(func(context.Context, server.CertificateOptions) error {
			calls++
			if calls == 1 {
				return server.ErrCertificateConfigured
			}
			return nil
		})
		t.Cleanup(restore)
		execution := executeContractCaseWithClosedStdin(t, []string{
			"server", "cert", "set", "--host", "edge.example",
		})
		assertStage5MutationSuccess(t, "xp2p server cert set", execution)
		if calls != 2 {
			t.Fatalf("certificate replacement path was not exercised: calls=%d", calls)
		}
	})
	t.Run("client mode selection", func(t *testing.T) {
		newClientMutationFixture(
			t,
			strings.Replace(
				clientMutationBase(false)+secondClientEndpoint(),
				"[client]\n",
				"[client]\ninstall_dir = \"C:/xp2p-client\"\n",
				1,
			),
			nil,
			nil,
			nil,
		)
		execution := executeContractCaseWithClosedStdin(t, []string{"client", "mode", "tun", "full"})
		assertStage5Error(t, "xp2p client mode", execution)
	})
	for _, item := range []struct {
		name    string
		path    string
		args    []string
		fixture func(*testing.T) mutationFixture
	}{
		{
			"client redirect selection",
			"xp2p client redirect add",
			[]string{"client", "redirect", "add", "--domain", "prompt.example"},
			clientPromptFixture(clientMutationBase(false) + secondClientEndpoint()),
		},
		{
			"server redirect selection",
			"xp2p server redirect add",
			[]string{"server", "redirect", "add", "--domain", "prompt.example"},
			serverPromptFixture(serverMutationBase(false, false) + secondServerReverse()),
		},
	} {
		item := item
		t.Run(item.name, func(t *testing.T) {
			item.fixture(t)
			execution := executeContractCaseWithClosedStdin(t, item.args)
			assertStage5Error(t, item.path, execution)
		})
	}
}

func executeContractCaseWithClosedStdin(t *testing.T, args []string) contractExecution {
	t.Helper()
	original := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdin = reader
	defer func() {
		os.Stdin = original
		_ = reader.Close()
	}()
	return executeContractCase(args, false)
}

func TestStage5AmbiguousUserHostRejectsWithoutPrompt(t *testing.T) {
	setupStage5AmbiguousUserHost(t)
	execution := executeContractCaseWithClosedStdin(t, []string{
		"server", "user", "add", "--id", "ambiguous-host", "--no-reverse",
	})
	assertStage5ErrorCode(t, "xp2p server user add", "ambiguous_selection", execution)
}

func setupStage5AmbiguousUserHost(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	certPath := filepath.Join(root, "ambiguous.pem")
	keyPath := filepath.Join(root, "ambiguous.key")
	writeContractCertificate(t, certPath, keyPath, "first.example", []string{"first.example", "second.example"}, nil)
	desired := fmt.Sprintf(
		"[server]\ninstall_dir = %q\ncertificate = %q\nkey = %q\n",
		safeServerInstallDir(),
		certPath,
		keyPath,
	)
	if err := os.WriteFile(filepath.Join(root, layout.ServerConfigFileName), []byte(desired), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestStage5HumanModePreservesPrompts(t *testing.T) {
	t.Run("server cert replacement", func(t *testing.T) {
		stage5BaseSetup(t)
		restore := servercmd.SetCertificateForTesting(func(context.Context, server.CertificateOptions) error {
			return server.ErrCertificateConfigured
		})
		t.Cleanup(restore)
		stdout, _, _ := executeHumanWithInput(t, []string{
			"server", "cert", "set", "--host", "edge.example",
		}, "n\n")
		assertHumanPrompt(t, stdout, "Replace existing certificate?")
	})
	t.Run("client mode selection", func(t *testing.T) {
		newClientMutationFixture(
			t,
			strings.Replace(
				clientMutationBase(false)+secondClientEndpoint(),
				"[client]\n",
				"[client]\ninstall_dir = \"C:/xp2p-client\"\n",
				1,
			),
			nil, nil, nil,
		)
		stdout, _, _ := executeHumanWithInput(t, []string{"client", "mode", "tun", "full"}, "1\n")
		assertHumanPrompt(t, stdout, "Available client endpoints:")
	})
	for _, item := range []struct {
		name    string
		args    []string
		fixture func(*testing.T) mutationFixture
		want    string
	}{
		{
			"client redirect selection",
			[]string{"client", "redirect", "add", "--domain", "human-prompt.example"},
			clientPromptFixture(clientMutationBase(false) + secondClientEndpoint()),
			"Available client endpoints:",
		},
		{
			"server redirect selection",
			[]string{"server", "redirect", "add", "--domain", "human-prompt.example"},
			serverPromptFixture(serverMutationBase(false, false) + secondServerReverse()),
			"Available reverse portals:",
		},
	} {
		item := item
		t.Run(item.name, func(t *testing.T) {
			item.fixture(t)
			stdout, _, _ := executeHumanWithInput(t, item.args, "1\n")
			assertHumanPrompt(t, stdout, item.want)
		})
	}
	t.Run("ambiguous user host", func(t *testing.T) {
		setupStage5AmbiguousUserHost(t)
		restore := servercmd.SetUserAddForTesting(func(context.Context, server.AddUserOptions) error {
			return nil
		})
		t.Cleanup(restore)
		stdout, _, _ := executeHumanWithInput(t, []string{
			"server", "user", "add", "--id", "human-host-choice", "--no-reverse",
		}, "1\n")
		assertHumanPrompt(t, stdout, "Select host for reverse portal/link generation:")
	})
}

func executeHumanWithInput(t *testing.T, args []string, input string) (string, string, error) {
	t.Helper()
	original := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.WriteString(input); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdin = reader
	defer func() {
		os.Stdin = original
		_ = reader.Close()
	}()
	return executeHumanContractCase(args)
}

func assertHumanPrompt(t *testing.T, output, expected string) {
	t.Helper()
	if !strings.Contains(output, expected) {
		t.Fatalf("human prompt is missing %q: %q", expected, output)
	}
}

func TestStage5ForegroundExceptionsRejectJSON(t *testing.T) {
	for _, item := range []struct {
		path string
		args []string
	}{
		{"xp2p client run", []string{"client", "run"}},
		{"xp2p server run", []string{"server", "run"}},
		{"xp2p server deploy", []string{"server", "deploy", "--link", "trojan://matrix@example.invalid:443"}},
		{"xp2p diag", []string{"diag"}},
		{"xp2p ping", []string{"ping", "example.invalid"}},
	} {
		item := item
		t.Run(item.path, func(t *testing.T) {
			execution := executeContractCase(item.args, false)
			assertStage5Error(t, item.path, execution)
			if !strings.Contains(execution.stderr, `"code":"unsupported_output_format"`) {
				t.Fatalf("wrong structured rejection: %q", execution.stderr)
			}
		})
	}
}

type stage5ServiceController struct {
	mu         sync.Mutex
	calls      []string
	failAction string
	active     bool
	diagnostic string
}

func (c *stage5ServiceController) Start(_ context.Context, role servicecontrol.Role) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, "start:"+string(role))
	c.emitDiagnostic()
	if c.failAction == "start" || c.failAction == "restart-start" ||
		c.failAction == "not-installed" {
		if c.failAction == "not-installed" {
			return servicecontrol.ErrUnsupported
		}
		return errors.New("stage 5 service start failure")
	}
	c.active = true
	return nil
}

func (c *stage5ServiceController) Stop(_ context.Context, role servicecontrol.Role) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, "stop:"+string(role))
	c.emitDiagnostic()
	if c.failAction == "not-installed" {
		return servicecontrol.ErrUnsupported
	}
	if c.failAction == "stop" || c.failAction == "restart" {
		return errors.New("stage 5 service stop failure")
	}
	c.active = false
	return nil
}

func (c *stage5ServiceController) Status(_ context.Context, role servicecontrol.Role) (servicecontrol.Status, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, "status:"+string(role))
	c.emitDiagnostic()
	if c.failAction == "not-installed" {
		return servicecontrol.Status{}, servicecontrol.ErrUnsupported
	}
	if c.active {
		return servicecontrol.Status{Active: true, State: "RUNNING"}, nil
	}
	return servicecontrol.Status{State: "STOPPED"}, nil
}

func (c *stage5ServiceController) emitDiagnostic() {
	if c.diagnostic != "" {
		_, _ = fmt.Fprintln(os.Stderr, c.diagnostic)
	}
}

func (c *stage5ServiceController) assertAction(t *testing.T, role servicecontrol.Role, action string) {
	t.Helper()
	c.assertBoundaryCall(t, role, action)
	c.mu.Lock()
	defer c.mu.Unlock()
	joined := strings.Join(c.calls, ",")
	if action == "restart" && !strings.Contains(joined, "start:"+string(role)) {
		t.Fatalf("restart did not call start boundary: %v", c.calls)
	}
}

func (c *stage5ServiceController) assertBoundaryCall(t *testing.T, role servicecontrol.Role, action string) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if expected := actionBoundary(action) + ":" + string(role); !containsCall(c.calls, expected) {
		t.Fatalf("%s boundary was not called: %v", action, c.calls)
	}
}

func containsCall(calls []string, expected string) bool {
	for _, call := range calls {
		if call == expected {
			return true
		}
	}
	return false
}

func (c *stage5ServiceController) assertRestartStartFailure(t *testing.T, role servicecontrol.Role) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	joined := strings.Join(c.calls, ",")
	for _, expected := range []string{"stop:" + string(role), "status:" + string(role), "start:" + string(role)} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("restart start-failure path missed %q: %v", expected, c.calls)
		}
	}
}

func actionBoundary(action string) string {
	if action == "restart" {
		return "stop"
	}
	return action
}

func assertStage5StatusSuccess(t *testing.T, path string, execution contractExecution) {
	t.Helper()
	if execution.exitCode != 0 || execution.err != nil || execution.stderr != "" {
		t.Fatalf("exit=%d err=%v stdout=%q stderr=%q", execution.exitCode, execution.err, execution.stdout, execution.stderr)
	}
	document := assertJSONDocument(t, execution.stdout)
	var envelope struct {
		SchemaVersion string `json:"schema_version"`
		Command       string `json:"command"`
		Result        struct {
			Active bool   `json:"active"`
			State  string `json:"state"`
		} `json:"result"`
	}
	if err := json.Unmarshal(document, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != clioutput.SchemaVersion || envelope.Command != path ||
		!envelope.Result.Active || envelope.Result.State != "RUNNING" {
		t.Fatalf("unexpected status result: %#v", envelope)
	}
}

func assertStage5MutationSuccess(t *testing.T, path string, execution contractExecution) {
	t.Helper()
	if execution.exitCode != 0 || execution.err != nil || execution.stderr != "" {
		t.Fatalf("exit=%d err=%v stdout=%q stderr=%q", execution.exitCode, execution.err, execution.stdout, execution.stderr)
	}
	document := assertJSONDocument(t, execution.stdout)
	var envelope struct {
		SchemaVersion string         `json:"schema_version"`
		Command       string         `json:"command"`
		Result        mutationResult `json:"result"`
	}
	if err := json.Unmarshal(document, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != clioutput.SchemaVersion || envelope.Command != path ||
		envelope.Result.Status != "completed" {
		t.Fatalf("unexpected mutation result: %#v", envelope)
	}
}

func assertStage5Error(t *testing.T, path string, execution contractExecution) {
	t.Helper()
	assertStage5ErrorCode(t, path, "", execution)
}

func assertStage5ErrorCode(t *testing.T, path, code string, execution contractExecution) {
	t.Helper()
	if execution.exitCode == 0 || execution.err == nil || execution.stdout != "" {
		t.Fatalf("invalid failure framing: exit=%d err=%v stdout=%q stderr=%q", execution.exitCode, execution.err, execution.stdout, execution.stderr)
	}
	document := assertJSONDocument(t, execution.stderr)
	var envelope clioutput.ErrorEnvelope
	if err := json.Unmarshal(document, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SchemaVersion != clioutput.SchemaVersion || envelope.Command != path ||
		code != "" && envelope.Error.Code != code {
		t.Fatalf("unexpected error envelope: %#v", envelope)
	}
}
