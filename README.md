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

## Views

The sidebar is generated from `GET /v1/schema` at runtime: one list per
type (except `section`, which Raven cannot query as `type:section`) and
one list per saved query. No Perch config is required.

Board and date layouts live in Perch's own config, defaulting to
`~/.config/perch/config.toml`:

```toml
[[views]]
name = "Projects by status"
source = "type:project"
kind = "board"
group_by = "status"

[[views]]
name = "Dated notes"
source = "type:date"
kind = "date"
group_by = "date"
granularity = "week"
```

`kind` is `list`, `board`, or `date`. Board `group_by` is an enum or ref
field. Date `group_by` is a date/datetime field; `granularity` is `day`,
`week`, or `month`. `d`/`w`/`m` retile a date view live.

The note view renders Markdown, shows frontmatter as a header, and cycles
wiki links from `read --json` `data.links` using UTF-8 byte offsets. Perch
does not parse `[[links]]` itself. Enter follows a resolved `object_id`.
A backlinks pane lists `data.backlinks`.

## Keys

| Key | Action |
| --- | --- |
| `hjkl` / arrows | Move |
| `tab` | Sidebar ↔ list; in a note, cycle wiki links |
| `enter` | Open note / follow link |
| `[` `]` | History back / forward |
| `/` | Filter the current view |
| `s` | Search via Raven `search` |
| `e` `o` | Open the current file in an editor |
| `r` | Refresh |
| `q` | Back, or quit |

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

Endpoints used: `GET /v1/health`, `GET /v1/session`, `GET /v1/schema`,
`POST /v1/invoke` with `query`, `query_saved_list`, `read`, `search`, and
`backlinks`. Write commands (`set`, `unset`, `new`, `move`) and `daily`
(which may create a file) are rejected in the client.

## Raven API gaps

1. **`open` is not allowlisted.** Correct for a read-only UI; Perch
   launches the editor itself using session `editor` / `editor_mode` and
   `vault_path` + `data.path`.
2. **Saved-query inputs.** Auto lists skip saved queries that declare
   `args`. Document how extra invoke args bind to `{{args.*}}`, or add a
   dedicated invoke field.
3. **Query sort.** There is no `sort:` in RQL yet. Perch sorts
   client-side from the saved view's `sort` field.

## Develop

```bash
make test
make vet
make build
```

License: MIT, matching Raven.
