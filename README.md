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

## Config

Default path:

```text
~/.config/sqio/config.toml
```

Example:

```toml
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

[[connections]]
name = "local"
driver = "sqlite"
database = "/tmp/sqio.db"
readonly = false
```

Encrypted passwords can be decrypted with an age identity file:

```bash
sqio exec --conn prod --age-identity ~/.config/sqio/keys.txt --sql 'select 1'
```

SSH tunnel options are available from CLI and config:

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
  --sql 'select 1'
```

`query --pick` uses `fzf` when it is installed. If `fzf` is missing,
sqio falls back to a deterministic internal picker.

Config values are overridden by environment variables and CLI options.

## Smoke Test

The repository includes Docker Compose services for PostgreSQL and MySQL.

```bash
docker compose up -d postgres mysql
bash scripts/smoke-db.sh
docker compose down
```
