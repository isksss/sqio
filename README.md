# sqio

English README: [README.en.md](README.en.md)

`sqio` は MySQL、PostgreSQL、SQLite に対応した Go 製の
TUI/CLI 統合データベースクライアントです。

CLI が主要なインターフェースです。TUI は同じサービス層の
フロントエンドとして動作するため、主要なワークフローは
スクリプト、CI、AI エージェントからも利用できます。

## コマンド

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

## インストール

ソースから build できます。

```bash
go install github.com/isksss/sqio/cmd/sqio@latest
```

このリポジトリを checkout 済みの場合:

```bash
go build -o /tmp/sqio ./cmd/sqio
/tmp/sqio --help
```

## 設定

現在のディレクトリにローカル設定を作成します。

```bash
sqio init
```

グローバル設定を作成します。

```bash
sqio init -g
```

設定ファイルのパス:

```text
~/.config/sqio/config.toml
sqio.toml
```

グローバル設定は常に適用されます。ローカルの `sqio.toml` は、
そのファイルがあるディレクトリ、またはその配下から sqio を実行した場合のみ
適用されます。設定は defaults、global config、nearest local `sqio.toml`、
environment variables、CLI options の順にマージされます。

例:

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
```

暗号化されたパスワードは age identity file で復号できます。

```bash
sqio exec --conn prod --age-identity ~/.config/sqio/keys.txt --sql 'select 1'
```

SSH tunnel のオプションは CLI と設定ファイルの両方で利用できます。

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

`query --pick` は `fzf` がインストールされている場合に `fzf` を使います。
`fzf` がない場合、sqio は決定的に動作する組み込み picker にフォールバックします。

設定値は environment variables と CLI options で上書きできます。

## メタデータ

`tables`、`columns`、`ddl`、`schema export`、`er` は SQLite、MySQL、
PostgreSQL のメタデータに対応しています。列メタデータには、driver が提供する場合、
type、nullability、primary key、unique、default、single-column foreign key
references が含まれます。

## Lint ルール

組み込みルールは、危険な書き込み (`delete-without-where`、
`update-without-where`、`truncate`、`drop-database`)、クエリ性能
(`select-star`、`leading-wildcard-like`、`or-abuse`、`implicit-join`、
`cartesian-join`、`limit-without-order`)、正確性 (`not-in-null`) を対象にします。
`keyword-case` は `--enable keyword-case` で有効化する opt-in ルールです。

## 開発

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
go build -o /tmp/sqio ./cmd/sqio
markdownlint-cli2 README.md
```

CI と同等の軽量チェック:

```bash
bash scripts/ci-check.sh
```

## Smoke Test

このリポジトリには PostgreSQL と MySQL の Docker Compose サービスが含まれています。

```bash
docker compose up -d postgres mysql
bash scripts/smoke-db.sh
docker compose down
```

format、unit test、build、README lint、DB smoke test をまとめて実行する場合:

```bash
bash scripts/test-all.sh
```
