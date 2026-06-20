package controlplane

import "time"

const (
	PathReady        = "/control/v1/ready"
	PathPing         = "/control/v1/ping"
	PathHeartbeat    = "/control/v1/heartbeat"
	PathSubscription = "/control/v1/subscription"
)

const (
	HeaderUser      = "X-XP2P-User"
	HeaderTimestamp = "X-XP2P-Timestamp"
	HeaderNonce     = "X-XP2P-Nonce"
	HeaderSignature = "X-XP2P-Signature"
)

type Runtime struct {
	Endpoint     Endpoint     `json:"endpoint"`
	Subscription Subscription `json:"subscription"`
	AuthUsers    []AuthUser   `json:"auth_users,omitempty"`
	TLS          TLSMetadata  `json:"tls,omitempty"`
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
}

type PingRequest struct {
	Nonce string `json:"nonce"`
}

type PingResponse struct {
	Nonce    string    `json:"nonce"`
	ServerAt time.Time `json:"server_at"`
}
