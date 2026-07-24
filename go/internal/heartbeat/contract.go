package heartbeat

const ContractVersion = 1

// Contract describes every stable value external heartbeat consumers may see.
type Contract struct {
	Schema             string         `json:"schema"`
	Version            int            `json:"version"`
	Modes              []Mode         `json:"modes"`
	Capabilities       []Capability   `json:"capabilities"`
	LegacyCapabilities []Capability   `json:"legacy_capabilities"`
	Checks             []string       `json:"checks"`
	Statuses           []Status       `json:"statuses"`
	FailureStages      []FailureStage `json:"failure_stages"`
	Thresholds         Thresholds     `json:"thresholds"`
}

type Thresholds struct {
	DiscoveryFailures int `json:"discovery_failures"`
	HealthFailures    int `json:"health_failures"`
}

// CurrentContract returns the machine-readable heartbeat contract.
func CurrentContract() Contract {
	return Contract{
		Schema:             "xp2p-heartbeat-contract",
		Version:            ContractVersion,
		Modes:              []Mode{ModeAuto, ModeRequired, ModeDisabled},
		Capabilities:       []Capability{CapabilityUnknown, CapabilityXP2PHeartbeat, CapabilityXP2PDiag},
		LegacyCapabilities: []Capability{CapabilityDetected},
		Checks:             []string{string(CapabilityUnknown), string(CapabilityXP2PHeartbeat), string(CapabilityXP2PDiag), "none"},
		Statuses:           []Status{StatusProbing, StatusNotDetected, StatusHealthy, StatusUnhealthy, StatusDisabled},
		FailureStages: []FailureStage{
			FailureStageMarker,
			FailureStageProbe,
			FailureStageReport,
			FailureStagePersistence,
		},
		Thresholds: Thresholds{
			DiscoveryFailures: DiscoveryFailureThreshold,
			HealthFailures:    HealthFailureThreshold,
		},
	}
}
