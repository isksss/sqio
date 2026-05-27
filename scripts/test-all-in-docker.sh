#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

wait_health() {
  local name="$1"
  for _ in $(seq 1 120); do
    status="$(docker_status "$name")"
    if [[ "$status" == "healthy" ]]; then
      return 0
    fi
    sleep 2
  done
  printf 'service did not become healthy: %s\n' "$name" >&2
  return 1
}

docker_status() {
  local host="$1"
  case "$host" in
    postgres) nc -z postgres 5432 >/dev/null 2>&1 && printf 'healthy' || printf 'starting' ;;
    mysql) nc -z mysql 3306 >/dev/null 2>&1 && printf 'healthy' || printf 'starting' ;;
    sqlserver) nc -z sqlserver 1433 >/dev/null 2>&1 && printf 'healthy' || printf 'starting' ;;
    oracle) nc -z oracle 1521 >/dev/null 2>&1 && printf 'healthy' || printf 'starting' ;;
    clickhouse) nc -z clickhouse 9000 >/dev/null 2>&1 && printf 'healthy' || printf 'starting' ;;
    ssh) ssh-keyscan -p 2222 ssh >/dev/null 2>&1 && printf 'healthy' || printf 'starting' ;;
    *) printf 'unknown' ;;
  esac
}

for service in postgres mysql sqlserver oracle clickhouse ssh; do
  wait_health "$service"
done

unformatted="$(gofmt -l cmd internal)"
if [[ -n "$unformatted" ]]; then
  printf 'gofmt required:\n%s\n' "$unformatted" >&2
  exit 1
fi

go test ./...
go build -buildvcs=false -o /tmp/sqio ./cmd/sqio
markdownlint-cli2 README.md README.en.md docs/*.md
bash scripts/smoke-db.sh
