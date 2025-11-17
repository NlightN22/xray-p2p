package heartbeat

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Payload describes the telemetry metrics reported by clients.
type Payload struct {
	Tag       string    `json:"tag"`
	Host      string    `json:"host"`
	ClientIP  string    `json:"client_ip,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	RTTMillis int64     `json:"rtt_ms"`
}

// Entry stores aggregated statistics for a single client tunnel.
type Entry struct {
	Tag            string    `json:"tag"`
	Host           string    `json:"host"`
	ClientIP       string    `json:"client_ip,omitempty"`
	LastRTTMillis  int64     `json:"last_rtt_ms"`
	MinRTTMillis   int64     `json:"min_rtt_ms"`
	MaxRTTMillis   int64     `json:"max_rtt_ms"`
	TotalRTTMillis int64     `json:"total_rtt_ms"`
	Samples        int64     `json:"samples"`
	LastSeen       time.Time `json:"last_seen"`
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

// NewStore loads the heartbeat state from disk (if available) and keeps future
// updates in memory. When path is empty, updates remain in memory only.
func NewStore(path string) (*Store, error) {
	state, err := readState(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			state = State{}
		} else {
			return nil, err
		}
	}
	state.ensure()
	return &Store{
		path:  strings.TrimSpace(path),
		state: state,
	}, nil
}

// Update applies the payload metrics to the in-memory state and persists
// the new snapshot. When persistence fails the in-memory map is still updated.
func (s *Store) Update(payload Payload) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, err := s.state.update(payload)
	if err != nil {
		return Entry{}, err
	}
	if err := s.saveLocked(); err != nil {
		return entry, err
	}
	return entry, nil
}

// Snapshot returns a sorted slice of tunnel records annotated with liveness
// information relative to the provided TTL.
func (s *Store) Snapshot(now time.Time, ttl time.Duration) []Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.snapshot(now, ttl)
}

// Snapshot returns a sorted slice of entries with derived liveness info.
func (s State) Snapshot(now time.Time, ttl time.Duration) []Snapshot {
	return s.snapshot(now, ttl)
}

// Load reads the heartbeat state from path without keeping it in memory.
func Load(path string) (State, error) {
	state, err := readState(path)
	if err != nil {
		return State{}, err
	}
	state.ensure()
	return state, nil
}

// Save writes the provided state to the given path.
func Save(path string, state State) error {
	state.ensure()
	return writeState(path, state)
}

// Snapshot returns a sorted view of the in-memory entries.
func (s State) snapshot(now time.Time, ttl time.Duration) []Snapshot {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	results := make([]Snapshot, 0, len(s.Entries))
	for _, entry := range s.Entries {
		age := time.Duration(0)
		if !entry.LastSeen.IsZero() {
			age = now.Sub(entry.LastSeen)
		}
		results = append(results, Snapshot{
			Entry:        entry,
			AvgRTTMillis: entry.AvgRTTMillis(),
			Alive:        ttl <= 0 || (entry.LastSeen.After(time.Time{}) && age <= ttl),
			Age:          age,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		leftTag := strings.ToLower(results[i].Entry.Tag)
		rightTag := strings.ToLower(results[j].Entry.Tag)
		if leftTag == rightTag {
			return strings.ToLower(results[i].Entry.Host) < strings.ToLower(results[j].Entry.Host)
		}
		return leftTag < rightTag
	})
	return results
}

func (s *State) ensure() {
	if s.Entries == nil {
		s.Entries = make(map[string]Entry)
	}
}

func (s *State) update(payload Payload) (Entry, error) {
	tag := strings.TrimSpace(payload.Tag)
	if tag == "" {
		return Entry{}, ErrTagRequired
	}
	host := strings.TrimSpace(payload.Host)
	if host == "" {
		return Entry{}, ErrHostRequired
	}
	key := strings.ToLower(tag)
	entry := s.Entries[key]
	entry.Tag = tag
	entry.Host = host
	if payload.ClientIP != "" {
		entry.ClientIP = strings.TrimSpace(payload.ClientIP)
	}

	if payload.Timestamp.IsZero() {
		entry.LastSeen = time.Now().UTC()
	} else {
		entry.LastSeen = payload.Timestamp.UTC()
	}

	if payload.RTTMillis < 0 {
		payload.RTTMillis = 0
	}
	entry.LastRTTMillis = payload.RTTMillis
	if entry.MinRTTMillis == 0 || payload.RTTMillis < entry.MinRTTMillis {
		entry.MinRTTMillis = payload.RTTMillis
	}
	if payload.RTTMillis > entry.MaxRTTMillis {
		entry.MaxRTTMillis = payload.RTTMillis
	}
	entry.TotalRTTMillis += payload.RTTMillis
	entry.Samples++

	s.Entries[key] = entry
	return entry, nil
}

func (s *Store) saveLocked() error {
	if s.path == "" {
		return nil
	}
	if err := writeState(s.path, s.state); err != nil {
		return fmt.Errorf("heartbeat: persist state: %w", err)
	}
	return nil
}

func readState(path string) (State, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return State{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return State{}, nil
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("heartbeat: parse %s: %w", path, err)
	}
	return state, nil
}

func writeState(path string, state State) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("heartbeat: ensure dir %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("heartbeat: encode state %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("heartbeat: write %s: %w", path, err)
	}
	return nil
}
