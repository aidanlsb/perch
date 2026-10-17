---
name: perch
description: "Manage Perch views and panel layouts through the perch CLI. Use when listing, adding, showing, or removing views/layouts, validating config.toml, opening a named view, or wiring {{selected}} bindings. Do not edit Perch config by hand or assume a vault schema."
version: 1
---

# Perch

Perch is a read-only TUI for Raven. Markdown stays the source of truth. Every
read goes through `rvn api`. Perch never writes to the vault.

Use this skill whenever the user wants Perch views or layouts created, listed,
changed, validated, or opened. Drive Perch through its CLI. Do not edit
`config.toml` by hand and do not operate the TUI.

## Operating rules

- Use `perch` with `--json` for deterministic output. Flags may appear anywhere.
- The envelope is Raven's: `{ "ok": true, "data": ... }` or
  `{ "ok": false, "error": { "code": "...", "message": "..." } }` with a
  nonzero exit. `error.details` may add line numbers or names.
- Prefer `perch views` / `perch layouts` / `perch config` over editing
  `~/.config/perch/config.toml` or driving the TUI.
- `perch config check --json` is structural only. It does not talk to a vault
  and does not consult `GET /v1/schema`.
- Do not assume a vault schema. Type, query, and field names in examples
  (`item`, `task`, `state`, `when`) are not reserved. Discover names with
  `rvn schema --json` or `rvn query` if the user has not given them.
- Browse in the TUI lists every type as a plain list. Do not invent board or
  date views from the schema. Only views the user (or you, via the CLI) declare
  exist.
- Writes are temp-file + rename. The existing validator runs before replace.
  Invalid config is not written.

## TUI sidebar

Prefer the CLI. If the user is already in the TUI: `ctrl-b` hides or shows
the sidebar (the main area expands). `ctrl-e` toggles focus between the
sidebar and the last-focused main panel, and shows the sidebar first if it
was hidden. Panel motions (`ctrl-h/j/k/l`, `ctrl-w` plus a direction, tab)
stay in the main area and never enter the sidebar. In-panel scrolling is
`ctrl-d` / `ctrl-u` only.

## Views

A view is a source plus one presentation: **list**, **board** (grouped by an
enum or ref field), or **date** (grouped by a date/datetime field). `source` is
a type name, a saved query name, or an RQL string. Optional `inputs` are sent
with saved-query `query` calls.

```bash
perch views list --json
perch views show projects --json
perch views add projects --source item --layout kind=board,group_by=state,fields=state,when --json
perch views add tasks --source task --inputs item='{{selected}}' --layout kind=list --json
perch views rm projects --json
perch views rm projects --force --json
```

`--layout` is `kind=board,group_by=state,fields=state,when`. Commas inside
`fields` belong to the list. `--layout` is an inline spec, not a layout name.

`views rm` of a view a layout still names is `CONFIRMATION_REQUIRED`. Pass
`--force` to drop those panels (and any layout left with no leaves).

## Layouts

A layout is a nested split of panels. Each leaf names a view. `split` is `row`
(side by side) or `col` (stacked). `bind` names the panel whose selection fills
`{{selected}}` in that panel.

```bash
perch layouts list --json
perch layouts show workspace --json
perch layouts add workspace --split row --panel view=projects --panel view=tasks,bind=projects --json
perch layouts add nested --from 'split = "row"
panels = [ { view = "projects" }, { split = "col", panels = [ { view = "tasks", bind = "projects" } ] } ]' --json
perch layouts rm workspace --json
```

`--from` is a TOML or JSON snippet, a file path, or `-` for stdin. Nested
splits belong in `--from`.

## Bindings and `{{selected}}`

`{{selected}}` in a view's `source` or `inputs` is filled from another panel's
current selection. The bound panel names that driver in `bind`.

```bash
perch views add tasks --source task --inputs item='{{selected}}' --layout kind=list --json
perch layouts add workspace --split row --panel view=projects --panel view=tasks,bind=projects --json
```

Bound panels update as the driving selection moves. Enter in the driving panel
moves focus to the bound panel. Do not invent a bind target the layout does
not name.

## Config

```bash
perch config path --json
perch config check --json
```

Default path is `~/.config/perch/config.toml` (`--config` or `PERCH_CONFIG`
override). `check` prints `path:line: message` and, with `--json`, the same
list in `error.details.errors`. It validates kinds, `group_by`, layout trees,
and unknown view references. It does not call Raven.

After writes, run `perch config check --json`. If it fails, fix through the
CLI; do not patch the TOML.

## Open

Bare `perch` launches the TUI. `perch open <name>` opens a named view or
layout (views win if both exist).

```bash
perch open projects --json
```

`--json` on `open` only affects launch errors. A successful open is the TUI.

## Error codes

| Code | When |
| --- | --- |
| `INVALID_ARGS` | Bad flags or extra arguments |
| `MISSING_ARGUMENT` | Required name or `--source` / `--panel` missing |
| `NOT_FOUND` | Unknown view, layout, or `open` target |
| `DUPLICATE_NAME` | `add` when the name already exists |
| `VALIDATION_FAILED` | Config fails the loader (kinds, `group_by`, layout trees) |
| `FILE_READ_ERROR` | Config or `--from` file could not be read |
| `FILE_WRITE_ERROR` | Atomic write failed |
| `CONFIRMATION_REQUIRED` | `views rm` while a layout still references the view; pass `--force` |
| `COMMAND_NOT_FOUND` | Unknown subcommand |
| `INVALID_INPUT` | Bad `--layout`, `--inputs`, `--panel`, `--from`, or `--scope` |
| `INVALID_JSON` | `--from` JSON did not parse |
| `SKILL_NOT_FOUND` | Unknown shipped skill name |
| `SKILL_NOT_INSTALLED` | `skill remove` of a skill that is not installed |
| `SKILL_PATH_UNRESOLVED` | Install root could not be resolved; pass `--dest` |

## This skill

Perch ships this file. Install it the same way Raven installs skills:

```bash
perch skill list --json
perch skill install --confirm --json
perch skill install perch --scope project --confirm --json
perch skill remove perch --confirm --json
perch skill doctor --json
```

`perch skills` is an alias. Preview is the default (`ok: true` with
`needs_confirm: true`). Agents cannot answer a y/N prompt; Perch never asks
for one. Pass `--confirm --json` to apply.

User scope writes `~/.agents/skills/perch`. Project scope writes
`.agents/skills/perch`. `--dest` overrides the root. A receipt
(`.perch-skill-receipt.json`) marks Perch-managed files. Directories without
that receipt are left alone.

## Typical flow

1. `perch config path --json` and `perch config check --json`.
2. Discover real type/query/field names with Raven if the user did not give them.
   Do not reuse example names from this skill unless they exist in the vault.
3. `perch views add … --json` and `perch layouts add … --json`.
4. `perch config check --json`.
5. `perch open <name>` only if the user wants the TUI.

## Safety

- Do not `sed`, `awk`, or overwrite `config.toml`.
- Do not write to the Raven vault from Perch.
- Do not assume `type:section` or any other type exists.
- Do not create board or date views unless the user named a `group_by` field
  that is actually an enum/ref (board) or date/datetime (date).
