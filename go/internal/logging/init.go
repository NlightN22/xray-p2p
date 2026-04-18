package logging

// init configures the default logger based on environment settings so the
// application has a usable logger without additional setup.
func init() {
	formatVar.Store(FormatText)
	initLoggerFromEnv()
}
