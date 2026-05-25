#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin="${SQIO_BIN:-/tmp/sqio}"
export SQIO_HISTORY_PATH="${SQIO_HISTORY_PATH:-$(mktemp -t sqio-history-XXXXXX.db)}"

go build -o "$bin" "$root/cmd/sqio"

sqlite_db="$(mktemp -t sqio-sqlite-XXXXXX.db)"
"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'create table users (id integer primary key, name text not null); insert into users (name) values ("sqlite");'
"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'select name from users' --format json
"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'select name from users' --format json --max-rows 1
"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'select name from users' --format json --explain
"$bin" tables --driver sqlite --database "$sqlite_db"
"$bin" columns --driver sqlite --database "$sqlite_db" --table users

"$bin" exec --driver postgres --dsn 'postgres://sqio:sqio@localhost:15432/sqio?sslmode=disable' --sql 'drop table if exists users; create table users (id serial primary key, name text not null); insert into users (name) values ($$postgres$$);'
"$bin" exec --driver postgres --dsn 'postgres://sqio:sqio@localhost:15432/sqio?sslmode=disable' --sql 'select name from users' --format json
"$bin" tables --driver postgres --dsn 'postgres://sqio:sqio@localhost:15432/sqio?sslmode=disable'
"$bin" columns --driver postgres --dsn 'postgres://sqio:sqio@localhost:15432/sqio?sslmode=disable' --table users

"$bin" exec --driver mysql --dsn 'sqio:sqio@tcp(127.0.0.1:13306)/sqio?multiStatements=true&parseTime=true' --sql 'drop table if exists users; create table users (id integer primary key auto_increment, name text not null); insert into users (name) values ("mysql");'
"$bin" exec --driver mysql --dsn 'sqio:sqio@tcp(127.0.0.1:13306)/sqio?multiStatements=true&parseTime=true' --sql 'select name from users' --format json
"$bin" tables --driver mysql --dsn 'sqio:sqio@tcp(127.0.0.1:13306)/sqio?multiStatements=true&parseTime=true'
"$bin" columns --driver mysql --dsn 'sqio:sqio@tcp(127.0.0.1:13306)/sqio?multiStatements=true&parseTime=true' --table users
