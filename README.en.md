# sqio

`sqio` is a Go TUI/CLI integrated database client for MySQL,
PostgreSQL, SQLite, SQL Server, Oracle, DuckDB, and ClickHouse.

The CLI is the primary interface. The TUI is a frontend over the same
service layer, so every core workflow remains usable from scripts, CI,
and AI agents.

## Commands

```bash
sqio exec --sql 'select 1' --format json
sqio explain --conn local --sql 'select * from users'
sqio config validate
sqio conn list
sqio conn add local --driver sqlite --database ./local.db
sqio conn remove local
sqio conn test --conn local
sqio complete --prefix 'sel'
sqio query --sql 'select 1'
sqio query --pick
sqio fmt --sql 'select * from users'
sqio lint --sql 'select * from users'
sqio schemas --conn local
sqio tables --conn local
sqio columns --conn local --schema public --table users
sqio indexes --conn local --table users
sqio roles --conn local
sqio grants --conn local --role app_user
sqio ddl --conn local --table users
sqio schema export --conn local --format json
sqio schema diff --from before.json --to after.json
sqio er --conn local --format mermaid
sqio er --conn local --format mermaid --clipboard
sqio dump --conn local --table users --format csv --out users.csv
sqio dump --conn local --table users --format csv --out users.csv.gz
sqio load --conn local --table users --file users.csv
sqio load --conn local --table users --file users.json --format json
sqio load --conn local --table users --file users.jsonl --format jsonl
sqio load --conn local --table users --file users.yaml --format yaml
sqio load --conn local --table users --file users.csv.gz --format csv
sqio edit insert --conn local --table users --set name=alice
sqio edit update --conn local --table users --set name=bob --where 'id = 1'
sqio edit delete --conn local --table users --where 'id = 1'
sqio migrate status --conn local --dir migrations
sqio migrate plan --conn local --dir migrations
sqio migrate apply --conn local --dir migrations
sqio migrate rollback --conn local --dir migrations
sqio plugin list
sqio plugin run hello --arg
sqio history --format json
sqio history --search 'select' --favorite
sqio history favorite 1
sqio history tag 1 --tags report
sqio history run 1 --conn local
sqio tui --conn local
```

## Installation

Install from source:

```bash
go install github.com/isksss/sqio/cmd/sqio@latest
```

When working from this repository:

```bash
go build -o /tmp/sqio ./cmd/sqio
/tmp/sqio --help
```

## Config

Create a local config in the current directory:

```bash
sqio init
```

Create a global config:

```bash
sqio init -g
```

Config paths:

```text
~/.config/sqio/config.toml
sqio.toml
```

The global config is applied everywhere. A local `sqio.toml` is applied
only when running sqio from that file's directory or its descendants.
Config is merged in this order: defaults, global config, nearest local
`sqio.toml`, environment variables, then CLI options.

Example:

```toml
theme = "dark"
editor = "vi"

[query]
timeout = "30s"
max_rows = 1000
format = "table"

[formatter]
dialect = "postgres"
keyword_case = "upper"
identifier_case = "lower"
indent = 2
line_width = 100

[lint]
level = "warning"
enable = []
disable = []

[[connections]]
name = "local"
driver = "sqlite"
database = "/tmp/sqio.db"
readonly = false

[[connections]]
name = "prod"
driver = "postgres"
host = "db.internal"
database = "app"
user = "app"
readonly = true

[connections.ssh_tunnel]
enabled = true
host = "bastion.example.com"
user = "deploy"
private_key = "~/.ssh/id_ed25519"
known_hosts = "~/.ssh/known_hosts"
keepalive = "30s"
reconnect = true
reconnect_attempts = 3
jump_host = "jump.example.com"
jump_user = "deploy"
jump_private_key = "~/.ssh/id_ed25519"
```

Encrypted passwords can be decrypted with an age identity file:
Database and SSH passwords also support `env:NAME` and `file:/path/to/secret`
references. External secret managers can be resolved through existing CLIs with
`op:REF`, `aws-sm:SECRET_ID`, and `gcloud-secret:SECRET_ID`.

```bash
sqio exec --conn prod --age-identity ~/.config/sqio/keys.txt --sql 'select 1'
```

SSH tunnel options are available from CLI and config:
SSH connections verify host keys with `known_hosts`. When `--ssh-known-hosts`
or `ssh_tunnel.known_hosts` is omitted, sqio uses `~/.ssh/known_hosts`.
Set `--ssh-keepalive` or `ssh_tunnel.keepalive` to send periodic SSH keepalive
requests.
`--ssh-reconnect` / `ssh_tunnel.reconnect` recreates the SSH client after remote
dial failures. `--ssh-jump-host` / `ssh_tunnel.jump_host` connects to the tunnel
host through a jump host.

```bash
sqio exec \
  --driver postgres \
  --host db.internal \
  --database app \
  --user app \
  --ssh-tunnel \
  --ssh-host bastion.example.com \
  --ssh-user deploy \
  --ssh-private-key ~/.ssh/id_ed25519 \
  --ssh-known-hosts ~/.ssh/known_hosts \
  --ssh-keepalive 30s \
  --ssh-reconnect \
  --ssh-reconnect-attempts 3 \
  --sql 'select 1'
```

The TUI masks password input in the inline connection form. MySQL DSNs are built
with the driver DSN formatter, so special characters in database names and
passwords follow the driver's parsing rules.
The TUI detail panel can switch between columns, indexes, DDL, and results.
In the result tab, `/` filters result rows and `s` toggles sorting by the first
column. In the result tab, `j/k` selects rows, `h/l` selects cells, `e` updates
the selected row, `c` updates the selected cell, and `x` deletes the row.
In the SQL console, `ctrl+n` completes SQL keywords, table names, and column
names.

## Execution and Output

`exec` and `query` support multiple output formats via `--format`.

```bash
sqio exec --sql 'select 1' --format table
sqio exec --sql 'select 1' --format json
sqio exec --sql 'select 1' --format jsonl
sqio exec --sql 'select 1' --format csv
sqio exec --sql 'select 1' --format tsv
sqio exec --sql 'select 1' --format markdown
sqio exec --sql 'select 1' --format yaml
```

For database-backed `exec` / `query` runs, the final row-returning statement is
written incrementally to the output writer. This avoids keeping the full result
set in memory before output. When `--transaction` is used, sqio keeps the
previous buffered result path so results are not emitted before commit.

```bash
sqio exec --conn local --sql 'select * from users' --max-rows 1000 --format jsonl
sqio exec --conn local --sql 'select * from users' --out users.csv --format csv
```

`--max-rows` limits how many rows are read. `--max-bytes` limits bytes written
to the output writer.

`query --pick` uses `fzf` when it is installed. If `fzf` is missing,
sqio falls back to a deterministic internal picker.

`explain` runs the target database's `EXPLAIN`. With `--analyze`, supported
databases use `EXPLAIN ANALYZE`.

`migrate status` / `migrate plan` / `migrate apply` / `migrate rollback` /
`migrate baseline` process `*.sql` migrations in filename order and record
applied versions, checksums, and dirty state in the `sqio_migrations` table.
Rollback uses matching `.down.sql` files.

`edit insert` / `edit update` / `edit delete` modify table rows. `update` and
`delete` require `--where` to avoid accidental broad changes.

`plugin` treats executable `sqio-plugin-*` files on `PATH` as external plugins.

Config values are overridden by environment variables and CLI options.
`conn add` / `conn remove` add and delete connection definitions in the config
file. Without `--config`, sqio writes to the user config path.

`complete` prints SQL keyword, table, and column completion candidates. With
`--conn`, it uses metadata from the target database.

`history` can be filtered with `--search`, `--conn`, `--favorite`, and `--tags`.
Use `history favorite`, `history unfavorite`, and `history tag` to organize entries,
and `history run` to re-run SQL from history. History stores success/failure,
error summaries, row counts, elapsed time, and driver names.
`exec --audit-log PATH` appends JSONL execution records without DSNs or passwords.

## Metadata

`schemas`, `tables`, `columns`, `ddl`, `schema export`, and `er` support SQLite,
DuckDB, MySQL, MariaDB, TiDB, PostgreSQL, CockroachDB, SQL Server, Oracle, and
ClickHouse metadata. MariaDB/TiDB are handled through MySQL compatibility, and
CockroachDB through PostgreSQL compatibility. Column metadata includes type, nullability,
primary key, unique, default, and single-column foreign key references when
the driver exposes them.
For PostgreSQL, MySQL, SQL Server, Oracle, DuckDB, and ClickHouse, `--schema`
selects the target schema/database.
`indexes` and `schema export` include index name, columns, and unique / primary
metadata.
`roles` / `grants` read PostgreSQL and MySQL access metadata. SQLite returns
empty results.

## Lint Rules

Built-in rules cover unsafe writes (`delete-without-where`,
`update-without-where`, `truncate`, `drop-database`), query performance
(`select-star`, `leading-wildcard-like`, `or-abuse`, `implicit-join`,
`cartesian-join`, `limit-without-order`), and correctness
(`not-in-null`). With `dialect` or `lint --dialect`, sqio also reports clear
PostgreSQL/MySQL/SQLite incompatibilities. `keyword-case` is opt-in via
`--enable keyword-case`.

## Development

This repository is structured as a Go module.

```text
cmd/sqio/              CLI entrypoint
internal/cli/          CLI command definitions
internal/config/       config loading and merge rules
internal/db/           database connections, DSN handling, metadata
internal/formatter/    SQL formatter
internal/linter/       SQL lint rules
internal/service/      shared application service layer
internal/tui/          TUI frontend
scripts/               CI and smoke test scripts
```

Typical checks:

```bash
gofmt -w cmd internal
go test ./...
go test ./... -covermode=atomic -coverprofile=/tmp/sqio-cover.out
go build -o /tmp/sqio ./cmd/sqio
markdownlint-cli2 README.md
```

Coverage summary:

```bash
go tool cover -func=/tmp/sqio-cover.out
```

Lightweight CI-equivalent checks:

```bash
bash scripts/ci-check.sh
```

## Smoke Test

The repository includes Docker Compose services for PostgreSQL and MySQL.

```bash
docker compose up -d postgres mysql
bash scripts/smoke-db.sh
docker compose down
```

To run formatting, unit tests, build, README lint, and DB smoke tests together:

```bash
bash scripts/test-all.sh
```
