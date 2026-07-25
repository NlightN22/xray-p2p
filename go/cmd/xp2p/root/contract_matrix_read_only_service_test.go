package root

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func serviceStatusContractCase(role string) contractCase {
	args := []string{role, "service", "status"}
	return contractCase{
		coverage: contractCovered,
		success:  args,
		empty:    args,
		failure:  args,
		setup: func(t *testing.T, mode string) {
			root := t.TempDir()
			t.Setenv("XP2P_CONFIG_ROOT", root)
			t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
			restore := servicecontrol.SetDefaultForTesting(contractServiceController{mode: mode})
			t.Cleanup(restore)
		},
		assertResult: func(t *testing.T, result map[string]any) {
			if result["state"] != "running" || result["active"] != true ||
				result["detail"] != role+" service Ω running\nhealthy" {
				t.Fatalf("%s service status changed: %#v", role, result)
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			if result["state"] != "running" || result["active"] != true || result["detail"] != nil {
				t.Fatalf("empty %s service detail changed: %#v", role, result)
			}
		},
		emptyResult:      "an active service with no manager detail publishes detail=null",
		credentialPolicy: "service status omits credentials and command output secrets",
		edgeCases:        []string{"boolean", "nullable detail", "Unicode/control characters", "warning isolation", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			expected := role + " service Ω running\nhealthy"
			if !strings.Contains(output, expected) {
				t.Fatalf("human service status is missing %q: output=%q diagnostics=%q", expected, output, diagnostics)
			}
		},
	}
}

type contractServiceController struct {
	mode string
}

func (c contractServiceController) Start(context.Context, servicecontrol.Role) error {
	return nil
}

func (c contractServiceController) Stop(context.Context, servicecontrol.Role) error {
	return nil
}

func (c contractServiceController) Status(_ context.Context, role servicecontrol.Role) (servicecontrol.Status, error) {
	if c.mode == "error" {
		return servicecontrol.Status{}, errors.New("service manager matrix failure")
	}
	if c.mode == "inactive" {
		return servicecontrol.Status{State: "stopped"}, nil
	}
	detail := ""
	if c.mode == "success" {
		detail = fmt.Sprintf("%s service Ω running\nhealthy", role)
	}
	return servicecontrol.Status{Active: true, State: "running", Detail: detail}, nil
}
