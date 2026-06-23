package ha

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

type Peer struct {
	ID       string `json:"id" toml:"id"`
	Endpoint string `json:"endpoint,omitempty" toml:"endpoint,omitempty"`
	Secret   string `json:"-" toml:"secret"`
}

type PrepareRequest struct {
	PeerID     string     `json:"peer_id"`
	Generation Generation `json:"generation"`
	Signature  string     `json:"signature"`
}

type CommitRequest struct {
	PeerID     string `json:"peer_id"`
	Generation uint64 `json:"generation"`
	Signature  string `json:"signature"`
}

type Acknowledgement struct {
	PeerID     string `json:"peer_id"`
	Generation uint64 `json:"generation"`
	Ready      bool   `json:"ready"`
	Error      string `json:"error,omitempty"`
}

// Store keeps immutable candidates separate from the last committed generation.
type Store struct {
	mu        sync.Mutex
	peers     map[string]Peer
	committed Generation
	pending   *Generation
	acks      map[string]Acknowledgement
	onCommit  func(Generation) error
}

func (s *Store) SetCommitter(commit func(Generation) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCommit = commit
}

func NewStore(peers []Peer, committed Generation) (*Store, error) {
	if committed.Number != 0 {
		if err := committed.Validate(); err != nil {
			return nil, err
		}
	}
	s := &Store{peers: make(map[string]Peer, len(peers)), committed: committed, acks: make(map[string]Acknowledgement)}
	for _, peer := range peers {
		if peer.ID == "" || peer.Secret == "" {
			return nil, errors.New("HA peer ID and secret are required")
		}
		s.peers[peer.ID] = peer
	}
	return s, nil
}

func Sign(peer Peer, generation Generation) (string, error) {
	payload, err := json.Marshal(generation)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(peer.Secret))
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func (s *Store) Prepare(req PrepareRequest) Acknowledgement {
	s.mu.Lock()
	defer s.mu.Unlock()
	peer, ok := s.peers[req.PeerID]
	ack := Acknowledgement{PeerID: req.PeerID, Generation: req.Generation.Number}
	if !ok {
		ack.Error = "HA peer is not authorized"
		return ack
	}
	signature, err := Sign(peer, req.Generation)
	if err != nil || !hmac.Equal([]byte(signature), []byte(req.Signature)) {
		ack.Error = "HA request authentication failed"
		return ack
	}
	if req.Generation.Number <= s.committed.Number {
		ack.Error = ErrGenerationOutOfOrder.Error()
		return ack
	}
	if err := req.Generation.Validate(); err != nil {
		ack.Error = err.Error()
		return ack
	}
	candidate := req.Generation
	s.pending = &candidate
	s.acks = map[string]Acknowledgement{req.PeerID: {PeerID: req.PeerID, Generation: candidate.Number, Ready: true}}
	ack.Ready = true
	return ack
}

func (s *Store) Acknowledge(ack Acknowledgement) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil || ack.Generation != s.pending.Number {
		return fmt.Errorf("HA generation %d is not pending", ack.Generation)
	}
	if _, ok := s.peers[ack.PeerID]; !ok {
		return errors.New("HA peer is not authorized")
	}
	s.acks[ack.PeerID] = ack
	return nil
}

func (s *Store) Commit() (Generation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil {
		return Generation{}, errors.New("HA generation is not prepared")
	}
	for id := range s.peers {
		ack, ok := s.acks[id]
		if !ok || !ack.Ready {
			return Generation{}, fmt.Errorf("HA peer %q has not acknowledged generation %d", id, s.pending.Number)
		}
	}
	if s.onCommit != nil {
		if err := s.onCommit(*s.pending); err != nil {
			return Generation{}, err
		}
	}
	s.committed = *s.pending
	s.pending = nil
	return s.committed, nil
}

func (s *Store) CommitAuthorized(request CommitRequest) (Generation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil || s.pending.Number != request.Generation {
		return Generation{}, errors.New("HA generation is not prepared")
	}
	peer, ok := s.peers[request.PeerID]
	if !ok {
		return Generation{}, errors.New("HA peer is not authorized")
	}
	signature, err := Sign(peer, *s.pending)
	if err != nil || !hmac.Equal([]byte(signature), []byte(request.Signature)) {
		return Generation{}, errors.New("HA request authentication failed")
	}
	if s.onCommit != nil {
		if err := s.onCommit(*s.pending); err != nil {
			return Generation{}, err
		}
	}
	s.committed = *s.pending
	s.pending = nil
	return s.committed, nil
}

func (s *Store) Committed() Generation { s.mu.Lock(); defer s.mu.Unlock(); return s.committed }
func (s *Store) Status() (Generation, *Generation, []Acknowledgement) {
	s.mu.Lock()
	defer s.mu.Unlock()
	acks := make([]Acknowledgement, 0, len(s.acks))
	for _, ack := range s.acks {
		acks = append(acks, ack)
	}
	return s.committed, s.pending, acks
}
