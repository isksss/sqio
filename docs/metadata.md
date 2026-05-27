# メタデータ

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
