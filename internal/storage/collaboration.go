package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type DocumentUpdate struct {
	ID     string `json:"id"`
	Data   []byte `json:"data"`
	Author string `json:"authorId"`
}

func (s *Store) RestoreManuscript(ctx context.Context, project, node string, version, expected int) (WritingNode, error) {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return WritingNode{}, e
	}
	defer tx.Rollback()
	var raw string
	var n, current WritingNode
	if e = tx.QueryRowContext(ctx, `SELECT data FROM writer_versions WHERE project_id=$1 AND node_id=$2 AND revision=$3`, project, node, version).Scan(&raw); e != nil {
		return n, e
	}
	if e = json.Unmarshal([]byte(raw), &n); e != nil {
		return n, e
	}
	if e = tx.QueryRowContext(ctx, `SELECT data FROM writer_nodes WHERE project_id=$1 AND id=$2`, project, node).Scan(&raw); e != nil {
		return n, e
	}
	if e = json.Unmarshal([]byte(raw), &current); e != nil {
		return n, e
	}
	if current.Revision != expected {
		return n, ErrWritingConflict
	}
	current.Content = n.Content
	current.Text = n.Text
	current.References = n.References
	if e = saveWriterNodeTx(ctx, tx, project, &current, fmt.Sprintf("Restored manuscript revision %d", version)); e != nil {
		return current, e
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM document_updates WHERE node_id=$1`, node); e != nil {
		return current, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE document_comments SET status='detached' WHERE node_id=$1 AND status='open'`, node); e != nil {
		return current, e
	}
	return current, tx.Commit()
}

type Comment struct {
	ID         string `json:"id"`
	NodeID     string `json:"nodeId"`
	Author     string `json:"authorId"`
	Name       string `json:"authorName"`
	Anchor     string `json:"anchor"`
	Body       string `json:"body"`
	Suggestion string `json:"suggestion"`
	Status     string `json:"status"`
	At         int64  `json:"at"`
}

func (s *Store) WriterNode(ctx context.Context, project, node string) (WritingNode, error) {
	var n WritingNode
	var raw string
	e := s.db.QueryRowContext(ctx, `SELECT data FROM writer_nodes WHERE id=$1 AND project_id=$2`, node, project).Scan(&raw)
	if e == nil {
		e = json.Unmarshal([]byte(raw), &n)
	}
	if e == nil && n.Deleted {
		e = sql.ErrNoRows
	}
	return n, e
}
func (s *Store) DocumentUpdates(ctx context.Context, node string) ([]DocumentUpdate, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT id,data,actor_id FROM document_updates WHERE node_id=$1 ORDER BY at,id`, node)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []DocumentUpdate{}
	for rows.Next() {
		var v DocumentUpdate
		if e = rows.Scan(&v.ID, &v.Data, &v.Author); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) AppendDocumentUpdate(ctx context.Context, node string, data []byte, seedRevision int) (string, error) {
	if len(data) == 0 || len(data) > 2<<20 {
		return "", fmt.Errorf("document update must be between 1 byte and 2 MiB")
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	var size int64
	if e = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(LENGTH(data)),0) FROM document_updates WHERE node_id=$1`, node).Scan(&size); e != nil {
		return "", e
	}
	id := TokenHash(string(data))
	if seedRevision > 0 {
		if size > 0 {
			return "", fmt.Errorf("document was initialized; reconnect")
		}
		var rev int
		if e = tx.QueryRowContext(ctx, `SELECT revision FROM writer_nodes WHERE id=$1`, node).Scan(&rev); e != nil {
			return "", e
		}
		if rev != seedRevision {
			return "", ErrWritingConflict
		}
		id = "seed:" + TokenHash(string(data))
	}
	if size+int64(len(data)) > 64<<20 {
		return "", fmt.Errorf("document update history reached its limit; export and create a new scene")
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO document_updates(node_id,id,actor_id,data,at) VALUES($1,$2,$3,$4,$5) ON CONFLICT(node_id,id) DO NOTHING`, node, id, Actor(ctx), data, time.Now().UnixMilli())
	if e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func (s *Store) Comments(ctx context.Context, node string) ([]Comment, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT c.id,c.node_id,c.actor_id,u.name,c.anchor,c.body,c.suggestion,c.status,c.at FROM document_comments c JOIN users u ON u.id=c.actor_id WHERE c.node_id=$1 ORDER BY c.at`, node)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Comment{}
	for rows.Next() {
		var c Comment
		if e = rows.Scan(&c.ID, &c.NodeID, &c.Author, &c.Name, &c.Anchor, &c.Body, &c.Suggestion, &c.Status, &c.At); e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) SaveComment(ctx context.Context, world, node string, c *Comment) error {
	role, e := s.Role(ctx, world, Actor(ctx))
	if e != nil || role == "viewer" {
		return ErrForbidden
	}
	if len(c.Body) > 20000 || len(c.Anchor) > 16000 || len(c.Suggestion) > 20000 {
		return fmt.Errorf("comment too large")
	}
	if c.ID == "" {
		if c.Body == "" || c.Anchor == "" {
			return fmt.Errorf("select text and enter a comment")
		}
		c.ID = randomID()
		c.Author = Actor(ctx)
		c.NodeID = node
		c.Status = "open"
		c.At = time.Now().UnixMilli()
		_, e = s.db.ExecContext(ctx, `INSERT INTO document_comments(id,node_id,actor_id,anchor,body,suggestion,status,at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, c.ID, node, c.Author, c.Anchor, c.Body, c.Suggestion, c.Status, c.At)
		return e
	}
	var author string
	if e = s.db.QueryRowContext(ctx, `SELECT actor_id FROM document_comments WHERE id=$1 AND node_id=$2`, c.ID, node).Scan(&author); e != nil {
		return e
	}
	if Actor(ctx) != author && !CanEdit(role) {
		return ErrForbidden
	}
	if c.Status != "resolved" && c.Status != "open" {
		return fmt.Errorf("invalid comment status")
	}
	_, e = s.db.ExecContext(ctx, `UPDATE document_comments SET status=$3 WHERE id=$1 AND node_id=$2`, c.ID, node, c.Status)
	return e
}
