package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// File is the optional Perch config (saved views). Raven knows nothing about it.
type File struct {
	Views []View `toml:"views"`
}

// View is a saved Perch view: source plus presentation.
type View struct {
	Name        string   `toml:"name"`
	Source      string   `toml:"source"`
	Kind        string   `toml:"kind"` // list, board, date
	GroupBy     string   `toml:"group_by"`
	Granularity string   `toml:"granularity"` // day, week, month (date views)
	Fields      []string `toml:"fields"`
	Sort        string   `toml:"sort"`
}

const (
	KindList  = "list"
	KindBoard = "board"
	KindDate  = "date"
	GranDay   = "day"
	GranWeek  = "week"
	GranMonth = "month"
)

// Load reads a TOML config. Missing file yields an empty File, not an error.
func Load(path string) (*File, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return &File{}, nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &File{}, nil
	}
	var f File
	if _, err := toml.DecodeFile(path, &f); err != nil {
		return nil, fmt.Errorf("perch config %s: %w", path, err)
	}
	for i := range f.Views {
		if err := f.Views[i].normalize(); err != nil {
			return nil, fmt.Errorf("views[%d]: %w", i, err)
		}
	}
	return &f, nil
}

func (v *View) normalize() error {
	v.Name = strings.TrimSpace(v.Name)
	v.Source = strings.TrimSpace(v.Source)
	v.Kind = strings.ToLower(strings.TrimSpace(v.Kind))
	v.GroupBy = strings.TrimSpace(v.GroupBy)
	v.Granularity = strings.ToLower(strings.TrimSpace(v.Granularity))
	v.Sort = strings.TrimSpace(v.Sort)
	if v.Kind == "" {
		v.Kind = KindList
	}
	switch v.Kind {
	case KindList, KindBoard, KindDate:
	default:
		return fmt.Errorf("unknown kind %q (list, board, date)", v.Kind)
	}
	if v.Source == "" {
		return fmt.Errorf("source is required")
	}
	if v.Name == "" {
		v.Name = v.Source
	}
	if v.Kind == KindDate && v.Granularity == "" {
		v.Granularity = GranDay
	}
	if v.Kind == KindDate {
		switch v.Granularity {
		case GranDay, GranWeek, GranMonth:
		default:
			return fmt.Errorf("unknown granularity %q (day, week, month)", v.Granularity)
		}
	}
	if v.Kind == KindBoard && v.GroupBy == "" {
		return fmt.Errorf("board views require group_by")
	}
	if v.Kind == KindDate && v.GroupBy == "" {
		return fmt.Errorf("date views require group_by")
	}
	return nil
}

// DefaultPath is ~/.config/perch/config.toml (XDG).
func DefaultPath() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return filepath.Join(dir, "perch", "config.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "perch.toml"
	}
	return filepath.Join(home, ".config", "perch", "config.toml")
}
