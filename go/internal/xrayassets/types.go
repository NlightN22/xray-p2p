package xrayassets

import (
	"time"

	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

const maxDownloadSize = 64 * 1024 * 1024

type Config struct {
	Files []File
}

type File struct {
	Name       string
	URL        string
	StaleAfter time.Duration
}

type Options struct {
	AssetDir       string
	Role           string
	XrayConfigPath string
	HTTPClient     ownedhttp.Doer
}
