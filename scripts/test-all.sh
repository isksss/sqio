#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

docker compose --profile test build sqio-test
docker compose up -d postgres mysql sqlserver oracle clickhouse ssh

cleanup() {
  docker compose down
}
trap cleanup EXIT

docker compose --profile test run --rm sqio-test bash scripts/test-all-in-docker.sh
