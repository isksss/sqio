# 開発

このリポジトリは Go module として構成されています。

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

通常の確認:

```bash
gofmt -w cmd internal
go test ./...
go test ./... -covermode=atomic -coverprofile=/tmp/sqio-cover.out
go build -o /tmp/sqio ./cmd/sqio
markdownlint-cli2 README.md docs/*.md
```

coverage の概要確認:

```bash
go tool cover -func=/tmp/sqio-cover.out
```

CI と同等の軽量チェック:

```bash
bash scripts/ci-check.sh
```

## Smoke Test

このリポジトリには PostgreSQL、MySQL、SQL Server、Oracle、ClickHouse、
SSH tunnel 検証用 OpenSSH、sqio 実行用 runner の Docker Compose サービスが
含まれています。SQLite と DuckDB は runner 内の一時DBファイルで検証します。

```bash
docker compose --profile test build sqio-test
docker compose up -d postgres mysql sqlserver oracle clickhouse ssh
docker compose --profile test run --rm sqio-test bash scripts/smoke-db.sh
docker compose down
```

format、unit test、build、README/docs lint、DB smoke test をまとめて実行する場合:

```bash
bash scripts/test-all.sh
```

SQL Server は compose の `ACCEPT_EULA=Y` で EULA に同意します。Oracle は
初回 pull と起動に時間がかかる場合があります。SSH smoke test は Docker 内の
OpenSSH service 経由で PostgreSQL へ接続します。
