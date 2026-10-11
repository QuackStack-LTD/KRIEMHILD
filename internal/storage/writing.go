package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Writing is deliberately independent of fictional Worlds, Ages and entities.
type WritingDocument struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Text      string `json:"text"`
	Revision  int    `json:"revision"`
	UpdatedAt int64  `json:"updatedAt"`
}

func (s *Store) Writing(ctx context.Context) ([]WritingDocument, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,body,revision,updated_at FROM writing_documents ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WritingDocument{}
	for rows.Next() {
		var d WritingDocument
		if err = rows.Scan(&d.ID, &d.Name, &d.Text, &d.Revision, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) SaveWriting(ctx context.Context, d *WritingDocument) error {
	if d.ID == "" || len(d.ID) > 100 || strings.TrimSpace(d.Name) == "" || len(d.Name) > 200 || len(d.Text) > 2<<20 {
		return fmt.Errorf("invalid writing document")
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
	_, err := s.db.ExecContext(ctx, "DELETE FROM writing_documents WHERE id=$1", id)
	return err
}
