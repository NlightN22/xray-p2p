package heartbeat

import (
	"errors"
	"sync"
	"time"
)

// Payload describes the telemetry metrics reported by clients.
type Payload struct {
	Tag       string    `json:"tag"`
	Host      string    `json:"host"`
	User      string    `json:"user,omitempty"`
	ClientIP  string    `json:"client_ip,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	RTTMillis int64     `json:"rtt_ms"`
	Healthy   *bool     `json:"healthy,omitempty"`
}

// Entry stores aggregated statistics for a single client tunnel.
type Entry struct {
	Tag            string    `json:"tag"`
	Host           string    `json:"host"`
	User           string    `json:"user,omitempty"`
	ClientIP       string    `json:"client_ip,omitempty"`
	LastRTTMillis  int64     `json:"last_rtt_ms"`
	MinRTTMillis   int64     `json:"min_rtt_ms"`
	MaxRTTMillis   int64     `json:"max_rtt_ms"`
	TotalRTTMillis int64     `json:"total_rtt_ms"`
	Samples        int64     `json:"samples"`
	LastSeen       time.Time `json:"last_seen"`
	Healthy        *bool     `json:"healthy,omitempty"`
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
}

// Store keeps heartbeat statistics in memory and mirrors them to disk.
type Store struct {
	mu    sync.RWMutex
	path  string
	state State
}

var (
	// ErrTagRequired signals that the payload lacks a tunnel tag.
	ErrTagRequired = errors.New("heartbeat: tag is required")
	// ErrHostRequired signals that the payload lacks a host identifier.
	ErrHostRequired = errors.New("heartbeat: host is required")
)
