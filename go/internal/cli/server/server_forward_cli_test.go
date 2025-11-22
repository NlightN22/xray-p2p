package servercmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func TestRunServerForwardAddSuccess(t *testing.T) {
	restore := stubServerForwardAdd(func(opts server.ForwardAddOptions) (server.ForwardAddResult, error) {
		if opts.InstallDir != `C:\xp2p` || opts.ConfigDir != "cfg" {
			t.Fatalf("unexpected install paths: %+v", opts)
		}
		if opts.Target != "192.0.2.10:22" {
			t.Fatalf("unexpected target %s", opts.Target)
		}
		if opts.ListenAddress != "0.0.0.0" || opts.ListenPort != 60022 {
			t.Fatalf("unexpected listen %s:%d", opts.ListenAddress, opts.ListenPort)
		}
		if opts.Protocol != forward.ProtocolTCP {
			t.Fatalf("unexpected proto %s", opts.Protocol)
		}
		return server.ForwardAddResult{
			Rule: forward.Rule{
				ListenAddress: "0.0.0.0",
				ListenPort:    60022,
				TargetIP:      "192.0.2.10",
				TargetPort:    22,
				Remark:        "ssh",
				Protocol:      forward.ProtocolTCP,
			},
			Routed: true,
		}, nil
	})
	t.Cleanup(restore)

	cfg := serverCfg(`C:\xp2p`, "cfg", "edge.example.com")
	args := []string{
		"--path", `C:\xp2p`,
		"--config-dir", "cfg",
		"--target", "192.0.2.10:22",
		"--listen", "0.0.0.0",
		"--listen-port", "60022",
		"--proto", "tcp",
	}
	if code := runServerForwardAdd(context.Background(), cfg, args); code != 0 {
		t.Fatalf("runServerForwardAdd exit code = %d, want 0", code)
	}
}

func TestRunServerForwardAddValidatesFlags(t *testing.T) {
	cfg := config.Config{}
	if code := runServerForwardAdd(context.Background(), cfg, []string{"--proto", "bad"}); code != 2 {
		t.Fatalf("expected validation failure, got %d", code)
	}
}

func TestRunServerForwardRemoveIgnoreMissing(t *testing.T) {
	restore := stubServerForwardRemove(func(opts server.ForwardRemoveOptions) (forward.Rule, error) {
		return forward.Rule{}, errors.New("missing")
	})
	t.Cleanup(restore)

	cfg := serverCfg(`C:\xp2p`, "cfg", "")
	args := []string{"--path", `C:\xp2p`, "--config-dir", "cfg", "--listen-port", "60022", "--ignore-missing"}
	if code := runServerForwardRemove(context.Background(), cfg, args); code != 0 {
		t.Fatalf("runServerForwardRemove exit code = %d, want 0", code)
	}
}

func TestRunServerForwardRemoveRequiresSelector(t *testing.T) {
	cfg := config.Config{}
	if code := runServerForwardRemove(context.Background(), cfg, nil); code != 2 {
		t.Fatalf("expected selector validation failure, got %d", code)
	}
}

func TestRunServerForwardListOutputs(t *testing.T) {
	restore := stubServerForwardList(func(opts server.ForwardListOptions) ([]forward.Rule, error) {
		return []forward.Rule{
			{
				ListenAddress: "127.0.0.1",
				ListenPort:    60022,
				TargetIP:      "192.0.2.10",
				TargetPort:    22,
				Protocol:      forward.ProtocolTCP,
				Remark:        "ssh",
			},
		}, nil
	})
	t.Cleanup(restore)

	output := captureStdout(t, func() {
		code := runServerForwardList(context.Background(), config.Config{}, []string{"--path", `C:\xp2p`, "--config-dir", "cfg"})
		if code != 0 {
			t.Fatalf("runServerForwardList exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(output, "LISTEN") || !strings.Contains(output, "ssh") {
		t.Fatalf("unexpected list output: %q", output)
	}
}
