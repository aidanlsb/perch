package skills

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/aidanlsb/perch/internal/frontmatter"
	skillsrc "github.com/aidanlsb/perch/skills"
)

// Scope controls where skills are installed.
type Scope string

const (
	ScopeUser    Scope = "user"
	ScopeProject Scope = "project"
)

// Spec is a shipped skill's identity.
type Spec struct {
	ID      string
	Title   string
	Version int
	Summary string
}

// Skill is a loaded bundle from the embedded library.
type Skill struct {
	Spec Spec
	// Files is the installable set, keyed by path relative to the skill root.
	Files map[string][]byte
}

// Summary is a lightweight list view.
type Summary struct {
	Name      string `json:"name"`
	Title     string `json:"title"`
	Version   int    `json:"version"`
	Summary   string `json:"summary"`
	Installed bool   `json:"installed,omitempty"`
	Path      string `json:"path,omitempty"`
}

// LoadCatalog loads every embedded */SKILL.md.
func LoadCatalog() (map[string]*Skill, error) {
	entries, err := fs.ReadDir(skillsrc.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded skill library: %w", err)
	}

	catalog := make(map[string]*Skill)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := fs.ReadFile(skillsrc.FS, path.Join(entry.Name(), "SKILL.md"))
		if err != nil {
			return nil, fmt.Errorf("load %s/SKILL.md: %w", entry.Name(), err)
		}
		skill, err := parseSkill(entry.Name(), data)
		if err != nil {
			return nil, err
		}
		catalog[skill.Spec.ID] = skill
	}
	if len(catalog) == 0 {
		return nil, fmt.Errorf("embedded skill library is empty")
	}
	return catalog, nil
}

func parseSkill(dir string, data []byte) (*Skill, error) {
	doc := frontmatter.Split(string(data))
	name := strings.TrimSpace(doc.Fields["name"])
	if name == "" {
		name = dir
	}
	summary := strings.TrimSpace(doc.Fields["description"])
	if summary == "" {
		return nil, fmt.Errorf("skill %s: missing description", dir)
	}
	title := strings.TrimSpace(doc.Fields["title"])
	if title == "" {
		title = name
	}
	version := 1
	if raw := strings.TrimSpace(doc.Fields["version"]); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("skill %s: version must be >= 1", dir)
		}
		version = n
	}
	return &Skill{
		Spec: Spec{ID: name, Title: title, Version: version, Summary: summary},
		Files: map[string][]byte{
			"SKILL.md": append([]byte(nil), data...),
		},
	}, nil
}

// SortedSummaries returns catalog summaries sorted by skill ID.
func SortedSummaries(catalog map[string]*Skill) []Summary {
	summaries := make([]Summary, 0, len(catalog))
	for _, skill := range catalog {
		summaries = append(summaries, Summary{
			Name:    skill.Spec.ID,
			Title:   skill.Spec.Title,
			Version: skill.Spec.Version,
			Summary: skill.Spec.Summary,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})
	return summaries
}

// ParseScope parses a user-provided install scope.
func ParseScope(raw string) (Scope, error) {
	s := Scope(strings.ToLower(strings.TrimSpace(raw)))
	switch s {
	case "", ScopeUser:
		return ScopeUser, nil
	case ScopeProject:
		return ScopeProject, nil
	default:
		return "", fmt.Errorf("unsupported scope %q (expected: user or project)", raw)
	}
}
