package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func atomicWrite(path string, data []byte) error {
	perm := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		perm = st.Mode().Perm()
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	_ = tmp.Chmod(perm)
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	ok = true
	return nil
}

func writeSkill(skill *Skill, scope, skillPath string, files map[string][]byte) (int, error) {
	written, err := writeSkillFiles(skillPath, files)
	if err != nil {
		return written, err
	}
	receiptWritten, err := writeSkillReceipt(skill, scope, skillPath, files)
	return written + receiptWritten, err
}

func writeSkillFiles(skillPath string, files map[string][]byte) (int, error) {
	if err := os.MkdirAll(skillPath, 0o755); err != nil {
		return 0, fmt.Errorf("create skill directory: %w", err)
	}
	applied := 0
	for _, rel := range sortedFilePaths(files) {
		abs, err := safeSkillPath(skillPath, rel)
		if err != nil {
			return applied, err
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return applied, fmt.Errorf("create parent directory for %s: %w", abs, err)
		}
		if err := atomicWrite(abs, files[rel]); err != nil {
			return applied, fmt.Errorf("write %s: %w", abs, err)
		}
		applied++
	}
	return applied, nil
}

func writeSkillReceipt(skill *Skill, scope, skillPath string, files map[string][]byte) (int, error) {
	receipt := newReceipt(skill.Spec.ID, skill.Spec.Version, scope, files)
	path, err := safeSkillPath(skillPath, receiptFileName)
	if err != nil {
		return 0, err
	}
	if err := writeReceipt(path, receipt); err != nil {
		return 0, err
	}
	return 1, nil
}

func safeSkillPath(skillPath, relPath string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(strings.TrimSpace(relPath)))
	if cleaned == "" || cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("skill path escapes skill root: %s", relPath)
	}

	rootInfo, err := os.Lstat(skillPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect skill root %s: %w", skillPath, err)
		}
		return filepath.Join(skillPath, cleaned), nil
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("skill root is a symlink: %s", skillPath)
	}

	current := skillPath
	for _, component := range strings.Split(cleaned, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return "", fmt.Errorf("inspect skill path %s: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("skill path traverses symlink: %s", current)
		}
	}
	return filepath.Join(skillPath, cleaned), nil
}

func removeManagedFile(skillPath, relPath string) (int, error) {
	abs, err := safeSkillPath(skillPath, relPath)
	if err != nil {
		return 0, err
	}
	if err := os.Remove(abs); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("remove %s: %w", abs, err)
	}
	return 1, nil
}

func removeReceiptManagedFiles(skillPath string, receipt *Receipt) (int, error) {
	if receipt == nil {
		return 0, nil
	}
	applied := 0
	for _, rel := range receipt.Files {
		n, err := removeManagedFile(skillPath, rel)
		if err != nil {
			return applied, err
		}
		applied += n
	}
	n, err := removeManagedFile(skillPath, receiptFileName)
	if err != nil {
		return applied, err
	}
	applied += n
	if err := removeEmptyDirs(skillPath); err != nil {
		return applied, err
	}
	return applied, nil
}

func removeStaleReceiptFiles(skillPath string, receipt *Receipt, files map[string][]byte) (int, error) {
	if receipt == nil {
		return 0, nil
	}
	current := make(map[string]struct{}, len(files))
	for rel := range files {
		current[rel] = struct{}{}
	}
	applied := 0
	for _, rel := range receipt.Files {
		if _, ok := current[rel]; ok {
			continue
		}
		n, err := removeManagedFile(skillPath, rel)
		if err != nil {
			return applied, err
		}
		applied += n
	}
	if err := removeEmptyDirs(skillPath); err != nil {
		return applied, err
	}
	return applied, nil
}

func removeEmptyDirs(skillPath string) error {
	var dirs []string
	if err := filepath.WalkDir(skillPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("walk %s: %w", skillPath, err)
	}
	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[i]) > len(dirs[j])
	})
	for _, dir := range dirs {
		if err := os.Remove(dir); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			entries, readErr := os.ReadDir(dir)
			if readErr == nil && len(entries) > 0 {
				continue
			}
			return fmt.Errorf("remove empty directory %s: %w", dir, err)
		}
	}
	return nil
}
