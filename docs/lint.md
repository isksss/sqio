# Lint ルール

組み込みルールは、危険な書き込み (`delete-without-where`、
`update-without-where`、`truncate`、`drop-database`)、クエリ性能
(`select-star`、`leading-wildcard-like`、`or-abuse`、`implicit-join`、
`cartesian-join`、`limit-without-order`)、正確性 (`not-in-null`) を対象にします。

`dialect` または `lint --dialect` 指定時は PostgreSQL/MySQL/SQLite の明確な
非互換構文も検出します。

`keyword-case` は `--enable keyword-case` で有効化する opt-in ルールです。
