package logging

import "os"

func initLoggerFromEnv() {
	levelVar.Set(parseLevel(os.Getenv(EnvLogLevel)))
	formatVar.Store(FormatText)
	setLogger(os.Stderr)
}
