//go:build linux || windows

package client

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func TestUpdateEndpointCredentialsStagesWhenRuntimeIsNotRunning(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)

	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
	initial := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{
				Hostname:      "edge.example",
				Tag:           "proxy-edge",
				Address:       "198.51.100.10",
				Port:          8443,
				User:          "old-user",
				Password:      "old-password",
				ServerName:    "edge.example",
				AllowInsecure: true,
			},
		},
		Redirects: []redirect.Rule{{CIDR: "10.20.0.0/16", OutboundTag: "proxy-edge"}},
		Reverse: map[string]clientReverseChannel{
			"oldedge-example.rev": {
				UserID:      "old-user",
				Host:        "edge.example",
				Tag:         "oldedge-example.rev",
				Domain:      "oldedge-example.rev",
				EndpointTag: "proxy-edge",
			},
		},
	}
	if err := initial.save(statePath); err != nil {
		t.Fatalf("save state: %v", err)
	}

	err := UpdateEndpointCredentials(context.Background(), UpdateEndpointOptions{
		Target:      "proxy-edge",
		User:        "new-user",
		Password:    "new-password",
		UserSet:     true,
		PasswordSet: true,
	})
	if err != nil {
		t.Fatalf("UpdateEndpointCredentials failed: %v", err)
	}

	updated, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("load updated state: %v", err)
	}
	if got := len(updated.Endpoints); got != 1 {
		t.Fatalf("expected one endpoint, got %d", got)
	}
	ep := updated.Endpoints[0]
	if ep.User != "new-user" || ep.Password != "new-password" {
		t.Fatalf("credentials were not staged: %+v", ep)
	}
	if ep.Tag != "proxy-edge" || ep.Hostname != "edge.example" || ep.Address != "198.51.100.10" {
		t.Fatalf("immutable endpoint fields changed: %+v", ep)
	}
	if len(updated.Redirects) != 1 || updated.Redirects[0].OutboundTag != "proxy-edge" {
		t.Fatalf("redirects changed: %+v", updated.Redirects)
	}
	channel := updated.Reverse["oldedge-example.rev"]
	if channel.UserID != "old-user" || channel.EndpointTag != "proxy-edge" {
		t.Fatalf("reverse channel changed: %+v", channel)
	}
	if _, err := os.Stat(config.ApplyRequestPath()); !os.IsNotExist(err) {
		t.Fatalf("apply request should not be written for runtime command: %v", err)
	}
}
