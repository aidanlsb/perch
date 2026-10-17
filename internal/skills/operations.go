package skills

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListRequest is the input to List.
type ListRequest struct {
	Scope         string
	Dest          string
	InstalledOnly bool
}

// ListResult is the JSON data for skill list.
type ListResult struct {
	Scope  string    `json:"scope"`
	Root   string    `json:"root"`
	Skills []Summary `json:"skills"`
}

// InstallRequest is the input to Install.
type InstallRequest struct {
	Names   []string
	Scope   string
	Dest    string
	Confirm bool
}

// Action is one planned file operation.
type Action struct {
	Op      string `json:"op"`
	Path    string `json:"path"`
	RelPath string `json:"rel_path,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// SyncPlan is the plan for one skill or a full-catalog reconcile.
type SyncPlan struct {
	Skill        string   `json:"skill,omitempty"`
	Scope        string   `json:"scope"`
	Root         string   `json:"root"`
	NeedsConfirm bool     `json:"needs_confirm"`
	Actions      []Action `json:"actions"`
	Warnings     []string `json:"warnings,omitempty"`
	Installed    int      `json:"installed,omitempty"`
	Updated      int      `json:"updated,omitempty"`
	Deleted      int      `json:"deleted,omitempty"`
	Skipped      int      `json:"skipped,omitempty"`

	items []syncItem
}

// InstallSkillResult reports the plan for one shipped or receipt-managed skill.
type InstallSkillResult struct {
	Name string    `json:"name"`
	Plan *SyncPlan `json:"plan"`
}

// InstallResult is the JSON data for skill install.
type InstallResult struct {
	Mode           string               `json:"mode"`
	Scope          string               `json:"scope"`
	Root           string               `json:"root"`
	NeedsConfirm   bool                 `json:"needs_confirm"`
	Requested      []string             `json:"requested"`
	Skills         []InstallSkillResult `json:"skills"`
	Installed      int                  `json:"installed"`
	Updated        int                  `json:"updated"`
	Deleted        int                  `json:"deleted"`
	Skipped        int                  `json:"skipped"`
	ActionsApplied int                  `json:"actions_applied,omitempty"`
}

// RemoveRequest is the input to Remove.
type RemoveRequest struct {
	Name    string
	Scope   string
	Dest    string
	Confirm bool
}

// RemovePlan is the plan for removing one skill directory.
type RemovePlan struct {
	Skill        string   `json:"skill"`
	Scope        string   `json:"scope"`
	Root         string   `json:"root"`
	SkillPath    string   `json:"skill_path"`
	Exists       bool     `json:"exists"`
	NeedsConfirm bool     `json:"needs_confirm"`
	Actions      []Action `json:"actions"`
}

// RemoveResult is the JSON data for skill remove.
type RemoveResult struct {
	Mode      string      `json:"mode"`
	Removed   bool        `json:"removed,omitempty"`
	SkillName string      `json:"skill_name"`
	Plan      *RemovePlan `json:"plan,omitempty"`
}

// DoctorRequest is the input to RunDoctor.
type DoctorRequest struct {
	Scope string
	Dest  string
}

// DoctorReport is one install-root inspection.
type DoctorReport struct {
	Scope     string    `json:"scope"`
	Root      string    `json:"root"`
	Exists    bool      `json:"exists"`
	Installed []Summary `json:"installed"`
	Issues    []string  `json:"issues,omitempty"`
}

// DoctorResult is the JSON data for skill doctor.
type DoctorResult struct {
	Reports []DoctorReport `json:"reports"`
}

type syncOp string

const (
	syncOpInstall syncOp = "install"
	syncOpUpdate  syncOp = "update"
	syncOpDelete  syncOp = "delete"
	syncOpSkip    syncOp = "skip"
)

type syncItem struct {
	op      syncOp
	skill   *Skill
	skillID string
	path    string
	files   map[string][]byte
	receipt *Receipt
}

// List returns shipped skills and whether they are present at the install root.
func List(req ListRequest) (*ListResult, error) {
	catalog, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	scope, root, err := resolveScopeRoot(req.Scope, req.Dest)
	if err != nil {
		return nil, err
	}
	items := InstalledSummaries(catalog, root)
	if req.InstalledOnly {
		filtered := make([]Summary, 0, len(items))
		for _, item := range items {
			if item.Installed {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	return &ListResult{Scope: string(scope), Root: root, Skills: items}, nil
}

// Install reconciles shipped Perch skills. With no names it aligns the
// complete Perch-managed set with the shipped catalog. It previews by
// default and applies writes only when req.Confirm is set.
func Install(req InstallRequest) (*InstallResult, error) {
	catalog, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	fullCatalog := installRequestIsFullCatalog(req.Names)
	names, err := resolveInstallNames(catalog, req.Names)
	if err != nil {
		return nil, err
	}
	scope, root, err := resolveScopeRoot(req.Scope, req.Dest)
	if err != nil {
		return nil, err
	}

	result := &InstallResult{
		Mode:      "preview",
		Scope:     string(scope),
		Root:      root,
		Requested: names,
	}

	var applyPlans []*SyncPlan
	if fullCatalog {
		plan, err := planSync(catalog, "", scope, root)
		if err != nil {
			return nil, err
		}
		result.Skills = installSkillResults(plan, names)
		result.Installed = plan.Installed
		result.Updated = plan.Updated
		result.Deleted = plan.Deleted
		result.Skipped = plan.Skipped
		result.NeedsConfirm = plan.NeedsConfirm
		applyPlans = append(applyPlans, plan)
	} else {
		for _, name := range names {
			plan, err := planSync(catalog, name, scope, root)
			if err != nil {
				return nil, err
			}
			result.Skills = append(result.Skills, InstallSkillResult{Name: name, Plan: plan})
			result.Installed += plan.Installed
			result.Updated += plan.Updated
			result.Skipped += plan.Skipped
			if plan.NeedsConfirm {
				result.NeedsConfirm = true
			}
			applyPlans = append(applyPlans, plan)
		}
	}

	if !req.Confirm {
		return result, nil
	}

	applied := 0
	for _, plan := range applyPlans {
		n, err := applySync(plan)
		if err != nil {
			return nil, fail(CodeFileWrite, err.Error())
		}
		applied += n
	}
	result.Mode = "applied"
	result.NeedsConfirm = false
	result.ActionsApplied = applied
	return result, nil
}

// Remove previews or deletes one installed skill directory.
func Remove(req RemoveRequest) (*RemoveResult, error) {
	skillName := strings.TrimSpace(req.Name)
	if skillName == "" {
		return nil, fail(CodeInvalidInput, "skill name is required")
	}
	catalog, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	if _, ok := catalog[skillName]; !ok {
		return nil, failDetails(CodeNotFound, "skill '"+skillName+"' not found", map[string]any{
			"available": catalogNames(catalog),
		})
	}
	scope, root, err := resolveScopeRoot(req.Scope, req.Dest)
	if err != nil {
		return nil, err
	}
	plan, err := planRemove(skillName, scope, root)
	if err != nil {
		return nil, fail(CodeInvalidInput, err.Error())
	}
	if !plan.Exists {
		return nil, fail(CodeNotInstalled, "skill '"+skillName+"' is not installed")
	}
	if !req.Confirm {
		return &RemoveResult{Mode: "preview", SkillName: skillName, Plan: plan}, nil
	}
	if err := applyRemove(plan); err != nil {
		return nil, fail(CodeFileWrite, err.Error())
	}
	return &RemoveResult{Mode: "applied", Removed: true, SkillName: skillName, Plan: plan}, nil
}

// RunDoctor inspects the resolved install root.
func RunDoctor(req DoctorRequest) (*DoctorResult, error) {
	catalog, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	scope, root, err := resolveScopeRoot(req.Scope, req.Dest)
	if err != nil {
		return nil, err
	}
	return &DoctorResult{Reports: []DoctorReport{doctor(catalog, scope, root)}}, nil
}

func resolveScopeRoot(rawScope, dest string) (Scope, string, error) {
	scope, err := ParseScope(strings.TrimSpace(rawScope))
	if err != nil {
		return "", "", fail(CodeInvalidInput, err.Error())
	}
	root, err := ResolveInstallRoot(scope, strings.TrimSpace(dest), "")
	if err != nil {
		return "", "", fail(CodePathUnresolved, err.Error())
	}
	return scope, root, nil
}

func installRequestIsFullCatalog(requested []string) bool {
	for _, raw := range requested {
		if strings.TrimSpace(raw) != "" {
			return false
		}
	}
	return true
}

func resolveInstallNames(catalog map[string]*Skill, requested []string) ([]string, error) {
	trimmed := make([]string, 0, len(requested))
	for _, raw := range requested {
		if name := strings.TrimSpace(raw); name != "" {
			trimmed = append(trimmed, name)
		}
	}
	if len(trimmed) == 0 {
		return catalogNames(catalog), nil
	}
	seen := make(map[string]struct{}, len(trimmed))
	names := make([]string, 0, len(trimmed))
	for _, name := range trimmed {
		if _, ok := catalog[name]; !ok {
			return nil, failDetails(CodeNotFound, "skill '"+name+"' not found", map[string]any{
				"available": catalogNames(catalog),
			})
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names, nil
}

func catalogNames(catalog map[string]*Skill) []string {
	summaries := SortedSummaries(catalog)
	names := make([]string, 0, len(summaries))
	for _, s := range summaries {
		names = append(names, s.Name)
	}
	return names
}

func planSync(catalog map[string]*Skill, skillName string, scope Scope, root string) (*SyncPlan, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("install root is empty")
	}
	skillName = strings.TrimSpace(skillName)
	plan := &SyncPlan{Skill: skillName, Scope: string(scope), Root: root}
	if skillName != "" {
		return planNamedSync(plan, catalog, skillName, root)
	}
	return planRootSync(plan, catalog, root)
}

func planNamedSync(plan *SyncPlan, catalog map[string]*Skill, skillName, root string) (*SyncPlan, error) {
	skill, ok := catalog[skillName]
	if !ok {
		return nil, fail(CodeNotFound, "skill '"+skillName+"' not found")
	}
	skillPath := filepath.Join(root, skillName)
	receipt, receiptErr := readReceipt(filepath.Join(skillPath, receiptFileName))
	if receiptErr != nil && !os.IsNotExist(receiptErr) {
		return nil, fmt.Errorf("read receipt for %s: %w", skillName, receiptErr)
	}
	if receipt == nil {
		if stat, err := os.Stat(skillPath); err == nil {
			if stat.IsDir() {
				addSyncSkip(plan, skillName, skillPath, "skill directory exists without Perch receipt")
				return plan, nil
			}
			addSyncSkip(plan, skillName, skillPath, "skill path exists but is not a directory")
			return plan, nil
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect %s: %w", skillPath, err)
		}
		addSyncInstall(plan, skill, skillPath)
		return plan, nil
	}
	if receipt.Skill != skillName {
		addSyncSkip(plan, skillName, skillPath, fmt.Sprintf("receipt skill %q does not match directory name", receipt.Skill))
		return plan, nil
	}
	needsUpdate, err := syncNeedsUpdate(skillPath, receipt, skill, plan.Scope)
	if err != nil {
		return nil, err
	}
	if needsUpdate {
		addSyncUpdate(plan, skill, skillPath, receipt)
	}
	return plan, nil
}

func planRootSync(plan *SyncPlan, catalog map[string]*Skill, root string) (*SyncPlan, error) {
	managed := make(map[string]struct{})
	entries, err := os.ReadDir(root)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read install root: %w", err)
		}
		entries = nil
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		skillPath := filepath.Join(root, dirName)
		receipt, err := readReceipt(filepath.Join(skillPath, receiptFileName))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read receipt for %s: %w", dirName, err)
		}
		managed[dirName] = struct{}{}
		if receipt.Skill != dirName {
			addSyncSkip(plan, dirName, skillPath, fmt.Sprintf("receipt skill %q does not match directory name", receipt.Skill))
			continue
		}
		skill, ok := catalog[dirName]
		if !ok {
			addSyncDelete(plan, dirName, skillPath, receipt)
			continue
		}
		needsUpdate, err := syncNeedsUpdate(skillPath, receipt, skill, plan.Scope)
		if err != nil {
			return nil, err
		}
		if needsUpdate {
			addSyncUpdate(plan, skill, skillPath, receipt)
		}
	}
	for _, summary := range SortedSummaries(catalog) {
		if _, ok := managed[summary.Name]; ok {
			continue
		}
		skillPath := filepath.Join(root, summary.Name)
		if stat, err := os.Stat(skillPath); err == nil {
			if stat.IsDir() {
				addSyncSkip(plan, summary.Name, skillPath, "skill directory exists without Perch receipt")
				continue
			}
			addSyncSkip(plan, summary.Name, skillPath, "skill path exists but is not a directory")
			continue
		} else if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect %s: %w", skillPath, err)
		}
		addSyncInstall(plan, catalog[summary.Name], skillPath)
	}
	finalizeSyncPlan(plan)
	return plan, nil
}

func installSkillResults(plan *SyncPlan, shippedNames []string) []InstallSkillResult {
	if plan == nil {
		return nil
	}
	results := make([]InstallSkillResult, 0, len(shippedNames))
	byName := make(map[string]*SyncPlan, len(shippedNames))
	addResult := func(name string) *SyncPlan {
		skillPlan := &SyncPlan{Skill: name, Scope: plan.Scope, Root: plan.Root}
		results = append(results, InstallSkillResult{Name: name, Plan: skillPlan})
		byName[name] = skillPlan
		return skillPlan
	}
	for _, name := range shippedNames {
		addResult(name)
	}
	for _, item := range plan.items {
		skillPlan, ok := byName[item.skillID]
		if !ok {
			skillPlan = addResult(item.skillID)
		}
		for _, action := range plan.Actions {
			if syncActionBelongsToItem(action, item) {
				skillPlan.Actions = append(skillPlan.Actions, action)
			}
		}
		switch item.op {
		case syncOpInstall:
			skillPlan.Installed++
			skillPlan.NeedsConfirm = true
		case syncOpUpdate:
			skillPlan.Updated++
			skillPlan.NeedsConfirm = true
		case syncOpDelete:
			skillPlan.Deleted++
			skillPlan.NeedsConfirm = true
		case syncOpSkip:
			skillPlan.Skipped++
		}
	}
	return results
}

func syncActionBelongsToItem(action Action, item syncItem) bool {
	actionPath := filepath.Clean(action.Path)
	itemPath := filepath.Clean(item.path)
	return actionPath == itemPath || strings.HasPrefix(actionPath, itemPath+string(filepath.Separator))
}

func applySync(plan *SyncPlan) (int, error) {
	if plan == nil {
		return 0, fmt.Errorf("sync plan is nil")
	}
	applied := 0
	for _, item := range plan.items {
		switch item.op {
		case syncOpInstall:
			n, err := writeSkill(item.skill, plan.Scope, item.path, item.files)
			if err != nil {
				return applied, err
			}
			applied += n
		case syncOpUpdate:
			n, err := writeSkillFiles(item.path, item.files)
			if err != nil {
				return applied, err
			}
			applied += n
			removed, err := removeStaleReceiptFiles(item.path, item.receipt, item.files)
			if err != nil {
				return applied, err
			}
			applied += removed
			receiptWritten, err := writeSkillReceipt(item.skill, plan.Scope, item.path, item.files)
			if err != nil {
				return applied, err
			}
			applied += receiptWritten
		case syncOpDelete:
			n, err := removeReceiptManagedFiles(item.path, item.receipt)
			if err != nil {
				return applied, err
			}
			applied += n
		case syncOpSkip:
			continue
		}
	}
	return applied, nil
}

func addSyncInstall(plan *SyncPlan, skill *Skill, skillPath string) {
	plan.Actions = append(plan.Actions, Action{Op: "install", Path: skillPath, Reason: "install missing shipped skill"})
	plan.Installed++
	plan.items = append(plan.items, syncItem{op: syncOpInstall, skill: skill, skillID: skill.Spec.ID, path: skillPath, files: skill.Files})
	finalizeSyncPlan(plan)
}

func addSyncUpdate(plan *SyncPlan, skill *Skill, skillPath string, receipt *Receipt) {
	plan.Actions = append(plan.Actions, Action{Op: "update", Path: skillPath, Reason: "align with shipped skill"})
	plan.Updated++
	plan.items = append(plan.items, syncItem{op: syncOpUpdate, skill: skill, skillID: skill.Spec.ID, path: skillPath, files: skill.Files, receipt: receipt})
	finalizeSyncPlan(plan)
}

func addSyncDelete(plan *SyncPlan, skillID, skillPath string, receipt *Receipt) {
	for _, rel := range receipt.Files {
		plan.Actions = append(plan.Actions, Action{Op: "delete_file", Path: filepath.Join(skillPath, filepath.FromSlash(rel)), RelPath: rel, Reason: "shipped skill is no longer available"})
	}
	plan.Actions = append(plan.Actions, Action{Op: "delete_receipt", Path: filepath.Join(skillPath, receiptFileName), RelPath: receiptFileName, Reason: "shipped skill is no longer available"})
	plan.Deleted++
	plan.items = append(plan.items, syncItem{op: syncOpDelete, skillID: skillID, path: skillPath, receipt: receipt})
	finalizeSyncPlan(plan)
}

func addSyncSkip(plan *SyncPlan, skillID, skillPath, reason string) {
	plan.Actions = append(plan.Actions, Action{Op: "skip_conflict", Path: skillPath, Reason: reason})
	plan.Skipped++
	plan.Warnings = append(plan.Warnings, fmt.Sprintf("%s: %s", skillID, reason))
	plan.items = append(plan.items, syncItem{op: syncOpSkip, skillID: skillID, path: skillPath})
	finalizeSyncPlan(plan)
}

func finalizeSyncPlan(plan *SyncPlan) {
	plan.NeedsConfirm = false
	for _, item := range plan.items {
		if item.op == syncOpInstall || item.op == syncOpUpdate || item.op == syncOpDelete {
			plan.NeedsConfirm = true
			return
		}
	}
}

func syncNeedsUpdate(skillPath string, receipt *Receipt, skill *Skill, scope string) (bool, error) {
	if !receiptMatches(receipt, skill, scope, skill.Files) {
		return true, nil
	}
	for rel, content := range skill.Files {
		abs, err := safeSkillPath(skillPath, rel)
		if err != nil {
			return false, err
		}
		existing, err := os.ReadFile(abs)
		if err != nil {
			if os.IsNotExist(err) {
				return true, nil
			}
			return false, fmt.Errorf("read %s: %w", abs, err)
		}
		if !bytes.Equal(existing, content) {
			return true, nil
		}
	}
	current := make(map[string]struct{}, len(skill.Files))
	for rel := range skill.Files {
		current[rel] = struct{}{}
	}
	for _, rel := range receipt.Files {
		if _, ok := current[rel]; ok {
			continue
		}
		abs, err := safeSkillPath(skillPath, rel)
		if err != nil {
			return false, err
		}
		if _, err := os.Stat(abs); err == nil {
			return true, nil
		} else if err != nil && !os.IsNotExist(err) {
			return false, fmt.Errorf("inspect %s: %w", abs, err)
		}
	}
	return false, nil
}

func planRemove(skillID string, scope Scope, root string) (*RemovePlan, error) {
	if strings.TrimSpace(skillID) == "" {
		return nil, fmt.Errorf("skill id is empty")
	}
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("install root is empty")
	}
	skillPath := filepath.Join(root, skillID)
	plan := &RemovePlan{Skill: skillID, Scope: string(scope), Root: root, SkillPath: skillPath}
	if stat, err := os.Stat(skillPath); err == nil {
		plan.Exists = stat.IsDir()
		if !stat.IsDir() {
			return nil, fmt.Errorf("skill path exists but is not a directory: %s", skillPath)
		}
		plan.Actions = append(plan.Actions, Action{Op: "delete", Path: skillPath, Reason: "remove installed skill directory"})
		plan.NeedsConfirm = true
	} else if os.IsNotExist(err) {
		plan.Exists = false
	} else {
		return nil, fmt.Errorf("inspect %s: %w", skillPath, err)
	}
	return plan, nil
}

func applyRemove(plan *RemovePlan) error {
	if plan == nil {
		return fmt.Errorf("remove plan is nil")
	}
	if !plan.Exists {
		return fmt.Errorf("skill is not installed")
	}
	absRoot, err := filepath.Abs(plan.Root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}
	absSkill, err := filepath.Abs(plan.SkillPath)
	if err != nil {
		return fmt.Errorf("resolve skill path: %w", err)
	}
	prefix := absRoot + string(filepath.Separator)
	if absSkill == absRoot || !strings.HasPrefix(absSkill, prefix) {
		return fmt.Errorf("refusing to remove path outside install root")
	}
	if err := os.RemoveAll(absSkill); err != nil {
		return fmt.Errorf("remove skill directory: %w", err)
	}
	return nil
}

func doctor(catalog map[string]*Skill, scope Scope, root string) DoctorReport {
	report := DoctorReport{Scope: string(scope), Root: root}
	stat, err := os.Stat(root)
	if err == nil {
		report.Exists = stat.IsDir()
		if !stat.IsDir() {
			report.Issues = append(report.Issues, "install root exists but is not a directory")
		}
	} else if os.IsNotExist(err) {
		report.Exists = false
		report.Issues = append(report.Issues, "install root does not exist yet (it will be created on install)")
	} else {
		report.Issues = append(report.Issues, fmt.Sprintf("failed to inspect install root: %v", err))
	}
	summaries := InstalledSummaries(catalog, root)
	report.Installed = make([]Summary, 0, len(summaries))
	for _, summary := range summaries {
		if summary.Installed {
			report.Installed = append(report.Installed, summary)
		}
	}
	sort.Slice(report.Installed, func(i, j int) bool {
		return report.Installed[i].Name < report.Installed[j].Name
	})
	return report
}

// InstalledSummaries marks catalog skills that have a directory at root.
func InstalledSummaries(catalog map[string]*Skill, root string) []Summary {
	summaries := SortedSummaries(catalog)
	for i := range summaries {
		skillPath := filepath.Join(root, summaries[i].Name)
		if stat, err := os.Stat(skillPath); err == nil && stat.IsDir() {
			summaries[i].Installed = true
			summaries[i].Path = skillPath
		}
	}
	return summaries
}
