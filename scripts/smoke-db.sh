#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin="${SQIO_BIN:-/tmp/sqio}"

db_name="${SQIO_SMOKE_DB:-sqio}"
db_user="${SQIO_SMOKE_USER:-sqio}"
db_password="${SQIO_SMOKE_PASSWORD:-sqio}"
postgres_host="${SQIO_POSTGRES_HOST:-postgres}"
mysql_host="${SQIO_MYSQL_HOST:-mysql}"
sqlserver_host="${SQIO_SQLSERVER_HOST:-sqlserver}"
oracle_host="${SQIO_ORACLE_HOST:-oracle}"
clickhouse_host="${SQIO_CLICKHOUSE_HOST:-clickhouse}"
ssh_host="${SQIO_SSH_HOST:-ssh}"
ssh_user="${SQIO_SSH_USER:-sqio}"
ssh_password="${SQIO_SSH_PASSWORD:-sqio-ssh}"
sqlserver_password="${SQIO_SQLSERVER_PASSWORD:-SqioStrong!2026}"
oracle_service="${SQIO_ORACLE_SERVICE:-FREEPDB1}"

postgres_dsn="postgres://${db_user}:${db_password}@${postgres_host}:5432/${db_name}?sslmode=disable"
mysql_dsn="${db_user}:${db_password}@tcp(${mysql_host}:3306)/${db_name}?multiStatements=true&parseTime=true"
sqlserver_dsn="sqlserver://sa:${sqlserver_password}@${sqlserver_host}:1433?database=master&encrypt=disable"
oracle_dsn="oracle://${db_user}:${db_password}@${oracle_host}:1521/${oracle_service}"
clickhouse_dsn="clickhouse://${db_user}:${db_password}@${clickhouse_host}:9000/${db_name}"

export SQIO_HISTORY_PATH="${SQIO_HISTORY_PATH:-$(mktemp -t sqio-history-XXXXXX.db)}"

go build -buildvcs=false -o "$bin" "$root/cmd/sqio"

exec </dev/null

suffix="$(date +%s%N)"
sqlite_users="users"
duckdb_users="duck_users_${suffix}"
postgres_users="pg_users_${suffix}"
mysql_users="my_users_${suffix}"
sqlserver_users="ms_users_${suffix}"
oracle_users="ORA_USERS_${suffix}"
clickhouse_users="ch_users_${suffix}"
ssh_users="ssh_users_${suffix}"

sqlite_db="$(mktemp -t sqio-sqlite-XXXXXX.db)"
"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'create table roles (id integer primary key, name text not null unique); create table users (id integer primary key, name text not null default "sqlite", email text unique, role_id integer references roles(id)); insert into roles (name) values ("admin"); insert into users (email, role_id) values ("sqlite@example.com", 1);'
"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'select name from users' --format json
"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'select name from users' --format json --max-rows 1
"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'select name from users' --format json --explain
"$bin" tables --driver sqlite --database "$sqlite_db"
"$bin" columns --driver sqlite --database "$sqlite_db" --table "$sqlite_users"
"$bin" columns --driver sqlite --database "$sqlite_db" --table "$sqlite_users" | grep -qi 'email	text	nullable unique'
"$bin" columns --driver sqlite --database "$sqlite_db" --table "$sqlite_users" | grep -qi 'role_id	integer	nullable references="roles"("id")'

duckdb_db="$(mktemp -u -t sqio-duckdb-XXXXXX.duckdb)"
"$bin" exec --driver duckdb --database "$duckdb_db" --sql "create table ${duckdb_users} (id integer primary key, name varchar not null default 'duckdb', email varchar unique); insert into ${duckdb_users} (id, email) values (1, 'duckdb@example.com');"
"$bin" exec --driver duckdb --database "$duckdb_db" --sql "select name from ${duckdb_users}" --format json
"$bin" tables --driver duckdb --database "$duckdb_db"
"$bin" columns --driver duckdb --database "$duckdb_db" --table "$duckdb_users"
"$bin" columns --driver duckdb --database "$duckdb_db" --table "$duckdb_users" | grep -qi 'email	VARCHAR'

"$bin" exec --driver postgres --dsn "$postgres_dsn" --sql "create table ${postgres_users} (id serial primary key, name text not null default 'postgres', email varchar(255) unique); insert into ${postgres_users} (email) values ('postgres@example.com');"
"$bin" exec --driver postgres --dsn "$postgres_dsn" --sql "select name from ${postgres_users}" --format json
"$bin" tables --driver postgres --dsn "$postgres_dsn"
"$bin" columns --driver postgres --dsn "$postgres_dsn" --table "$postgres_users"
"$bin" columns --driver postgres --dsn "$postgres_dsn" --table "$postgres_users" | grep -qi 'email	character varying(255)	nullable unique'

"$bin" exec --driver mysql --dsn "$mysql_dsn" --sql "create table ${mysql_users} (id integer primary key auto_increment, name varchar(255) not null default 'mysql', email varchar(255) unique); insert into ${mysql_users} (email) values ('mysql@example.com');"
"$bin" exec --driver mysql --dsn "$mysql_dsn" --sql "select name from ${mysql_users}" --format json
"$bin" tables --driver mysql --dsn "$mysql_dsn"
"$bin" columns --driver mysql --dsn "$mysql_dsn" --table "$mysql_users"
"$bin" columns --driver mysql --dsn "$mysql_dsn" --table "$mysql_users" | grep -qi 'email	varchar(255)	nullable unique'

"$bin" exec --driver sqlserver --dsn "$sqlserver_dsn" --sql "create table ${sqlserver_users} (id int identity(1,1) primary key, name nvarchar(255) not null default 'sqlserver', email nvarchar(255) unique); insert into ${sqlserver_users} (email) values ('sqlserver@example.com');"
"$bin" exec --driver sqlserver --dsn "$sqlserver_dsn" --sql "select name from ${sqlserver_users}" --format json
"$bin" tables --driver sqlserver --dsn "$sqlserver_dsn"
"$bin" columns --driver sqlserver --dsn "$sqlserver_dsn" --table "$sqlserver_users"
"$bin" columns --driver sqlserver --dsn "$sqlserver_dsn" --table "$sqlserver_users" | grep -qi 'email	nvarchar(255)'

"$bin" exec --driver oracle --dsn "$oracle_dsn" --sql "create table ${oracle_users} (id number generated by default as identity primary key, name varchar2(255) default 'oracle' not null, email varchar2(255) unique); insert into ${oracle_users} (email) values ('oracle@example.com');"
"$bin" exec --driver oracle --dsn "$oracle_dsn" --sql "select name from ${oracle_users}" --format json
"$bin" tables --driver oracle --dsn "$oracle_dsn"
"$bin" columns --driver oracle --dsn "$oracle_dsn" --table "$oracle_users"
"$bin" columns --driver oracle --dsn "$oracle_dsn" --table "$oracle_users" | grep -qi 'EMAIL	VARCHAR2'

"$bin" exec --driver clickhouse --dsn "$clickhouse_dsn" --sql "create table ${clickhouse_users} (id UInt64, name String default 'clickhouse', email Nullable(String)) engine = MergeTree order by id; insert into ${clickhouse_users} (id, email) values (1, 'clickhouse@example.com');"
"$bin" exec --driver clickhouse --dsn "$clickhouse_dsn" --sql "select name from ${clickhouse_users}" --format json
"$bin" tables --driver clickhouse --dsn "$clickhouse_dsn"
"$bin" columns --driver clickhouse --dsn "$clickhouse_dsn" --table "$clickhouse_users"
"$bin" columns --driver clickhouse --dsn "$clickhouse_dsn" --table "$clickhouse_users" | grep -qi 'email	Nullable(String)'

known_hosts="$(mktemp -t sqio-known-hosts-XXXXXX)"
ssh-keyscan -p 2222 "$ssh_host" >"$known_hosts" 2>/dev/null
"$bin" exec \
  --driver postgres \
  --host "$postgres_host" \
  --port 5432 \
  --database "$db_name" \
  --user "$db_user" \
  --password "$db_password" \
  --ssh-tunnel \
  --ssh-host "$ssh_host" \
  --ssh-port 2222 \
  --ssh-user "$ssh_user" \
  --ssh-password "$ssh_password" \
  --ssh-known-hosts "$known_hosts" \
  --sql "create table ${ssh_users} (id serial primary key, name text not null default 'ssh', email varchar(255) unique); insert into ${ssh_users} (email) values ('ssh@example.com');"
"$bin" exec \
  --driver postgres \
  --host "$postgres_host" \
  --port 5432 \
  --database "$db_name" \
  --user "$db_user" \
  --password "$db_password" \
  --ssh-tunnel \
  --ssh-host "$ssh_host" \
  --ssh-port 2222 \
  --ssh-user "$ssh_user" \
  --ssh-password "$ssh_password" \
  --ssh-known-hosts "$known_hosts" \
  --sql "select name from ${ssh_users}" \
  --format json
