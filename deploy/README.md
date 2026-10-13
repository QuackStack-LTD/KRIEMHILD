# Self-hosted accounts and collaboration

This stack builds the local application and runs one Go server, Keycloak 26.8.0,
two private PostgreSQL databases, Caddy HTTPS and a daily database backup job.
Only Caddy publishes network ports. The app has no anonymous hosted mode.

## Configure and start

1. Point two DNS names at the server; allow inbound TCP 80/443 and optionally UDP
   443. Install Docker with Compose v2.
2. Copy `deploy/.env.example` to `deploy/.env`. Supply the DNS names, independent
   random URL-safe secrets, certificate contact and working SMTP credentials.
   Protect this file and exclude it from source control.
3. From the repository root run:

   ```sh
   docker compose --env-file deploy/.env -f deploy/compose.yaml config --quiet
   docker compose --env-file deploy/.env -f deploy/compose.yaml up -d --build
   docker compose --env-file deploy/.env -f deploy/compose.yaml ps
   ```

4. Open the application HTTPS URL. Register through Keycloak, verify email, then
   create a world. Invite a second verified account through Share and test roles.
   Invitations appear in the recipient's app; KRIEMHILD does not email invitation
   tokens. SMTP is used by Keycloak for verification and password recovery.
5. Sign into Keycloak administration at the identity host, replace the bootstrap
   administrator with a permanent administrator, and enroll MFA. Configure any
   social login providers there; they require their own client credentials.

The realm import sets exact callback/logout URLs, confidential client credentials,
PKCE S256, verified email, password recovery, registration and brute-force
protection. Keycloak imports it only when the realm is first created. Subsequent
SMTP/client-secret changes must also be made in the existing realm through its
administration UI; editing `.env` does not overwrite an existing realm. Caddy
obtains/renews publicly trusted certificates and forwards WebSocket upgrades.

The migration service runs before the application. Take backups before upgrades;
pin reviewed image digests in a production release. The root `docker-compose.yml`
is the alternative for an already managed Keycloak/reverse proxy.

## Backup and recovery

`backups` writes custom-format PostgreSQL dumps to `deploy/backups` immediately
and every 24 hours, retaining completed dumps for 30 days. Set the two BACKUP
variables to change the interval and retention. Failed dumps leave `.partial`
files and cause a visible container failure/restart; only successful dumps update
`last-success`. Monitor that timestamp and copy encrypted backups off the host.
Do not expose this directory through a web server. Application dumps include all
worlds, terrain/Library blobs, writer versions, CRDT logs, comments and access
records. Identity dumps include accounts, realm configuration and credentials.
Save `.env` securely and retain Caddy volumes or reissue certificates on recovery.

Each dump is transactionally consistent within its database. The pair is not a
distributed transaction. Stop application and identity writes when a coordinated
disaster-recovery checkpoint is required. World ZIP exports do not replace these
database backups.

Restore into **fresh databases/volumes** with the same PostgreSQL major version.
Keep Go and Keycloak stopped while restoring. For example, with a separate
recovery Compose project and environment:

```sh
docker compose --env-file deploy/recovery.env -p kriemhild-recovery -f deploy/compose.yaml up -d postgres keycloak-db
docker compose --env-file deploy/recovery.env -p kriemhild-recovery -f deploy/compose.yaml cp deploy/backups/app-TIMESTAMP.dump postgres:/tmp/app.dump
docker compose --env-file deploy/recovery.env -p kriemhild-recovery -f deploy/compose.yaml exec -T postgres pg_restore --exit-on-error --no-owner -U kriemhild -d kriemhild /tmp/app.dump
docker compose --env-file deploy/recovery.env -p kriemhild-recovery -f deploy/compose.yaml cp deploy/backups/identity-TIMESTAMP.dump keycloak-db:/tmp/identity.dump
docker compose --env-file deploy/recovery.env -p kriemhild-recovery -f deploy/compose.yaml exec -T keycloak-db pg_restore --exit-on-error --no-owner -U keycloak -d keycloak /tmp/identity.dump
```

Do not start a second proxy on the same host ports. Validate the restored database
in isolation or on a separate recovery host, then bring the recovery stack up.
Use the original issuer hostname during final cutover: changing the issuer
changes identity mappings. After restoration revoke all application sessions
before allowing clients back in, since restored sessions may predate a revocation:

```sh
docker compose --env-file deploy/recovery.env -p kriemhild-recovery -f deploy/compose.yaml exec -T postgres psql -U kriemhild -d kriemhild -c 'DELETE FROM app_sessions;'
```

Verify world counts, world history, named manuscript versions, merged live text,
invitations and member roles with two accounts before cutover. A dump that merely
parses is not a verified restore. Never use `down -v` on the source installation.

## Operational checks

- Monitor container health, failed backups and `last-success` age, database disk
  space, and `request_rejected` logs (especially 401/403/409/429/503).
- Keycloak health/metrics remain on private port 9000. The public app's readiness
  route exposes only readiness, without world or account information.
- API limits and Keycloak brute-force protection are enabled. Add per-IP edge
  controls if the shared proxy login budget is insufficient; Go intentionally
  ignores untrusted forwarding headers.
- Run one Go server per database. Its PostgreSQL advisory lease rejects a second
  writer. Distributed map synchronization is not implemented.
- Check upgrades, email delivery, real OIDC redirects and PostgreSQL restoration
  on the target server. Local tests alone do not certify the hosted deployment.

Configuration follows the official [Keycloak container guide](https://www.keycloak.org/server/containers),
[realm import guide](https://www.keycloak.org/server/importExport),
[health checks](https://www.keycloak.org/observability/health), and
[Caddy reverse proxy documentation](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy).
