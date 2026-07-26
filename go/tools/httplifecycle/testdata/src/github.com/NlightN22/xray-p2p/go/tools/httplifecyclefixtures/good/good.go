package good

import (
	"context"
	"net/http"

	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

type doer interface {
	Do(*http.Request) (*http.Response, error)
}

func ownedFactory() ownedhttp.OwnedClient {
	return ownedhttp.NewClient(ownedhttp.ClientOptions{})
}

func sharedFactory(client ownedhttp.OwnedClient) func() doer {
	return func() doer {
		return client
	}
}

func scopedClient(ctx context.Context) {
	client := ownedhttp.NewClient(ownedhttp.ClientOptions{})
	_ = client.Shutdown(ctx)
}

var sharedOwner ownedhttp.OwnedClient

func existingSharedClient() ownedhttp.OwnedClient {
	return sharedOwner
}

func accessorFactory() doer {
	return existingSharedClient()
}

func shared(client doer, req *http.Request) {
	_, _ = client.Do(req)
}

// nethttp-lifecycle:allow http-client-constructor owner=provider lifetime=process reason=shared process default transport
var provider = &http.Client{}
