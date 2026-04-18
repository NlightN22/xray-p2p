package apply

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/configio"
)

type PendingSet struct {
	LiveConfigFile    string
	PendingConfigFile string
	LiveConfigDir     string
	PendingConfigDir  string
	LiveRoot          string
	LkgRoot           string
	AuditPath         string
}

func ApplyPending(set PendingSet) (*Rollback, bool, error) {
	pendingFiles, err := listFilesInDir(set.PendingConfigDir)
	if err != nil {
		return nil, false, err
	}
	pendingConfigExists, err := fileExists(set.PendingConfigFile)
	if err != nil {
		return nil, false, err
	}
	if !pendingConfigExists && len(pendingFiles) == 0 {
		return nil, false, nil
	}

	if pendingConfigExists {
		data, err := os.ReadFile(set.PendingConfigFile)
		if err != nil {
			return nil, false, fmt.Errorf("apply: read pending config %s: %w", set.PendingConfigFile, err)
		}
		if err := configio.WriteBytes(set.LiveConfigFile, data, configio.WriteOptions{
			AuditPath:         set.AuditPath,
			KeepLastKnownGood: true,
			IgnoreAuditErrors: true,
		}); err != nil {
			_ = RestoreLiveFromLkg(set.LiveRoot, set.LkgRoot)
			return nil, false, err
		}
	}

	for _, rel := range pendingFiles {
		source := filepath.Join(set.PendingConfigDir, rel)
		data, err := os.ReadFile(source)
		if err != nil {
			return nil, false, fmt.Errorf("apply: read pending file %s: %w", source, err)
		}
		target := filepath.Join(set.LiveConfigDir, rel)
		if err := configio.WriteBytes(target, data, configio.WriteOptions{
			AuditPath:         set.AuditPath,
			IgnoreAuditErrors: true,
		}); err != nil {
			_ = RestoreLiveFromLkg(set.LiveRoot, set.LkgRoot)
			return nil, false, err
		}
	}

	if len(pendingFiles) > 0 {
		if err := removeMissingLiveFiles(set.LiveConfigDir, pendingFiles); err != nil {
			_ = RestoreLiveFromLkg(set.LiveRoot, set.LkgRoot)
			return nil, false, err
		}
	}

	return &Rollback{liveRoot: set.LiveRoot, lkgRoot: set.LkgRoot}, true, nil
}
