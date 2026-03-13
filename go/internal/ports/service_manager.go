package ports

import "context"

type ServiceState string

const (
	ServiceStateUnknown         ServiceState = "unknown"
	ServiceStateStopped         ServiceState = "stopped"
	ServiceStateStartPending    ServiceState = "start_pending"
	ServiceStateStopPending     ServiceState = "stop_pending"
	ServiceStateRunning         ServiceState = "running"
	ServiceStateContinuePending ServiceState = "continue_pending"
	ServiceStatePausePending    ServiceState = "pause_pending"
	ServiceStatePaused          ServiceState = "paused"
)

type ServiceInfo struct {
	Name        string
	DisplayName string
	State       ServiceState
}

type ServiceManager interface {
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Status(ctx context.Context, name string) (ServiceInfo, error)
	List(ctx context.Context, names []string) ([]ServiceInfo, error)
}
