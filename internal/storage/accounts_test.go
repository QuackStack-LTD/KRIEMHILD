package storage

import (
	"context"
	"errors"
	"kriemhild/internal/history"
	"testing"
	"time"
)

func TestAccountClosurePreservesSharedWorldAndAttribution(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a, err := db.ResolveUser(ctx, "issuer", "alice", "Alice", "alice@test.example", true)
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.ResolveUser(ctx, "issuer", "bob", "Bob", "bob@test.example", true)
	if err != nil {
		t.Fatal(err)
	}
	ac, bc := WithActor(ctx, a.ID), WithActor(ctx, b.ID)
	d := history.New("Shared world")
	if err = db.CreateHistory(ac, d); err != nil {
		t.Fatal(err)
	}
	secret, _, err := db.NewSession(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.CloseAccount(ac); !errors.Is(err, ErrOwnsWorlds) {
		t.Fatal("owner allowed to orphan world", err)
	}
	invite, err := db.Invite(ac, d.World.ID, b.Email, "editor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.AcceptInvitation(bc, b, invite); err != nil {
		t.Fatal(err)
	}
	if err = db.TransferOwner(ac, d.World.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if err = db.VisitWorld(ac, d.World.ID); err != nil {
		t.Fatal(err)
	}
	if err = db.CloseAccount(ac); err != nil {
		t.Fatal(err)
	}
	if _, _, err = db.Session(ctx, secret); err == nil {
		t.Fatal("closed account session accepted")
	}
	if _, _, err = db.NewSession(ctx, a.ID); !errors.Is(err, ErrAccountClosed) {
		t.Fatal("closed account received session", err)
	}
	if _, err = db.ResolveUser(ctx, "issuer", "alice", "Resurrected", "alice@test.example", true); !errors.Is(err, ErrAccountClosed) {
		t.Fatal("OIDC reopened closed account", err)
	}
	if _, err = db.Role(ctx, d.World.ID, a.ID); err == nil {
		t.Fatal("membership survived closure")
	}
	if role, err := db.Role(ctx, d.World.ID, b.ID); err != nil || role != "owner" {
		t.Fatal("other owner changed", role, err)
	}
	if world, err := db.History(ctx, d.World.ID); err != nil || world.World.Name != "Shared world" {
		t.Fatal("world deleted", err)
	}
	revisions, err := db.Revisions(ctx, d.World.ID, 0)
	if err != nil || len(revisions) != 1 || revisions[0].Author != a.ID || revisions[0].Name != "Closed account" {
		t.Fatal("attribution lost", revisions, err)
	}
}

func TestInvitationRevocationDeclineAndRetention(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a, _ := db.ResolveUser(ctx, "issuer", "a", "A", "a@test.example", true)
	b, _ := db.ResolveUser(ctx, "issuer", "b", "B", "b@test.example", true)
	ac, bc := WithActor(ctx, a.ID), WithActor(ctx, b.ID)
	d := history.New("Invitations")
	if err = db.CreateHistory(ac, d); err != nil {
		t.Fatal(err)
	}
	token, err := db.Invite(ac, d.World.ID, b.Email, "editor")
	if err != nil {
		t.Fatal(err)
	}
	invitations, err := db.WorldInvitations(ac, d.World.ID)
	if err != nil || len(invitations) != 1 {
		t.Fatal(invitations, err)
	}
	if err = db.RevokeInvitation(bc, d.World.ID, invitations[0].ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("outsider revoked invite", err)
	}
	if err = db.RevokeInvitation(ac, d.World.ID, invitations[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.AcceptInvitation(bc, b, token); err == nil {
		t.Fatal("revoked invitation accepted")
	}
	token, err = db.Invite(ac, d.World.ID, b.Email, "viewer")
	if err != nil {
		t.Fatal(err)
	}
	invitations, _ = db.Invitations(ctx, b)
	if err = db.DeclineInvitation(ac, a, invitations[0].ID); err == nil {
		t.Fatal("wrong recipient declined invitation")
	}
	if err = db.DeclineInvitation(bc, b, invitations[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.AcceptInvitation(bc, b, token); err == nil {
		t.Fatal("declined invitation accepted")
	}
	secret, _, err := db.NewSession(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.MaintainAccounts(ctx, time.Now().Add(38*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, _, err = db.Session(ctx, secret); err == nil {
		t.Fatal("expired session remained usable")
	}
	var count int
	if err = db.db.QueryRow(`SELECT COUNT(*) FROM world_invitations`).Scan(&count); err != nil || count != 0 {
		t.Fatal("expired invites retained", err, count)
	}
	if revisions, err := db.Revisions(ctx, d.World.ID, 0); err != nil || len(revisions) != 1 {
		t.Fatal("retention purged content history", err)
	}
}

func TestVersionOneAccessMigrationPreservesOwnership(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	u, err := db.ResolveUser(ctx, "issuer", "existing", "Existing", "e@test.example", true)
	if err != nil {
		t.Fatal(err)
	}
	d := history.New("Existing")
	if err = db.CreateHistory(WithActor(ctx, u.ID), d); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`DROP TABLE world_visits`, `DROP INDEX sessions_expiry`, `ALTER TABLE users DROP COLUMN closed_at`, `DELETE FROM access_schema WHERE version=2`} {
		if _, err = db.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if role, err := db.Role(ctx, d.World.ID, u.ID); err != nil || role != "owner" {
		t.Fatal("migration lost owner", err)
	}
	if err = db.VisitWorld(WithActor(ctx, u.ID), d.World.ID); err != nil {
		t.Fatal(err)
	}
	if visits, err := db.WorldVisits(WithActor(ctx, u.ID)); err != nil || visits[d.World.ID] == 0 {
		t.Fatal("new migration unusable", err)
	}
}
