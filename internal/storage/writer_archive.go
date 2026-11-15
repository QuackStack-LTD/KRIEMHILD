package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	"sort"
	"strings"
	"time"
)

func writerContainer(kind string) bool {
	return kind == "folder" || kind == "book" || kind == "part" || kind == "chapter"
}

func validateWriterFile(ctx context.Context, tx *sql.Tx, world string, n WritingNode) error {
	if n.Format != "" && !strings.Contains("|docx|pdf|txt|md|", "|"+n.Format+"|") {
		return fmt.Errorf("unsupported document format")
	}
	if n.FileHash == "" {
		return nil
	}
	if len(n.FileName) == 0 || len(n.FileName) > 255 || strings.ContainsAny(n.FileName, "/\\\x00\r\n") || len(n.MIME) > 160 || n.FileSize < 0 || n.FileSize > history.MaxLibraryFileSize {
		return fmt.Errorf("invalid archive file")
	}
	var size int64
	if err := tx.QueryRowContext(ctx, `SELECT length(data) FROM writer_files WHERE world_id=$1 AND hash=$2`, world, n.FileHash).Scan(&size); err != nil {
		return fmt.Errorf("archive file is missing")
	}
	if size != n.FileSize {
		return fmt.Errorf("archive file size does not match")
	}
	return nil
}

func writerBundleFiles(ctx context.Context, tx *sql.Tx, b WriterBundle) (map[string][]byte, error) {
	out := map[string][]byte{}
	nodes := append([]WritingNode(nil), b.Nodes...)
	for _, v := range b.Versions {
		nodes = append(nodes, v.Node)
	}
	for _, n := range nodes {
		hashes := writerImageHashes(n.Content, b.Project.WorldID)
		if n.FileHash != "" {
			hashes = append(hashes, n.FileHash)
		}
		for _, h := range hashes {
			if _, ok := out[h]; ok {
				continue
			}
			var raw []byte
			if err := tx.QueryRowContext(ctx, `SELECT data FROM writer_files WHERE world_id=$1 AND hash=$2`, b.Project.WorldID, h).Scan(&raw); err != nil {
				return nil, err
			}
			out[h] = raw
		}
	}
	return out, nil
}

func writerImageHashes(content json.RawMessage, world string) []string {
	var node struct {
		Type  string `json:"type"`
		Attrs struct {
			Src string `json:"src"`
		} `json:"attrs"`
		Content []json.RawMessage `json:"content"`
	}
	if json.Unmarshal(content, &node) != nil {
		return nil
	}
	out := []string{}
	prefix := "/api/worlds/" + world + "/writer-archive/files/"
	if node.Type == "image" && strings.HasPrefix(node.Attrs.Src, prefix) {
		out = append(out, strings.TrimPrefix(node.Attrs.Src, prefix))
	}
	for _, child := range node.Content {
		out = append(out, writerImageHashes(child, world)...)
	}
	return out
}

func (s *Store) WriterArchiveFile(ctx context.Context, world, checksum string) ([]byte, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx, `SELECT data FROM writer_files WHERE world_id=$1 AND hash=$2`, world, checksum).Scan(&data)
	return data, err
}

func importWriterFiles(ctx context.Context, tx *sql.Tx, world string, files map[string][]byte) error {
	for h, data := range files {
		if len(data) > history.MaxLibraryFileSize || h != hash(data) {
			return fmt.Errorf("invalid archive file checksum or size")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO writer_files(world_id,hash,data) VALUES($1,$2,$3) ON CONFLICT(world_id,hash) DO NOTHING`, world, h, data); err != nil {
			return err
		}
	}
	return nil
}

type WriterArchive struct {
	Projects []WritingProject `json:"projects"`
	Nodes    []WritingNode    `json:"nodes"`
}

// This is a world-level read model, independent of the current Age and its biosphere.
func (s *Store) WriterArchive(ctx context.Context, world string) (WriterArchive, error) {
	out := WriterArchive{Projects: []WritingProject{}, Nodes: []WritingNode{}}
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if _, err = readViewWorld(ctx, tx, world); err != nil {
		return out, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT data FROM writer_projects WHERE world_id=$1 ORDER BY id`, world)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var p WritingProject
		var raw string
		if err = rows.Scan(&raw); err != nil {
			break
		}
		if err = json.Unmarshal([]byte(raw), &p); err != nil {
			break
		}
		out.Projects = append(out.Projects, p)
	}
	rowErr := rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if rowErr != nil {
		return out, rowErr
	}
	for _, p := range out.Projects {
		ns, e := writerNodes(ctx, tx, p.ID)
		if e != nil {
			return out, e
		}
		for i := range ns {
			ns[i].Content = nil
			ns[i].Text = ""
			ns[i].OriginalText = ""
		}
		out.Nodes = append(out.Nodes, ns...)
	}
	return out, tx.Commit()
}

func (s *Store) EnsureWriterArchive(ctx context.Context, world string) (WritingProject, error) {
	p, err := writerProject(ctx, s.db, "archive-"+world)
	if err == nil {
		return p, nil
	}
	if err != sql.ErrNoRows {
		return p, err
	}
	p = WritingProject{ID: "archive-" + world, WorldID: world, Name: "Writer's Archive", Type: "archive"}
	err = s.SaveWriterProject(ctx, &p)
	if err != nil {
		if existing, e := writerProject(ctx, s.db, p.ID); e == nil {
			return existing, nil
		}
	}
	return p, err
}

func (s *Store) SaveWriterFile(ctx context.Context, project string, n *WritingNode, name, mime string, data []byte) error {
	if len(data) > history.MaxLibraryFileSize {
		return fmt.Errorf("file exceeds 64 MiB")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	p, err := writerProject(ctx, tx, project)
	if err != nil {
		return err
	}
	if p.Deleted {
		return fmt.Errorf("restore the folder before editing")
	}
	n.FileHash = hash(data)
	n.FileName = name
	n.MIME = mime
	n.FileSize = int64(len(data))
	if err = importWriterFiles(ctx, tx, p.WorldID, map[string][]byte{n.FileHash: data}); err != nil {
		return err
	}
	if err = saveWriterNodeTx(ctx, tx, project, n, "Imported file version"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) WriterFile(ctx context.Context, project, node, version string) ([]byte, WritingNode, error) {
	var n WritingNode
	var raw string
	p, err := writerProject(ctx, s.db, project)
	if err != nil {
		return nil, n, err
	}
	if version == "" {
		err = s.db.QueryRowContext(ctx, `SELECT data FROM writer_nodes WHERE project_id=$1 AND id=$2`, project, node).Scan(&raw)
	} else {
		err = s.db.QueryRowContext(ctx, `SELECT data FROM writer_versions WHERE project_id=$1 AND node_id=$2 AND revision=$3`, project, node, version).Scan(&raw)
	}
	if err != nil {
		return nil, n, err
	}
	if err = json.Unmarshal([]byte(raw), &n); err != nil {
		return nil, n, err
	}
	if n.FileHash == "" {
		return nil, n, sql.ErrNoRows
	}
	var data []byte
	err = s.db.QueryRowContext(ctx, `SELECT data FROM writer_files WHERE world_id=$1 AND hash=$2`, p.WorldID, n.FileHash).Scan(&data)
	return data, n, err
}

type WriterArchiveAction struct {
	Action        string `json:"action"`
	ProjectID     string `json:"projectId"`
	NodeID        string `json:"nodeId"`
	Revision      int    `json:"revision"`
	TargetProject string `json:"targetProject"`
	ParentID      string `json:"parentId"`
}

// Folder operations are atomic. A trash group restores only the descendants
// removed by this action, without resurrecting older, separately deleted files.
func (s *Store) WriterArchiveAction(ctx context.Context, world string, a WriterArchiveAction) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE writer_projects SET revision=revision WHERE id=$1`, a.ProjectID); err != nil {
		return err
	}
	p, err := writerProject(ctx, tx, a.ProjectID)
	if err != nil {
		return err
	}
	if p.WorldID != world {
		return fmt.Errorf("folder belongs to another World")
	}
	if a.NodeID == "" {
		if p.ID == "archive-"+world {
			return fmt.Errorf("the World archive cannot be deleted")
		}
		if p.Revision != a.Revision {
			return ErrWritingConflict
		}
		if a.Action != "trash" && a.Action != "restore" {
			return fmt.Errorf("unsupported folder action")
		}
		p.Deleted = a.Action == "trash"
		p.Revision++
		p.UpdatedAt = time.Now().UnixMilli()
		raw, _ := json.Marshal(p)
		_, err = tx.ExecContext(ctx, `UPDATE writer_projects SET data=$2,revision=$3 WHERE id=$1`, p.ID, string(raw), p.Revision)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if p.Deleted {
		return fmt.Errorf("restore the folder first")
	}
	nodes, err := writerNodes(ctx, tx, p.ID)
	if err != nil {
		return err
	}
	byID := map[string]WritingNode{}
	for _, n := range nodes {
		byID[n.ID] = n
	}
	root, ok := byID[a.NodeID]
	if !ok {
		return sql.ErrNoRows
	}
	if root.Revision != a.Revision {
		return ErrWritingConflict
	}
	ids := map[string]bool{root.ID: true}
	for changed := true; changed; {
		changed = false
		for _, n := range nodes {
			if ids[n.ParentID] && !ids[n.ID] {
				ids[n.ID] = true
				changed = true
			}
		}
	}
	depth := func(n WritingNode) int {
		d := 0
		seen := map[string]bool{}
		for n.ParentID != "" && !seen[n.ID] {
			seen[n.ID] = true
			d++
			n = byID[n.ParentID]
		}
		return d
	}
	sort.SliceStable(nodes, func(i, j int) bool { return depth(nodes[i]) < depth(nodes[j]) })
	switch a.Action {
	case "trash":
		for i := len(nodes) - 1; i >= 0; i-- {
			n := nodes[i]
			if !ids[n.ID] || n.Deleted {
				continue
			}
			n.Deleted = true
			n.TrashRootID = root.ID
			if err = saveWriterNodeTx(ctx, tx, p.ID, &n, "Moved to trash"); err != nil {
				return err
			}
		}
	case "restore":
		for _, n := range nodes {
			if !ids[n.ID] || !n.Deleted || (n.ID != root.ID && n.TrashRootID != root.ID) {
				continue
			}
			n.Deleted = false
			n.TrashRootID = ""
			if par, ok := byID[n.ParentID]; ok && par.Deleted && !ids[par.ID] {
				n.ParentID = ""
			}
			if err = saveWriterNodeTx(ctx, tx, p.ID, &n, "Restored from trash"); err != nil {
				return err
			}
		}
	case "move":
		if root.Deleted {
			return fmt.Errorf("restore the item before moving")
		}
		target, err := writerProject(ctx, tx, a.TargetProject)
		if err != nil {
			return err
		}
		if target.WorldID != world || target.Deleted {
			return fmt.Errorf("choose a folder in this World")
		}
		targetNodes, err := writerNodes(ctx, tx, target.ID)
		if err != nil {
			return err
		}
		if a.ParentID != "" {
			valid := false
			for _, n := range targetNodes {
				if n.ID == a.ParentID && !n.Deleted && writerContainer(n.Type) {
					valid = true
				}
			}
			if !valid || ids[a.ParentID] {
				return fmt.Errorf("invalid destination folder")
			}
		}
		for _, n := range nodes {
			if !ids[n.ID] {
				continue
			}
			n.ProjectID = target.ID
			if n.ID == root.ID {
				n.ParentID = a.ParentID
				n.Position = float64(time.Now().UnixMilli())
			}
			n.Revision++
			n.UpdatedAt = time.Now().UnixMilli()
			raw, _ := json.Marshal(n)
			if _, err = tx.ExecContext(ctx, `UPDATE writer_nodes SET project_id=$2,revision=$3,data=$4 WHERE id=$1`, n.ID, target.ID, n.Revision, string(raw)); err != nil {
				return err
			}
			// History travels with the file. Update its container identities too,
			// so exporting just the destination project remains self-contained.
			if err = moveWriterVersions(ctx, tx, n.ID, target.ID, a.ParentID, n.ID == root.ID); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO writer_versions(node_id,revision,project_id,label,data) VALUES($1,$2,$3,'Moved folder or file',$4)`, n.ID, n.Revision, target.ID, string(raw)); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown archive action")
	}
	return tx.Commit()
}

func moveWriterVersions(ctx context.Context, tx *sql.Tx, node, project, parent string, root bool) error {
	rows, err := tx.QueryContext(ctx, `SELECT data FROM writer_versions WHERE node_id=$1`, node)
	if err != nil {
		return err
	}
	versions := []WritingNode{}
	for rows.Next() {
		var n WritingNode
		var raw string
		if err = rows.Scan(&raw); err != nil {
			break
		}
		if err = json.Unmarshal([]byte(raw), &n); err != nil {
			break
		}
		n.ProjectID = project
		if root {
			n.ParentID = parent
		}
		versions = append(versions, n)
	}
	rowErr := rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if rowErr != nil {
		return rowErr
	}
	for _, n := range versions {
		raw, _ := json.Marshal(n)
		if _, err = tx.ExecContext(ctx, `UPDATE writer_versions SET project_id=$2,data=$3 WHERE node_id=$1 AND revision=$4`, node, project, string(raw), n.Revision); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) InworldArchive(ctx context.Context, world string) (*history.Document, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d := &history.Document{}
	if d.World, err = readViewWorld(ctx, tx, world); err != nil {
		return nil, err
	}
	if d.Ages, err = readViewRows[history.Age](ctx, tx, "ages", "world_id=$1", world); err != nil {
		return nil, err
	}
	args := []any{world}
	keys := []string{}
	for kind, t := range history.Registry.Types {
		if t.Category == "records" {
			args = append(args, kind)
			keys = append(keys, fmt.Sprintf("$%d", len(args)))
		}
	}
	if d.Entities, err = readViewRows[history.Entity](ctx, tx, "entities", "world_id=$1 AND kind IN ("+strings.Join(keys, ",")+")", args...); err != nil {
		return nil, err
	}
	// Include every file-backed record, including custom record types.
	files, err := readViewRows[history.LibraryFile](ctx, tx, "library_files", "world_id=$1", world)
	if err != nil {
		return nil, err
	}
	d.LibraryFiles = files
	return d, tx.Commit()
}
