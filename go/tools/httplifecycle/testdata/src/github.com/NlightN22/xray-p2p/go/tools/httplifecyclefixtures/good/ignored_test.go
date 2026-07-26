package good

import "net/http"

var testClient = &http.Client{Transport: &http.Transport{}}
