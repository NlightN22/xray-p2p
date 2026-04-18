package logging

import (
	"log/slog"
	"sync/atomic"
)

const (
	defaultServiceName = "xp2p"
	EnvLogLevel        = "XP2P_LOG_LEVEL"
)

var (
	levelVar  slog.LevelVar
	activeLog atomic.Pointer[slog.Logger]
	formatVar atomic.Value
)
