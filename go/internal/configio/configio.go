package configio

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

type WriteOptions struct {
	AuditPath         string
	KeepLastKnownGood bool
}

func WriteJSON(path string, doc any, opts WriteOptions) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("configio: encode json %s: %w", path, err)
	}
	data = append(data, '\n')
	return WriteBytes(path, data, opts)
}

func WriteBytes(path string, data []byte, opts WriteOptions) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("configio: ensure directory %s: %w", filepath.Dir(path), err)
	}

	oldHash, oldSize, err := readFileHash(path)
	if err != nil {
		return err
	}
	newHash := hashBytes(data)

	if opts.KeepLastKnownGood && oldHash != "" {
		if err := writeBackup(path); err != nil {
			return err
		}
	}

	if err := writeAtomic(path, data); err != nil {
		return err
	}

	if opts.AuditPath != "" && oldHash != newHash {
		entry := auditEntry{
			Timestamp: time.Now().UTC(),
			User:      currentUser(),
			Command:   strings.Join(os.Args, " "),
			Path:      path,
			OldHash:   oldHash,
			NewHash:   newHash,
			OldSize:   oldSize,
			NewSize:   int64(len(data)),
		}
		if err := appendAudit(opts.AuditPath, entry); err != nil {
			return err
		}
	}
	return nil
}

type auditEntry struct {
	Timestamp time.Time
	User      string
	Command   string
	Path      string
	OldHash   string
	NewHash   string
	OldSize   int64
	NewSize   int64
}

func appendAudit(path string, entry auditEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("configio: ensure audit directory %s: %w", filepath.Dir(path), err)
	}
	line := fmt.Sprintf(
		"%s user=%s file=%s old_hash=%s new_hash=%s old_size=%d new_size=%d cmd=%s\n",
		entry.Timestamp.Format(time.RFC3339),
		entry.User,
		entry.Path,
		entry.OldHash,
		entry.NewHash,
		entry.OldSize,
		entry.NewSize,
		entry.Command,
	)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("configio: open audit log %s: %w", path, err)
	}
	defer file.Close()
	if _, err := file.WriteString(line); err != nil {
		return fmt.Errorf("configio: write audit log %s: %w", path, err)
	}
	return nil
}

func readFileHash(path string) (string, int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", 0, nil
		}
		return "", 0, fmt.Errorf("configio: stat %s: %w", path, err)
	}
	if info.IsDir() {
		return "", 0, fmt.Errorf("configio: %s is a directory", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("configio: open %s: %w", path, err)
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", 0, fmt.Errorf("configio: hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), info.Size(), nil
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeBackup(path string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("configio: read backup source %s: %w", path, err)
	}
	backup := path + ".lkg"
	if err := os.WriteFile(backup, source, 0o644); err != nil {
		return fmt.Errorf("configio: write backup %s: %w", backup, err)
	}
	return nil
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tempName := fmt.Sprintf(".tmp-%d-%d", os.Getpid(), time.Now().UnixNano())
	tmpPath := filepath.Join(dir, tempName)
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("configio: write temp %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("configio: rename temp %s: %w", path, err)
	}
	return nil
}

func currentUser() string {
	u, err := user.Current()
	if err != nil || u.Username == "" {
		return "unknown"
	}
	return u.Username
}
