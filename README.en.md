# sqio

`sqio` is a Go TUI/CLI integrated database client for MySQL,
PostgreSQL, and SQLite.

The CLI is the primary interface. The TUI is a frontend over the same
service layer, so every core workflow remains usable from scripts, CI,
and AI agents.

## Commands

```bash
sqio exec --sql 'select 1' --format json
sqio query --sql 'select 1'
sqio query --pick
sqio fmt --sql 'select * from users'
sqio lint --sql 'select * from users'
sqio tables --conn local
sqio columns --conn local --table users
sqio ddl --conn local --table users
sqio schema export --conn local --format json
sqio er --conn local --format mermaid
sqio er --conn local --format mermaid --clipboard
sqio history --format json
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
```

Encrypted passwords can be decrypted with an age identity file:

```bash
sqio exec --conn prod --age-identity ~/.config/sqio/keys.txt --sql 'select 1'
```

SSH tunnel options are available from CLI and config:
SSH connections verify host keys with `known_hosts`. When `--ssh-known-hosts`
or `ssh_tunnel.known_hosts` is omitted, sqio uses `~/.ssh/known_hosts`.

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
  --sql 'select 1'
```

The TUI masks password input in the inline connection form. MySQL DSNs are built
with the driver DSN formatter, so special characters in database names and
passwords follow the driver's parsing rules.

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

Config values are overridden by environment variables and CLI options.

## Metadata

`tables`, `columns`, `ddl`, `schema export`, and `er` support SQLite,
MySQL, and PostgreSQL metadata. Column metadata includes type, nullability,
primary key, unique, default, and single-column foreign key references when
the driver exposes them.

## Lint Rules

Built-in rules cover unsafe writes (`delete-without-where`,
`update-without-where`, `truncate`, `drop-database`), query performance
(`select-star`, `leading-wildcard-like`, `or-abuse`, `implicit-join`,
`cartesian-join`, `limit-without-order`), and correctness
(`not-in-null`). `keyword-case` is opt-in via `--enable keyword-case`.

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
