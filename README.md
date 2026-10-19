# Perch

A read-only terminal UI for [Raven](https://github.com/aidanlsb/raven).
Markdown stays the source of truth. Every read goes through `rvn api`.
Perch does not import Raven internals and does not write to the vault.

## Install

Requires Go 1.22+ and a `rvn` binary on `PATH`.

```bash
go install github.com/aidanlsb/perch/cmd/perch@latest
```

Or from a checkout:

```bash
git clone https://github.com/aidanlsb/perch
cd perch
make build
./perch --help
```

## Run

From a vault, or with an explicit path:

```bash
perch
perch --vault-path /path/to/vault
perch --url http://127.0.0.1:7438 --token "$RAVEN_API_TOKEN" --no-start
perch --poll-interval 2s
```

Perch requires an `rvn api` that reports an index revision (`GET /v1/revision`,
Raven PR 312). Older servers are a startup error. The TUI polls that endpoint
about once a second (`--poll-interval`, or `PERCH_POLL_INTERVAL`) and refreshes
visible panels when the vault changes.

If nothing is listening, Perch starts `rvn api --json --port 0` as a child
and sends SIGTERM to that child on exit. If a server is already up and you
pass a token, Perch connects and leaves it running.

## Views and layouts

A **view** is exactly one of **type** (a schema type name) or **query**
(a Raven saved query name), plus one presentation: **list**, **board**
(grouped by an enum or ref field), or **date** (grouped by a
date/datetime field). Inline RQL is not allowed in config; `:` in the
TUI runs an ad-hoc query that stays ephemeral. Optional `inputs` are
sent with saved-query `query` calls. `{{selected}}` in `inputs` or
`filter` is filled from another panel's current selection.
For trait, section, and link rows this is the containing file object
(`object_id` / `file_object_id` / `source_id`), not the row id. A
section-scoped id (`fileObject#slug`) is bound as the file object.

A **layout** is a nested split of panels. Each leaf names a view. `split`
is `row` (side by side) or `col` (stacked). `bind` names the panel whose
selection fills `{{selected}}` in that panel. Bound panels update as the
driving selection moves. Enter in the driving panel moves focus to the
bound panel.

The type picker (`t`) lists every schema type as a plain list.
Perch does not invent board or date views from the schema. `.` still
cycles date granularity (day → week → month) inside a date view.
`section` is omitted because Raven cannot query `type:section`. There is
no sidebar: navigation is pickers and hotkeys only.

Bare `perch` opens the first view in config file order as that view's
list, board, or date. If no views are configured, the type picker opens.
It does not pick a schema type on its own.

## Config

Defaults live in Perch's own file (Raven never reads it), defaulting to
`~/.config/perch/config.toml`. See `examples/config.toml`. Type, query,
and field names in examples are not special.

```toml
[views.projects]
type = "item"
layout = { kind = "board", group_by = "state", fields = ["state", "when"] }

[views.tasks]
type = "task"
inputs = { item = "{{selected}}" }
layout = { kind = "list" }

[layouts.workspace]
split = "row"
panels = [ { view = "projects" }, { view = "tasks", bind = "projects" } ]
```

Exactly one of `type` or `query` is required. A `source` key is a startup
error. `kind` is `list`, `board`, or `date`. Board `group_by` is an enum
or ref field. Date `group_by` is a date/datetime field; `granularity` is
`day`, `week`, or `month`. Optional `filter` is an RQL predicate appended
to the type query or saved-query body.

`[[types.*.layouts]]` and `[[queries.*.layouts]]` are a startup error
pointing at this shape. The old single `layout =` form and `[[views]]`
array of tables are likewise rejected.

At startup Perch validates views against `GET /v1/schema` and
`query_saved_list`, and shows warnings in the footer (with line numbers)
for unknown types, queries, or fields.

`,` opens the config file in the editor from `GET /v1/session`, jumping
to the current view's table where the editor supports a line argument.
On return Perch reloads and validates; errors include line numbers and
the last good config is kept. `S` prompts for a name and writes the
on-screen view (`type` or `query` plus layout settings) as `[views.x]`.
Ad-hoc `:` queries cannot be saved that way: save them in Raven first,
then reference the name with `query = "…"`. Writes go only to Perch's
config, never the vault.

## CLI (for agents)

Do not edit `config.toml` by hand and do not drive the TUI. The CLI is the
stable way to list, add, and remove views and layouts. Bare `perch` still
opens the TUI. `perch open <name>` opens the TUI on that view or layout
(views win if both exist).

```bash
perch views list
perch views show projects
perch views add projects --type item --layout kind=board,group_by=state,fields=state,when
perch views add tasks --type task --inputs item='{{selected}}' --layout kind=list
perch views add inbox --query inbox --layout kind=list
perch views rm projects            # errors if a layout references it
perch views rm projects --force    # drops those panels (and empty layouts)

perch layouts list
perch layouts show workspace
perch layouts add workspace --split row --panel view=projects --panel view=tasks,bind=projects
perch layouts add nested --from 'split = "row"
panels = [ { view = "projects" }, { split = "col", panels = [ { view = "tasks", bind = "projects" } ] } ]'
perch layouts rm workspace

perch config path
perch config check
perch open projects
perch open workspace

perch skill list
perch skill install --confirm --json
perch skill remove perch --confirm --json
perch skill doctor
```

Every command accepts `--json` (anywhere) and `--config path`. On success the
envelope is `{ "ok": true, "data": ... }`. On failure it is
`{ "ok": false, "error": { "code": "...", "message": "..." } }` and the
process exits nonzero. `error.details` may add line numbers or names.

| Code | When |
| --- | --- |
| `INVALID_ARGS` | Bad flags or extra arguments |
| `MISSING_ARGUMENT` | Required name or `--type` / `--query` / `--panel` missing |
| `NOT_FOUND` | Unknown view, layout, or `open` target |
| `DUPLICATE_NAME` | `add` when the name already exists |
| `VALIDATION_FAILED` | Config fails the existing loader (kinds, `group_by`, layout trees) |
| `CONFIG_INVALID` | (reserved; decode failures are reported as `VALIDATION_FAILED` with a line) |
| `FILE_READ_ERROR` | Config or `--from` file could not be read |
| `FILE_WRITE_ERROR` | Atomic write failed |
| `CONFIRMATION_REQUIRED` | `views rm` while a layout still references the view; pass `--force` |
| `COMMAND_NOT_FOUND` | Unknown subcommand |
| `INVALID_INPUT` | Bad `--layout`, `--inputs`, `--panel`, or `--from` TOML |
| `INVALID_JSON` | `--from` JSON did not parse |
| `MULTIPLE_MATCHES` | (reserved) |
| `SKILL_NOT_FOUND` | Unknown shipped skill name |
| `SKILL_NOT_INSTALLED` | `skill remove` of a skill that is not installed |
| `SKILL_PATH_UNRESOLVED` | Install root could not be resolved; pass `--dest` |

`config check` prints `path:line: message` and, with `--json`, puts the same
list in `error.details.errors`. It does not talk to a vault. Writes are
temp-file + rename, keep comments on untouched tables, and refuse to save
anything the validator would reject.

`--layout` is `kind=board,group_by=state,fields=state,when`. Commas inside
`fields` belong to the list (`fields=state,when`). `--from` is a TOML or JSON
snippet, a file path, or `-` for stdin.

## Agent skill

Perch ships an [Agent Skills](https://agentskills.io) file at
`skills/perch/SKILL.md`. It teaches the CLI above: `--json`, `config check`,
`{{selected}}` bindings, and the rule that Perch does not assume a vault
schema. Install it the same way Raven installs skills:

```bash
perch skill list --json
perch skill install --confirm --json
perch skill install perch --scope project --confirm --json
perch skill remove perch --confirm --json
perch skill doctor --json
```

`perch skills` is an alias. User scope writes `~/.agents/skills`. Project
scope writes `.agents/skills` in the current directory. `--dest` overrides
the root.

Install and remove preview by default (`ok: true` with `needs_confirm: true`)
and write nothing. Agents cannot answer a y/N prompt; Perch never asks for
one. Pass `--confirm` to apply. A `.perch-skill-receipt.json` marks
Perch-managed files. A skill directory without that receipt is left alone.

## Pickers and search

Pickers are single keys in normal mode. There is no Space leader.

| Key | Opens |
| --- | --- |
| `t` | Types from `GET /v1/schema` (fuzzy). Selecting fills the main pane with that type's objects as a plain list. |
| `d` | Daily-note date picker, starting on today (`GET /v1/session` `daily.today_id` when present). `h`/`l` move a day, `j`/`k` a week, `t` today. Enter invokes Raven's read-only `date` hub and opens that day's daily note when `daily_exists`. |
| `q` | Saved queries (fuzzy). `q` does not quit; `ctrl+c` quits. |
| `v` | Saved views and layouts |
| `ctrl-p` / `f` | Notes: fuzzy over the `objects` corpus (paginated, cached, background), plus Raven `search` after two characters |
| `s` | Full-text content search (`POST /v1/invoke` command `search`) |
| `/` | Fuzzy search in the current view; `n` / `N` next/previous hit |
| `:` | Ad-hoc RQL |
| `,` | Config at the current view's entry |
| `S` | Save the on-screen view as `[views.name]` |

Saved queries that declare `args` open an argument prompt. Values are sent as
`query` `inputs` (`{{args.*}}`). Esc cancels.

## Preview and editor

`p` toggles a preview pane of the selected row's rendered Markdown.
Trait, section, and link rows preview the containing object, scrolled to
the row's source line. Reads are cached; moving the cursor does not block
the UI.

`e` opens the selected row's file in the editor from browse, preview, note
view, or the notes picker. When the row carries `file_path` and `line`
(as trait queries do), the editor jumps to that location. Perch does not
wait on `read` when `file_path` is already on the row.

Enter on a nested row opens the containing object's note view, scrolled
to the row's line. `S` saves the on-screen view's type or query and
layout; it does not treat the selected row as an object. Ad-hoc RQL
stays ephemeral.

## Keys

`?` lists this table inside the TUI. `esc` / `backspace` leave a note.
Overlays still close with `q` / `esc`. `ctrl+c` quits.

| Key | Action |
| --- | --- |
| `hjkl` / arrows | Move inside the focused panel |
| `ctrl-h/j/k/l` | Move focus among panels |
| `ctrl-w h/j/k/l` | Same as `ctrl-h/j/k/l` |
| `tab` / `shift-tab` | Cycle focus among panels |
| `ctrl-w o` | Zoom the focused panel |
| `ctrl-w =` | Restore splits |
| `gg` / `G` | Top / bottom |
| `ctrl-d` / `ctrl-u` | Half-page down / up |
| `enter` | Open note / follow link / move to the bound panel |
| `t` | Type picker |
| `d` | Daily-note date picker |
| `q` | Saved query picker (does not quit) |
| `v` | Views and layouts |
| `ctrl-p` / `f` | Notes picker |
| `s` | Full-text content search |
| `/` | Fuzzy search in view |
| `n` / `N` | Next / previous search hit |
| `.` | Cycle date granularity |
| `p` | Toggle preview |
| `:` | Ad-hoc RQL |
| `,` | Open config at this view |
| `S` | Save on-screen view |
| `e` | Open in editor |
| `r` | Refresh (and reload config) |
| `[` `]` | Note history back / forward |
| `[d` `]d` | Previous / next daily note (in a daily note) |
| `?` | Help |
| `esc` | Close overlay / clear search / back from a note |
| `ctrl+c` | Quit |

Focus follows the panel. The focused panel has a highlighted border.
Help and the status line show keys for that panel. In-panel movement is
unchanged (`hjkl` in a board still moves columns and rows). In-panel
scrolling is `ctrl-d` / `ctrl-u` only.

## Editor

`GET /v1/session` supplies `vault_path`, `editor`, and `editor_mode`
(`auto`, `terminal`, or `gui`), matching `rvn open`.

- `terminal`: suspend the TUI, run the editor in the foreground, refresh on return.
- `gui`: spawn the editor detached and keep the TUI running.
- `auto`: same heuristic as `rvn open` (vim/nvim/helix/nano/emacs/… in the
  foreground; `code`, `quill`, and other GUI commands in the background).

Line jumps use editor-specific arguments: vim-family `+N`, Helix `path:line`,
VS Code `-g path:line`. quill (`quill path:line`, GUI, no suspend) is treated
as a GUI editor, so `auto` does not suspend the TUI.

If `editor` is empty, or the server has no `/v1/session` (older `rvn`),
Perch falls back to `$VISUAL`, then `$EDITOR`.

## Architecture

```
perch  →  HTTP 127.0.0.1  →  rvn api  →  commandexec.Invoker
```

Endpoints used: `GET /v1/health`, `GET /v1/session` (including `daily` and
`revision`), `GET /v1/revision`, `GET /v1/schema`, `POST /v1/invoke` with
`query` (and `inputs` for saved-query args), `query_saved_list`, `objects`,
`read`, `search`, `date`, and `backlinks`. Write commands (`set`, `unset`,
`new`, `move`) and `daily` (which may create a file) are rejected in the
client.

Query, `read`, and `objects` rows are cached in memory. The notes picker is
backed by paginated `objects` loaded in the background. Older `rvn` without
`objects` falls back to per-type query prefetch. Daily notes use Raven's
read-only `date` hub (`daily_note_id` when `daily_exists`). The picker
anchors "today" on `session.daily.today_id` when present. Perch never
guesses a type or directory and never invokes `daily`.

Live refresh polls `GET /v1/revision` for `{generation, counter}`. A new
generation drops the in-memory cache and refetches schema plus every visible
panel. A higher counter refetches visible panels and any open note, debounced,
keeping cursor, selection, and scroll by object id. Bound panels re-resolve
`{{selected}}` after that refetch. Poll and refresh failures show a short
status-line mark and back off; they do not take over the footer.

The note view renders Markdown, shows frontmatter as a header, and cycles
wiki links from `read --json` `data.links` using UTF-8 byte offsets. Perch
does not parse `[[links]]` itself. Enter follows a resolved `object_id`.
A backlinks pane lists `data.backlinks`.

## Raven API gaps

1. **`open` is not allowlisted.** Correct for a read-only UI; Perch
   launches the editor itself using session `editor` / `editor_mode` and
   `vault_path` + `data.path`.
2. **Query sort.** There is no `sort:` in RQL yet. Perch sorts
   client-side from the view `sort` field.

Closed by Raven PR 308 (consumed here): `GET /v1/session` `daily`, `query`
`inputs` for saved-query args, and the paginated `objects` corpus. Perch
still never invokes `daily` because it may create files.

Closed by Raven PR 312 (consumed here): `GET /v1/revision` and
`session.revision`. Perch will not start against an `rvn api` that omits them.

## Develop

```bash
make test
make vet
make build
```

License: MIT, matching Raven.
