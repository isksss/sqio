# 接続

sqio は SQLite、DuckDB、MySQL、MariaDB、TiDB、PostgreSQL、CockroachDB、
SQL Server、Oracle、ClickHouse に対応しています。
MariaDB/TiDB は MySQL 互換、CockroachDB は PostgreSQL 互換として扱います。

## Driver 名

設定や CLI で指定できる driver 名:

| 分類 | driver / alias |
| --- | --- |
| SQLite | `sqlite`, `sqlite3` |
| DuckDB | `duckdb` |
| PostgreSQL | `postgres`, `postgresql`, `pgx`, `cockroach`, `cockroachdb` |
| MySQL | `mysql`, `mariadb`, `tidb` |
| SQL Server | `sqlserver`, `mssql` |
| Oracle | `oracle` |
| ClickHouse | `clickhouse`, `ch` |

DSN を直接渡す場合:

```bash
sqio exec \
  --driver postgres \
  --dsn 'postgres://sqio:sqio@localhost:15432/sqio?sslmode=disable' \
  --sql 'select 1'
```

接続フィールドから DSN を組み立てる場合:

```bash
sqio exec \
  --driver postgres \
  --host localhost \
  --port 15432 \
  --database sqio \
  --user sqio \
  --password sqio \
  --sql 'select 1'
```

MySQL の DSN は driver の DSN formatter で組み立てるため、database 名や
password に特殊文字が含まれる場合も driver の解釈に合わせて扱われます。

## SSH Tunnel

SSH tunnel は CLI と設定ファイルの両方で利用できます。
SSH 接続では `known_hosts` による host key verification を行います。
`--ssh-known-hosts` または `ssh_tunnel.known_hosts` を省略した場合は、
`~/.ssh/known_hosts` を使います。

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

`--ssh-keepalive` または `ssh_tunnel.keepalive` を指定すると、SSH keepalive
request を定期送信します。
`--ssh-reconnect` / `ssh_tunnel.reconnect` は remote dial 失敗時に SSH client を
張り直します。`--ssh-jump-host` / `ssh_tunnel.jump_host` を指定すると jump host
経由で tunnel host に接続します。

## TUI

TUI の新規 DB 接続フォームでは password 入力はマスク表示されます。
detail panel では columns / indexes / DDL / result を切り替えられます。
result tab では `/` で結果行をフィルタし、`s` で先頭 column のソートを
切り替えられます。

result tab で `j/k` により行、`h/l` により cell を選択し、`e` で選択行、
`c` で選択 cell を update、`x` で delete できます。
SQL console では `ctrl+n` で SQL キーワード、テーブル名、カラム名を補完できます。
