package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrAccountClosed = errors.New("this KRIEMHILD account is closed")
var ErrOwnsWorlds = errors.New("transfer or explicitly delete your owned worlds before closing your account")

type operationKey struct{}

func WithOperation(ctx context.Context, kind string) context.Context {
	return context.WithValue(ctx, operationKey{}, kind)
}

type OwnedWorld struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type AccountSummary struct {
	OwnedWorlds    []OwnedWorld `json:"ownedWorlds"`
	ActiveSessions int          `json:"activeSessions"`
}

func (s *Store) Account(ctx context.Context) (AccountSummary, error) {
	result := AccountSummary{OwnedWorlds: []OwnedWorld{}}
	rows, err := s.db.QueryContext(ctx, `SELECT a.world_id,COALESCE(h.name,'Legacy terrain project') FROM world_access a LEFT JOIN history_worlds h ON h.id=a.world_id WHERE a.owner_id=$1 ORDER BY a.world_id`, Actor(ctx))
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var w OwnedWorld
		if err = rows.Scan(&w.ID, &w.Name); err != nil {
			rows.Close()
			return result, err
		}
		result.OwnedWorlds = append(result.OwnedWorlds, w)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_sessions WHERE user_id=$1 AND expires_at>$2`, Actor(ctx), time.Now().UnixMilli()).Scan(&result.ActiveSessions)
	return result, err
}
func (s *Store) RevokeUserSessions(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM app_sessions WHERE user_id=$1`, Actor(ctx)); err != nil {
		return err
	}
	if err = audit(ctx, tx, "", "sessions-revoked", ""); err != nil {
		return err
	}
	return tx.Commit()
}

// Keep immutable identity and authored records for attribution; remove access and
// profile information. Closing never cascades into someone else's shared world.
func (s *Store) CloseAccount(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	user := Actor(ctx)
	var owned int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM world_access WHERE owner_id=$1`, user).Scan(&owned); err != nil {
		return err
	}
	if owned > 0 {
		return ErrOwnsWorlds
	}
	var issuer string
	if err = tx.QueryRowContext(ctx, `SELECT issuer FROM users WHERE id=$1 AND closed_at=0`, user).Scan(&issuer); err != nil {
		return err
	}
	if issuer == "kriemhild:local" {
		return ErrForbidden
	}
	if err = audit(ctx, tx, "", "account-closed", ""); err != nil {
		return err
	}
	for _, q := range []string{
		`UPDATE world_invitations SET status='revoked' WHERE status='pending' AND (created_by=$1 OR email=(SELECT email FROM users WHERE id=$1))`,
		`DELETE FROM app_sessions WHERE user_id=$1`,
		`DELETE FROM world_visits WHERE user_id=$1`,
		`DELETE FROM world_members WHERE user_id=$1`,
		`DELETE FROM workspace_members WHERE user_id=$1`,
		`UPDATE workspaces SET name='Closed account workspace' WHERE owner_id=$1`,
	} {
		if _, err = tx.ExecContext(ctx, q, user); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET name='Closed account',email='',verified=0,closed_at=$2 WHERE id=$1`, user, time.Now().UnixMilli()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) WorldInvitations(ctx context.Context, world string) ([]Invitation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,world_id,email,role,expires_at,status FROM world_invitations WHERE world_id=$1 AND status='pending' AND expires_at>$2 ORDER BY expires_at DESC`, world, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Invitation{}
	for rows.Next() {
		var v Invitation
		if err = rows.Scan(&v.ID, &v.WorldID, &v.Email, &v.Role, &v.Expires, &v.Status); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (s *Store) RevokeInvitation(ctx context.Context, world, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var acting, target string
	if err = tx.QueryRowContext(ctx, `SELECT role FROM world_members WHERE world_id=$1 AND user_id=$2 AND status='active'`, world, Actor(ctx)).Scan(&acting); err != nil || !CanManage(acting) {
		return ErrForbidden
	}
	if err = tx.QueryRowContext(ctx, `SELECT role FROM world_invitations WHERE id=$1 AND world_id=$2 AND status='pending'`, id, world).Scan(&target); err != nil {
		return err
	}
	if target == "administrator" && acting != "owner" {
		return ErrForbidden
	}
	if _, err = tx.ExecContext(ctx, `UPDATE world_invitations SET status='revoked' WHERE id=$1`, id); err != nil {
		return err
	}
	if err = audit(ctx, tx, world, "invitation-revoked", id); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) DeclineInvitation(ctx context.Context, u User, id string) error {
	if !u.Verified {
		return ErrForbidden
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var world string
	if err = tx.QueryRowContext(ctx, `SELECT world_id FROM world_invitations WHERE id=$1 AND email=$2 AND status='pending' AND expires_at>$3`, id, u.Email, time.Now().UnixMilli()).Scan(&world); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `UPDATE world_invitations SET status='declined' WHERE id=$1 AND status='pending'`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return sql.ErrNoRows
	}
	if err = audit(ctx, tx, world, "invitation-declined", id); err != nil {
		return err
	}
	return tx.Commit()
}

type AuditEvent struct {
	ID     string `json:"id"`
	Author string `json:"authorId"`
	Name   string `json:"authorName"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
	At     int64  `json:"at"`
}

func (s *Store) AuditEvents(ctx context.Context, world string, before int64) ([]AuditEvent, error) {
	if before <= 0 {
		before = time.Now().UnixMilli() + 1
	}
	rows, err := s.db.QueryContext(ctx, `SELECT e.id,e.actor_id,u.name,e.kind,e.detail,e.at FROM audit_events e JOIN users u ON u.id=e.actor_id WHERE e.world_id=$1 AND e.at<$2 ORDER BY e.at DESC,e.id DESC LIMIT 100`, world, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []AuditEvent{}
	for rows.Next() {
		var e AuditEvent
		if err = rows.Scan(&e.ID, &e.Author, &e.Name, &e.Kind, &e.Detail, &e.At); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// Credentials expire after 12h; invitation/security records are retained 30/365
// days. Content revisions and attribution are retained until explicit deletion.
func (s *Store) MaintainAccounts(ctx context.Context, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, step := range []struct {
		query  string
		cutoff int64
	}{
		{`DELETE FROM app_sessions WHERE expires_at<=$1`, now.UnixMilli()},
		{`DELETE FROM world_invitations WHERE expires_at<$1`, now.Add(-30 * 24 * time.Hour).UnixMilli()},
		{`DELETE FROM audit_events WHERE at<$1`, now.Add(-365 * 24 * time.Hour).UnixMilli()},
	} {
		if _, err = tx.ExecContext(ctx, step.query, step.cutoff); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) VisitWorld(ctx context.Context, world string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO world_visits(user_id,world_id,opened_at) SELECT user_id,world_id,$3 FROM world_members WHERE user_id=$1 AND world_id=$2 AND status='active' ON CONFLICT(user_id,world_id) DO UPDATE SET opened_at=excluded.opened_at`, Actor(ctx), world, time.Now().UnixMilli())
	return err
}
func (s *Store) WorldVisits(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT v.world_id,v.opened_at FROM world_visits v JOIN world_members m ON m.world_id=v.world_id AND m.user_id=v.user_id WHERE v.user_id=$1 AND m.status='active'`, Actor(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]int64{}
	for rows.Next() {
		var id string
		var at int64
		if err = rows.Scan(&id, &at); err != nil {
			return nil, err
		}
		result[id] = at
	}
	return result, rows.Err()
}
