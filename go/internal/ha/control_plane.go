package ha

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrQuorumUnavailable         = errors.New("HA quorum is unavailable")
	ErrForceReconfigureForbidden = errors.New("HA force-reconfiguration is not authorized")
)

type MembershipRole string

const (
	RoleVoter   MembershipRole = "voter"
	RoleWitness MembershipRole = "witness"
)

type Election struct {
	Coordinator string   `json:"coordinator"`
	Voters      []string `json:"voters"`
	Quorum      int      `json:"quorum"`
}

type QuorumState struct {
	Voters      []string `json:"voters"`
	Ready       []string `json:"ready"`
	Required    int      `json:"required"`
	Available   bool     `json:"available"`
	Witnesses   []string `json:"witnesses,omitempty"`
	Unavailable []string `json:"unavailable,omitempty"`
}

type RecoveryState struct {
	CommittedGeneration uint64      `json:"committed_generation"`
	Quorum              QuorumState `json:"quorum"`
	Election            Election    `json:"election"`
	Pending             bool        `json:"pending"`
}

type ForceReconfiguration struct {
	Authorization string `json:"authorization"`
	Reason        string `json:"reason"`
}

func (p Peer) MembershipRole() MembershipRole {
	if p.Witness {
		return RoleWitness
	}
	return RoleVoter
}

func (p Peer) Voting() bool {
	return !p.NonVoting
}

func QuorumSize(voters int) int {
	if voters <= 0 {
		return 0
	}
	return voters/2 + 1
}

func ElectCoordinator(localID string, peers []Peer) Election {
	voters := votingPeerIDs(localID, peers)
	sort.Strings(voters)
	election := Election{Voters: voters, Quorum: QuorumSize(len(voters))}
	if len(voters) > 0 {
		election.Coordinator = voters[0]
	}
	return election
}

func (s *Store) RecoveryState() RecoveryState {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending := s.pending != nil
	return RecoveryState{
		CommittedGeneration: s.committed.Number,
		Quorum:              s.quorumStateLocked(),
		Election:            ElectCoordinator(s.localID, mapPeers(s.peers)),
		Pending:             pending,
	}
}

func (s *Store) ForceReconfigure(candidate Generation, request ForceReconfiguration) (Generation, error) {
	if strings.TrimSpace(request.Authorization) != "force" || strings.TrimSpace(request.Reason) == "" {
		return Generation{}, ErrForceReconfigureForbidden
	}
	if err := candidate.Validate(); err != nil {
		return Generation{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if candidate.Number <= s.committed.Number {
		return Generation{}, ErrGenerationOutOfOrder
	}
	if len(s.votingPeerIDsLocked()) != 2 {
		return Generation{}, fmt.Errorf("HA force-reconfiguration requires a two-voter old membership")
	}
	if s.onCommit != nil {
		if err := s.onCommit(candidate); err != nil {
			return Generation{}, err
		}
	}
	s.committed = candidate
	s.pending = nil
	s.acks = make(map[string]Acknowledgement)
	return s.committed, nil
}

func (s *Store) hasQuorumLocked() bool {
	state := s.quorumStateLocked()
	return state.Available
}

func (s *Store) quorumStateLocked() QuorumState {
	voters := s.votingPeerIDsLocked()
	readySet := map[string]struct{}{}
	for _, ack := range s.acks {
		if ack.Ready {
			readySet[strings.ToLower(strings.TrimSpace(ack.PeerID))] = struct{}{}
		}
	}
	if s.localID != "" {
		readySet[strings.ToLower(s.localID)] = struct{}{}
	}
	ready := make([]string, 0, len(voters))
	unavailable := make([]string, 0, len(voters))
	for _, voter := range voters {
		if _, ok := readySet[strings.ToLower(voter)]; ok {
			ready = append(ready, voter)
		} else {
			unavailable = append(unavailable, voter)
		}
	}
	witnesses := make([]string, 0)
	for _, peer := range s.peers {
		if peer.Witness {
			witnesses = append(witnesses, peer.ID)
		}
	}
	sort.Strings(witnesses)
	required := QuorumSize(len(voters))
	return QuorumState{
		Voters:      voters,
		Ready:       ready,
		Required:    required,
		Available:   len(ready) >= required,
		Witnesses:   witnesses,
		Unavailable: unavailable,
	}
}

func (s *Store) votingPeerIDsLocked() []string {
	return votingPeerIDs(s.localID, mapPeers(s.peers))
}

func votingPeerIDs(localID string, peers []Peer) []string {
	seen := map[string]struct{}{}
	ids := make([]string, 0, len(peers)+1)
	if id := strings.TrimSpace(localID); id != "" {
		seen[strings.ToLower(id)] = struct{}{}
		ids = append(ids, id)
	}
	for _, peer := range peers {
		if !peer.Voting() {
			continue
		}
		id := strings.TrimSpace(peer.ID)
		key := strings.ToLower(id)
		if id == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func mapPeers(peers map[string]Peer) []Peer {
	out := make([]Peer, 0, len(peers))
	for _, peer := range peers {
		out = append(out, peer)
	}
	return out
}
