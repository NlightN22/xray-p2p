package root

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	observatorycommand "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/observatorycommand"
	observatoryconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/observatoryconfig"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func clientObsContractCase() contractCase {
	args := []string{"client", "obs"}
	return contractCase{
		coverage: contractCovered,
		success:  args,
		empty:    args,
		failure:  args,
		setup:    setupClientObsCase,
		assertResult: func(t *testing.T, result map[string]any) {
			items, ok := result["observations"].([]any)
			if !ok || len(items) != 2 {
				t.Fatalf("observations=%#v", result["observations"])
			}
			zulu, ok := items[0].(map[string]any)
			if !ok || zulu["tag"] != "zulu Ω outbound" || zulu["alive"] != true ||
				zulu["delay_millis"] != float64(42) ||
				zulu["last_try_at"] != "2023-11-14T22:13:20Z" ||
				zulu["last_seen_at"] != "2023-11-14T22:15:00Z" ||
				zulu["health_checks"] != float64(5) ||
				zulu["health_failures"] != float64(1) ||
				zulu["health_average_millis"] != float64(45) {
				t.Fatalf("first observation changed: %#v", items[0])
			}
			alpha, ok := items[1].(map[string]any)
			if !ok || alpha["tag"] != "alpha" || alpha["alive"] != false ||
				alpha["last_try_at"] != nil || alpha["last_seen_at"] != nil ||
				alpha["error"] != "timeout\nretry" {
				t.Fatalf("second observation changed: %#v", items[1])
			}
			raw := fmt.Sprintf("%v", result)
			for _, secret := range []string{"password", "token", "private_key"} {
				if strings.Contains(raw, secret) {
					t.Fatalf("client obs leaked incidental credentials: %#v", result)
				}
			}
		},
		assertEmpty: func(t *testing.T, result map[string]any) {
			items, ok := result["observations"].([]any)
			if !ok || items == nil || len(items) != 0 {
				t.Fatalf("empty observations must be []: %#v", result["observations"])
			}
		},
		emptyResult:      "observations is a non-nil empty array when Observatory reports no outbounds",
		credentialPolicy: "observations omit endpoint credentials and private material",
		edgeCases:        []string{"number", "boolean", "nullable UTC timestamps", "Unicode/spaces/control characters", "ANSI-free streams"},
		platform:         "windows,linux",
		human:            args,
		assertHuman: func(t *testing.T, output, diagnostics string) {
			for _, expected := range []string{"TAG", "ALIVE", "DELAY", "LAST TRY", "LAST SEEN", "zulu Ω outbound", "42ms", "timeout"} {
				if !strings.Contains(output, expected) {
					t.Fatalf("human baseline is missing %q: output=%q diagnostics=%q", expected, output, diagnostics)
				}
			}
		},
	}
}

func setupClientObsCase(t *testing.T, mode string) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for Observatory fixture: %v", err)
	}
	service := &contractObservatoryServer{mode: mode}
	server := grpc.NewServer()
	observatorycommand.RegisterObservatoryServiceServer(server, service)
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	go func() {
		_ = server.Serve(listener)
	}()

	liveDir, err := config.LiveRoleDir(apply.RoleClient)
	if err != nil {
		t.Fatalf("resolve client live directory: %v", err)
	}
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatalf("create client live directory: %v", err)
	}
	fixture := fmt.Sprintf(`{"api":{"listen":%q}}`, listener.Addr().String())
	writeContractFixture(t, filepath.Join(liveDir, layout.XrayConfigFileName), fixture)
}

type contractObservatoryServer struct {
	observatorycommand.UnimplementedObservatoryServiceServer
	mode string
}

func (s *contractObservatoryServer) GetOutboundStatus(context.Context, *observatorycommand.GetOutboundStatusRequest) (*observatorycommand.GetOutboundStatusResponse, error) {
	if s.mode == "error" {
		return nil, status.Error(codes.Unavailable, "matrix Observatory unavailable")
	}
	var statuses []*observatoryconfig.OutboundStatus
	if s.mode == "success" {
		statuses = []*observatoryconfig.OutboundStatus{
			{
				OutboundTag:  "zulu Ω outbound",
				Alive:        true,
				Delay:        42,
				LastTryTime:  1700000000,
				LastSeenTime: 1700000100,
				HealthPing:   &observatoryconfig.HealthPingMeasurementResult{All: 5, Fail: 1, Average: 45},
			},
			{OutboundTag: "alpha", LastErrorReason: "timeout\nretry"},
		}
	}
	return &observatorycommand.GetOutboundStatusResponse{
		Status: &observatoryconfig.ObservationResult{Status: statuses},
	}, nil
}
