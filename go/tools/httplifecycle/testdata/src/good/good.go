package good

import (
	"net/http"
)

type doer interface {
	Do(*http.Request) (*http.Response, error)
}

func shared(client doer, req *http.Request) {
	_, _ = client.Do(req)
}

// nethttp-lifecycle:allow http-client-constructor owner=provider lifetime=process reason=shared-default-transport
var provider = &http.Client{}
