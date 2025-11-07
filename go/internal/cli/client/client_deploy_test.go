package clientcmd

import (
	"testing"

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
	if opts.runtime.encLink.Key == "" || len(opts.runtime.encLink.Ciphertext) == 0 {
		t.Fatalf("encrypted link data not stored in runtime options: %#v", opts.runtime.encLink)
	}

	parsed, err := deploylink.Parse(linkURL)
	if err != nil {
		t.Fatalf("parse returned error: %v", err)
	}

	want := spec.Manifest{
		Host:           "edge.internal",
		Version:        2,
		InstallDir:     "/srv/xp2p",
		TrojanPort:     "65001",
		TrojanUser:     "user@example.invalid",
		TrojanPassword: "p@ssw0rd",
	}
	got := parsed.Manifest
	got.ExpiresAt = 0

	if got != want {
		t.Fatalf("manifest mismatch: got %#v want %#v", got, want)
	}
}
