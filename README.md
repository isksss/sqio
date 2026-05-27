# sqio

English README: [README.en.md](README.en.md)

`sqio` は MySQL、PostgreSQL、SQLite、SQL Server、Oracle、DuckDB、
ClickHouse に対応した Go 製の
TUI/CLI 統合データベースクライアントです。

CLI が主要なインターフェースです。TUI は同じサービス層の
フロントエンドとして動作するため、主要なワークフローは
スクリプト、CI、AI エージェントからも利用できます。

## コマンド

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

暗号化されたパスワードは age identity file で復号できます。
password と SSH password は `env:NAME` と `file:/path/to/secret` 参照にも
対応しています。外部シークレットマネージャーは既存 CLI 経由で
`op:REF`、`aws-sm:SECRET_ID`、`gcloud-secret:SECRET_ID` を解決できます。

```bash
sqio exec --conn prod --age-identity ~/.config/sqio/keys.txt --sql 'select 1'
```

SSH tunnel のオプションは CLI と設定ファイルの両方で利用できます。
SSH 接続では `known_hosts` による host key verification を行います。
`--ssh-known-hosts` または `ssh_tunnel.known_hosts` を省略した場合は、
`~/.ssh/known_hosts` を使います。
`--ssh-keepalive` または `ssh_tunnel.keepalive` を指定すると、
SSH keepalive request を定期送信します。
`--ssh-reconnect` / `ssh_tunnel.reconnect` は remote dial 失敗時に
SSH client を張り直します。`--ssh-jump-host` / `ssh_tunnel.jump_host` を
指定すると jump host 経由で tunnel host に接続します。

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

TUI の新規 DB 接続フォームでは password 入力はマスク表示されます。
MySQL の DSN は driver の DSN formatter で組み立てるため、database 名や
password に特殊文字が含まれる場合も driver の解釈に合わせて扱われます。
TUI の detail panel では columns / indexes / DDL / result を切り替えられます。
result tab では `/` で結果行をフィルタし、`s` で先頭 column のソートを切り替えられます。
result tab で `j/k` により行、`h/l` により cell を選択し、`e` で選択行、
`c` で選択 cell を update、`x` で delete できます。
SQL console では `ctrl+n` で SQL キーワード、テーブル名、カラム名を補完できます。

## 実行と出力

`exec` と `query` は `--format` で出力形式を選べます。

```bash
sqio exec --sql 'select 1' --format table
sqio exec --sql 'select 1' --format json
sqio exec --sql 'select 1' --format jsonl
sqio exec --sql 'select 1' --format csv
sqio exec --sql 'select 1' --format tsv
sqio exec --sql 'select 1' --format markdown
sqio exec --sql 'select 1' --format yaml
```

DB 接続ありの `exec` / `query` は、行を返す最後の statement を writer に
逐次出力します。これにより、大きな結果セットでも全行をメモリに保持せずに
出力できます。`--transaction` 使用時は commit 前に結果を出力しないため、
従来どおり一度結果を保持してから出力します。

```bash
sqio exec --conn local --sql 'select * from users' --max-rows 1000 --format jsonl
sqio exec --conn local --sql 'select * from users' --out users.csv --format csv
```

`--max-rows` は読み取る行数を制限します。`--max-bytes` は出力先 writer の
byte 数を制限します。

`query --pick` は `fzf` がインストールされている場合に `fzf` を使います。
`fzf` がない場合、sqio は決定的に動作する組み込み picker にフォールバックします。

`explain` は対象 DB の `EXPLAIN` を実行します。`--analyze` を付けると、
対応 DB では `EXPLAIN ANALYZE` を使います。

`migrate status` / `migrate plan` / `migrate apply` / `migrate rollback` /
`migrate baseline` は directory 内の `*.sql` migration を filename 順に扱い、
適用済み version、checksum、dirty state を `sqio_migrations` table に記録します。
rollback は対応する `.down.sql` を使います。

`edit insert` / `edit update` / `edit delete` は table row を変更します。
`update` と `delete` は誤操作を避けるため `--where` が必須です。

`plugin` は `PATH` 上の `sqio-plugin-*` executable を外部 plugin として扱います。

設定値は environment variables と CLI options で上書きできます。
`conn add` / `conn remove` は config file の接続定義を追加・削除します。
`--config` 未指定時は user config path に保存します。

`complete` は SQL キーワード、テーブル名、カラム名の補完候補を出力します。
`--conn` 付きの場合は接続先のメタデータを使います。

`history` は `--search`、`--conn`、`--favorite`、`--tags` で絞り込めます。
`history favorite` / `history unfavorite` / `history tag` で履歴を整理し、
`history run` で過去の SQL を再実行できます。履歴には実行成否、エラー概要、
行数、elapsed time、driver も保存されます。
`exec --audit-log PATH` は DSN/password を含めず、実行結果を JSONL で追記します。

## メタデータ

`schemas`、`tables`、`columns`、`ddl`、`schema export`、`er` は SQLite、
DuckDB、MySQL、MariaDB、TiDB、PostgreSQL、CockroachDB、SQL Server、Oracle、
ClickHouse のメタデータに対応しています。MariaDB/TiDB は MySQL 互換、
CockroachDB は PostgreSQL 互換として扱います。
列メタデータには、driver が提供する場合、
type、nullability、primary key、unique、default、single-column foreign key
references が含まれます。
PostgreSQL / MySQL / SQL Server / Oracle / DuckDB / ClickHouse では
`--schema` で対象 schema/database を指定できます。
`indexes` と `schema export` には index 名、対象 column、unique / primary 情報が
含まれます。
`roles` / `grants` は PostgreSQL と MySQL の権限情報を参照します。SQLite では
空結果になります。

## Lint ルール

組み込みルールは、危険な書き込み (`delete-without-where`、
`update-without-where`、`truncate`、`drop-database`)、クエリ性能
(`select-star`、`leading-wildcard-like`、`or-abuse`、`implicit-join`、
`cartesian-join`、`limit-without-order`)、正確性 (`not-in-null`) を対象にします。
`dialect` または `lint --dialect` 指定時は PostgreSQL/MySQL/SQLite の明確な
非互換構文も検出します。
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
go test ./... -covermode=atomic -coverprofile=/tmp/sqio-cover.out
go build -o /tmp/sqio ./cmd/sqio
markdownlint-cli2 README.md
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
