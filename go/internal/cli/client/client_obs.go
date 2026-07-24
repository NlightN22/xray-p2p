package clientcmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/xrayapi"
)

type clientObsOptions struct {
	Path    string
	XrayAPI string
}

var clientObsStatusesFunc = func(ctx context.Context, opts xrayapi.ObservatoryOptions) ([]xrayapi.OutboundObservation, error) {
	return xrayapi.GetOutboundStatuses(ctx, opts)
}

func newClientObsCmd(cfg commandConfig) *cobra.Command {
	opts := clientObsOptions{}
	cmd := &cobra.Command{
		Use:   "obs",
		Short: "Show Xray outbound observations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runClientObs(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "client installation directory")
	flags.StringVarP(&opts.XrayAPI, "xray-api", "A", "", "Xray API address")
	return cmd
}

func runClientObs(ctx context.Context, cfg config.Config, opts clientObsOptions) int {
	address := strings.TrimSpace(opts.XrayAPI)
	if address == "" {
		liveConfig, err := clientObsLiveConfigPath(cfg, opts)
		if err != nil {
			logging.Error("client obs: failed to resolve live config", "err", err)
			return 2
		}
		data, err := os.ReadFile(liveConfig)
		if err != nil {
			logging.Error("client obs: failed to read live xray config", "err", err)
			return 1
		}
		address, err = xrayapi.APIListenFromConfig(data)
		if err != nil {
			logging.Error("client obs: failed to read Xray API address", "err", err)
			return 1
		}
		if address == "" {
			logging.Error("client obs: Xray API address is missing")
			return 1
		}
	}

	statuses, err := clientObsStatusesFunc(ctx, xrayapi.ObservatoryOptions{
		Address: address,
		Timeout: xrayapi.DefaultTimeout,
	})
	if err != nil {
		logging.Error("client obs: failed to query observations", "err", err)
		return 1
	}
	if clioutput.EnabledContext(ctx) {
		type observationResult struct {
			Tag          string  `json:"tag"`
			Alive        bool    `json:"alive"`
			DelayMillis  int64   `json:"delay_millis"`
			LastTryAt    *string `json:"last_try_at"`
			LastSeenAt   *string `json:"last_seen_at"`
			Error        *string `json:"error"`
			HealthChecks int64   `json:"health_checks"`
			HealthFails  int64   `json:"health_failures"`
			HealthAvgMS  int64   `json:"health_average_millis"`
		}
		result := struct {
			Observations []observationResult `json:"observations"`
		}{Observations: make([]observationResult, 0, len(statuses))}
		for _, status := range statuses {
			result.Observations = append(result.Observations, observationResult{
				Tag: status.Tag, Alive: status.Alive, DelayMillis: status.DelayMillis,
				LastTryAt: optionalUnixTime(status.LastTryUnix), LastSeenAt: optionalUnixTime(status.LastSeenUnix),
				Error: optionalString(status.LastError), HealthChecks: status.HealthAll,
				HealthFails: status.HealthFail, HealthAvgMS: status.HealthAverageMs,
			})
		}
		if err := clioutput.SetResultContext(ctx, result); err != nil {
			logging.Error("client obs: publish JSON result failed", "err", err)
			return 1
		}
		return 0
	}
	renderClientObs(statuses)
	return 0
}

func optionalUnixTime(value int64) *string {
	if value <= 0 {
		return nil
	}
	formatted := time.Unix(value, 0).UTC().Format(time.RFC3339)
	return &formatted
}

func clientObsLiveConfigPath(cfg config.Config, opts clientObsOptions) (string, error) {
	if strings.TrimSpace(opts.Path) == "" {
		liveDir, err := config.LiveRoleDir(apply.RoleClient)
		if err != nil {
			return "", err
		}
		return filepath.Join(liveDir, layout.XrayConfigFileName), nil
	}
	configDir := strings.TrimSpace(cfg.Client.ConfigDir)
	if configDir == "" {
		configDir = layout.ClientConfigDir
	}
	return filepath.Join(strings.TrimSpace(opts.Path), layout.StateDirName, layout.LiveDirName, configDir, layout.XrayConfigFileName), nil
}

func renderClientObs(statuses []xrayapi.OutboundObservation) {
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "TAG\tALIVE\tDELAY\tLAST TRY\tLAST SEEN\tERROR")
	for _, status := range statuses {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n",
			emptyDash(status.Tag),
			formatAlive(status.Alive),
			formatDelay(status.DelayMillis),
			formatUnix(status.LastTryUnix),
			formatUnix(status.LastSeenUnix),
			emptyDash(status.LastError),
		)
	}
	_ = writer.Flush()
}

func formatAlive(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func formatDelay(value int64) string {
	if value <= 0 {
		return "-"
	}
	return fmt.Sprintf("%dms", value)
}

func formatUnix(value int64) string {
	if value <= 0 {
		return "-"
	}
	return time.Unix(value, 0).UTC().Format(time.RFC3339)
}

func emptyDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}
