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

接続フィールド、driver 名、SSH tunnel、TUI の接続操作は
[接続](connection.md) を参照してください。
