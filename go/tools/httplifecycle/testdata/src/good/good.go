package good

import (
	"context"
	"net/http"
)

type doer interface {
	Do(*http.Request) (*http.Response, error)
}

type ownedDoer interface {
	doer
	Shutdown(context.Context) error
}

type ownedClient struct{}

func (ownedClient) Do(*http.Request) (*http.Response, error) { return nil, nil }
func (ownedClient) Shutdown(context.Context) error           { return nil }

func newOwnedClient() ownedDoer {
	return ownedClient{}
}

func ownedFactory() ownedDoer {
	return newOwnedClient()
}

func sharedFactory(client ownedDoer) func() doer {
	return func() doer {
		return client
	}
}

func scopedClient(ctx context.Context) {
	client := newOwnedClient()
	_ = client.Shutdown(ctx)
}

func shared(client doer, req *http.Request) {
	_, _ = client.Do(req)
}

// nethttp-lifecycle:allow http-client-constructor owner=provider lifetime=process reason=shared process default transport
var provider = &http.Client{}
