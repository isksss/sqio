#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin="${SQIO_BIN:-/tmp/sqio}"

db_name="${SQIO_SMOKE_DB:-sqio}"
db_user="${SQIO_SMOKE_USER:-sqio}"
db_password="${SQIO_SMOKE_PASSWORD:-sqio}"
smoke_drivers="${SQIO_SMOKE_DRIVERS:-sqlite,duckdb,postgres,mysql,sqlserver,oracle,clickhouse,ssh}"
postgres_host="${SQIO_POSTGRES_HOST:-postgres}"
postgres_port="${SQIO_POSTGRES_PORT:-5432}"
mysql_host="${SQIO_MYSQL_HOST:-mysql}"
mysql_port="${SQIO_MYSQL_PORT:-3306}"
sqlserver_host="${SQIO_SQLSERVER_HOST:-sqlserver}"
sqlserver_port="${SQIO_SQLSERVER_PORT:-1433}"
oracle_host="${SQIO_ORACLE_HOST:-oracle}"
oracle_port="${SQIO_ORACLE_PORT:-1521}"
clickhouse_host="${SQIO_CLICKHOUSE_HOST:-clickhouse}"
clickhouse_port="${SQIO_CLICKHOUSE_PORT:-9000}"
ssh_host="${SQIO_SSH_HOST:-ssh}"
ssh_port="${SQIO_SSH_PORT:-2222}"
ssh_user="${SQIO_SSH_USER:-sqio}"
ssh_password="${SQIO_SSH_PASSWORD:-sqio-ssh}"
sqlserver_password="${SQIO_SQLSERVER_PASSWORD:-SqioStrong!2026}"
oracle_service="${SQIO_ORACLE_SERVICE:-FREEPDB1}"

postgres_dsn="postgres://${db_user}:${db_password}@${postgres_host}:${postgres_port}/${db_name}?sslmode=disable"
mysql_dsn="${db_user}:${db_password}@tcp(${mysql_host}:${mysql_port})/${db_name}?multiStatements=true&parseTime=true"
sqlserver_dsn="sqlserver://sa:${sqlserver_password}@${sqlserver_host}:${sqlserver_port}?database=master&encrypt=disable"
oracle_dsn="oracle://${db_user}:${db_password}@${oracle_host}:${oracle_port}/${oracle_service}"
clickhouse_dsn="clickhouse://${db_user}:${db_password}@${clickhouse_host}:${clickhouse_port}/${db_name}"

has_driver() {
	case ",${smoke_drivers}," in
	*",$1,"*) return 0 ;;
	*) return 1 ;;
	esac
}

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

if has_driver sqlite; then
	sqlite_db="$(mktemp -t sqio-sqlite-XXXXXX.db)"
	"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'create table roles (id integer primary key, name text not null unique); create table users (id integer primary key, name text not null default "sqlite", email text unique, role_id integer references roles(id)); insert into roles (name) values ("admin"); insert into users (email, role_id) values ("sqlite@example.com", 1);'
	"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'select name from users' --format json
	"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'select name from users' --format json --max-rows 1
	"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'select name from users' --format json --explain
	"$bin" tables --driver sqlite --database "$sqlite_db"
	"$bin" columns --driver sqlite --database "$sqlite_db" --table "$sqlite_users"
	"$bin" columns --driver sqlite --database "$sqlite_db" --table "$sqlite_users" | grep -qi 'email	text	nullable unique'
	"$bin" columns --driver sqlite --database "$sqlite_db" --table "$sqlite_users" | grep -qi 'role_id	integer	nullable references="roles"("id")'
fi

if has_driver duckdb; then
	duckdb_db="$(mktemp -u -t sqio-duckdb-XXXXXX.duckdb)"
	"$bin" exec --driver duckdb --database "$duckdb_db" --sql "create table ${duckdb_users} (id integer primary key, name varchar not null default 'duckdb', email varchar unique); insert into ${duckdb_users} (id, email) values (1, 'duckdb@example.com');"
	"$bin" exec --driver duckdb --database "$duckdb_db" --sql "select name from ${duckdb_users}" --format json
	"$bin" tables --driver duckdb --database "$duckdb_db"
	"$bin" columns --driver duckdb --database "$duckdb_db" --table "$duckdb_users"
	"$bin" columns --driver duckdb --database "$duckdb_db" --table "$duckdb_users" | grep -qi 'email	VARCHAR'
fi

if has_driver postgres; then
	"$bin" exec --driver postgres --dsn "$postgres_dsn" --sql "create table ${postgres_users} (id serial primary key, name text not null default 'postgres', email varchar(255) unique); insert into ${postgres_users} (email) values ('postgres@example.com');"
	"$bin" exec --driver postgres --dsn "$postgres_dsn" --sql "select name from ${postgres_users}" --format json
	"$bin" tables --driver postgres --dsn "$postgres_dsn"
	"$bin" columns --driver postgres --dsn "$postgres_dsn" --table "$postgres_users"
	"$bin" columns --driver postgres --dsn "$postgres_dsn" --table "$postgres_users" | grep -qi 'email	character varying(255)	nullable unique'
fi

if has_driver mysql; then
	"$bin" exec --driver mysql --dsn "$mysql_dsn" --sql "create table ${mysql_users} (id integer primary key auto_increment, name varchar(255) not null default 'mysql', email varchar(255) unique); insert into ${mysql_users} (email) values ('mysql@example.com');"
	"$bin" exec --driver mysql --dsn "$mysql_dsn" --sql "select name from ${mysql_users}" --format json
	"$bin" tables --driver mysql --dsn "$mysql_dsn"
	"$bin" columns --driver mysql --dsn "$mysql_dsn" --table "$mysql_users"
	"$bin" columns --driver mysql --dsn "$mysql_dsn" --table "$mysql_users" | grep -qi 'email	varchar(255)	nullable unique'
fi

if has_driver sqlserver; then
	"$bin" exec --driver sqlserver --dsn "$sqlserver_dsn" --sql "create table ${sqlserver_users} (id int identity(1,1) primary key, name nvarchar(255) not null default 'sqlserver', email nvarchar(255) unique); insert into ${sqlserver_users} (email) values ('sqlserver@example.com');"
	"$bin" exec --driver sqlserver --dsn "$sqlserver_dsn" --sql "select name from ${sqlserver_users}" --format json
	"$bin" tables --driver sqlserver --dsn "$sqlserver_dsn"
	"$bin" columns --driver sqlserver --dsn "$sqlserver_dsn" --table "$sqlserver_users"
	"$bin" columns --driver sqlserver --dsn "$sqlserver_dsn" --table "$sqlserver_users" | grep -qi 'email	nvarchar(255)'
fi

if has_driver oracle; then
	"$bin" exec --driver oracle --dsn "$oracle_dsn" --sql "create table ${oracle_users} (id number generated by default as identity primary key, name varchar2(255) default 'oracle' not null, email varchar2(255) unique); insert into ${oracle_users} (email) values ('oracle@example.com');"
	"$bin" exec --driver oracle --dsn "$oracle_dsn" --sql "select name from ${oracle_users}" --format json
	"$bin" tables --driver oracle --dsn "$oracle_dsn"
	"$bin" columns --driver oracle --dsn "$oracle_dsn" --table "$oracle_users"
	"$bin" columns --driver oracle --dsn "$oracle_dsn" --table "$oracle_users" | grep -qi 'EMAIL	VARCHAR2'
fi

if has_driver clickhouse; then
	"$bin" exec --driver clickhouse --dsn "$clickhouse_dsn" --sql "create table ${clickhouse_users} (id UInt64, name String default 'clickhouse', email Nullable(String)) engine = MergeTree order by id; insert into ${clickhouse_users} (id, email) values (1, 'clickhouse@example.com');"
	"$bin" exec --driver clickhouse --dsn "$clickhouse_dsn" --sql "select name from ${clickhouse_users}" --format json
	"$bin" tables --driver clickhouse --dsn "$clickhouse_dsn"
	"$bin" columns --driver clickhouse --dsn "$clickhouse_dsn" --table "$clickhouse_users"
	"$bin" columns --driver clickhouse --dsn "$clickhouse_dsn" --table "$clickhouse_users" | grep -qi 'email	Nullable(String)'
fi

if has_driver ssh; then
	known_hosts="$(mktemp -t sqio-known-hosts-XXXXXX)"
	ssh-keyscan -p "$ssh_port" "$ssh_host" >"$known_hosts" 2>/dev/null
	"$bin" exec \
		--driver postgres \
		--host "$postgres_host" \
		--port "$postgres_port" \
		--database "$db_name" \
		--user "$db_user" \
		--password "$db_password" \
		--ssh-tunnel \
		--ssh-host "$ssh_host" \
		--ssh-port "$ssh_port" \
		--ssh-user "$ssh_user" \
		--ssh-password "$ssh_password" \
		--ssh-known-hosts "$known_hosts" \
		--sql "create table ${ssh_users} (id serial primary key, name text not null default 'ssh', email varchar(255) unique); insert into ${ssh_users} (email) values ('ssh@example.com');"
	"$bin" exec \
		--driver postgres \
		--host "$postgres_host" \
		--port "$postgres_port" \
		--database "$db_name" \
		--user "$db_user" \
		--password "$db_password" \
		--ssh-tunnel \
		--ssh-host "$ssh_host" \
		--ssh-port "$ssh_port" \
		--ssh-user "$ssh_user" \
		--ssh-password "$ssh_password" \
		--ssh-known-hosts "$known_hosts" \
		--sql "select name from ${ssh_users}" \
		--format json
fi
