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
through a named `Spec`: **list**, **board** (grouped by an enum or ref
field), or **date** (grouped by a date/datetime field). Each type and each
saved query has an **ordered list** of named layouts. The first is the
default. The header shows the layout name.

`L` cycles forward through that list only; `gL` cycles backward.
(`L` is already shift+l in the terminal, so reverse cannot be shift-L.)
`gl` or `space l` opens a fuzzy layout picker. Number keys `1`–`9` jump
to layout N (1-based). `.` cycles date granularity (day → week → month)
inside a date layout and no longer invents extra group-by fields.

Zero-config: when a type has no `[[layouts]]`, Perch synthesizes a small
set from the live schema (this is the entire L-cycle):

1. **List** always (and it is the default)
2. **Board** if the type has a scalar enum field, grouped by the first
   enum, preferring a field named `status` (named "Board" when the field
   is `status`, else "By \<enum\>")
3. **Date** if the type has a scalar date/datetime field, grouped by the
   first such field, preferring `date` (named "Date" when the field is
   `date`, else "By \<field\>"; granularity day)

Ref fields do not produce a synthesized board. Saved queries with no
configured layouts start as List and pick up the same synthesis once the
result type is known.

## Config

Defaults live in Perch's own file (Raven never reads it), defaulting to
`~/.config/perch/config.toml`. See `examples/config.toml`.

```toml
[[types.project.layouts]]
name = "Board"
kind = "board"
group_by = "status"
fields = ["status", "due"]

[[types.project.layouts]]
name = "By due"
kind = "list"
fields = ["status", "due"]
sort = "due"

[[types.project.layouts]]
name = "Active"
kind = "list"
filter = ".status==active"

[[types.date.layouts]]
name = "Week"
kind = "date"
group_by = "date"
granularity = "week"

[[queries.open.layouts]]
name = "Open"
kind = "list"
```

`kind` is `list`, `board`, or `date`. Board `group_by` is an enum or ref
field. Date `group_by` is a date/datetime field; `granularity` is `day`,
`week`, or `month`. Optional `filter` is an RQL predicate appended to the
type source (`type:project` + `.status==active`). For a saved query it is
appended to the query's RQL text.

The old single `layout =` per type and the `[[views]]` table are removed.
If they appear, Perch prints an error pointing at this shape.

At startup Perch validates configured layouts against `GET /v1/schema`
and shows warnings in the footer for unknown types, queries, or fields.

## Pickers and search

| Key | Opens |
| --- | --- |
| `t` / `space t` | Types (fuzzy) |
| `q` / `space q` | Saved queries (fuzzy) |
| `gl` / `space l` | Named layouts for the current source |
| `ctrl-p` / `space f` | Notes: fuzzy over the `objects` corpus (paginated, cached, background), plus Raven `search` after two characters |
| `/` | Fuzzy search in the current view; `n` / `N` next/previous hit |
| `:` | Ad-hoc RQL |
| `d` / `space d` | Today's daily note from `GET /v1/session` `daily` if it exists, else the `date` list |

Saved queries that declare `args` open an argument prompt. Values are sent as
`query` `inputs` (`{{args.*}}`). Esc cancels.

The sidebar is generated from `GET /v1/schema` (types and saved queries).
`section` is omitted because Raven cannot query `type:section`.

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
| `L` | Next named layout |
| `gL` | Previous named layout |
| `gl` / `space l` | Layout picker |
| `1`–`9` | Jump to layout N |
| `.` | Cycle date granularity |
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
