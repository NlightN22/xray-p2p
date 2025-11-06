package servercmd

// deployManifest holds installation parameters, used by both v1 and v2 flows.
type deployManifest struct {
	Host       string `json:"host"`
	Version    int    `json:"version"`
	TrojanPort string `json:"trojan_port"`
	InstallDir string `json:"install_dir"`
	User       string `json:"user"`
	Password   string `json:"password"`
}
