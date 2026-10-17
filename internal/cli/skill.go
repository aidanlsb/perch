package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/aidanlsb/perch/internal/skills"
)

const skillUsage = `perch skill list|install|remove|doctor

  list                 List shipped skills and install status
  install [name...]    Preview or apply skill install
  remove <name>        Preview or apply skill removal
  doctor               Inspect the install root

perch skills is an alias. Preview is the default. Pass --confirm to apply.
Agents cannot answer a y/N prompt; Perch never asks for one.

Flags:
  --scope user|project   Default user (~/.agents/skills)
  --dest path            Override the install root
  --installed            list: only installed skills
  --confirm              Apply writes (install and remove)
  --json                 Machine-readable envelope
`

func runSkill(args []string, stdout, stderr io.Writer) error {
	p, err := parseArgs(args, argSpec{
		bools: []string{"json", "help", "h", "confirm", "installed"},
		vals:  []string{"scope", "dest", "config"},
	})
	jsonOut := p.Bool["json"]
	if err != nil {
		return writeErr(stdout, stderr, jsonOut, err)
	}
	if p.Bool["help"] || len(p.Pos) == 0 {
		if jsonOut && len(p.Pos) == 0 && !p.Bool["help"] {
			return writeErr(stdout, stderr, true, fail(CodeInvalidArgs, "usage: perch skill list|install|remove|doctor"))
		}
		fmt.Fprint(stderr, skillUsage)
		return nil
	}
	sub, rest := p.Pos[0], p.Pos[1:]
	scope, dest := p.Val["scope"], p.Val["dest"]
	var runErr error
	var data any
	var human string
	switch sub {
	case "list":
		if len(rest) > 0 {
			runErr = fail(CodeInvalidArgs, "skill list takes no arguments")
			break
		}
		data, human, runErr = skillList(scope, dest, p.Bool["installed"])
	case "install":
		data, human, runErr = skillInstall(rest, scope, dest, p.Bool["confirm"])
	case "remove", "rm":
		var name string
		name, runErr = requireName(rest, "skill")
		if runErr == nil {
			data, human, runErr = skillRemove(name, scope, dest, p.Bool["confirm"])
		}
	case "doctor":
		if len(rest) > 0 {
			runErr = fail(CodeInvalidArgs, "skill doctor takes no arguments")
			break
		}
		data, human, runErr = skillDoctor(scope, dest)
	default:
		runErr = fail(CodeCommandNotFound, "unknown skill command "+sub)
	}
	if runErr != nil {
		return writeErr(stdout, stderr, jsonOut, runErr)
	}
	return writeOK(stdout, jsonOut, data, human)
}

func skillList(scope, dest string, installedOnly bool) (any, string, error) {
	result, err := skills.List(skills.ListRequest{Scope: scope, Dest: dest, InstalledOnly: installedOnly})
	if err != nil {
		return nil, "", err
	}
	var lines []string
	lines = append(lines, "scope\t"+result.Scope)
	lines = append(lines, "root\t"+result.Root)
	if len(result.Skills) == 0 {
		lines = append(lines, "(none)")
	}
	for _, item := range result.Skills {
		status := "available"
		if item.Installed {
			status = "installed"
		}
		lines = append(lines, fmt.Sprintf("%s\tv%d\t%s\t%s", item.Name, item.Version, status, item.Summary))
	}
	return result, strings.Join(lines, "\n"), nil
}

func skillInstall(names []string, scope, dest string, confirm bool) (any, string, error) {
	result, err := skills.Install(skills.InstallRequest{Names: names, Scope: scope, Dest: dest, Confirm: confirm})
	if err != nil {
		return nil, "", err
	}
	var b strings.Builder
	if result.Mode == "applied" {
		fmt.Fprintf(&b, "reconciled Perch skills at %s\napplied %d file changes", result.Root, result.ActionsApplied)
		if result.Skipped > 0 {
			fmt.Fprintf(&b, "\n%d skill path(s) skipped (unmanaged)", result.Skipped)
		}
		return result, b.String(), nil
	}
	fmt.Fprintf(&b, "skill install plan\ntarget:\t%s", result.Root)
	for _, entry := range result.Skills {
		fmt.Fprintf(&b, "\n%s\t(%s)", entry.Name, skillPlanStatus(entry.Plan))
	}
	if result.NeedsConfirm {
		fmt.Fprintf(&b, "\npreview only — re-run with --confirm to apply")
	} else if result.Skipped > 0 {
		fmt.Fprintf(&b, "\nno managed changes pending; %d path(s) skipped", result.Skipped)
	} else {
		fmt.Fprintf(&b, "\nalready aligned")
	}
	return result, b.String(), nil
}

func skillPlanStatus(plan *skills.SyncPlan) string {
	if plan == nil {
		return "no changes"
	}
	switch {
	case plan.Installed > 0:
		return "install"
	case plan.Updated > 0:
		return "update"
	case plan.Deleted > 0:
		return "remove (no longer shipped)"
	case plan.Skipped > 0:
		return "skipped (unmanaged)"
	default:
		return "up to date"
	}
}

func skillRemove(name, scope, dest string, confirm bool) (any, string, error) {
	result, err := skills.Remove(skills.RemoveRequest{Name: name, Scope: scope, Dest: dest, Confirm: confirm})
	if err != nil {
		return nil, "", err
	}
	if result.Mode == "preview" {
		human := "preview remove\ntarget:\t" + result.Plan.SkillPath + "\nre-run with --confirm to apply"
		return result, human, nil
	}
	return result, "removed " + result.SkillName + " from " + result.Plan.SkillPath, nil
}

func skillDoctor(scope, dest string) (any, string, error) {
	result, err := skills.RunDoctor(skills.DoctorRequest{Scope: scope, Dest: dest})
	if err != nil {
		return nil, "", err
	}
	var lines []string
	for _, report := range result.Reports {
		lines = append(lines, "scope\t"+report.Scope)
		lines = append(lines, "root\t"+report.Root)
		if len(report.Installed) == 0 {
			lines = append(lines, "installed\tnone")
		} else {
			for _, item := range report.Installed {
				lines = append(lines, "installed\t"+item.Name)
			}
		}
		for _, issue := range report.Issues {
			lines = append(lines, "issue\t"+issue)
		}
	}
	return result, strings.Join(lines, "\n"), nil
}
