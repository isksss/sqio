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

通常の確認や Docker を使った実 DB smoke test は [テスト](testing.md) を参照してください。

CI と同等の軽量チェック:

```bash
bash scripts/ci-check.sh
```
