package apply

import (
	"fmt"
	"strings"
)

type Rollback struct {
	liveRoot string
	lkgRoot  string
}

func NewRollback(liveRoot, lkgRoot string) *Rollback {
	return &Rollback{
		liveRoot: liveRoot,
		lkgRoot:  lkgRoot,
	}
}

func (r *Rollback) Restore(auditPath string) error {
	_ = auditPath
	if r == nil {
		return nil
	}
	if strings.TrimSpace(r.liveRoot) == "" || strings.TrimSpace(r.lkgRoot) == "" {
		return fmt.Errorf("apply: rollback paths are not set")
	}
	return RestoreLiveFromLkg(r.liveRoot, r.lkgRoot)
}
