# Accounts and collaborative worlds

The implementation follows the identity-provider / native-Go split in
`KRIEMHILD_User_Management_Authentication_Collaboration_Guide.md`.
Keycloak handles passwords, registration, verification, recovery and optional
MFA. Go owns application sessions, world ownership, invitations, permissions,
history, and collaborative persistence. No provider tokens enter browser storage.

## Running locally

`npm start` uses an explicit local identity and personal workspace on loopback.
Existing historical worlds and standalone terrain projects are assigned to that
workspace without rewriting world IDs or payloads. This mode refuses wildcard
and non-loopback listen addresses. Local scripts may omit map revision headers;
the browser sends them, and hosted mode requires them.

Unassigned older writing projects appear under **Your account → Unassigned legacy
drafts**. Choose their world explicitly. Text and named versions are preserved.

## Hosted configuration

For the complete self-hosted stack, see [deploy/README.md](../deploy/README.md).
It includes Keycloak, separate identity/application PostgreSQL databases, Caddy
HTTPS, controlled migration, and daily backups. The root Compose file remains
available for an externally managed identity provider and reverse proxy.

Configure a confidential OIDC client in one Keycloak realm:

- Standard authorization code flow enabled, PKCE S256 required.
- Redirect URI: `https://your-app.example/auth/callback`.
- Post-logout redirect URI: `https://your-app.example/`.
- Enable registration, email verification, recovery and MFA according to your
  identity policy. Invitations require a verified email claim.
- Disable direct access grants if they are not otherwise required.

Set `KRIEMHILD_AUTH_MODE=oidc`, `KRIEMHILD_PUBLIC_URL`,
`KRIEMHILD_OIDC_ISSUER`, `KRIEMHILD_OIDC_CLIENT_ID` and
`KRIEMHILD_OIDC_CLIENT_SECRET`. Use PostgreSQL through `DATABASE_URL` or
`DATABASE_URL_FILE`. `.env.example` and Compose show the configuration.
Issuer and public origin must use HTTPS. An operator must provide the running
identity provider, certificates, DNS and HTTPS reverse proxy; the application
does not provision these services.

The proxy must forward WebSocket upgrades and preserve the external Host and
Origin. The API limits hosted login flows, invitations, imports, generation and
collaboration traffic; add per-client edge limits for larger deployments. Health/readiness
remain unauthenticated and contain no private-world metadata. Vite forwards
both API WebSockets and authentication routes for local development.

Run **one Go server per database**. A PostgreSQL advisory lease rejects a second
hosted server because map caches are process-local. Distributed map editing and
horizontal scaling require a separate synchronization design. SQLite remains
supported for local installations and tests; PostgreSQL is the hosted default.

## Sessions and access

OIDC callbacks validate signed tokens, issuer, audience, expiry, state, nonce and
PKCE. Provider issuer plus immutable subject identifies the user. Application
sessions last 12 hours, are stored as token hashes, and use a Secure, HttpOnly,
SameSite=Lax `__Host-` cookie. Mutations require the session's CSRF token.
Logout revokes the local session and returns the provider logout URL.

Each world has one owner and one owning personal workspace. World roles are
owner, administrator, editor, commenter and viewer. Administrators cannot
delete worlds, change owners, or create/promote another administrator. Ownership
transfer moves the world to the recipient's personal workspace. World membership
includes Ages, maps, archives, files and manuscripts. Team workspaces and
document-specific sharing are intentionally outside the initial model.

The API filters lists and validates direct resource IDs, writer parent worlds,
import destinations, map sessions and WebSocket rooms. Unknown/private resources
return 404; absent sessions return 401; insufficient role/CSRF returns 403.
Legacy unscoped draft APIs are inaccessible in hosted mode.

Invitation tokens are random and stored only as hashes. They expire after seven
days and require the intended verified recipient. The app shows pending
invitations and also accepts a pasted token. It does not send email. Membership,
invitation and ownership changes produce audit records, visible to owners and
administrators under History. Share lists pending invitations and supports
cancellation; recipients can decline. Membership is checked on
every request and collaboration message; removal closes active world sockets.
Idle sockets recheck sessions and membership every two seconds.

## Migration and retention

The additive `access_schema` migrations add identity, workspace, membership,
session, invitation, revision, comment and CRDT-update tables with foreign keys
and indexes. Version 2 adds account closure and per-user recent worlds.
Existing world and writer storage remains authoritative. Run `kriemhild
-migrate-only` during controlled deployment; startup also checks migrations.

For hosted legacy migration, set `KRIEMHILD_LEGACY_OWNER_SUBJECT` to the intended
Keycloak subject. That identity claims unowned records when it signs in and can
assign ambiguous writing projects. The first person to sign in does **not**
automatically become the legacy owner. For an installation with several existing
owners, an operator must map ownership explicitly; a single blanket mapping is
not appropriate. Remove the migration variable once assignments are complete.

Your account shows active session counts and **Sign out everywhere**. **Close
account** requires explicit confirmation and refuses while any world is owned;
transfer ownership or explicitly delete those worlds first. Closure revokes all
sessions, memberships, pending invitations and live connections, removes the
display name/email and prevents automatic reactivation through OIDC. Immutable
issuer/subject and user IDs remain to prevent accidental reactivation and preserve
revision attribution. Shared manuscripts, comments and world content remain.
Identity-provider deletion is a separate administrator action; deleting there
alone does not revoke an already issued application session. Provider logout
alone does not revoke other application sessions.

Hourly maintenance deletes expired sessions, invitations expired for over 30
days, and audit events older than 365 days. Content revisions are retained until
explicit content deletion, and backups follow their separate retention policy.
World deletion is
owner-only and removes its world-scoped manuscripts and memberships. Shared
immutable terrain/Library blobs remain in the database until a future garbage
collection policy can prove that no retained snapshot references them.

## Editing, comments, and recovery

World commands use optimistic revisions. Hosted map mutations require
`If-Match` with the map-session revision returned in `X-Map-Revision`; stale
commands return 409 and missing revisions return 428. Accepted hosted map changes
are persisted before the API reports success. Local mode keeps explicit Save.
Changed historical worlds invalidate stale map edits. World history includes
actor IDs, names, timestamps and snapshots; restoring makes another revision.
The header exposes presence and a refresh action when shared world changes arrive.
History labels automatic recovery points and restorations and pages through older
revisions. The hub filters owned/shared worlds and sorts recently opened worlds
using account-specific visits without changing the shared world's revision.

Tiptap uses Yjs for concurrent manuscript text. Go authenticates document rooms,
persists binary updates before acknowledging them and broadcasts accepted
updates. Reconnect replays the durable log and pending local updates. Pending
updates are isolated by account and document and retained until acknowledgment.
Named writer versions remain separate from the live update log. Version previews
use revisioned snapshots; manuscript exports reconstruct content from acknowledged
live updates, including edits newer than the latest snapshot. Remote cursors and
selections use relative text positions and authenticated collaborator names.

Comments use Yjs relative positions plus the original quotation. They follow text
movement; deleted anchors retain their quotation and report that the text is
gone. Suggestions require an editor to accept them and verify that the quoted
text still matches. Restoring a manuscript version resets the live document and
detaches old open comment anchors; other editors must reload. Unsynced local
edits should be recovered into a separate copy before a deliberate restoration.
Reconnecting clients detect restoration boundaries and retain older pending
updates locally instead of merging them into the restored manuscript.

World ZIPs include writing projects, saved versions, comments and CRDT updates.
Import creates independent IDs and derives ownership solely from the caller.
Permissions and session credentials are never imported. Historical world copies
remap writing references to the copied world; editable manuscript references are
normalized when the CRDT editor opens. ZIPs are portable content archives, not
replacements for full database backups containing world revision history and
account records.

## Backup and operations

For SQLite, `kriemhild -data data -backup-sqlite backups/kriemhild.sqlite` creates
a consistent online snapshot including WAL state. The destination must not exist.
Restore with the server stopped by placing the snapshot at
`<data-directory>/kriemhild.sqlite` in a fresh data directory.

For PostgreSQL use `pg_dump --format=custom` and `pg_restore` into a fresh database,
with credentials supplied by the operator's normal secure environment. Back up
Keycloak's database separately. Monitor readiness, database errors, disk usage,
proxy rejections and backup age; verify restores before deployment. Protect
database dumps because they contain private worlds and active session hashes.

Limits: 12 live connections per user, 2 MiB per manuscript delta, 64 MiB of update
history per scene, bounded send queues, and five-minute login flows. Hosted
token-bucket limits are 60 login requests/minute per direct peer, 20 invitation
mutations/10 minutes per account, 10 imports/minute, 4 generation starts/minute,
600 ordinary API requests/minute and 2,400 live messages/minute per account.
Forwarded client-IP headers are not trusted by Go. Structured rejection logs
include request class, method and status, without bodies, tokens or query strings.
Monitor 401/403/409/429 rates and private readiness/Keycloak metrics.
Log compaction, centralized alert delivery and multi-replica synchronization
remain operator/future work. CRDT payloads are
opaque to Go; named-version recovery and database backups are the recovery path
for a damaged document. Do not claim hosted production certification from local
tests alone.

## Validation

`go test ./...`, `go vet ./...`, `npm test`, `npm run check:ui`, `npm run build`.
Tests cover OIDC+PKCE, CSRF, IDOR, roles, revocation, invitation recipient/reuse,
stale map writes, attribution, legacy migration, SQLite backup/restore, portable
writing state, two concurrent manuscript clients, offline replay and restart.
Additional tests cover version-1 migration, account closure with shared content,
session revocation, invitation cancellation/decline, retention and abuse limits.
Container smoke checks require a running Docker daemon and Linux host networking.
