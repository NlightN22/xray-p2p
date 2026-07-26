package bad

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

type alias = http.Client

type doer interface {
	Do(*http.Request) (*http.Response, error)
}

func customDial(context.Context, string, string) (net.Conn, error) {
	return nil, nil
}

func clientFactory() doer {
	return http.DefaultClient
}

func haClientFactory() doer {
	return ownedhttp.NewClient(ownedhttp.ClientOptions{ // want "return an owned HTTP client from this factory"
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	})
}

func assignedSCIMClientFactory() doer {
	client := ownedhttp.NewClient(ownedhttp.ClientOptions{ // want "return an owned HTTP client from this factory"
		TLSConfig:   &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext: customDial,
	})
	return client
}

func namedClientFactory() (client doer) {
	client = ownedhttp.NewClient(ownedhttp.ClientOptions{}) // want "return an owned HTTP client from this factory"
	return
}

var callbackFactory = func() doer {
	return ownedhttp.NewClient(ownedhttp.ClientOptions{}) // want "return an owned HTTP client from this factory"
}

func tupleClientFactory() (doer, error) {
	client, err := ownedhttp.NewClientWithError(ownedhttp.ClientOptions{}) // want "return an owned HTTP client from this factory"
	return client, err
}

var sharedClient ownedhttp.OwnedClient

func branchingClientFactory(useShared bool) doer {
	client := ownedhttp.NewClient(ownedhttp.ClientOptions{}) // want "return an owned HTTP client from this factory"
	if useShared {
		client = sharedClient
	}
	return client
}

type clientHolder struct {
	client ownedhttp.OwnedClient
}

func structClientFactory() doer {
	holder := clientHolder{
		client: ownedhttp.NewClient(ownedhttp.ClientOptions{}), // want "return an owned HTTP client from this factory"
	}
	return holder.client
}

func closureClientFactory() doer {
	client := ownedhttp.NewClient(ownedhttp.ClientOptions{}) // want "return an owned HTTP client from this factory"
	accessor := func() doer {
		return client
	}
	return accessor()
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

/*nethttp-lifecycle:allow http-server-constructor owner=unused lifetime=scope reason=exception is not attached*/ // want "HTTP lifecycle allowance does not match"
var unused = 1

/*nethttp-lifecycle:allow unknown-rule owner=unused lifetime=scope reason=sufficient explanation*/ // want "unknown HTTP lifecycle allowance rule"
var unknownRule = 1

/*nethttp-lifecycle:allow http-client-constructor owner=shortReason lifetime=request reason=x*/ // want "HTTP lifecycle allowance reason must contain at least three words and 16 characters"
var shortReason = http.DefaultClient

/*nethttp-lifecycle:allow http-client-constructor owner=x lifetime=request reason=this reason has enough words*/ // want "invalid HTTP lifecycle allowance"
var shortOwner = http.DefaultClient

/*nethttp-lifecycle:allow http-client-constructor owner=shortOwner lifetime=x reason=this reason has enough words*/ // want "invalid HTTP lifecycle allowance"
var invalidLifetime = http.DefaultClient

/*nethttp-lifecycle:allow http-client-constructor owner=absentOwner lifetime=request reason=this reason has enough words*/ // want "owner must name a declaration"
var missingOwner = http.DefaultClient
