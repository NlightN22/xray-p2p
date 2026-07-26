package nethttp

import (
	"io"
	"net/http"
)

func DrainAndClose(response *http.Response, limit int64) error {
	if response == nil || response.Body == nil {
		return nil
	}
	if limit < 0 {
		limit = 0
	}
	_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, limit))
	closeErr := response.Body.Close()
	if readErr != nil {
		return readErr
	}
	return closeErr
}
