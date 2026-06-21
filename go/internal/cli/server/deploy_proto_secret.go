package servercmd

import "github.com/NlightN22/xray-p2p/go/internal/identity"

func generateSecret(size int) (string, error) {
	return identity.NewSecret(size)
}
