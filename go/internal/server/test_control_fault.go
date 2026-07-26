package server

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
)

const testControlStatusFileEnv = "XP2P_TEST_CONTROL_STATUS_FILE"

func testControlFaultHandler(next http.Handler) http.Handler {
	path := strings.TrimSpace(os.Getenv(testControlStatusFileEnv))
	if os.Getenv("XP2P_TEST_MODE") != "1" || path == "" {
		return next
	}
	var failures atomic.Uint64
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == controlplane.PathSubscription {
			if value, err := os.ReadFile(path); err == nil {
				status, parseErr := strconv.Atoi(strings.TrimSpace(string(value)))
				if parseErr == nil && status >= 400 && status <= 599 {
					count := failures.Add(1)
					_ = os.WriteFile(path+".count", []byte(strconv.FormatUint(count, 10)), 0o600)
					http.Error(w, http.StatusText(status), status)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
