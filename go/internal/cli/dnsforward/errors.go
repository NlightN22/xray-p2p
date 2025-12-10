package dnsforwardcmd

type exitError struct {
	code int
}

func (e exitError) Error() string {
	return "exit"
}

func (e exitError) ExitCode() int {
	return e.code
}
