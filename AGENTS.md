# AGENTS.md

## プロジェクト概要

`sqio` は Go 製の TUI/CLI 統合データベースクライアントです。
MySQL、PostgreSQL、SQLite を対象に、SQL 実行、対話的クエリ入力、
ストリーミング出力、SQL format/lint、メタデータ取得、ER 図出力、
履歴管理を提供します。

CLI が主要インターフェースで、TUI は同じ service 層を利用する
フロントエンドとして扱います。

## 構成

```text
cmd/sqio/              CLI entrypoint
internal/cli/          Cobra command definitions and CLI option handling
internal/config/       Config loading and merge rules
internal/db/           Database connections, DSN handling, metadata
internal/editor/       External editor integration
internal/formatter/    SQL formatter
internal/history/      Query history storage
internal/linter/       SQL lint rules
internal/output/       Table/JSON/YAML output
internal/picker/       fzf/internal picker
internal/query/        Query input and safety helpers
internal/secret/       age-based secret handling
internal/service/      Shared application service layer
internal/tui/          Bubble Tea TUI model
internal/tunnel/       SSH tunnel support
scripts/               CI and smoke test scripts
```

## 開発方針

- 変更は最小化し、既存の package 境界を尊重する。
- CLI から使う業務ロジックは可能な限り `internal/service` や下位 package に置き、
  `internal/cli` は option 解析と入出力の接続に寄せる。
- TUI 専用の状態・表示は `internal/tui` に閉じ込める。
- DB driver 固有処理は `internal/db` に集約する。
- 既存の標準 library と導入済み依存を優先し、依存追加は必要最小限にする。
- 秘密情報、DSN、`.env`、credential を出力しない。
- SSH tunnel は `known_hosts` による host key verification を前提にする。
- TUI の password 入力は必ずマスク表示する。
- DB 接続ありの `exec` / `query` は、可能な限り結果行を逐次出力し、
  大きな結果セットを全件メモリ保持しない。`--transaction` 時は commit 前出力を避ける。
- MySQL DSN は `go-sql-driver/mysql` の `Config.FormatDSN()` など
  driver 公式の組み立て API を優先する。

## よく使うコマンド

```bash
go test ./...
go test ./... -covermode=atomic -coverprofile=/tmp/sqio-cover.out
go build -o /tmp/sqio ./cmd/sqio
gofmt -w cmd internal
bash scripts/ci-check.sh
```

DB smoke test:

```bash
docker compose up -d postgres mysql
bash scripts/smoke-db.sh
docker compose down
```

全体確認:

```bash
bash scripts/test-all.sh
```

`scripts/test-all.sh` は `gofmt`、unit test、build、README の
markdownlint、PostgreSQL/MySQL smoke test を実行します。

## 検証ルール

- Go 変更時は最低限 `go test ./...` を実行する。
- coverage を確認する場合は
  `go test ./... -covermode=atomic -coverprofile=/tmp/sqio-cover.out` を使う。
- CLI や build に関わる変更時は `go build -o /tmp/sqio ./cmd/sqio` も実行する。
- README 変更時は `markdownlint-cli2 README.md README.en.md` または
  `bash scripts/ci-check.sh` で確認する。
- PostgreSQL/MySQL 連携に触れた場合は `scripts/smoke-db.sh` を実行する。
- 実行できない検証がある場合は、理由と未検証範囲を明記する。

## 注意点

- ユーザー指示なしに commit しない。
- `sqio.toml`、`.env`、ローカル DB、履歴 DB などの個人環境ファイルを
  変更・出力しない。
- `docker compose down` はこのリポジトリの smoke test 用サービスに限定して扱う。
- 大規模リファクタや package 再編は、明示依頼がある場合のみ行う。
