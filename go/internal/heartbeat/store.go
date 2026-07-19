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
			Alive:        entryAlive(entry, age, ttl),
			Age:          age,
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

func entryAlive(entry Entry, age, ttl time.Duration) bool {
	if entry.Healthy != nil && !*entry.Healthy {
		return false
	}
	return ttl <= 0 || (entry.LastSeen.After(time.Time{}) && age <= ttl)
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
	user := strings.TrimSpace(payload.User)
	key := entryKey(tag, user)
	entry := s.Entries[key]
	entry.Tag = tag
	entry.Host = host
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
	entry.LastRTTMillis = payload.RTTMillis
	entry.Healthy = payload.Healthy
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
