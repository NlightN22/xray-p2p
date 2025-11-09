package servercmd

import (
	"context"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func TestRunServerCertSet(t *testing.T) {
	yes := true
	tests := []struct {
		name      string
		cfg       config.Config
		opts      serverCertSetOptions
		host      string
		hostErr   error
		prompt    *bool
		promptErr error
		certErrs  []error
		wantCode  int
		wantCalls []server.CertificateOptions
	}{
		{
			name: "uses flags",
			cfg:  serverCfg(`C:\xp2p`, server.DefaultServerConfigDir, ""),
			opts: serverCertSetOptions{
				Path:      `D:\xp2p`,
				ConfigDir: "cfg-custom",
				Cert:      `C:\certs\server.pem`,
				Key:       `C:\certs\server.key`,
				Host:      "cert.example.test",
				Force:     true,
			},
			wantCode: 0,
			wantCalls: []server.CertificateOptions{
				{
					InstallDir:      `D:\xp2p`,
					ConfigDir:       "cfg-custom",
					CertificateFile: `C:\certs\server.pem`,
					KeyFile:         `C:\certs\server.key`,
					Host:            "cert.example.test",
					Force:           true,
				},
			},
		},
		{
			name:     "detects host when missing",
			cfg:      serverCfg(`C:\xp2p`, server.DefaultServerConfigDir, ""),
			opts:     serverCertSetOptions{Path: `C:\xp2p`},
			host:     "198.51.100.20",
			wantCode: 0,
			wantCalls: []server.CertificateOptions{
				{InstallDir: `C:\xp2p`, ConfigDir: server.DefaultServerConfigDir, Host: "198.51.100.20"},
			},
		},
		{
			name:     "retries when certificate exists",
			cfg:      serverCfg(`C:\xp2p`, server.DefaultServerConfigDir, "configured.example.test"),
			opts:     serverCertSetOptions{Path: `C:\xp2p`},
			prompt:   &yes,
			certErrs: []error{server.ErrCertificateConfigured, nil},
			wantCode: 0,
			wantCalls: []server.CertificateOptions{
				{InstallDir: `C:\xp2p`, ConfigDir: server.DefaultServerConfigDir, Host: "configured.example.test"},
				{InstallDir: `C:\xp2p`, ConfigDir: server.DefaultServerConfigDir, Host: "configured.example.test", Force: true},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			code, calls := execCertSet(tt.cfg, tt.opts, tt.host, tt.hostErr, tt.prompt, tt.promptErr, tt.certErrs)
			if code != tt.wantCode {
				t.Fatalf("exit code: got %d want %d", code, tt.wantCode)
			}
			if len(calls) != len(tt.wantCalls) {
				t.Fatalf("call count: got %d want %d", len(calls), len(tt.wantCalls))
			}
			for i := range tt.wantCalls {
				requireEqual(t, calls[i], tt.wantCalls[i], "certificate options")
			}
		})
	}
}

func execCertSet(cfg config.Config, opts serverCertSetOptions, host string, hostErr error, prompt *bool, promptErr error, certErrs []error) (int, []server.CertificateOptions) {
	var calls []server.CertificateOptions
	restoreCert := stubServerSetCertificate(func(ctx context.Context, opts server.CertificateOptions) error {
		calls = append(calls, opts)
		idx := len(calls) - 1
		if idx < len(certErrs) {
			return certErrs[idx]
		}
		return nil
	})
	defer restoreCert()
	defer stubDetectPublicHost(host, hostErr)()
	if prompt != nil {
		defer stubPromptYesNo(*prompt, promptErr)()
	}
	code := runServerCertSet(context.Background(), cfg, opts)
	return code, calls
}
