package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Legacy draft API. Migrated drafts delegate writes to the revisioned writer store.
type WritingDocument struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Text      string `json:"text"`
	Revision  int    `json:"revision"`
	UpdatedAt int64  `json:"updatedAt"`
}

func (s *Store) Writing(ctx context.Context) ([]WritingDocument, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT d.id,d.name,d.body,d.revision,d.updated_at,COALESCE(n.data,'') FROM writing_documents d LEFT JOIN writer_nodes n ON n.id=d.id ORDER BY d.updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WritingDocument{}
	for rows.Next() {
		var d WritingDocument
		var current string
		if err = rows.Scan(&d.ID, &d.Name, &d.Text, &d.Revision, &d.UpdatedAt, &current); err != nil {
			return nil, err
		}
		if current != "" {
			var n WritingNode
			if err = json.Unmarshal([]byte(current), &n); err != nil {
				return nil, err
			}
			if n.Deleted {
				continue
			}
			d.Name = n.Title
			d.Text = n.Text
			d.Revision = n.Revision
			d.UpdatedAt = n.UpdatedAt
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) SaveWriting(ctx context.Context, d *WritingDocument) error {
	if d.ID == "" || len(d.ID) > 100 || strings.TrimSpace(d.Name) == "" || len(d.Name) > 200 || len(d.Text) > 2<<20 {
		return fmt.Errorf("invalid writing document")
	}
	var raw string
	err := s.db.QueryRowContext(ctx, "SELECT data FROM writer_nodes WHERE id=$1", d.ID).Scan(&raw)
	if err == nil {
		var n WritingNode
		if err = json.Unmarshal([]byte(raw), &n); err != nil {
			return err
		}
		if n.Revision != d.Revision {
			return ErrWritingConflict
		}
		n.Title = d.Name
		n.Content = plainWriterContent(d.Text)
		if err = s.SaveWriterNode(ctx, n.ProjectID, &n, "Legacy editor save"); err != nil {
			return err
		}
		d.Revision = n.Revision
		d.UpdatedAt = n.UpdatedAt
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	d.UpdatedAt = time.Now().UnixMilli()
	if d.Revision == 0 {
		_, err := s.db.ExecContext(ctx, "INSERT INTO writing_documents(id,name,body,revision,updated_at) VALUES($1,$2,$3,1,$4)", d.ID, d.Name, d.Text, d.UpdatedAt)
		if err != nil {
			return err
		}
		d.Revision = 1
		return nil
	}
	result, err := s.db.ExecContext(ctx, "UPDATE writing_documents SET name=$2,body=$3,revision=revision+1,updated_at=$4 WHERE id=$1 AND revision=$5", d.ID, d.Name, d.Text, d.UpdatedAt, d.Revision)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return fmt.Errorf("document changed; reload before saving")
	}
	d.Revision++
	return nil
}
func (s *Store) DeleteWriting(ctx context.Context, id string) error {
	var raw string
	err := s.db.QueryRowContext(ctx, "SELECT data FROM writer_nodes WHERE id=$1", id).Scan(&raw)
	if err == nil {
		var n WritingNode
		if err = json.Unmarshal([]byte(raw), &n); err != nil {
			return err
		}
		n.Deleted = true
		return s.SaveWriterNode(ctx, n.ProjectID, &n, "Legacy editor deletion")
	}
	if err != sql.ErrNoRows {
		return err
	}

	_, err = s.db.ExecContext(ctx, "DELETE FROM writing_documents WHERE id=$1", id)
	return err
}
