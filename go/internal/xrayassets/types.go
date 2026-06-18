package xrayassets

import (
	"time"
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
}
