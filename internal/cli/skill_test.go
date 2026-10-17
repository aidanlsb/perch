package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillListJSON(t *testing.T) {
	root := t.TempDir()
	env := runJSON(t, "skill", "list", "--dest", root)
	if !env.OK || env.Error != nil {
		t.Fatalf("%+v", env)
	}
	data := asMap(t, env.Data)
	if data["scope"] != "user" {
		t.Fatalf("%v", data)
	}
	if data["root"] != root {
		t.Fatalf("root %v want %s", data["root"], root)
	}
	skills := asSlice(t, data["skills"])
	if len(skills) != 1 {
		t.Fatalf("%v", skills)
	}
	item := asMap(t, skills[0])
	if item["name"] != "perch" {
		t.Fatalf("%v", item)
	}
	if item["installed"] != nil && item["installed"] != false {
		t.Fatalf("want not installed: %v", item)
	}
}

func TestSkillsAliasListJSON(t *testing.T) {
	root := t.TempDir()
	env := runJSON(t, "skills", "list", "--dest", root, "--scope", "project")
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	if asMap(t, env.Data)["scope"] != "project" {
		t.Fatalf("%v", env.Data)
	}
}

func TestSkillInstallPreviewJSON(t *testing.T) {
	root := t.TempDir()
	env := runJSON(t, "skill", "install", "--dest", root, "--scope", "project")
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	data := asMap(t, env.Data)
	if data["mode"] != "preview" || data["needs_confirm"] != true {
		t.Fatalf("%v", data)
	}
	if data["installed"] != float64(1) {
		t.Fatalf("installed %v", data["installed"])
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatalf("preview wrote %v", entries)
	}
}

func TestSkillInstallConfirmJSON(t *testing.T) {
	root := t.TempDir()
	env := runJSON(t, "skill", "install", "perch", "--dest", root, "--scope", "project", "--confirm")
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	data := asMap(t, env.Data)
	if data["mode"] != "applied" || data["needs_confirm"] != false {
		t.Fatalf("%v", data)
	}
	if _, err := os.Stat(filepath.Join(root, "perch", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "perch", ".perch-skill-receipt.json")); err != nil {
		t.Fatal(err)
	}
	list := runJSON(t, "skill", "list", "--dest", root, "--installed")
	items := asSlice(t, asMap(t, list.Data)["skills"])
	if len(items) != 1 || asMap(t, items[0])["name"] != "perch" {
		t.Fatalf("%v", items)
	}
}

func TestSkillInstallUnknownJSON(t *testing.T) {
	fail := runJSONErr(t, "skill", "install", "nope", "--dest", t.TempDir())
	if fail.OK || fail.Error == nil || fail.Error.Code != CodeSkillNotFound {
		t.Fatalf("%+v", fail)
	}
}

func TestSkillRemovePreviewAndConfirmJSON(t *testing.T) {
	root := t.TempDir()
	if env := runJSON(t, "skill", "install", "--dest", root, "--confirm"); !env.OK {
		t.Fatalf("%+v", env)
	}
	preview := runJSON(t, "skill", "remove", "perch", "--dest", root)
	if !preview.OK {
		t.Fatalf("%+v", preview)
	}
	data := asMap(t, preview.Data)
	if data["mode"] != "preview" {
		t.Fatalf("%v", data)
	}
	if _, err := os.Stat(filepath.Join(root, "perch", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	applied := runJSON(t, "skill", "remove", "perch", "--dest", root, "--confirm")
	if !applied.OK {
		t.Fatalf("%+v", applied)
	}
	if asMap(t, applied.Data)["removed"] != true {
		t.Fatalf("%v", applied.Data)
	}
	if _, err := os.Stat(filepath.Join(root, "perch")); !os.IsNotExist(err) {
		t.Fatalf("still present: %v", err)
	}
}

func TestSkillRemoveNotInstalledJSON(t *testing.T) {
	fail := runJSONErr(t, "skill", "remove", "perch", "--dest", t.TempDir())
	if fail.OK || fail.Error == nil || fail.Error.Code != CodeSkillNotInstalled {
		t.Fatalf("%+v", fail)
	}
}

func TestSkillDoctorJSON(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	env := runJSON(t, "skill", "doctor", "--dest", root)
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	reports := asSlice(t, asMap(t, env.Data)["reports"])
	if len(reports) != 1 {
		t.Fatalf("%v", reports)
	}
	report := asMap(t, reports[0])
	if report["exists"] != false {
		t.Fatalf("%v", report)
	}
	issues := asSlice(t, report["issues"])
	if len(issues) == 0 {
		t.Fatalf("%v", report)
	}
}

func TestSkillBadScopeJSON(t *testing.T) {
	fail := runJSONErr(t, "skill", "list", "--scope", "galaxy")
	if fail.OK || fail.Error == nil || fail.Error.Code != CodeInvalidInput {
		t.Fatalf("%+v", fail)
	}
}

func TestSkillUnknownSubcommandJSON(t *testing.T) {
	fail := runJSONErr(t, "skill", "explode")
	if fail.Error == nil || fail.Error.Code != CodeCommandNotFound {
		t.Fatalf("%+v", fail)
	}
}

func TestHumanSkillList(t *testing.T) {
	root := t.TempDir()
	var out, errb strings.Builder
	if err := Run([]string{"skill", "list", "--dest", root}, &out, &errb); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "perch") {
		t.Fatalf("%s", out.String())
	}
}
