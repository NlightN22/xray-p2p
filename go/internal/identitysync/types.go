package identitysync

import "time"

const SchemaVersion = 1

type ProviderKind string

const (
	ProviderLDAP ProviderKind = "ldap"
	ProviderSCIM ProviderKind = "scim"
)

type SyncStatus string

const (
	SyncStatusNever    SyncStatus = "never"
	SyncStatusSuccess  SyncStatus = "success"
	SyncStatusError    SyncStatus = "error"
	SyncStatusPartial  SyncStatus = "partial"
	SyncStatusDetached SyncStatus = "detached"
)

type ProviderRef struct {
	InstanceID string       `json:"instance_id"`
	Kind       ProviderKind `json:"kind"`
	Scope      []string     `json:"scope,omitempty"`
}

type State struct {
	SchemaVersion int          `json:"schema_version"`
	Provider      *ProviderRef `json:"provider,omitempty"`
	Current       *Generation  `json:"current,omitempty"`
	Pending       *Generation  `json:"pending,omitempty"`
	Transaction   *Transaction `json:"transaction,omitempty"`
	Status        Status       `json:"status"`
}

type Transaction struct {
	PreviousGenerationID  string `json:"previous_generation_id,omitempty"`
	CandidateGenerationID string `json:"candidate_generation_id"`
	StartedAt             string `json:"started_at"`
}

type Status struct {
	State       SyncStatus `json:"state"`
	LastSuccess string     `json:"last_success,omitempty"`
	Error       string     `json:"error,omitempty"`
	CacheAge    string     `json:"cache_age,omitempty"`
}

type Generation struct {
	ID                 string             `json:"id"`
	ProviderInstanceID string             `json:"provider_instance_id"`
	CreatedAt          string             `json:"created_at"`
	Subjects           map[string]Subject `json:"subjects"`
	Groups             map[string]Group   `json:"groups"`
	Detached           bool               `json:"detached,omitempty"`
}

type Subject struct {
	ExternalSubject string   `json:"external_subject"`
	UserLabel       string   `json:"user_label"`
	DisplayName     string   `json:"display_name,omitempty"`
	Active          bool     `json:"active"`
	Provisioned     bool     `json:"provisioned,omitempty"`
	DirectGroups    []string `json:"direct_groups,omitempty"`
}

type Group struct {
	ID            string   `json:"id"`
	DisplayName   string   `json:"display_name,omitempty"`
	DirectMembers []string `json:"direct_members,omitempty"`
	DirectGroups  []string `json:"direct_groups,omitempty"`
}

type Snapshot struct {
	Provider ProviderRef
	Complete bool
	Subjects []SnapshotSubject
	Groups   []SnapshotGroup
}

type SnapshotSubject struct {
	ExternalSubject string
	DisplayName     string
	DN              string
}

type SnapshotGroup struct {
	ID             string
	DisplayName    string
	DN             string
	MemberSubjects []string
	MemberGroups   []string
	MemberDNs      []string
}

func NewState(now time.Time) State {
	return State{
		SchemaVersion: SchemaVersion,
		Status: Status{
			State: SyncStatusNever,
		},
	}
}
