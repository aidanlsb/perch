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
```

If nothing is listening, Perch starts `rvn api --json --port 0` as a child
and sends SIGTERM to that child on exit. If a server is already up and you
pass a token, Perch connects and leaves it running.

## Layouts

Every source — a type, a saved query, or an ad-hoc RQL string — renders
through one `Spec`: **list**, **board** (grouped by an enum or ref field),
or **date** (grouped by a date/datetime field). Config chooses the default;
`L` cycles at runtime.

Zero-config heuristics (first match wins; kept small on purpose):

1. Scalar enum field named `status` → board grouped by `status`
2. Type named `date` with a date/datetime field (prefer `date`) → date grouping by day
3. Otherwise list

`L` then walks every viable option for the current type: list, each enum/ref
board, then each date field. `.` cycles board group-by, or date granularity
(day → week → month).

## Config

Defaults live in Perch's own file (Raven never reads it), defaulting to
`~/.config/perch/config.toml`. See `examples/config.toml`.

```toml
[types.project]
layout = "board"
group_by = "status"
fields = ["status", "due"]
sort = "name"

[types.date]
layout = "date"
group_by = "date"
granularity = "week"

[queries.open]
layout = "list"

[[views]]
name = "This month"
source = "type:date"
layout = "date"
group_by = "date"
granularity = "month"
```

`layout` is `list`, `board`, or `date`. `kind` is accepted as an alias.
Board `group_by` is an enum or ref field. Date `group_by` is a date/datetime
field; `granularity` is `day`, `week`, or `month`.

Named `[[views]]` rows still appear in the sidebar and the query picker.
They are extra sources, not a second layout system.

## Pickers and search

| Key | Opens |
| --- | --- |
| `t` / `space t` | Types (fuzzy) |
| `q` / `space q` | Saved queries and named views (fuzzy) |
| `ctrl-p` / `space f` | Notes: fuzzy over the `objects` corpus (paginated, cached, background), plus Raven `search` after two characters |
| `/` | Fuzzy search in the current view; `n` / `N` next/previous hit |
| `:` | Ad-hoc RQL |
| `d` / `space d` | Today's daily note from `GET /v1/session` `daily` if it exists, else the `date` list |

Saved queries that declare `args` open an argument prompt. Values are sent as
`query` `inputs` (`{{args.*}}`). Esc cancels.

The sidebar is still generated from `GET /v1/schema` (types, saved queries,
named views). `section` is omitted because Raven cannot query `type:section`.

## Preview and editor

`p` toggles a preview pane of the selected object's rendered Markdown.
Reads are cached; moving the cursor does not block the UI.

`e` opens the selected row's file in the editor from browse, preview, note
view, or the notes picker. Perch does not wait on `read` when `file_path`
is already on the row.

## Keys

`?` lists this table inside the TUI. `q` / `esc` close overlays and leave a
note; from browse, `q` is the query picker. `ctrl+c` quits.

| Key | Action |
| --- | --- |
| `hjkl` / arrows | Move |
| `tab` | Sidebar ↔ list ↔ preview |
| `gg` / `G` | Top / bottom |
| `ctrl-d` / `ctrl-u` | Half-page down / up |
| `enter` | Open note / follow link |
| `t` | Type picker |
| `q` | Saved query picker (back, in a note or overlay) |
| `ctrl-p` / `space f` | Notes picker |
| `/` | Fuzzy search in view |
| `n` / `N` | Next / previous search hit |
| `d` | Daily note |
| `L` | Cycle layout |
| `.` | Cycle group-by / date granularity |
| `p` | Toggle preview |
| `:` | Ad-hoc RQL |
| `e` | Open in editor |
| `r` | Refresh |
| `[` `]` | Note history back / forward |
| `?` | Help |
| `esc` | Close overlay / clear search / back |
| `ctrl+c` | Quit |

## Editor

`GET /v1/session` supplies `vault_path`, `editor`, and `editor_mode`
(`auto`, `terminal`, or `gui`), matching `rvn open`.

- `terminal`: suspend the TUI, run the editor in the foreground, refresh on return.
- `gui`: spawn the editor detached and keep the TUI running.
- `auto`: same heuristic as `rvn open` (vim/nvim/helix/nano/emacs/… in the
  foreground; `code` and other GUI commands in the background).

If `editor` is empty, or the server has no `/v1/session` (older `rvn`),
Perch falls back to `$VISUAL`, then `$EDITOR`.

## Architecture

```
perch  →  HTTP 127.0.0.1  →  rvn api  →  commandexec.Invoker
```

Endpoints used: `GET /v1/health`, `GET /v1/session` (including `daily`),
`GET /v1/schema`, `POST /v1/invoke` with `query` (and `inputs` for saved-query
args), `query_saved_list`, `objects`, `read`, `search`, `backlinks`, and the
read-only `date` hub. Write commands (`set`, `unset`, `new`, `move`) and
`daily` (which may create a file) are rejected in the client.

Query, `read`, and `objects` rows are cached in memory. The notes picker is
backed by paginated `objects` loaded in the background. Older `rvn` without
`objects` falls back to per-type query prefetch. Older `rvn` without
`session.daily` falls back to the `date` command, then `type:date`.

The note view renders Markdown, shows frontmatter as a header, and cycles
wiki links from `read --json` `data.links` using UTF-8 byte offsets. Perch
does not parse `[[links]]` itself. Enter follows a resolved `object_id`.
A backlinks pane lists `data.backlinks`.

## Raven API gaps

1. **`open` is not allowlisted.** Correct for a read-only UI; Perch
   launches the editor itself using session `editor` / `editor_mode` and
   `vault_path` + `data.path`.
2. **Query sort.** There is no `sort:` in RQL yet. Perch sorts
   client-side from the layout `sort` field.

Closed by Raven PR 308 (consumed here): `GET /v1/session` `daily`, `query`
`inputs` for saved-query args, and the paginated `objects` corpus. Perch
still never invokes `daily` because it may create files.

## Develop

```bash
make test
make vet
make build
```

License: MIT, matching Raven.
