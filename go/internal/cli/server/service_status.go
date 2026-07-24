package servercmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	clioutput "github.com/NlightN22/xray-p2p/go/internal/cli/output"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func newServerServiceStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show xp2p server service status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerServiceStatus(commandContext(cmd), cmd.OutOrStdout())
			return errorForCode(code)
		},
	}
}

func runServerServiceStatus(ctx context.Context, out any) int {
	ctrl := servicecontrol.Default()
	status, err := ctrl.Status(ctx, servicecontrol.RoleServer)
	if err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Error("xp2p server service status is not supported on this platform")
		} else {
			logging.Error("failed to query xp2p server service status", "err", err)
		}
		return 1
	}
	if clioutput.EnabledContext(ctx) {
		detail := strings.TrimSpace(status.Detail)
		var detailValue *string
		if detail != "" {
			detailValue = &detail
		}
		if err := clioutput.SetResultContext(ctx, struct {
			State  string  `json:"state"`
			Active bool    `json:"active"`
			Detail *string `json:"detail"`
		}{State: status.State, Active: status.Active, Detail: detailValue}); err != nil {
			logging.Error("xp2p server service status: publish JSON result failed", "err", err)
			return 1
		}
	}

	if !clioutput.EnabledContext(ctx) {
		if writer, ok := out.(interface{ Write([]byte) (int, error) }); ok {
			if detail := strings.TrimSpace(status.Detail); detail != "" {
				_, _ = writer.Write([]byte(detail + "\n"))
			} else {
				text := fmt.Sprintf("xp2p server service state: %s\n", status.State)
				_, _ = writer.Write([]byte(text))
			}
		}
	}
	if status.Active {
		return 0
	}
	return 3
}
