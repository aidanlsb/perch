package skills

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCatalogHasPerch(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	skill, ok := catalog["perch"]
	if !ok {
		t.Fatalf("catalog keys %v", catalogNames(catalog))
	}
	if skill.Spec.Version < 1 || skill.Spec.Summary == "" {
		t.Fatalf("%+v", skill.Spec)
	}
	body := string(skill.Files["SKILL.md"])
	for _, want := range []string{"--json", "config check", "{{selected}}", "vault schema", "views add", "layouts add"} {
		if !strings.Contains(body, want) {
			t.Fatalf("SKILL.md missing %q", want)
		}
	}
}

func TestListInstalledUsesResolvedRoot(t *testing.T) {
	root := t.TempDir()
	result, err := List(ListRequest{Dest: root, InstalledOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Scope != "user" {
		t.Fatalf("scope %q", result.Scope)
	}
	if result.Root != root {
		t.Fatalf("root %q want %q", result.Root, root)
	}
	if len(result.Skills) != 0 {
		t.Fatalf("installed %d", len(result.Skills))
	}
}

func TestInstallDefaultsToAgentSkillsRoot(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	result, err := Install(InstallRequest{Names: []string{"perch"}})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".agents", "skills")
	if result.Root != want {
		t.Fatalf("root %q want %q", result.Root, want)
	}
	if result.Scope != "user" {
		t.Fatalf("scope %q", result.Scope)
	}
}

func TestInstallPreviewDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	result, err := Install(InstallRequest{Scope: "project", Dest: root})
	if err != nil {
		t.Fatal(err)
	}
	if result.Mode != "preview" || !result.NeedsConfirm {
		t.Fatalf("%+v", result)
	}
	if len(result.Skills) != len(catalog) || result.Installed != len(catalog) {
		t.Fatalf("planned %+v catalog %d", result, len(catalog))
	}
	if result.ActionsApplied != 0 {
		t.Fatalf("applied %d", result.ActionsApplied)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatalf("preview wrote %d entries", len(entries))
	}
}

func TestInstallConfirmWritesSkillAndReceipt(t *testing.T) {
	root := t.TempDir()
	result, err := Install(InstallRequest{Scope: "project", Dest: root, Confirm: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Mode != "applied" || result.NeedsConfirm || result.ActionsApplied == 0 {
		t.Fatalf("%+v", result)
	}
	skillPath := filepath.Join(root, "perch", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "perch", receiptFileName)); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "name: perch") {
		t.Fatalf("%s", body)
	}

	rerun, err := Install(InstallRequest{Scope: "project", Dest: root})
	if err != nil {
		t.Fatal(err)
	}
	if rerun.NeedsConfirm || rerun.Installed != 0 || rerun.Updated != 0 {
		t.Fatalf("rerun %+v", rerun)
	}
}

func TestInstallSkipsUnmanagedDirectory(t *testing.T) {
	root := t.TempDir()
	unmanaged := filepath.Join(root, "perch")
	if err := os.MkdirAll(unmanaged, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unmanaged, "SKILL.md"), []byte("hands off"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Install(InstallRequest{Names: []string{"perch"}, Scope: "project", Dest: root, Confirm: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Skipped != 1 || result.Installed != 0 {
		t.Fatalf("%+v", result)
	}
	got, err := os.ReadFile(filepath.Join(unmanaged, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hands off" {
		t.Fatalf("%s", got)
	}
}

func TestInstallUnknownSkill(t *testing.T) {
	_, err := Install(InstallRequest{Names: []string{"nope"}, Dest: t.TempDir()})
	if err == nil {
		t.Fatal("expected error")
	}
	var se *Error
	if !errors.As(err, &se) || se.Code != CodeNotFound {
		t.Fatalf("%v", err)
	}
}

func TestInstallUpdatesManagedSkill(t *testing.T) {
	root := t.TempDir()
	if _, err := Install(InstallRequest{Scope: "project", Dest: root, Confirm: true}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "perch", "SKILL.md")
	if err := os.WriteFile(path, []byte("outdated"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Install(InstallRequest{Scope: "project", Dest: root, Confirm: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated != 1 {
		t.Fatalf("%+v", result)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == "outdated" {
		t.Fatal("did not replace managed content")
	}
}

func TestRemovePreviewAndConfirm(t *testing.T) {
	root := t.TempDir()
	if _, err := Install(InstallRequest{Names: []string{"perch"}, Scope: "project", Dest: root, Confirm: true}); err != nil {
		t.Fatal(err)
	}
	preview, err := Remove(RemoveRequest{Name: "perch", Scope: "project", Dest: root})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Mode != "preview" || preview.Removed {
		t.Fatalf("%+v", preview)
	}
	if _, err := os.Stat(filepath.Join(root, "perch", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	applied, err := Remove(RemoveRequest{Name: "perch", Scope: "project", Dest: root, Confirm: true})
	if err != nil {
		t.Fatal(err)
	}
	if applied.Mode != "applied" || !applied.Removed {
		t.Fatalf("%+v", applied)
	}
	if _, err := os.Stat(filepath.Join(root, "perch")); !os.IsNotExist(err) {
		t.Fatalf("still present: %v", err)
	}
}

func TestRemoveNotInstalled(t *testing.T) {
	_, err := Remove(RemoveRequest{Name: "perch", Dest: t.TempDir()})
	if err == nil {
		t.Fatal("expected error")
	}
	var se *Error
	if !errors.As(err, &se) || se.Code != CodeNotInstalled {
		t.Fatalf("%v", err)
	}
}

func TestDoctorReportsMissingRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	result, err := RunDoctor(DoctorRequest{Dest: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Reports) != 1 || result.Reports[0].Exists {
		t.Fatalf("%+v", result)
	}
	if len(result.Reports[0].Issues) == 0 {
		t.Fatal("want issue about missing root")
	}
}

func TestParseScope(t *testing.T) {
	s, err := ParseScope("")
	if err != nil || s != ScopeUser {
		t.Fatalf("%q %v", s, err)
	}
	s, err = ParseScope("project")
	if err != nil || s != ScopeProject {
		t.Fatalf("%q %v", s, err)
	}
	if _, err := ParseScope("galaxy"); err == nil {
		t.Fatal("expected error")
	}
}
