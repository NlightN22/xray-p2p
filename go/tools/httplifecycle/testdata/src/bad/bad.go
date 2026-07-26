package bad

import (
	"net/http"
)

type alias = http.Client

type doer interface {
	Do(*http.Request) (*http.Response, error)
}

func clientFactory() doer {
	return http.DefaultClient
}

func bad(req *http.Request) {
	_ = http.Client{}              // want "construct http.Client through"
	_ = &alias{}                   // want "construct http.Client through"
	_ = http.Transport{}           // want "construct http.Transport through"
	_ = &http.Server{}             // want "construct http.Server through"
	_ = new(http.Client)           // want "construct http.Client through"
	_, _ = clientFactory().Do(req) // want "store and own the HTTP client"
	_ = &http.Client{              // want "construct http.Client through"
		Transport: &http.Transport{}, // want "construct http.Transport through"
	}
}

/*nethttp-lifecycle:allow http-client-constructor owner=bad*/ // want "invalid HTTP lifecycle allowance"
var malformed = http.Client{}                                 // want "construct http.Client through"

/*nethttp-lifecycle:allow http-server-constructor owner=unused lifetime=scope reason=not-attached*/ // want "HTTP lifecycle allowance does not match"
var unused = 1
