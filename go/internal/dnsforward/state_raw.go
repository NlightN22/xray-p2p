//go:build linux

package dnsforward

type rawState struct {
	Entries map[string]rawStateEntry `json:"entries,omitempty"`
}

type rawStateEntry struct {
	Target            string `json:"target"`
	Server            string `json:"server"`
	ForwardListenPort int    `json:"forward_listen_port,omitempty"`
	ForwardTag        string `json:"forward_tag,omitempty"`
	ForwardOwner      string `json:"forward_owner,omitempty"`
	AutoForward       *bool  `json:"auto_forward,omitempty"`
	RebindDomain      string `json:"rebind_domain,omitempty"`
}
