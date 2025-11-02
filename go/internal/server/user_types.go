package server

// AddUserOptions describes parameters for adding a Trojan client to the server configuration.
type AddUserOptions struct {
	InstallDir string
	ConfigDir  string
	UserID     string
	Password   string
}

// RemoveUserOptions describes parameters for removing a Trojan client from the server configuration.
type RemoveUserOptions struct {
	InstallDir string
	ConfigDir  string
	UserID     string
}
