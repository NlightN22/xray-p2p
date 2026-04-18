package clientcmd

import (
	"context"
	"errors"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	servicecontrol "github.com/NlightN22/xray-p2p/go/internal/service/control"
)

func restartClientServiceIfActive(ctx context.Context) error {
	ctrl := servicecontrol.Default()
	status, err := ctrl.Status(ctx, servicecontrol.RoleClient)
	if err != nil {
		if errors.Is(err, servicecontrol.ErrUnsupported) {
			logging.Warn("xp2p client mode: service status not supported; pending changes require manual restart")
			return nil
		}
		return err
	}
	if !status.Active {
		return nil
	}
	logging.Info("xp2p client mode: apply request recorded; service will restart automatically")
	return nil
}
