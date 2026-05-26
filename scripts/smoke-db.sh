#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin="${SQIO_BIN:-/tmp/sqio}"
export SQIO_HISTORY_PATH="${SQIO_HISTORY_PATH:-$(mktemp -t sqio-history-XXXXXX.db)}"

go build -o "$bin" "$root/cmd/sqio"

sqlite_db="$(mktemp -t sqio-sqlite-XXXXXX.db)"
"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'create table roles (id integer primary key, name text not null unique); create table users (id integer primary key, name text not null default "sqlite", email text unique, role_id integer references roles(id)); insert into roles (name) values ("admin"); insert into users (email, role_id) values ("sqlite@example.com", 1);'
"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'select name from users' --format json
"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'select name from users' --format json --max-rows 1
"$bin" exec --driver sqlite --database "$sqlite_db" --sql 'select name from users' --format json --explain
"$bin" tables --driver sqlite --database "$sqlite_db"
"$bin" columns --driver sqlite --database "$sqlite_db" --table users
"$bin" columns --driver sqlite --database "$sqlite_db" --table users | grep -qi 'email	text	nullable unique'
"$bin" columns --driver sqlite --database "$sqlite_db" --table users | grep -qi 'role_id	integer	nullable references="roles"("id")'

"$bin" exec --driver postgres --dsn 'postgres://sqio:sqio@localhost:15432/sqio?sslmode=disable' --sql 'drop table if exists users; drop table if exists roles; create table roles (id serial primary key, name text not null unique); create table users (id serial primary key, name text not null default $$postgres$$, email varchar(255) unique, role_id integer references roles(id)); insert into roles (name) values ($$admin$$); insert into users (email, role_id) values ($$postgres@example.com$$, 1);'
"$bin" exec --driver postgres --dsn 'postgres://sqio:sqio@localhost:15432/sqio?sslmode=disable' --sql 'select name from users' --format json
"$bin" tables --driver postgres --dsn 'postgres://sqio:sqio@localhost:15432/sqio?sslmode=disable'
"$bin" columns --driver postgres --dsn 'postgres://sqio:sqio@localhost:15432/sqio?sslmode=disable' --table users
"$bin" columns --driver postgres --dsn 'postgres://sqio:sqio@localhost:15432/sqio?sslmode=disable' --table users | grep -qi 'email	character varying(255)	nullable unique'
"$bin" columns --driver postgres --dsn 'postgres://sqio:sqio@localhost:15432/sqio?sslmode=disable' --table users | grep -qi 'role_id	integer	nullable references=public.roles(id)'

"$bin" exec --driver mysql --dsn 'sqio:sqio@tcp(127.0.0.1:13306)/sqio?multiStatements=true&parseTime=true' --sql 'drop table if exists users; drop table if exists roles; create table roles (id integer primary key auto_increment, name varchar(255) not null unique); create table users (id integer primary key auto_increment, name varchar(255) not null default "mysql", email varchar(255) unique, role_id integer, constraint fk_users_role foreign key (role_id) references roles(id)); insert into roles (name) values ("admin"); insert into users (email, role_id) values ("mysql@example.com", 1);'
"$bin" exec --driver mysql --dsn 'sqio:sqio@tcp(127.0.0.1:13306)/sqio?multiStatements=true&parseTime=true' --sql 'select name from users' --format json
"$bin" tables --driver mysql --dsn 'sqio:sqio@tcp(127.0.0.1:13306)/sqio?multiStatements=true&parseTime=true'
"$bin" columns --driver mysql --dsn 'sqio:sqio@tcp(127.0.0.1:13306)/sqio?multiStatements=true&parseTime=true' --table users
"$bin" columns --driver mysql --dsn 'sqio:sqio@tcp(127.0.0.1:13306)/sqio?multiStatements=true&parseTime=true' --table users | grep -qi 'email	varchar(255)	nullable unique'
"$bin" columns --driver mysql --dsn 'sqio:sqio@tcp(127.0.0.1:13306)/sqio?multiStatements=true&parseTime=true' --table users | grep -qi 'role_id	int	nullable references=roles(id)'
