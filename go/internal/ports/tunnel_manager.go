package ports

import "context"

type TunnelMode string

const (
	TunnelModeSplit TunnelMode = "split"
	TunnelModeFull  TunnelMode = "full"
)

type TunStatusRequest struct {
	Name         string
	Addr         string
	RequireIPv4  bool
	RequireUp    bool
	RequireReady bool
}

type TunEnsureRequest struct {
	Name    string
	Addr    string
	Timeout string
	Verbose bool
}

type TunStatus struct {
	IfIndex    int
	IP         string
	Prefix     int
	OperStatus string
	DadState   string
	Ready      bool
}

type ModeConfigRequest struct {
	Role        string
	Mode        TunnelMode
	TunName     string
	TunAddr     string
	TunMTU      int
	FullTag     string
	ConfigPath  string
	Force       bool
}

type ServiceWaitRequest struct {
	Role    string
	Timeout string
}

type SplitRouteRequest struct {
	Name        string
	Addr        string
	CIDRs       []string
	AssignIP    bool
	AssignWait  string
	Verbose     bool
}

type FullRouteRequest struct {
	Name           string
	Addr           string
	BypassTargets  []string
	ForceDefault   bool
	Verbose        bool
}

type RouteRestoreRequest struct {
	Name    string
	Addr    string
	Verbose bool
}

type RouteResult struct {
	Applied bool
	Details string
	Status  TunStatus
}

type TunCleanupRequest struct {
	Name       string
	WintunPath string
}

type TunCleanupResult struct {
	Result string
	Errors []string
}

type TunnelManager interface {
	ApplyModeConfig(ctx context.Context, req ModeConfigRequest) error
	WaitServiceRestart(ctx context.Context, req ServiceWaitRequest) error
	Status(ctx context.Context, req TunStatusRequest) (TunStatus, error)
	EnsureReady(ctx context.Context, req TunEnsureRequest) (TunStatus, error)
	ApplySplitRoutes(ctx context.Context, req SplitRouteRequest) (RouteResult, error)
	ApplyFullRoutes(ctx context.Context, req FullRouteRequest) (RouteResult, error)
	RestoreRoutes(ctx context.Context, req RouteRestoreRequest) (RouteResult, error)
	CleanupTun(ctx context.Context, req TunCleanupRequest) (TunCleanupResult, error)
}
