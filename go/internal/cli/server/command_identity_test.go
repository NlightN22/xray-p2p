package servercmd

import (
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/usecase"
)

func TestPrintIdentityStateIncludesRedirectStatus(t *testing.T) {
	out := captureStdout(t, func() {
		printIdentityState(usecase.IdentityStatusView{
			Status:     "success",
			Generation: "gen-1",
			Redirects: []usecase.IdentityRedirectView{
				{Type: "domain", Value: "engineering.internal", OutboundTag: "alice.rev", Host: "10.0.0.1", State: "disabled_by_policy"},
			},
		})
	})
	if !strings.Contains(out, "redirect domain engineering.internal tag=alice.rev host=10.0.0.1 state=disabled_by_policy") {
		t.Fatalf("redirect status missing from identity output:\n%s", out)
	}
}
