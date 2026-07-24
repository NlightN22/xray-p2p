package servercmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

type serverCertStateOptions struct {
	Path      string
	ConfigDir string
	Pending   bool
}

func runServerCertState(ctx context.Context, cfg config.Config, opts serverCertStateOptions) int {
	state, err := serverCertStateFunc(server.CertificateStateOptions{
		InstallDir: firstNonEmpty(opts.Path, cfg.Server.InstallDir),
		ConfigDir:  firstNonEmpty(opts.ConfigDir, cfg.Server.ConfigDir),
		Pending:    opts.Pending,
	})
	if err != nil {
		logging.Error("xp2p server cert state failed", "err", err)
		return 1
	}
	if clioutput.EnabledContext(ctx) {
		var notBefore, notAfter *string
		if !state.NotBefore.IsZero() {
			value := state.NotBefore.UTC().Format(time.RFC3339)
			notBefore = &value
		}
		if !state.NotAfter.IsZero() {
			value := state.NotAfter.UTC().Format(time.RFC3339)
			notAfter = &value
		}
		result := struct {
			CertificatePath string   `json:"certificate_path"`
			KeyPath         string   `json:"key_path"`
			Subject         string   `json:"subject"`
			DNSNames        []string `json:"dns_names"`
			IPAddresses     []string `json:"ip_addresses"`
			SelfSigned      bool     `json:"self_signed"`
			NotBefore       *string  `json:"not_before"`
			NotAfter        *string  `json:"not_after"`
			Status          string   `json:"status"`
			RemainingDays   int      `json:"remaining_days"`
			Issues          []string `json:"issues"`
		}{
			CertificatePath: state.CertPath, KeyPath: state.KeyPath, Subject: state.Subject,
			DNSNames: append([]string(nil), state.DNSNames...), IPAddresses: append([]string(nil), state.IPAddresses...),
			SelfSigned: state.SelfSigned, NotBefore: notBefore, NotAfter: notAfter,
			Status: string(state.Status), RemainingDays: state.RemainingDays,
			Issues: append([]string(nil), state.Issues...),
		}
		if result.DNSNames == nil {
			result.DNSNames = []string{}
		}
		if result.IPAddresses == nil {
			result.IPAddresses = []string{}
		}
		if result.Issues == nil {
			result.Issues = []string{}
		}
		if err := clioutput.SetResultContext(ctx, result); err != nil {
			logging.Error("xp2p server cert state: publish JSON result failed", "err", err)
			return 1
		}
	} else {
		renderCertificateState(state)
	}

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
	fmt.Printf("Self-signed: %s\n", formatYesNo(state.SelfSigned))
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

func formatYesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
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
