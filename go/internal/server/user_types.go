package server

// AddUserOptions describes parameters for adding a Trojan client to the server configuration.
type AddUserOptions struct {
	InstallDir string
	ConfigDir  string
	UserID     string
	Password   string
	Host       string
	NoReverse  bool
	Force      bool
}

// RemoveUserOptions describes parameters for removing a Trojan client from the server configuration.
type RemoveUserOptions struct {
	InstallDir string
	ConfigDir  string
	UserID     string
	Host       string
}

// ListUsersOptions describes parameters for enumerating Trojan users and generating connection links.
type ListUsersOptions struct {
	InstallDir string
	ConfigDir  string
	Host       string
	Pending    bool
}

// UserLinkOptions describes parameters for generating a connection link for a specific Trojan user.
type UserLinkOptions struct {
	InstallDir string
	ConfigDir  string
	Host       string
	UserID     string
	Pending    bool
}

// UserLink contains the essential details for a Trojan user, including a ready-to-use connection link.
type UserLink struct {
	UserID   string
	Password string
	Link     string
	Disabled bool
}
