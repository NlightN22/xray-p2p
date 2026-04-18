//go:build windows || linux

package server

import (
	"embed"
)

//go:embed assets/templates/*
var serverTemplates embed.FS
