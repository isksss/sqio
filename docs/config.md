# 設定

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
result tab では `/` で結果行をフィルタし、`s` で先頭 column のソートを
切り替えられます。
result tab で `j/k` により行、`h/l` により cell を選択し、`e` で選択行、
`c` で選択 cell を update、`x` で delete できます。
SQL console では `ctrl+n` で SQL キーワード、テーブル名、カラム名を補完できます。
