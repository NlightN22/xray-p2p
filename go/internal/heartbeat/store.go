package heartbeat

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// IsCorrupt reports whether an error indicates a malformed heartbeat state file.
func IsCorrupt(err error) bool {
	if err == nil {
		return false
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return true
	}
	var typeErr *json.UnmarshalTypeError
	return errors.As(err, &typeErr)
}

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
		path:    strings.TrimSpace(path),
		state:   state,
		persist: writeState,
	}, nil
}

// Update applies and persists a payload atomically. The in-memory state is
// unchanged when persistence fails.
func (s *Store) Update(payload Payload) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	candidate := s.state.clone()
	entry, err := candidate.update(payload)
	if err != nil {
		return Entry{}, err
	}
	if err := s.save(candidate); err != nil {
		entry.FailureStage = FailureStagePersistence
		entry.Failure = normalizeFailure(err.Error())
		failureState := State{Entries: map[string]Entry{payloadKey(payload): entry}}
		_ = writeState(persistenceFailurePath(s.path), failureState)
		return entry, err
	}
	s.state = candidate
	if s.path != "" {
		_ = os.Remove(persistenceFailurePath(s.path))
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
		alive, reason := entryAlive(entry, age, ttl)
		displayAge := age
		if displayAge < 0 && displayAge >= -MaxFutureClockSkew {
			displayAge = 0
		}
		if !alive && entry.Status == StatusHealthy && (reason == "expired" || reason == "clock_skew") {
			entry.Status = StatusUnhealthy
		}
		results = append(results, Snapshot{
			Entry:        entry,
			AvgRTTMillis: entry.AvgRTTMillis(),
			Alive:        alive,
			Age:          displayAge,
			Reason:       reason,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		leftTag := strings.ToLower(results[i].Entry.Tag)
		rightTag := strings.ToLower(results[j].Entry.Tag)
		if leftTag == rightTag {
			leftHost := strings.ToLower(results[i].Entry.Host)
			rightHost := strings.ToLower(results[j].Entry.Host)
			if leftHost == rightHost {
				leftUser := strings.ToLower(results[i].Entry.User)
				rightUser := strings.ToLower(results[j].Entry.User)
				if leftUser == rightUser {
					return strings.ToLower(results[i].Entry.ClientIP) < strings.ToLower(results[j].Entry.ClientIP)
				}
				return leftUser < rightUser
			}
			return leftHost < rightHost
		}
		return leftTag < rightTag
	})
	return results
}

func entryAlive(entry Entry, age, ttl time.Duration) (bool, string) {
	if age < -MaxFutureClockSkew {
		return false, "clock_skew"
	}
	if age < 0 {
		age = 0
	}
	if entry.Status != "" {
		if entry.Status == StatusDisabled || entry.Status == StatusNotDetected || entry.Status == StatusUnhealthy {
			return false, string(entry.Status)
		}
		if entry.Status == StatusProbing {
			return false, string(entry.Status)
		}
	}
	if entry.Healthy != nil && !*entry.Healthy {
		return false, "attempt_failed"
	}
	if ttl <= 0 || (entry.LastSeen.After(time.Time{}) && age <= ttl) {
		return true, ""
	}
	return false, "expired"
}

func (s *State) ensure() {
	if s.Entries == nil {
		s.Entries = make(map[string]Entry)
	}
}

func (s State) clone() State {
	clone := State{Entries: make(map[string]Entry, len(s.Entries))}
	for key, entry := range s.Entries {
		clone.Entries[key] = entry
	}
	return clone
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
	user := strings.TrimSpace(payload.User)
	endpointID := strings.TrimSpace(payload.EndpointID)
	key := payloadKey(payload)
	entry := s.Entries[key]
	if entry.Host != "" && !strings.EqualFold(strings.TrimSpace(entry.Host), host) {
		entry = Entry{}
	}
	entry.Tag = tag
	entry.Host = host
	entry.EndpointID = endpointID
	if payload.User != "" {
		entry.User = user
	}
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
	entry.Healthy = payload.Healthy
	entry.Attempts++
	mode := normalizeMode(payload.Mode)
	entry.Mode = mode
	if entry.Capability == "" {
		entry.Capability = CapabilityUnknown
	}
	if mode == ModeDisabled {
		entry.Status = StatusDisabled
		entry.FailureStage = ""
		entry.Failure = ""
	} else if payload.Healthy != nil && *payload.Healthy {
		entry.Capability = CapabilityDetected
		entry.Status = StatusHealthy
		entry.LastSuccess = entry.LastSeen
		entry.ConsecutiveFailures = 0
		entry.FailureStage = ""
		entry.Failure = ""
	} else if payload.Healthy != nil {
		entry.ConsecutiveFailures++
		entry.FailureStage = payload.Stage
		entry.Failure = normalizeFailure(payload.Failure)
		entry.Status = failureStatus(mode, entry.Capability, entry.ConsecutiveFailures)
	}
	if payload.RTTValid || payload.Healthy == nil || *payload.Healthy {
		entry.LastRTTMillis = payload.RTTMillis
		if entry.Samples == 0 || payload.RTTMillis < entry.MinRTTMillis {
			entry.MinRTTMillis = payload.RTTMillis
		}
		if payload.RTTMillis > entry.MaxRTTMillis {
			entry.MaxRTTMillis = payload.RTTMillis
		}
		entry.TotalRTTMillis += payload.RTTMillis
		entry.Samples++
	}

	s.Entries[key] = entry
	return entry, nil
}

func payloadKey(payload Payload) string {
	if endpointID := strings.TrimSpace(payload.EndpointID); endpointID != "" {
		return "v1|" + endpointID
	}
	return entryKey(payload.Tag, payload.User)
}

func normalizeMode(mode Mode) Mode {
	switch mode {
	case ModeAuto, ModeDisabled, ModeRequired:
		return mode
	default:
		return ModeRequired
	}
}

func failureStatus(mode Mode, capability Capability, failures int) Status {
	if mode == ModeAuto && capability != CapabilityDetected {
		if failures >= DiscoveryFailureThreshold {
			return StatusNotDetected
		}
		return StatusProbing
	}
	if failures >= HealthFailureThreshold {
		return StatusUnhealthy
	}
	if capability == CapabilityDetected {
		return StatusHealthy
	}
	return StatusProbing
}

func normalizeFailure(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 256 {
		value = value[:256]
	}
	return value
}

func (s *Store) save(state State) error {
	if s.path == "" {
		return nil
	}
	persist := s.persist
	if persist == nil {
		persist = writeState
	}
	if err := persist(s.path, state); err != nil {
		return fmt.Errorf("heartbeat: persist state: %w", err)
	}
	return nil
}

func entryKey(tag, user string) string {
	key := strings.ToLower(strings.TrimSpace(tag))
	if key == "" {
		return ""
	}
	user = strings.ToLower(strings.TrimSpace(user))
	if user == "" {
		return key
	}
	return key + "|" + user
}
