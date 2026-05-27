# 実行と出力

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
