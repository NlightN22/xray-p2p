//go:build windows

package server

import (
	"context"
	"fmt"

	"golang.org/x/sys/windows/svc"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

// RunService launches the managed server service loop on Windows.
func RunService(ctx context.Context, opts ServiceOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if runningAsWindowsService() {
		handler := &serverWindowsService{opts: opts}
		if err := svc.Run("xp2p-server", handler); err != nil {
			return fmt.Errorf("xp2p server windows service: %w", err)
		}
		return handler.err
	}
	return runServerServiceLoop(ctx, opts)
}

func runServerServiceLoop(ctx context.Context, opts ServiceOptions) error {
	return runServerServiceCommon(ctx, opts)
}

type serverWindowsService struct {
	opts ServiceOptions
	err  error
}

func (s *serverWindowsService) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepts = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- runServerServiceLoop(ctx, s.opts)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: accepts}

	for {
		select {
		case err := <-errCh:
			s.err = err
			if err != nil {
				logging.Error("xp2p server service failed", "err", err)
				changes <- svc.Status{State: svc.Stopped}
				return false, 1
			}
			changes <- svc.Status{State: svc.Stopped}
			return false, 0
		case change := <-r:
			switch change.Cmd {
			case svc.Interrogate:
				changes <- change.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending, Accepts: accepts}
				cancel()
			default:
			}
		}
	}
}

func runningAsWindowsService() bool {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return false
	}
	return isService
}
