package controlplane

import (
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/ha"
)

const (
	PathReady             = "/control/v1/ready"
	PathPing              = "/control/v1/ping"
	PathHeartbeat         = "/control/v1/heartbeat"
	PathSubscription      = "/control/v1/subscription"
	PathCredentialsRotate = "/control/v1/credentials/rotate"
	PathCredentialsAck    = "/control/v1/credentials/ack"
)

const (
	HeaderUser            = "X-XP2P-User"
	HeaderTimestamp       = "X-XP2P-Timestamp"
	HeaderNonce           = "X-XP2P-Nonce"
	HeaderSignature       = "X-XP2P-Signature"
	HeaderKnownGeneration = "X-XP2P-Known-Generation"
)

type Runtime struct {
	Endpoint      Endpoint       `json:"endpoint"`
	Subscription  Subscription   `json:"subscription"`
	AuthUsers     []AuthUser     `json:"auth_users,omitempty"`
	RotationUsers []RotationUser `json:"rotation_users,omitempty"`
	TLS           TLSMetadata    `json:"tls,omitempty"`
}

// RotationUser is control-plane-only state. It never contributes a tunnel credential.
type RotationUser struct {
	UserLabel                     string    `json:"user_label"`
	ActiveCredential              string    `json:"active_credential"`
	PreviousCredentialForRotation string    `json:"previous_credential_for_rotation,omitempty"`
	RotationExpiresAt             time.Time `json:"rotation_expires_at,omitempty"`
	CredentialGeneration          int       `json:"credential_generation"`
}

type RotationRequest struct {
	UserLabel string `json:"user_label"`
	Action    string `json:"action,omitempty"`
	Nonce     string `json:"nonce,omitempty"`
	Proof     string `json:"proof,omitempty"`
}

type RotationChallenge struct {
	Nonce     string    `json:"nonce"`
	ExpiresAt time.Time `json:"expires_at"`
}

type RotationResponse struct {
	RotationPending        bool   `json:"rotation_pending"`
	ActiveCredential       string `json:"active_credential,omitempty"`
	CredentialGeneration   int    `json:"credential_generation,omitempty"`
	SubscriptionGeneration string `json:"subscription_generation,omitempty"`
}

type Endpoint struct {
	Scheme string `json:"scheme"`
	Host   string `json:"host,omitempty"`
	Port   int    `json:"port"`
}

type AuthUser struct {
	Label      string `json:"label"`
	Credential string `json:"credential,omitempty"`
}

type TLSMetadata struct {
	ServerName             string `json:"server_name,omitempty"`
	CertificatePath        string `json:"certificate_path,omitempty"`
	PinnedPeerCertSHA256   string `json:"pinned_peer_cert_sha256,omitempty"`
	VerifyPeerCertByName   string `json:"verify_peer_cert_by_name,omitempty"`
	SelfSigned             bool   `json:"self_signed,omitempty"`
	ClientMayAllowInsecure bool   `json:"client_may_allow_insecure,omitempty"`
}

type Subscription struct {
	Generation string            `json:"generation"`
	Profile    string            `json:"profile"`
	Protocol   string            `json:"protocol"`
	Transport  string            `json:"transport"`
	Security   string            `json:"security"`
	Host       string            `json:"host"`
	Port       int               `json:"port"`
	ServerName string            `json:"server_name,omitempty"`
	TLS        TLSMetadata       `json:"tls,omitempty"`
	Parameters map[string]string `json:"parameters,omitempty"`
	IssuedAt   time.Time         `json:"issued_at"`
	ValidUntil time.Time         `json:"valid_until"`
	Topology   *Topology         `json:"topology,omitempty"`
}

// Topology is the complete client-visible, committed HA state. It deliberately
// excludes peer credentials and node-local server configuration.
type Topology struct {
	Generation uint64       `json:"generation"`
	Group      ha.Group     `json:"group"`
	Channels   []ha.Channel `json:"channels,omitempty"`
}

type PingRequest struct {
	Nonce string `json:"nonce"`
}

type PingResponse struct {
	Nonce    string    `json:"nonce"`
	ServerAt time.Time `json:"server_at"`
}
