#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  printf 'Usage: %s /absolute/offsite-staging-directory\n' "$0" >&2
  exit 2
fi

cd "$(dirname "$0")/.."
backup_destination=$1
mkdir -p "$backup_destination"
backup_destination=$(realpath "$backup_destination")
umask 077

compose=(docker compose)
if [[ -n "${LINKPAY_COMPOSE_PROJECT:-}" ]]; then
  compose+=(-p "$LINKPAY_COMPOSE_PROJECT")
fi
if [[ -n "${LINKPAY_ENV_FILE:-}" ]]; then
  compose+=(--env-file "$LINKPAY_ENV_FILE")
fi

database_temp=$(mktemp "$backup_destination/.linkpay-database.XXXXXX")
media_temp=$(mktemp "$backup_destination/.linkpay-media.XXXXXX")
trap 'rm -f "$database_temp" "$media_temp"' EXIT

"${compose[@]}" exec -T db sh -c 'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$database_temp"
"${compose[@]}" exec -T db pg_restore -l < "$database_temp" > /dev/null
"${compose[@]}" exec -T api tar -C /app/media -cf - . > "$media_temp"
tar -tf "$media_temp" > /dev/null

timestamp=$(date -u +%Y%m%dT%H%M%SZ)
mv "$database_temp" "$backup_destination/linkpay-database-$timestamp.dump"
mv "$media_temp" "$backup_destination/linkpay-media-$timestamp.tar"
printf 'Created database and media backups in %s\n' "$backup_destination"
