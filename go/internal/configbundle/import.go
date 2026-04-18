package configbundle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func ImportConfigRoot(root, inputPath string) error {
	return ImportRoleConfigRoot("any", root, inputPath)
}

func ImportRoleConfigRoot(role, root, inputPath string) error {
	cleanRoot := filepath.Clean(strings.TrimSpace(root))
	if cleanRoot == "" {
		return fmt.Errorf("configbundle: config root is empty")
	}
	cleanInput := strings.TrimSpace(inputPath)
	if cleanInput == "" {
		return fmt.Errorf("configbundle: input path is empty")
	}

	format, err := DetectArchiveFormat(cleanInput)
	if err != nil {
		return err
	}

	parent := filepath.Dir(cleanRoot)
	tempDir, err := os.MkdirTemp(parent, ".xp2p-import-")
	if err != nil {
		return fmt.Errorf("configbundle: create temp dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	if err := extractArchive(cleanInput, tempDir, format); err != nil {
		return err
	}

	if err := validateRoleBundle(role, tempDir); err != nil {
		return err
	}
	if err := applyRoleBundle(role, tempDir, cleanRoot); err != nil {
		return err
	}
	if err := ensureApplyRequest(role, cleanRoot); err != nil {
		return err
	}
	return nil
}

func validateRoleBundle(role, extractedRoot string) error {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		role = "any"
	}
	includeClient := role == "client" || role == "any"
	includeServer := role == "server" || role == "any"

	return filepath.WalkDir(extractedRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(extractedRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, layout.StateDirName+"/") {
			return fmt.Errorf("configbundle: runtime artifacts are not allowed in import: %s", rel)
		}
		if d.IsDir() {
			return nil
		}

		allowed := false
		switch {
		case includeClient && rel == layout.ClientConfigFileName:
			allowed = true
		case includeServer && rel == layout.ServerConfigFileName:
			allowed = true
		case includeClient && strings.HasPrefix(rel, layout.ClientConfigDir+"/") && strings.HasSuffix(strings.ToLower(rel), ".json"):
			allowed = true
		case includeServer && strings.HasPrefix(rel, layout.ServerConfigDir+"/") && strings.HasSuffix(strings.ToLower(rel), ".json"):
			allowed = true
		case includeServer && strings.HasPrefix(rel, "tls/server/"):
			allowed = true
		}
		if !allowed {
			return fmt.Errorf("configbundle: unexpected file in import: %s", rel)
		}
		return nil
	})
}

func applyRoleBundle(role, extractedRoot, targetRoot string) error {
	return filepath.WalkDir(extractedRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(extractedRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(targetRoot, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.MkdirAll(target, normalizeDirMode(info.Mode()))
		}
		return copyFile(path, target, info.Mode())
	})
}

func ensureApplyRequest(role, root string) error {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		role = "any"
	}
	if role != apply.RoleClient && role != apply.RoleServer && role != apply.RoleAny {
		role = apply.RoleAny
	}
	req, err := apply.NewRequest(role)
	if err != nil {
		return err
	}
	path := filepath.Join(root, layout.StateDirName, layout.ApplyRequestFileName)
	auditPath := filepath.Join(root, layout.AuditLogFileName)
	return apply.WriteRequest(path, req, auditPath)
}
