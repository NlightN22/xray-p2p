package apply

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/configio"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

const (
	RoleClient = "client"
	RoleServer = "server"
	RoleAny    = "any"
)

type Request struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Role      string    `json:"role"`
}

func NewRequest(role string) (Request, error) {
	id, err := newUUID()
	if err != nil {
		return Request{}, err
	}
	return Request{
		ID:        id,
		Timestamp: time.Now().UTC(),
		Role:      strings.TrimSpace(strings.ToLower(role)),
	}, nil
}

func (r Request) MatchesRole(role string) bool {
	want := strings.TrimSpace(strings.ToLower(role))
	if want == "" {
		return false
	}
	if r.Role == "" {
		return true
	}
	if strings.EqualFold(r.Role, want) {
		return true
	}
	if strings.EqualFold(r.Role, RoleAny) {
		return true
	}
	return false
}

func WriteRequest(path string, req Request, auditPath string) error {
	if existing, exists, err := ReadRequest(path); err == nil {
		if exists && existing.ID != "" {
			if existing.MatchesRole(req.Role) {
				return nil
			}
			if existing.Role != "" && req.Role != "" && !strings.EqualFold(existing.Role, RoleAny) && !strings.EqualFold(req.Role, RoleAny) {
				req.Role = RoleAny
			}
		}
	}
	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return fmt.Errorf("apply: encode request: %w", err)
	}
	data = append(data, '\n')
	return configio.WriteBytes(path, data, configio.WriteOptions{
		AuditPath:         auditPath,
		IgnoreAuditErrors: true,
	})
}

func ReadRequest(path string) (Request, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Request{}, false, nil
		}
		return Request{}, false, fmt.Errorf("apply: read request: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return Request{}, true, nil
	}
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		trimmed := strings.TrimSpace(string(data))
		repaired := strings.ReplaceAll(trimmed, "\\r\\n", "\n")
		repaired = strings.ReplaceAll(repaired, "\\n", "\n")
		if retryErr := json.Unmarshal([]byte(repaired), &req); retryErr != nil {
			return Request{}, true, fmt.Errorf("apply: parse request: %w", err)
		}
	}
	req.Role = strings.TrimSpace(strings.ToLower(req.Role))
	return req, true, nil
}

func RemoveRequest(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("apply: remove request: %w", err)
	}
	return nil
}

type PendingSet struct {
	LiveConfigFile    string
	PendingConfigFile string
	LiveConfigDir     string
	PendingConfigDir  string
	LiveRoot          string
	LkgRoot           string
	AuditPath         string
}

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

func listFilesInDir(dir string) ([]string, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil
	}
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("apply: stat pending dir %s: %w", dir, err)
	}
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == layout.StateDirName {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("apply: list pending dir %s: %w", dir, err)
	}
	return files, nil
}

func removeMissingLiveFiles(liveDir string, pendingFiles []string) error {
	if strings.TrimSpace(liveDir) == "" {
		return nil
	}
	liveFiles, err := listFilesInDir(liveDir)
	if err != nil {
		return err
	}
	if len(liveFiles) == 0 {
		return nil
	}
	keep := make(map[string]struct{}, len(pendingFiles))
	for _, rel := range pendingFiles {
		keep[filepath.Clean(rel)] = struct{}{}
	}
	for _, rel := range liveFiles {
		clean := filepath.Clean(rel)
		if _, ok := keep[clean]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(liveDir, clean)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("apply: remove live file %s: %w", filepath.Join(liveDir, clean), err)
		}
	}
	return removeEmptyDirs(liveDir)
}

func removeEmptyDirs(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("apply: read dir %s: %w", root, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		child := filepath.Join(root, entry.Name())
		if err := removeEmptyDirs(child); err != nil {
			return err
		}
	}
	entries, err = os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("apply: read dir %s: %w", root, err)
	}
	if len(entries) == 0 {
		if err := os.Remove(root); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("apply: remove dir %s: %w", root, err)
		}
	}
	return nil
}

func fileExists(path string) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, nil
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("apply: stat %s: %w", path, err)
	}
	return true, nil
}

func newUUID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("apply: generate id: %w", err)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	parts := []string{
		hex.EncodeToString(buf[0:4]),
		hex.EncodeToString(buf[4:6]),
		hex.EncodeToString(buf[6:8]),
		hex.EncodeToString(buf[8:10]),
		hex.EncodeToString(buf[10:16]),
	}
	return strings.Join(parts, "-"), nil
}
