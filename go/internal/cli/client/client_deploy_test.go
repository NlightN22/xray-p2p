package clientcmd

import (
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	deploylink "github.com/NlightN22/xray-p2p/go/internal/deploy/link"
	"github.com/NlightN22/xray-p2p/go/internal/deploy/spec"
)

func TestBuildDeployLinkPersistsManifest(t *testing.T) {
	opts := deployOptions{
		manifest: manifestOptions{
			installDir:     "/srv/xp2p",
			trojanPort:     "65001",
			trojanUser:     "user@example.invalid",
			trojanPassword: "p@ssw0rd",
		},
		runtime: runtimeOptions{
			remoteHost: "deploy.gw.local",
			deployPort: "62025",
			serverHost: "edge.internal",
		},
	}

	linkURL, err := buildDeployLink(&opts)
	if err != nil {
		t.Fatalf("buildDeployLink error: %v", err)
	}
	if len(opts.runtime.ciphertext) == 0 {
		t.Fatalf("ciphertext not stored in runtime options")
	}

	gotManifest, err := deploylink.Decrypt(linkURL, opts.runtime.ciphertext)
	if err != nil {
		t.Fatalf("decrypt returned error: %v", err)
	}

	want := spec.Manifest{
		Host:           "edge.internal",
		Version:        2,
		InstallDir:     "/srv/xp2p",
		TrojanPort:     "65001",
		TrojanUser:     "user@example.invalid",
		TrojanPassword: "p@ssw0rd",
	}
	got := gotManifest
	got.ExpiresAt = 0

	if got != want {
		t.Fatalf("manifest mismatch: got %#v want %#v", got, want)
	}
}

func TestEnsureDeployTargetAvailableRejectsDuplicateHostPort(t *testing.T) {
	restore := stubClientList(func(opts client.ListOptions) ([]client.EndpointRecord, error) {
		return []client.EndpointRecord{
			{Hostname: "edge.local", Port: 62070},
		}, nil
	})
	defer restore()

	cfg := clientCfg("/etc/xp2p", "config-client")
	opts := deployOptions{
		manifest: manifestOptions{
			trojanPort: "62070",
		},
		runtime: runtimeOptions{
			serverHost: "edge.local",
			remoteHost: "deploy.local",
		},
	}

	if err := ensureDeployTargetAvailable(cfg, opts); err == nil {
		t.Fatalf("expected duplicate deploy target to be rejected")
	}
}
