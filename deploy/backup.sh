#!/bin/sh
set -eu
umask 077
interval=${BACKUP_INTERVAL_SECONDS:-86400}
retention=${BACKUP_KEEP_DAYS:-30}
case "$interval:$retention" in *[!0-9:]*|:*|*:) echo 'Invalid backup interval or retention' >&2; exit 1;; esac
[ "$interval" -ge 60 ] && [ "$retention" -ge 1 ]
mkdir -p /backups
while :; do
  stamp=$(date -u +%Y%m%dT%H%M%SZ)
  # Independent pg_dump snapshots include all worlds, revisions, CRDT updates,
  # Library/terrain blobs, permissions and identity-provider data.
  if PGPASSWORD="$POSTGRES_PASSWORD" pg_dump -h postgres -U kriemhild -d kriemhild -Fc -f "/backups/app-$stamp.partial" &&
     PGPASSWORD="$KEYCLOAK_DB_PASSWORD" pg_dump -h keycloak-db -U keycloak -d keycloak -Fc -f "/backups/identity-$stamp.partial"; then
    mv "/backups/app-$stamp.partial" "/backups/app-$stamp.dump"
    mv "/backups/identity-$stamp.partial" "/backups/identity-$stamp.dump"
    touch /backups/last-success
    # Delete only completed files created by this job in the dedicated mount.
    find /backups -maxdepth 1 -type f \( -name 'app-*.dump' -o -name 'identity-*.dump' \) -mtime +"$retention" -exec rm -f {} \;
    echo "database_backups_complete timestamp=$stamp"
  else
    echo "database_backup_failed timestamp=$stamp" >&2
    exit 1
  fi
  [ "${BACKUP_ONCE:-0}" = 1 ] && exit 0
  sleep "$interval"
done
