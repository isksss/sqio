#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

gofmt -w cmd internal
go test ./...
go build -o /tmp/sqio ./cmd/sqio
markdownlint-cli2 README.md docs/*.md

docker compose up -d postgres mysql
cleanup() {
  docker compose down
}
trap cleanup EXIT

for _ in $(seq 1 60); do
  pg="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' sqio-postgres-1)"
  my="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' sqio-mysql-1)"
  if [[ "$pg" == "healthy" && "$my" == "healthy" ]]; then
    bash scripts/smoke-db.sh
    exit 0
  fi
  sleep 2
done

docker compose ps
exit 1
