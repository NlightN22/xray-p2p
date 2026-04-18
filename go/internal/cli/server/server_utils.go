package servercmd

import (
	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
)

func firstNonEmpty(values ...string) string {
	return clishared.FirstNonEmpty(values...)
}
