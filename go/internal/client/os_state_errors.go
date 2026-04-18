package client

import (
	"time"
)

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func nowOrZero(err error) time.Time {
	if err == nil {
		return time.Time{}
	}
	return time.Now().UTC()
}
