package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type actorKey struct{}

func WithActor(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, actorKey{}, id)
}
func Actor(ctx context.Context) string { id, _ := ctx.Value(actorKey{}).(string); return id }
func randomID() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func TokenHash(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }

type User struct {
	ID       string `json:"id"`
	Issuer   string `json:"-"`
	Subject  string `json:"-"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Verified bool   `json:"emailVerified"`
}
type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}
type Member struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}
type Invitation struct {
	ID      string `json:"id"`
	WorldID string `json:"worldId"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	Expires int64  `json:"expiresAt"`
	Status  string `json:"status"`
}
type Revision struct {
	Revision int    `json:"revision"`
	Author   string `json:"authorId"`
	Name     string `json:"authorName"`
	At       int64  `json:"at"`
	Kind     string `json:"kind"`
}

var ErrForbidden = errors.New("permission denied")

func CanEdit(role string) bool   { return role == "owner" || role == "administrator" || role == "editor" }
func CanManage(role string) bool { return role == "owner" || role == "administrator" }
func validRole(role string) bool {
	return role == "administrator" || role == "editor" || role == "commenter" || role == "viewer"
}

// Versioned additive migration: world payloads and their stable IDs stay intact.
func (s *Store) migrateAccess(ctx context.Context) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if s.driver == "pgx" {
		if _, e = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(1263683917)"); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS access_schema(version INTEGER PRIMARY KEY)`); e != nil {
		return e
	}
	var version int
	if e = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM access_schema`).Scan(&version); e != nil {
		return e
	}
	if version > 1 {
		return fmt.Errorf("access schema is newer than this server")
	}
	if version == 1 {
		return tx.Commit()
	}
	for _, q := range []string{
		`CREATE TABLE users(id TEXT PRIMARY KEY,issuer TEXT NOT NULL,subject TEXT NOT NULL,name TEXT NOT NULL,email TEXT NOT NULL,verified INTEGER NOT NULL,created_at BIGINT NOT NULL,UNIQUE(issuer,subject))`,
		`CREATE TABLE workspaces(id TEXT PRIMARY KEY,owner_id TEXT NOT NULL UNIQUE REFERENCES users(id),name TEXT NOT NULL)`,
		`CREATE TABLE workspace_members(workspace_id TEXT NOT NULL REFERENCES workspaces(id),user_id TEXT NOT NULL REFERENCES users(id),role TEXT NOT NULL,PRIMARY KEY(workspace_id,user_id))`,
		`CREATE INDEX workspace_members_user ON workspace_members(user_id)`,
		`CREATE TABLE world_access(world_id TEXT PRIMARY KEY,workspace_id TEXT NOT NULL REFERENCES workspaces(id),owner_id TEXT NOT NULL REFERENCES users(id))`,
		`CREATE TABLE world_members(world_id TEXT NOT NULL REFERENCES world_access(world_id) ON DELETE CASCADE,user_id TEXT NOT NULL REFERENCES users(id),role TEXT NOT NULL CHECK(role IN ('owner','administrator','editor','commenter','viewer')),status TEXT NOT NULL DEFAULT 'active',PRIMARY KEY(world_id,user_id))`,
		`CREATE UNIQUE INDEX world_one_owner ON world_members(world_id) WHERE role='owner'`,
		`CREATE INDEX world_members_user ON world_members(user_id,status)`,
		`CREATE TABLE world_invitations(id TEXT PRIMARY KEY,world_id TEXT NOT NULL REFERENCES world_access(world_id) ON DELETE CASCADE,token_hash TEXT NOT NULL UNIQUE,email TEXT NOT NULL,role TEXT NOT NULL,expires_at BIGINT NOT NULL,status TEXT NOT NULL,created_by TEXT NOT NULL REFERENCES users(id))`,
		`CREATE INDEX invitations_recipient ON world_invitations(email,status)`,
		`CREATE TABLE app_sessions(token_hash TEXT PRIMARY KEY,user_id TEXT NOT NULL REFERENCES users(id),csrf TEXT NOT NULL,expires_at BIGINT NOT NULL)`,
		`CREATE INDEX sessions_user ON app_sessions(user_id)`,
		`CREATE TABLE audit_events(id TEXT PRIMARY KEY,world_id TEXT NOT NULL,actor_id TEXT NOT NULL REFERENCES users(id),kind TEXT NOT NULL,detail TEXT NOT NULL,at BIGINT NOT NULL)`,
		`CREATE INDEX audit_world ON audit_events(world_id,at)`,
		`CREATE TABLE world_revisions(world_id TEXT NOT NULL REFERENCES world_access(world_id) ON DELETE CASCADE,revision INTEGER NOT NULL,actor_id TEXT NOT NULL REFERENCES users(id),at BIGINT NOT NULL,kind TEXT NOT NULL,snapshot TEXT NOT NULL,PRIMARY KEY(world_id,revision))`,
		`CREATE TABLE document_updates(node_id TEXT NOT NULL REFERENCES writer_nodes(id),id TEXT NOT NULL,actor_id TEXT NOT NULL REFERENCES users(id),data BYTEA NOT NULL,at BIGINT NOT NULL,PRIMARY KEY(node_id,id))`,
		`CREATE INDEX document_updates_time ON document_updates(node_id,at)`,
		`CREATE TABLE document_comments(id TEXT PRIMARY KEY,node_id TEXT NOT NULL REFERENCES writer_nodes(id),actor_id TEXT NOT NULL REFERENCES users(id),anchor TEXT NOT NULL,body TEXT NOT NULL,suggestion TEXT NOT NULL,status TEXT NOT NULL,at BIGINT NOT NULL)`,
		`INSERT INTO access_schema(version) VALUES(1)`,
	} {
		if _, e = tx.ExecContext(ctx, q); e != nil {
			return e
		}
	}
	return tx.Commit()
}

func (s *Store) ResolveUser(ctx context.Context, issuer, subject, name, email string, verified bool) (User, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return User{}, e
	}
	defer tx.Rollback()
	id := randomID()
	v := 0
	if verified {
		v = 1
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO users(id,issuer,subject,name,email,verified,created_at) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(issuer,subject) DO UPDATE SET name=excluded.name,email=excluded.email,verified=excluded.verified`, id, issuer, subject, name, strings.ToLower(strings.TrimSpace(email)), v, time.Now().UnixMilli())
	if e != nil {
		return User{}, e
	}
	u, e := readUser(ctx, tx, `issuer=$1 AND subject=$2`, issuer, subject)
	if e != nil {
		return u, e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO workspaces(id,owner_id,name) VALUES($1,$2,$3) ON CONFLICT(owner_id) DO NOTHING`, "personal-"+u.ID, u.ID, u.Name+"'s workspace")
	if e != nil {
		return u, e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO workspace_members(workspace_id,user_id,role) VALUES($1,$2,'owner') ON CONFLICT(workspace_id,user_id) DO NOTHING`, "personal-"+u.ID, u.ID)
	if e != nil {
		return u, e
	}
	return u, tx.Commit()
}
func readUser(ctx context.Context, q historyQuery, where string, args ...any) (User, error) {
	var u User
	var v int
	e := q.QueryRowContext(ctx, `SELECT id,issuer,subject,name,email,verified FROM users WHERE `+where, args...).Scan(&u.ID, &u.Issuer, &u.Subject, &u.Name, &u.Email, &v)
	u.Verified = v == 1
	return u, e
}
func (s *Store) User(ctx context.Context, id string) (User, error) {
	return readUser(ctx, s.db, "id=$1", id)
}
func (s *Store) Workspaces(ctx context.Context, user string) ([]Workspace, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT w.id,w.name,m.role FROM workspaces w JOIN workspace_members m ON m.workspace_id=w.id WHERE m.user_id=$1`, user)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Workspace{}
	for rows.Next() {
		var w Workspace
		if e = rows.Scan(&w.ID, &w.Name, &w.Role); e != nil {
			return nil, e
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
func ensureOwnership(ctx context.Context, tx *sql.Tx, id string) error {
	user := Actor(ctx)
	if user == "" {
		return nil
	} // Internal tests and explicitly unconfigured local embedding.
	_, e := tx.ExecContext(ctx, `INSERT INTO world_access(world_id,workspace_id,owner_id) VALUES($1,$2,$3) ON CONFLICT(world_id) DO NOTHING`, id, "personal-"+user, user)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO world_members(world_id,user_id,role,status) SELECT world_id,owner_id,'owner','active' FROM world_access WHERE world_id=$1 ON CONFLICT(world_id,user_id) DO NOTHING`, id)
	return e
}

// Only an explicit operator-selected identity may claim legacy records.
func (s *Store) AssignLegacy(ctx context.Context, user string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	ctx = WithActor(ctx, user)
	rows, e := tx.QueryContext(ctx, `SELECT id FROM history_worlds UNION SELECT id FROM kriemhild_projects WHERE id NOT IN (SELECT project_id FROM history_maps)`)
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		var old string
		lookup := tx.QueryRowContext(ctx, `SELECT a.owner_id FROM world_access a JOIN users u ON u.id=a.owner_id WHERE a.world_id=$1 AND u.issuer='kriemhild:local' AND u.subject='local'`, id).Scan(&old)
		if lookup == nil && old != user {
			if _, e = tx.ExecContext(ctx, `UPDATE world_access SET owner_id=$2,workspace_id=$3 WHERE world_id=$1`, id, user, "personal-"+user); e != nil {
				return e
			}
			if _, e = tx.ExecContext(ctx, `DELETE FROM world_members WHERE world_id=$1 AND user_id=$2`, id, old); e != nil {
				return e
			}
			if _, e = tx.ExecContext(ctx, `INSERT INTO world_members(world_id,user_id,role,status) VALUES($1,$2,'owner','active') ON CONFLICT(world_id,user_id) DO UPDATE SET role='owner',status='active'`, id, user); e != nil {
				return e
			}
			if e = audit(ctx, tx, id, "legacy-owner-assigned", user); e != nil {
				return e
			}
		} else if lookup != nil && lookup != sql.ErrNoRows {
			return lookup
		}
		if e = ensureOwnership(ctx, tx, id); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *Store) AccessWorld(ctx context.Context, id string) string {
	var parent string
	if s.db.QueryRowContext(ctx, `SELECT world_id FROM history_maps WHERE project_id=$1`, id).Scan(&parent) == nil {
		return parent
	}
	return id
}
func (s *Store) Role(ctx context.Context, id, user string) (string, error) {
	id = s.AccessWorld(ctx, id)
	var role string
	e := s.db.QueryRowContext(ctx, `SELECT role FROM world_members WHERE world_id=$1 AND user_id=$2 AND status='active'`, id, user).Scan(&role)
	return role, e
}
func (s *Store) Members(ctx context.Context, id string) ([]Member, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT u.id,u.name,m.role FROM world_members m JOIN users u ON u.id=m.user_id WHERE m.world_id=$1 AND m.status='active' ORDER BY u.name`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if e = rows.Scan(&m.UserID, &m.Name, &m.Role); e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func audit(ctx context.Context, tx *sql.Tx, world, kind, detail string) error {
	if Actor(ctx) == "" {
		return nil
	}
	_, e := tx.ExecContext(ctx, `INSERT INTO audit_events(id,world_id,actor_id,kind,detail,at) VALUES($1,$2,$3,$4,$5,$6)`, randomID(), world, Actor(ctx), kind, detail, time.Now().UnixMilli())
	return e
}
func (s *Store) SetMember(ctx context.Context, world, user, role string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var acting, target string
	if e = tx.QueryRowContext(ctx, `SELECT role FROM world_members WHERE world_id=$1 AND user_id=$2 AND status='active'`, world, Actor(ctx)).Scan(&acting); e != nil || !CanManage(acting) {
		return ErrForbidden
	}
	if e = tx.QueryRowContext(ctx, `SELECT role FROM world_members WHERE world_id=$1 AND user_id=$2`, world, user).Scan(&target); e != nil {
		return e
	}
	if target == "owner" || acting != "owner" && (target == "administrator" || role == "administrator") {
		return ErrForbidden
	}
	if role == "" {
		_, e = tx.ExecContext(ctx, `DELETE FROM world_members WHERE world_id=$1 AND user_id=$2`, world, user)
	} else {
		if !validRole(role) {
			return fmt.Errorf("invalid role")
		}
		_, e = tx.ExecContext(ctx, `UPDATE world_members SET role=$3 WHERE world_id=$1 AND user_id=$2`, world, user, role)
	}
	if e != nil {
		return e
	}
	if e = audit(ctx, tx, world, "membership", user+":"+role); e != nil {
		return e
	}
	if role == "" {
		if _, e = tx.ExecContext(ctx, `UPDATE world_invitations SET status='revoked' WHERE world_id=$1 AND email=(SELECT email FROM users WHERE id=$2) AND status='pending'`, world, user); e != nil {
			return e
		}
	}
	if !CanManage(role) {
		if _, e = tx.ExecContext(ctx, `UPDATE world_invitations SET status='revoked' WHERE world_id=$1 AND created_by=$2 AND status='pending'`, world, user); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *Store) TransferOwner(ctx context.Context, world, user string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var role string
	if e = tx.QueryRowContext(ctx, `SELECT role FROM world_members WHERE world_id=$1 AND user_id=$2`, world, user).Scan(&role); e != nil {
		return e
	}
	if user == Actor(ctx) {
		return fmt.Errorf("choose another member")
	}
	res, e := tx.ExecContext(ctx, `UPDATE world_access SET owner_id=$3,workspace_id=$4 WHERE world_id=$1 AND owner_id=$2`, world, Actor(ctx), user, "personal-"+user)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return ErrForbidden
	}
	if _, e = tx.ExecContext(ctx, `UPDATE world_members SET role='administrator' WHERE world_id=$1 AND user_id=$2`, world, Actor(ctx)); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE world_members SET role='owner' WHERE world_id=$1 AND user_id=$2`, world, user); e != nil {
		return e
	}
	if e = audit(ctx, tx, world, "ownership-transfer", user); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) Invite(ctx context.Context, world, email, role string) (string, error) {
	acting, e := s.Role(ctx, world, Actor(ctx))
	if e != nil || !CanManage(acting) || role == "administrator" && acting != "owner" {
		return "", ErrForbidden
	}
	if !validRole(role) || !strings.Contains(email, "@") || len(email) > 254 {
		return "", fmt.Errorf("valid email and role required")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	secret := randomID()
	_, e = tx.ExecContext(ctx, `INSERT INTO world_invitations(id,world_id,token_hash,email,role,expires_at,status,created_by) VALUES($1,$2,$3,$4,$5,$6,'pending',$7)`, randomID(), world, TokenHash(secret), strings.ToLower(strings.TrimSpace(email)), role, time.Now().Add(7*24*time.Hour).UnixMilli(), Actor(ctx))
	if e != nil {
		return "", e
	}
	if e = audit(ctx, tx, world, "invitation-created", email); e != nil {
		return "", e
	}
	return secret, tx.Commit()
}
func (s *Store) Invitations(ctx context.Context, u User) ([]Invitation, error) {
	out := []Invitation{}
	if !u.Verified {
		return out, nil
	}
	rows, e := s.db.QueryContext(ctx, `SELECT id,world_id,email,role,expires_at,status FROM world_invitations WHERE email=$1 AND status='pending' AND expires_at>$2`, u.Email, time.Now().UnixMilli())
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var v Invitation
		if e = rows.Scan(&v.ID, &v.WorldID, &v.Email, &v.Role, &v.Expires, &v.Status); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) AcceptInvitation(ctx context.Context, u User, secret string) (string, error) {
	if !u.Verified {
		return "", fmt.Errorf("verify your email with the identity provider first")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	var id, world, role string
	e = tx.QueryRowContext(ctx, `SELECT id,world_id,role FROM world_invitations WHERE (token_hash=$1 OR id=$2) AND email=$3 AND status='pending' AND expires_at>$4`, TokenHash(secret), secret, u.Email, time.Now().UnixMilli()).Scan(&id, &world, &role)
	if e != nil {
		return "", fmt.Errorf("invitation is invalid, expired, or belongs to another account")
	}
	res, e := tx.ExecContext(ctx, `UPDATE world_invitations SET status='accepted' WHERE id=$1 AND status='pending'`, id)
	if e != nil {
		return "", e
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return "", ErrForbidden
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO world_members(world_id,user_id,role,status) VALUES($1,$2,$3,'active') ON CONFLICT(world_id,user_id) DO NOTHING`, world, u.ID, role)
	if e != nil {
		return "", e
	}
	if e = audit(ctx, tx, world, "invitation-accepted", u.ID); e != nil {
		return "", e
	}
	return world, tx.Commit()
}
func (s *Store) NewSession(ctx context.Context, user string) (string, string, error) {
	secret, csrf := randomID(), randomID()
	_, e := s.db.ExecContext(ctx, `INSERT INTO app_sessions(token_hash,user_id,csrf,expires_at) VALUES($1,$2,$3,$4)`, TokenHash(secret), user, csrf, time.Now().Add(12*time.Hour).UnixMilli())
	return secret, csrf, e
}
func (s *Store) Session(ctx context.Context, secret string) (User, string, error) {
	var id, csrf string
	e := s.db.QueryRowContext(ctx, `SELECT user_id,csrf FROM app_sessions WHERE token_hash=$1 AND expires_at>$2`, TokenHash(secret), time.Now().UnixMilli()).Scan(&id, &csrf)
	if e != nil {
		return User{}, "", e
	}
	u, e := s.User(ctx, id)
	return u, csrf, e
}
func (s *Store) RevokeSession(ctx context.Context, secret string) error {
	_, e := s.db.ExecContext(ctx, `DELETE FROM app_sessions WHERE token_hash=$1`, TokenHash(secret))
	return e
}
func recordRevision(ctx context.Context, tx *sql.Tx, id string, revision int, snapshot any) error {
	if Actor(ctx) == "" {
		return nil
	}
	b, e := json.Marshal(snapshot)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO world_revisions(world_id,revision,actor_id,at,kind,snapshot) VALUES($1,$2,$3,$4,'world-save',$5)`, id, revision, Actor(ctx), time.Now().UnixMilli(), string(b))
	return e
}
func (s *Store) Revisions(ctx context.Context, world string, after int) ([]Revision, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT r.revision,r.actor_id,u.name,r.at,r.kind FROM world_revisions r JOIN users u ON u.id=r.actor_id WHERE r.world_id=$1 AND r.revision>$2 ORDER BY r.revision DESC LIMIT 200`, world, after)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Revision{}
	for rows.Next() {
		var v Revision
		if e = rows.Scan(&v.Revision, &v.Author, &v.Name, &v.At, &v.Kind); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) RevisionSnapshot(ctx context.Context, world string, revision int) (string, error) {
	var b string
	e := s.db.QueryRowContext(ctx, `SELECT snapshot FROM world_revisions WHERE world_id=$1 AND revision=$2`, world, revision).Scan(&b)
	return b, e
}
func (s *Store) WriterWorld(ctx context.Context, project string) (string, error) {
	var id string
	e := s.db.QueryRowContext(ctx, `SELECT world_id FROM writer_projects WHERE id=$1`, project).Scan(&id)
	return id, e
}

func (s *Store) CurrentRevision(ctx context.Context, world string) (int, error) {
	var revision int
	e := s.db.QueryRowContext(ctx, `SELECT revision FROM history_worlds WHERE id=$1`, world).Scan(&revision)
	return revision, e
}
