package servercmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

type serverCertStateOptions struct {
	Path      string
	ConfigDir string
}

func runServerCertState(cfg config.Config, opts serverCertStateOptions) int {
	state, err := serverCertStateFunc(server.CertificateStateOptions{
		InstallDir: firstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
	})
	if err != nil {
		logging.Error("xp2p server cert state failed", "err", err)
		return 1
	}

	renderCertificateState(state)

	if state.Status != server.CertificateStatusOK || len(state.Issues) > 0 {
		return 1
	}
	return 0
}

func renderCertificateState(state server.CertificateState) {
	fmt.Printf("Certificate: %s\n", state.CertPath)
	fmt.Printf("Key:         %s\n", state.KeyPath)
	if state.Subject != "" {
		fmt.Printf("Subject:     %s\n", state.Subject)
	}
	if san := formatSAN(state); san != "" {
		fmt.Printf("SAN:         %s\n", san)
	}
	if !state.NotBefore.IsZero() && !state.NotAfter.IsZero() {
		fmt.Printf("Validity:    %s -> %s\n", state.NotBefore.UTC().Format(time.RFC3339), state.NotAfter.UTC().Format(time.RFC3339))
	}

	statusLine := fmt.Sprintf("Status:      %s", strings.ToUpper(string(state.Status)))
	if !state.NotAfter.IsZero() {
		statusLine = fmt.Sprintf("%s (expires in %d days)", statusLine, state.RemainingDays)
	}
	fmt.Println(statusLine)

	for _, issue := range state.Issues {
		fmt.Printf("Warning:     %s\n", issue)
	}
}

func formatSAN(state server.CertificateState) string {
	var parts []string
	if len(state.DNSNames) > 0 {
		parts = append(parts, "DNS: "+strings.Join(state.DNSNames, ", "))
	}
	if len(state.IPAddresses) > 0 {
		parts = append(parts, "IP: "+strings.Join(state.IPAddresses, ", "))
	}
	return strings.Join(parts, "; ")
}
