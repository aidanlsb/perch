package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"time"
)

const receiptFileName = ".perch-skill-receipt.json"

// Receipt marks a skill directory as Perch-managed.
type Receipt struct {
	Skill       string   `json:"skill"`
	Version     int      `json:"version"`
	Scope       string   `json:"scope"`
	Checksum    string   `json:"checksum"`
	Files       []string `json:"files"`
	InstalledAt string   `json:"installed_at"`
}

func newReceipt(skillID string, version int, scope string, files map[string][]byte) *Receipt {
	return &Receipt{
		Skill:       skillID,
		Version:     version,
		Scope:       scope,
		Checksum:    checksumForFiles(files),
		Files:       sortedFilePaths(files),
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func writeReceipt(path string, receipt *Receipt) error {
	body, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return atomicWrite(path, body)
}

func readReceipt(path string) (*Receipt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var receipt Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return nil, err
	}
	return &receipt, nil
}

func checksumForFiles(files map[string][]byte) string {
	h := sha256.New()
	for _, rel := range sortedFilePaths(files) {
		_, _ = h.Write([]byte(rel))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(files[rel])
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func receiptMatches(receipt *Receipt, skill *Skill, scope string, files map[string][]byte) bool {
	if receipt == nil || skill == nil {
		return false
	}
	return receipt.Checksum == checksumForFiles(files) &&
		receipt.Skill == skill.Spec.ID &&
		receipt.Version == skill.Spec.Version &&
		receipt.Scope == scope
}

func sortedFilePaths(files map[string][]byte) []string {
	paths := make([]string, 0, len(files))
	for rel := range files {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	return paths
}
