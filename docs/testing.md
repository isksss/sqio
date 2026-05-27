# テスト

## 通常確認

Go の変更時は unit test と build を確認します。

```bash
gofmt -w cmd internal
go test ./...
go build -o /tmp/sqio ./cmd/sqio
```

ドキュメント変更時は Markdown lint を確認します。

```bash
markdownlint-cli2 README.md README.en.md docs/*.md
```

coverage を確認する場合:

```bash
go test ./... -covermode=atomic -coverprofile=/tmp/sqio-cover.out
go tool cover -func=/tmp/sqio-cover.out
```

## Docker Runner

`sqio-test` は Go、OpenSSH client、Markdown lint を含むテスト実行用
container です。DB と sqio の実行環境を Docker network 内に閉じます。

```bash
docker compose --profile test build sqio-test
docker compose --profile test run --rm sqio-test bash scripts/test-all-in-docker.sh
docker compose down
```

`scripts/test-all.sh` は上記 runner を使い、format check、unit test、build、
Markdown lint、実 DB smoke test をまとめて実行します。

```bash
bash scripts/test-all.sh
```

## 実 DB Smoke Test

Docker Compose には PostgreSQL、MySQL、SQL Server、Oracle、ClickHouse、
SSH tunnel 検証用 OpenSSH、sqio 実行用 runner が含まれています。
SQLite と DuckDB は runner 内の一時 DB ファイルで検証します。

```bash
docker compose --profile test build sqio-test
docker compose up -d postgres mysql sqlserver oracle clickhouse ssh
docker compose --profile test run --rm sqio-test bash scripts/smoke-db.sh
docker compose down
```

`scripts/smoke-db.sh` は各 DB で table 作成、insert、select、metadata 取得を
確認します。SSH smoke test は Docker 内の OpenSSH service 経由で PostgreSQL へ
接続します。

SQL Server は compose の `ACCEPT_EULA=Y` で EULA に同意します。
Oracle は初回 pull と起動に時間がかかる場合があります。
