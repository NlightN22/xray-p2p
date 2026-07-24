package heartbeat

import (
	"errors"
	"sync"
	"time"
)

// Payload describes the telemetry metrics reported by clients.
type Payload struct {
	Tag        string       `json:"tag"`
	Host       string       `json:"host"`
	User       string       `json:"user,omitempty"`
	ClientIP   string       `json:"client_ip,omitempty"`
	Timestamp  time.Time    `json:"timestamp"`
	RTTMillis  int64        `json:"rtt_ms"`
	Healthy    *bool        `json:"healthy,omitempty"`
	Mode       Mode         `json:"mode,omitempty"`
	Capability Capability   `json:"capability,omitempty"`
	Stage      FailureStage `json:"failure_stage,omitempty"`
	Failure    string       `json:"failure,omitempty"`
	EndpointID string       `json:"endpoint_id,omitempty"`
	RTTValid   bool         `json:"rtt_valid,omitempty"`
}

type Mode string

const (
	ModeAuto     Mode = "auto"
	ModeRequired Mode = "required"
	ModeDisabled Mode = "disabled"
)

type Capability string

const (
	CapabilityUnknown       Capability = "unknown"
	CapabilityDetected      Capability = "detected"
	CapabilityXP2PHeartbeat Capability = "xp2p-heartbeat"
	CapabilityXP2PDiag      Capability = "xp2p-diag"
)

type Status string

const (
	StatusProbing     Status = "probing"
	StatusNotDetected Status = "not-detected"
	StatusHealthy     Status = "healthy"
	StatusUnhealthy   Status = "unhealthy"
	StatusDisabled    Status = "disabled"
)

type FailureStage string

const (
	FailureStageMarker      FailureStage = "marker"
	FailureStageProbe       FailureStage = "probe"
	FailureStageReport      FailureStage = "report"
	FailureStagePersistence FailureStage = "persistence"
)

const (
	DiscoveryFailureThreshold = 3
	HealthFailureThreshold    = 3
	MaxFutureClockSkew        = 30 * time.Second
)

// Entry stores aggregated statistics for a single client tunnel.
type Entry struct {
	Tag                 string       `json:"tag"`
	Host                string       `json:"host"`
	User                string       `json:"user,omitempty"`
	ClientIP            string       `json:"client_ip,omitempty"`
	LastRTTMillis       int64        `json:"last_rtt_ms"`
	MinRTTMillis        int64        `json:"min_rtt_ms"`
	MaxRTTMillis        int64        `json:"max_rtt_ms"`
	TotalRTTMillis      int64        `json:"total_rtt_ms"`
	Samples             int64        `json:"samples"`
	LastSeen            time.Time    `json:"last_seen"`
	Healthy             *bool        `json:"healthy,omitempty"`
	Mode                Mode         `json:"mode,omitempty"`
	Capability          Capability   `json:"capability,omitempty"`
	Status              Status       `json:"status,omitempty"`
	LastSuccess         time.Time    `json:"last_success,omitempty"`
	FailureStage        FailureStage `json:"failure_stage,omitempty"`
	Failure             string       `json:"failure,omitempty"`
	ConsecutiveFailures int          `json:"consecutive_failures,omitempty"`
	EndpointID          string       `json:"endpoint_id,omitempty"`
	Attempts            int64        `json:"attempts,omitempty"`
}

// AvgRTTMillis returns the average RTT observed so far.
func (e Entry) AvgRTTMillis() float64 {
	if e.Samples == 0 {
		return 0
	}
	return float64(e.TotalRTTMillis) / float64(e.Samples)
}

// State is the persisted representation of heartbeat statistics.
type State struct {
	Entries map[string]Entry `json:"entries"`
}

// Snapshot summarizes an entry with live status derived from TTL.
type Snapshot struct {
	Entry        Entry
	AvgRTTMillis float64
	Alive        bool
	Age          time.Duration
	Reason       string
}

// Store keeps heartbeat statistics in memory and mirrors them to disk.
type Store struct {
	mu      sync.RWMutex
	path    string
	state   State
	persist func(string, State) error
}

var (
	// ErrTagRequired signals that the payload lacks a tunnel tag.
	ErrTagRequired = errors.New("heartbeat: tag is required")
	// ErrHostRequired signals that the payload lacks a host identifier.
	ErrHostRequired = errors.New("heartbeat: host is required")
)
