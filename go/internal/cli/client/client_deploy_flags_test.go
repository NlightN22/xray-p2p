package clientcmd

import (
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
)

func TestParseDeployFlagsPopulatesOptions(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{
			Host: "srv.example.com",
			Port: "62022",
		},
		Client: config.ClientConfig{
			User:     "default@example.com",
			Password: "default-pass",
		},
	}

	args := []string{
		"--remote-host", "deploy.example.com",
		"--deploy-port", "62030",
		"--user", "branch@example.com",
		"--password", "secret",
		"--trojan-port", "65010",
	}

	opts, err := parseDeployFlags(cfg, args)
	if err != nil {
		t.Fatalf("parseDeployFlags returned error: %v", err)
	}
	if opts.runtime.remoteHost != "deploy.example.com" {
		t.Fatalf("runtime remote host = %s", opts.runtime.remoteHost)
	}
	if opts.runtime.deployPort != "62030" {
		t.Fatalf("runtime deploy port = %s", opts.runtime.deployPort)
	}
	if opts.runtime.serverHost != "srv.example.com" {
		t.Fatalf("runtime server host = %s", opts.runtime.serverHost)
	}
	if opts.manifest.trojanPort != "65010" {
		t.Fatalf("manifest trojan port = %s", opts.manifest.trojanPort)
	}
	if opts.manifest.trojanUser != "branch@example.com" {
		t.Fatalf("manifest user = %s", opts.manifest.trojanUser)
	}
	if opts.manifest.trojanPassword != "secret" {
		t.Fatalf("manifest password = %s", opts.manifest.trojanPassword)
	}
}

func TestParseDeployFlagsRequiresRemoteHost(t *testing.T) {
	_, err := parseDeployFlags(config.Config{}, []string{"--user", "demo", "--password", "secret"})
	if err == nil {
		t.Fatalf("expected error for missing remote host")
	}
}

func TestBuildInstallOptionsFromLinkUsesConfigDefaults(t *testing.T) {
	cfg := config.Config{
		Client: config.ClientConfig{
			InstallDir: `C:\xp2p`,
			ConfigDir:  "cfg-client",
		},
	}

	opts := buildInstallOptionsFromLink(cfg, trojanLink{
		ServerAddress: "edge.example.com",
		ServerPort:    "62022",
		User:          "user@example.com",
		Password:      "secret",
		ServerName:    "edge.example.com",
		AllowInsecure: true,
	})

	if opts.InstallDir != `C:\xp2p` || opts.ConfigDir != "cfg-client" {
		t.Fatalf("unexpected install paths: %+v", opts)
	}
	if opts.ServerAddress != "edge.example.com" || opts.ServerPort != "62022" {
		t.Fatalf("unexpected target: %+v", opts)
	}
	if !opts.AllowInsecure {
		t.Fatalf("expected allow insecure")
	}
}
