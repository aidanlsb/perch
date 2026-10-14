# Perch

A read-only terminal UI for [Raven](https://github.com/aidanlsb/raven).
Markdown stays the source of truth. Every read goes through `rvn api`.
Perch does not import Raven internals and does not write to the vault.

## Install

Requires Go 1.22+ and a `rvn` binary on `PATH`.

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

Perch suspends the TUI, opens the file, and refreshes on return.

Raven's `editor` setting in `config.toml` is **not** exposed on `rvn api`
(not on `/v1/health`, `/v1/schema`, `/v1/commands`, or the invoke
allowlist). Perch falls back to `$VISUAL`, then `$EDITOR`. See
[Raven API gaps](#raven-api-gaps).

## Architecture

```
perch  →  HTTP 127.0.0.1  →  rvn api  →  commandexec.Invoker
```

Endpoints used: `GET /v1/health`, `GET /v1/schema`, `GET /v1/commands`
(unused beyond discovery), `POST /v1/invoke` with `query`,
`query_saved_list`, `read`, `search`, and `backlinks`. Write commands
(`set`, `unset`, `new`, `move`) and `daily` (which may create a file) are
rejected in the client.

## Raven API gaps

Concrete changes that would help a companion UI, none of which Perch
implements by talking to Raven internals:

1. **Editor.** Add `editor` and `editor_mode` to `GET /v1/health` (or a
   new `GET /v1/session`). Today the only way to read Raven's editor is
   `config.toml`, which is outside the API contract. Alternative: put a
   read-only `config_show` on the invoke allowlist.
2. **Vault path on health.** `meta.vault_context.path` is present on
   invoke responses, and the `--json` startup line has `vault_path`.
   Attach-only clients still have to invoke something to learn the path.
   Include `vault_path` on `GET /v1/health`.
3. **`open` is not allowlisted.** Correct for a read-only UI; Perch
   therefore spawns the editor itself and needs an absolute path
   (`vault_path` + `data.path`).
4. **Saved-query inputs.** Auto lists skip saved queries that declare
   `args`. Document how extra invoke args bind to `{{args.*}}`, or add a
   dedicated invoke field.
5. **Query sort.** There is no `sort:` in RQL yet. Perch sorts
   client-side from the saved view's `sort` field.

## Develop

```bash
make test
make vet
make build
```

License: MIT, matching Raven.
