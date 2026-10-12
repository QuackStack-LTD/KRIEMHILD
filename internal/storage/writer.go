package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrWritingConflict = errors.New("writing changed in another session; recover your local edits or reload")
var writerSchema = []string{
	`CREATE TABLE IF NOT EXISTS writer_projects(id TEXT PRIMARY KEY,world_id TEXT NOT NULL,revision INTEGER NOT NULL,data TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS writer_nodes(id TEXT PRIMARY KEY,project_id TEXT NOT NULL REFERENCES writer_projects(id),revision INTEGER NOT NULL,data TEXT NOT NULL)`,
	`CREATE INDEX IF NOT EXISTS writer_nodes_project ON writer_nodes(project_id)`,
	`CREATE TABLE IF NOT EXISTS writer_versions(node_id TEXT NOT NULL REFERENCES writer_nodes(id),revision INTEGER NOT NULL,project_id TEXT NOT NULL,label TEXT NOT NULL,data TEXT NOT NULL,PRIMARY KEY(node_id,revision))`,
}

type WriterSettings struct {
	Font          string  `json:"font,omitempty"`
	FontSize      int     `json:"fontSize,omitempty"`
	LineHeight    float64 `json:"lineHeight,omitempty"`
	Theme         string  `json:"theme,omitempty"`
	Target        int     `json:"target,omitempty"`
	SessionTarget int     `json:"sessionTarget,omitempty"`
	DefaultAge    string  `json:"defaultAge,omitempty"`
}
type WritingProject struct {
	ImportedWorldID string         `json:"importedWorldId,omitempty"`
	ID              string         `json:"id"`
	WorldID         string         `json:"worldId"`
	Name            string         `json:"name"`
	Type            string         `json:"projectType"`
	Settings        WriterSettings `json:"settings"`
	Revision        int            `json:"revision"`
	CreatedAt       int64          `json:"createdAt"`
	UpdatedAt       int64          `json:"updatedAt"`
}
type WritingReference struct {
	WorldID  string `json:"worldId"`
	EntityID string `json:"entityId"`
	AgeID    string `json:"ageId"`
	Type     string `json:"type"`
	Wording  string `json:"wording,omitempty"`
}
type WritingNode struct {
	AuthorID     string             `json:"authorId,omitempty"`
	ID           string             `json:"id"`
	ProjectID    string             `json:"projectId"`
	ParentID     string             `json:"parentId"`
	Type         string             `json:"type"`
	Position     float64            `json:"position"`
	Title        string             `json:"title"`
	Synopsis     string             `json:"synopsis"`
	Status       string             `json:"status"`
	Tags         string             `json:"tags"`
	Content      json.RawMessage    `json:"content"`
	Schema       int                `json:"contentSchema"`
	Text         string             `json:"text"`
	Notes        string             `json:"notes"`
	Purpose      string             `json:"purpose"`
	Conflict     string             `json:"conflict"`
	Outcome      string             `json:"outcome"`
	StoryAge     string             `json:"storyAgeId"`
	StoryYear    *int               `json:"storyYear"`
	POV          string             `json:"povEntityId"`
	Location     string             `json:"locationEntityId"`
	Target       int                `json:"targetWords"`
	References   []WritingReference `json:"references"`
	OriginalName string             `json:"originalName,omitempty"`
	OriginalText string             `json:"originalText,omitempty"`
	Deleted      bool               `json:"deleted,omitempty"`
	Revision     int                `json:"revision"`
	CreatedAt    int64              `json:"createdAt"`
	UpdatedAt    int64              `json:"updatedAt"`
}
type WritingVersion struct {
	Node  WritingNode `json:"node"`
	Label string      `json:"label"`
}
type WriterBundle struct {
	Comments []Comment                   `json:"comments,omitempty"`
	Updates  map[string][]DocumentUpdate `json:"updates,omitempty"`
	Format   string                      `json:"format"`
	Version  int                         `json:"version"`
	Project  WritingProject              `json:"project"`
	Nodes    []WritingNode               `json:"nodes"`
	Versions []WritingVersion            `json:"versions"`
}

func plainWriterContent(text string) json.RawMessage {
	blocks := []any{}
	for _, line := range strings.Split(text, "\n") {
		p := map[string]any{"type": "paragraph"}
		if line != "" {
			p["content"] = []any{map[string]any{"type": "text", "text": line}}
		}
		blocks = append(blocks, p)
	}
	b, _ := json.Marshal(map[string]any{"type": "doc", "content": blocks})
	return b
}
func migrateWriter(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,name,body,revision,updated_at FROM writing_documents WHERE id NOT IN (SELECT id FROM writer_nodes)`)
	if err != nil {
		return err
	}
	docs := []WritingDocument{}
	for rows.Next() {
		var d WritingDocument
		if err = rows.Scan(&d.ID, &d.Name, &d.Text, &d.Revision, &d.UpdatedAt); err != nil {
			rows.Close()
			return err
		}
		docs = append(docs, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(docs) == 0 {
		return nil
	}
	p := WritingProject{ID: "legacy-loose-drafts", Name: "Loose Drafts", Type: "personal", Revision: 1, CreatedAt: time.Now().UnixMilli(), UpdatedAt: time.Now().UnixMilli()}
	raw, _ := json.Marshal(p)
	if _, err = tx.ExecContext(ctx, `INSERT INTO writer_projects(id,world_id,revision,data) VALUES($1,'',1,$2) ON CONFLICT(id) DO NOTHING`, p.ID, string(raw)); err != nil {
		return err
	}
	for i, d := range docs {
		n := WritingNode{ID: d.ID, ProjectID: p.ID, Type: "scene", Title: d.Name, Position: float64(i), Content: plainWriterContent(d.Text), Schema: 1, Text: d.Text, Status: "Drafting", Revision: d.Revision, CreatedAt: d.UpdatedAt, UpdatedAt: d.UpdatedAt}
		b, _ := json.Marshal(n)
		if _, err = tx.ExecContext(ctx, `INSERT INTO writer_nodes(id,project_id,revision,data) VALUES($1,$2,$3,$4)`, n.ID, p.ID, n.Revision, string(b)); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO writer_versions(node_id,revision,project_id,label,data) VALUES($1,$2,$3,'Migrated original draft',$4)`, n.ID, n.Revision, p.ID, string(b)); err != nil {
			return err
		}
	}
	return nil
}
func writerProject(ctx context.Context, q historyQuery, id string) (WritingProject, error) {
	var p WritingProject
	var b string
	err := q.QueryRowContext(ctx, `SELECT data FROM writer_projects WHERE id=$1`, id).Scan(&b)
	if err == nil {
		err = json.Unmarshal([]byte(b), &p)
	}
	return p, err
}
func writerNodes(ctx context.Context, q historyQuery, id string) ([]WritingNode, error) {
	rows, err := q.QueryContext(ctx, `SELECT data FROM writer_nodes WHERE project_id=$1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WritingNode{}
	for rows.Next() {
		var b string
		var n WritingNode
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(b), &n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
func (s *Store) WriterProjects(ctx context.Context, world string) ([]WritingProject, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = migrateWriter(ctx, tx); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `SELECT data FROM writer_projects WHERE world_id=$1 OR world_id='' ORDER BY id`, world)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WritingProject{}
	for rows.Next() {
		var b string
		var p WritingProject
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(b), &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) SaveWriterProject(ctx context.Context, p *WritingProject) error {
	if p.ID == "" || len(p.ID) > 100 || strings.TrimSpace(p.Name) == "" || len(p.Name) > 200 || p.Settings.Target < 0 || p.Settings.SessionTarget < 0 {
		return fmt.Errorf("invalid project")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if p.WorldID != "" {
		var id string
		if err = tx.QueryRowContext(ctx, `SELECT id FROM history_worlds WHERE id=$1`, p.WorldID).Scan(&id); err != nil {
			return fmt.Errorf("world does not exist")
		}
	}
	expected := p.Revision
	if expected > 0 {
		old, e := writerProject(ctx, tx, p.ID)
		if e != nil {
			return e
		}
		if old.WorldID != "" && old.WorldID != p.WorldID {
			return fmt.Errorf("project world cannot be reassigned")
		}
		p.CreatedAt = old.CreatedAt
	}
	p.UpdatedAt = time.Now().UnixMilli()
	if p.CreatedAt == 0 {
		p.CreatedAt = p.UpdatedAt
	}
	p.Revision++
	b, _ := json.Marshal(p)
	if expected == 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO writer_projects(id,world_id,revision,data) VALUES($1,$2,$3,$4)`, p.ID, p.WorldID, p.Revision, string(b))
	} else {
		var r sql.Result
		r, err = tx.ExecContext(ctx, `UPDATE writer_projects SET world_id=$2,revision=$3,data=$4 WHERE id=$1 AND revision=$5`, p.ID, p.WorldID, p.Revision, string(b), expected)
		if err == nil {
			count, _ := r.RowsAffected()
			if count != 1 {
				return ErrWritingConflict
			}
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) WriterBundle(ctx context.Context, id string) (WriterBundle, error) {
	return s.writerBundle(ctx, id, true)
}
func (s *Store) WriterCurrent(ctx context.Context, id string) (WriterBundle, error) {
	return s.writerBundle(ctx, id, false)
}
func (s *Store) writerBundle(ctx context.Context, id string, versions bool) (WriterBundle, error) {
	b := WriterBundle{Format: "kriemhild-writing", Version: 1, Versions: []WritingVersion{}}
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return b, err
	}
	defer tx.Rollback()
	b, err = writerBundleTx(ctx, tx, id, versions)
	if err != nil {
		return b, err
	}
	return b, tx.Commit()
}
func writerBundleTx(ctx context.Context, tx *sql.Tx, id string, versions bool) (WriterBundle, error) {
	b := WriterBundle{Format: "kriemhild-writing", Version: 1, Versions: []WritingVersion{}}
	var err error
	b.Project, err = writerProject(ctx, tx, id)
	if err != nil {
		return b, err
	}
	b.Nodes, err = writerNodes(ctx, tx, id)
	if err != nil {
		return b, err
	}
	if !versions {
		return b, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT data,label FROM writer_versions WHERE project_id=$1 ORDER BY node_id,revision`, id)
	if err != nil {
		return b, err
	}
	for rows.Next() {
		var raw string
		var v WritingVersion
		if err = rows.Scan(&raw, &v.Label); err != nil {
			break
		}
		if err = json.Unmarshal([]byte(raw), &v.Node); err != nil {
			break
		}
		b.Versions = append(b.Versions, v)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return b, err
	}
	b.Updates = map[string][]DocumentUpdate{}
	for _, n := range b.Nodes {
		rows, e := tx.QueryContext(ctx, `SELECT id,data,actor_id FROM document_updates WHERE node_id=$1 ORDER BY at,id`, n.ID)
		if e != nil {
			return b, e
		}
		updates := []DocumentUpdate{}
		for rows.Next() {
			var u DocumentUpdate
			if e = rows.Scan(&u.ID, &u.Data, &u.Author); e != nil {
				rows.Close()
				return b, e
			}
			updates = append(updates, u)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return b, e
		}
		if len(updates) > 0 {
			b.Updates[n.ID] = updates
		}
	}
	rows, err = tx.QueryContext(ctx, `SELECT id,node_id,actor_id,anchor,body,suggestion,status,at FROM document_comments WHERE node_id IN (SELECT id FROM writer_nodes WHERE project_id=$1) ORDER BY at`, id)
	if err != nil {
		return b, err
	}
	for rows.Next() {
		var c Comment
		if err = rows.Scan(&c.ID, &c.NodeID, &c.Author, &c.Anchor, &c.Body, &c.Suggestion, &c.Status, &c.At); err != nil {
			rows.Close()
			return b, err
		}
		b.Comments = append(b.Comments, c)
	}
	err = rows.Err()
	rows.Close()
	return b, err
}

// Validate both hierarchy and semantic rich content before persisting it. Text is derived, never trusted.
func validateWriterContent(raw json.RawMessage) (string, error) {
	var root struct {
		Type    string            `json:"type"`
		Content []json.RawMessage `json:"content"`
	}
	if len(raw) > 2<<20 || json.Unmarshal(raw, &root) != nil || root.Type != "doc" {
		return "", fmt.Errorf("invalid editor document")
	}
	count := 0
	var walk func(json.RawMessage, int) (string, error)
	walk = func(b json.RawMessage, depth int) (string, error) {
		count++
		if depth > 40 || count > 100000 {
			return "", fmt.Errorf("editor document too complex")
		}
		var n struct {
			Type    string            `json:"type"`
			Text    string            `json:"text"`
			Content []json.RawMessage `json:"content"`
			Marks   []struct {
				Type  string         `json:"type"`
				Attrs map[string]any `json:"attrs"`
			} `json:"marks"`
		}
		if err := json.Unmarshal(b, &n); err != nil {
			return "", err
		}
		if !strings.Contains("|doc|paragraph|heading|text|blockquote|bulletList|orderedList|listItem|horizontalRule|hardBreak|codeBlock|", "|"+n.Type+"|") {
			return "", fmt.Errorf("unsupported editor node %s", n.Type)
		}
		for _, m := range n.Marks {
			if !strings.Contains("|bold|italic|strike|underline|code|link|worldReference|", "|"+m.Type+"|") {
				return "", fmt.Errorf("unsupported text mark")
			}
			if m.Type == "link" {
				href, _ := m.Attrs["href"].(string)
				if !strings.HasPrefix(href, "https://") && !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "mailto:") {
					return "", fmt.Errorf("unsupported link protocol")
				}
			}
		}
		if n.Type == "text" {
			if n.Text == "" || len(n.Content) > 0 {
				return "", fmt.Errorf("invalid text node")
			}
			return n.Text, nil
		}
		for _, child := range n.Content {
			var c struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(child, &c); err != nil {
				return "", err
			}
			inline := c.Type == "text" || c.Type == "hardBreak"
			switch n.Type {
			case "paragraph", "heading", "codeBlock":
				if !inline {
					return "", fmt.Errorf("block nested inside paragraph")
				}
			case "bulletList", "orderedList":
				if c.Type != "listItem" {
					return "", fmt.Errorf("list requires list items")
				}
			case "doc", "blockquote", "listItem":
				if inline || c.Type == "doc" || c.Type == "listItem" {
					return "", fmt.Errorf("invalid block structure")
				}
			case "hardBreak", "horizontalRule":
				return "", fmt.Errorf("leaf node contains children")
			}
		}
		if n.Type == "horizontalRule" {
			return "\n***\n", nil
		}
		if n.Type == "hardBreak" {
			return "\n", nil
		}
		var out strings.Builder
		for _, c := range n.Content {
			t, e := walk(c, depth+1)
			if e != nil {
				return "", e
			}
			out.WriteString(t)
		}
		if n.Type == "paragraph" || n.Type == "heading" || n.Type == "codeBlock" {
			out.WriteString("\n")
		}
		return out.String(), nil
	}
	t, e := walk(raw, 0)
	return strings.TrimSpace(t), e
}
func validateWriterNode(n *WritingNode, p WritingProject, nodes []WritingNode) error {
	if n.ID == "" || len(n.ID) > 100 || n.ProjectID != p.ID || strings.TrimSpace(n.Title) == "" || len(n.Title) > 300 || n.Schema != 1 || n.Target < 0 {
		return fmt.Errorf("invalid manuscript node")
	}
	if !strings.Contains("|book|part|chapter|scene|note|research|outline|frontMatter|backMatter|", "|"+n.Type+"|") {
		return fmt.Errorf("invalid node type")
	}
	if !strings.Contains("|Idea|Outlined|Drafting|Needs revision|Final|", "|"+n.Status+"|") {
		return fmt.Errorf("invalid scene status")
	}
	byID := map[string]WritingNode{}
	for _, x := range nodes {
		byID[x.ID] = x
	}
	seen := map[string]bool{n.ID: true}
	parent := n.ParentID
	for parent != "" {
		if seen[parent] {
			return fmt.Errorf("manuscript hierarchy cannot contain cycles")
		}
		seen[parent] = true
		x, ok := byID[parent]
		if !ok || (x.Deleted && !n.Deleted) {
			return fmt.Errorf("parent is not in this project")
		}
		if !strings.Contains("|book|part|chapter|", "|"+x.Type+"|") {
			return fmt.Errorf("parent must be a book, part or chapter")
		}
		parent = x.ParentID
	}
	if n.Type != "book" && n.Type != "part" && n.Type != "chapter" {
		for _, x := range nodes {
			if x.ParentID == n.ID && !x.Deleted {
				return fmt.Errorf("move child nodes before converting this container")
			}
		}
	}
	var err error
	n.Text, err = validateWriterContent(n.Content)
	if err != nil {
		return err
	}
	if len(n.OriginalText) > 4<<20 || len(n.Notes) > 2<<20 || len(n.References) > 1000 {
		return fmt.Errorf("node metadata too large")
	}
	// Inline links are part of canonical rich content, not merely client metadata.
	var linkWalk func(json.RawMessage) error
	linkWalk = func(raw json.RawMessage) error {
		var item struct {
			Text    string            `json:"text"`
			Content []json.RawMessage `json:"content"`
			Marks   []struct {
				Type  string `json:"type"`
				Attrs struct {
					WorldID  string `json:"worldId"`
					EntityID string `json:"entityId"`
					AgeID    string `json:"ageId"`
				} `json:"attrs"`
			} `json:"marks"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			return err
		}
		for _, mark := range item.Marks {
			if mark.Type == "worldReference" {
				a := mark.Attrs
				found := false
				for _, r := range n.References {
					if r.Type == "inline" && r.WorldID == a.WorldID && r.EntityID == a.EntityID && r.AgeID == a.AgeID {
						found = true
					}
				}
				if !found {
					n.References = append(n.References, WritingReference{WorldID: a.WorldID, EntityID: a.EntityID, AgeID: a.AgeID, Type: "inline", Wording: item.Text})
				}
			}
		}
		for _, child := range item.Content {
			if err := linkWalk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err = linkWalk(n.Content); err != nil {
		return err
	}
	for _, r := range n.References {
		if p.WorldID == "" || r.WorldID != p.WorldID || r.EntityID == "" || r.AgeID == "" || (r.Type != "inline" && r.Type != "metadata") {
			return fmt.Errorf("invalid world reference")
		}
	}
	return nil
}
func (s *Store) SaveWriterNode(ctx context.Context, project string, n *WritingNode, label string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = saveWriterNodeTx(ctx, tx, project, n, label); err != nil {
		return err
	}
	return tx.Commit()
}
func saveWriterNodeTx(ctx context.Context, tx *sql.Tx, project string, n *WritingNode, label string) error {
	var err error
	// Serialize structural edits, including moves, against the project row.
	if _, err = tx.ExecContext(ctx, `UPDATE writer_projects SET revision=revision WHERE id=$1`, project); err != nil {
		return err
	}
	p, err := writerProject(ctx, tx, project)
	if err != nil {
		return err
	}
	nodes, err := writerNodes(ctx, tx, project)
	if err != nil {
		return err
	}
	if err = validateWriterNode(n, p, nodes); err != nil {
		return err
	}
	var old *WritingNode
	for i := range nodes {
		if nodes[i].ID == n.ID {
			old = &nodes[i]
		}
	}
	if (old == nil && n.Revision != 0) || (old != nil && old.Revision != n.Revision) {
		return ErrWritingConflict
	}
	if old != nil {
		n.CreatedAt = old.CreatedAt
	}
	if n.Deleted {
		for _, c := range nodes {
			if c.ParentID == n.ID && !c.Deleted {
				return fmt.Errorf("move or delete child nodes first")
			}
		}
	}
	// Existing broken references remain recoverable; new links and scene context must resolve.
	needsWorld := old == nil || old.StoryAge != n.StoryAge || old.POV != n.POV || old.Location != n.Location
	if !needsWorld {
		for _, r := range n.References {
			found := false
			for _, o := range old.References {
				if o.WorldID == r.WorldID && o.EntityID == r.EntityID && o.AgeID == r.AgeID {
					found = true
					break
				}
			}
			if !found {
				needsWorld = true
				break
			}
		}
	}
	if p.WorldID != "" && needsWorld {
		d, e := readHistory(ctx, tx, p.WorldID)
		if e != nil {
			return e
		}
		if n.StoryAge != "" {
			found := false
			for _, a := range d.Ages {
				if a.ID == n.StoryAge {
					found = true
				}
			}
			if !found && (old == nil || old.StoryAge != n.StoryAge) {
				return fmt.Errorf("story Age does not belong to world")
			}
		}
		for _, r := range n.References {
			existed := false
			if old != nil {
				for _, o := range old.References {
					if o.WorldID == r.WorldID && o.EntityID == r.EntityID && o.AgeID == r.AgeID {
						existed = true
					}
				}
			}
			if !existed && d.Entity(r.AgeID, r.EntityID) == nil {
				return fmt.Errorf("referenced entity is missing in selected Age")
			}
		}
		for _, id := range []string{n.POV, n.Location} {
			if id != "" && d.Entity(n.StoryAge, id) == nil && (old == nil || (id != old.POV && id != old.Location)) {
				return fmt.Errorf("scene entity is missing in story Age")
			}
		}
	}
	expected := n.Revision
	n.Revision++
	n.UpdatedAt = time.Now().UnixMilli()
	n.AuthorID = Actor(ctx)
	if n.CreatedAt == 0 {
		n.CreatedAt = n.UpdatedAt
	}
	raw, _ := json.Marshal(n)
	if old == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO writer_nodes(id,project_id,revision,data) VALUES($1,$2,$3,$4)`, n.ID, project, n.Revision, string(raw))
	} else {
		var r sql.Result
		r, err = tx.ExecContext(ctx, `UPDATE writer_nodes SET revision=$3,data=$4 WHERE id=$1 AND project_id=$2 AND revision=$5`, n.ID, project, n.Revision, string(raw), expected)
		if err == nil {
			count, _ := r.RowsAffected()
			if count != 1 {
				return ErrWritingConflict
			}
		}
	}
	if err != nil {
		return err
	}
	if len(label) > 200 {
		return fmt.Errorf("snapshot label too long")
	}
	if label == "" {
		label = "Autosave"
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO writer_versions(node_id,revision,project_id,label,data) VALUES($1,$2,$3,$4,$5)`, n.ID, n.Revision, project, label, string(raw)); err != nil {
		return err
	}
	return nil
}

// ImportWriter creates an independent project in one transaction, retaining revision history.
func (s *Store) ImportWriter(ctx context.Context, b WriterBundle, world string) (WritingProject, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WritingProject{}, err
	}
	defer tx.Rollback()
	p, err := importWriterTx(ctx, tx, b, world)
	if err != nil {
		return p, err
	}
	return p, tx.Commit()
}
func importWriterTx(ctx context.Context, tx *sql.Tx, b WriterBundle, world string) (WritingProject, error) {
	p := b.Project
	if b.Format != "kriemhild-writing" || b.Version != 1 || len(b.Nodes) > 10000 || len(b.Versions) > 100000 {
		return p, fmt.Errorf("unsupported or oversized writing backup")
	}
	var err error
	if world != "" {
		var id string
		if err = tx.QueryRowContext(ctx, `SELECT id FROM history_worlds WHERE id=$1`, world).Scan(&id); err != nil {
			return p, fmt.Errorf("target world is missing")
		}
	}
	// Cross-world links are never silently reattached to similarly named entities.
	if p.WorldID != "" && p.WorldID != world {
		return p, fmt.Errorf("restore the referenced world first or import into the original world")
	}
	p.ID = writerID()
	p.WorldID = world
	p.Name += " (restored)"
	p.Revision = 1
	p.UpdatedAt = time.Now().UnixMilli()
	if p.CreatedAt == 0 {
		p.CreatedAt = p.UpdatedAt
	}
	if strings.TrimSpace(p.Name) == "" || len(p.Name) > 220 {
		return p, fmt.Errorf("invalid project title")
	}
	ids := map[string]string{}
	for _, n := range b.Nodes {
		if n.ID == "" || ids[n.ID] != "" {
			return p, fmt.Errorf("duplicate node identity")
		}
		ids[n.ID] = writerID()
	}
	remap := func(n WritingNode) (WritingNode, error) {
		id := ids[n.ID]
		if id == "" {
			return n, fmt.Errorf("version references missing node")
		}
		n.ID = id
		n.ProjectID = p.ID
		n.AuthorID = Actor(ctx)
		if n.ParentID != "" {
			n.ParentID = ids[n.ParentID]
			if n.ParentID == "" {
				return n, fmt.Errorf("missing parent")
			}
		}
		return n, nil
	}
	nodes := []WritingNode{}
	for _, n := range b.Nodes {
		x, e := remap(n)
		if e != nil {
			return p, e
		}
		nodes = append(nodes, x)
	}
	raw, _ := json.Marshal(p)
	if _, err = tx.ExecContext(ctx, `INSERT INTO writer_projects(id,world_id,revision,data) VALUES($1,$2,1,$3)`, p.ID, world, string(raw)); err != nil {
		return p, err
	}
	for i := range nodes {
		n := &nodes[i]
		if n.Revision < 1 {
			return p, fmt.Errorf("invalid node revision")
		}
		if err = validateWriterNode(n, p, nodes); err != nil {
			return p, err
		}
		raw, _ = json.Marshal(n)
		if _, err = tx.ExecContext(ctx, `INSERT INTO writer_nodes(id,project_id,revision,data) VALUES($1,$2,$3,$4)`, n.ID, p.ID, n.Revision, string(raw)); err != nil {
			return p, err
		}
	}
	versions := map[string]map[int]bool{}
	for _, v := range b.Versions {
		n, e := remap(v.Node)
		if e != nil {
			return p, e
		}
		var current WritingNode
		for _, x := range nodes {
			if x.ID == n.ID {
				current = x
			}
		}
		if n.Revision < 1 || n.Revision > current.Revision {
			return p, fmt.Errorf("invalid snapshot revision")
		}
		if _, err = validateWriterContent(n.Content); err != nil {
			return p, err
		}
		raw, _ = json.Marshal(n)
		if _, err = tx.ExecContext(ctx, `INSERT INTO writer_versions(node_id,revision,project_id,label,data) VALUES($1,$2,$3,$4,$5)`, n.ID, n.Revision, p.ID, v.Label, string(raw)); err != nil {
			return p, err
		}
		if versions[n.ID] == nil {
			versions[n.ID] = map[int]bool{}
		}
		versions[n.ID][n.Revision] = true
	}
	for _, n := range nodes {
		if !versions[n.ID][n.Revision] {
			raw, _ = json.Marshal(n)
			if _, err = tx.ExecContext(ctx, `INSERT INTO writer_versions(node_id,revision,project_id,label,data) VALUES($1,$2,$3,'Imported backup',$4)`, n.ID, n.Revision, p.ID, string(raw)); err != nil {
				return p, err
			}
		}
	}
	if Actor(ctx) != "" {
		for old, updates := range b.Updates {
			node := ids[old]
			if node == "" {
				return p, fmt.Errorf("updates reference missing scene")
			}
			size := 0
			for _, u := range updates {
				size += len(u.Data)
				if size > 64<<20 || len(u.Data) > 2<<20 {
					return p, fmt.Errorf("document update history too large")
				}
				id := TokenHash(string(u.Data))
				if strings.HasPrefix(u.ID, "seed") {
					id = "seed:" + id
				}
				if _, err = tx.ExecContext(ctx, `INSERT INTO document_updates(node_id,id,actor_id,data,at) VALUES($1,$2,$3,$4,$5) ON CONFLICT(node_id,id) DO NOTHING`, node, id, Actor(ctx), u.Data, time.Now().UnixMilli()); err != nil {
					return p, err
				}
			}
		}
	}
	if Actor(ctx) != "" {
		for _, c := range b.Comments {
			node := ids[c.NodeID]
			if node == "" || len(c.Anchor) > 16000 || len(c.Body) > 20000 || len(c.Suggestion) > 20000 {
				return p, fmt.Errorf("invalid imported comment")
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO document_comments(id,node_id,actor_id,anchor,body,suggestion,status,at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, randomID(), node, Actor(ctx), c.Anchor, c.Body, c.Suggestion, c.Status, c.At); err != nil {
				return p, err
			}
		}
	}
	return p, nil
}

func writerID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
