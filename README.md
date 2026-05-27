# sqio

English README: [README.en.md](README.en.md)

`sqio` は MySQL、PostgreSQL、SQLite、SQL Server、Oracle、DuckDB、
ClickHouse に対応した Go 製の TUI/CLI 統合データベースクライアントです。

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

## ドキュメント

- [ドキュメント一覧](docs/README.md)
- [設定](docs/config.md)
- [接続](docs/connection.md)
- [実行と出力](docs/execution.md)
- [メタデータ](docs/metadata.md)
- [Lint ルール](docs/lint.md)
- [テスト](docs/testing.md)
- [開発](docs/development.md)
