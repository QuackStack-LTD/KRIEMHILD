package project

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type GitInfo struct {
	Available   bool   `json:"available"`
	Initialized bool   `json:"initialized"`
	Changes     string `json:"changes"`
	Branch      string `json:"branch"`
	Remote      string `json:"remote"`
}
type GitCommand struct {
	Action  string `json:"action"`
	Message string `json:"message"`
	Remote  string `json:"remote"`
}

func (s *Store) git(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base := []string{"-C", s.Dir, "-c", "core.hooksPath=" + filepath.Join(s.Dir, ".kriemhild", "no-hooks"), "-c", "protocol.ext.allow=never", "-c", "protocol.file.allow=never"}
	cmd := exec.CommandContext(ctx, "git", append(base, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var output limitedBuffer
	output.limit = ArchiveLimit
	cmd.Stdout = &output
	var stderr limitedBuffer
	stderr.limit = 8192
	cmd.Stderr = &stderr
	if e := cmd.Run(); e != nil {
		return nil, fmt.Errorf("git %s failed: %s", args[0], strings.TrimSpace(stderr.String()))
	}
	return output.Bytes(), nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, fmt.Errorf("output limit reached")
	}
	return b.Buffer.Write(p)
}
func (s *Store) GitStatus() (GitInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := GitInfo{}
	if _, e := exec.LookPath("git"); e != nil {
		return out, nil
	}
	out.Available = true
	if _, e := os.Stat(filepath.Join(s.Dir, ".git")); e != nil {
		return out, nil
	}
	out.Initialized = true
	b, e := s.git("status", "--porcelain=v1")
	if e != nil {
		return out, e
	}
	out.Changes = string(b)
	b, _ = s.git("branch", "--show-current")
	out.Branch = strings.TrimSpace(string(b))
	b, _ = s.git("remote", "get-url", "origin")
	out.Remote = strings.TrimSpace(string(b))
	return out, nil
}
func (s *Store) GitAction(c GitCommand) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var e error
	switch c.Action {
	case "init":
		if _, e = s.git("init", "-b", "main"); e != nil {
			return e
		}
		return atomicWrite(filepath.Join(s.Dir, ".gitignore"), []byte(".kriemhild/\n*.tmp\n"))
	case "commit":
		if !nameOK(c.Message) {
			return fmt.Errorf("commit message is required")
		}
		if e = s.stageProject(); e != nil {
			return e
		}
		_, e = s.git("-c", "user.name=KRIEMHILD author", "-c", "user.email=author@kriemhild.local", "commit", "-m", c.Message)
		return e
	case "remote":
		if e = validateRemote(c.Remote); e != nil {
			return e
		}
		if _, e = s.git("remote", "get-url", "origin"); e != nil {
			_, e = s.git("remote", "add", "origin", c.Remote)
		} else {
			_, e = s.git("remote", "set-url", "origin", c.Remote)
		}
		return e
	case "fetch":
		_, e = s.git("fetch", "--no-tags", "origin", "main")
		return e
	case "push":
		_, e = s.git("push", "origin", "HEAD:refs/heads/main")
		return e
	default:
		return fmt.Errorf("unsupported repository action")
	}
}

func (s *Store) stageProject() error {
	args := []string{"add", "--"}
	for _, name := range []string{"objects", "snapshots", "revisions", "assets", "kriemhild.json", "project.head.json", ".gitignore"} {
		info, e := os.Stat(filepath.Join(s.Dir, name))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		if info.IsDir() {
			entries, e := os.ReadDir(filepath.Join(s.Dir, name))
			if e != nil {
				return e
			}
			if len(entries) == 0 {
				continue
			}
		}
		args = append(args, name)
	}
	_, e := s.git(args...)
	return e
}

type MergeConflict struct {
	Path   string `json:"path"`
	Base   any    `json:"base"`
	Local  any    `json:"local"`
	Remote any    `json:"remote"`
}
type MergeRequest struct {
	Expected string            `json:"expected"`
	Remote   string            `json:"remote"`
	Choices  map[string]string `json:"choices"`
	Apply    bool              `json:"apply"`
}
type MergeResult struct {
	AlreadyIncluded bool            `json:"alreadyIncluded"`
	Revision        string          `json:"revision"`
	Remote          string          `json:"remote"`
	Base            string          `json:"base"`
	Conflicts       []MergeConflict `json:"conflicts"`
	Applied         bool            `json:"applied"`
}

func equalJSON(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
func mergeJSON(base, local, remote any, path string, choices map[string]string, conflicts *[]MergeConflict) any {
	if equalJSON(local, remote) {
		return local
	}
	if equalJSON(base, local) {
		return remote
	}
	if equalJSON(base, remote) {
		return local
	}
	b, bok := base.(map[string]any)
	l, lok := local.(map[string]any)
	r, rok := remote.(map[string]any)
	if bok && lok && rok {
		out := map[string]any{}
		keys := map[string]bool{}
		for k := range b {
			keys[k] = true
		}
		for k := range l {
			keys[k] = true
		}
		for k := range r {
			keys[k] = true
		}
		ordered := []string{}
		for k := range keys {
			ordered = append(ordered, k)
		}
		sort.Strings(ordered)
		for _, k := range ordered {
			v := mergeJSON(b[k], l[k], r[k], path+"/"+strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1"), choices, conflicts)
			if v != nil {
				out[k] = v
			}
		}
		return out
	}
	if choices[path] == "local" {
		return local
	}
	if choices[path] == "remote" {
		return remote
	}
	*conflicts = append(*conflicts, MergeConflict{path, base, local, remote})
	return local
}
func (s *Store) logical(root Root) (map[string]any, error) {
	ages := map[string]any{}
	for id, a := range root.Ages {
		records, e := s.records(a.Snapshot)
		if e != nil {
			return nil, e
		}
		metadata := map[string]any{"id": a.ID, "name": a.Name, "sourceAge": a.SourceAge, "sourceSnapshot": a.SourceSnapshot, "sourceRevision": a.SourceRevision}
		ages[id] = map[string]any{"metadata": metadata, "records": records}
	}
	raw, _ := json.Marshal(map[string]any{"ages": ages})
	var out map[string]any
	json.Unmarshal(raw, &out)
	return out, nil
}

// Fetch never modifies the active head. Reconciliation resolves logical fields
// in a temporary checkout, validates the merged closure, then updates the root.
func (s *Store) GitMerge(c MergeRequest) (MergeResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rev, local, e := s.root()
	out := MergeResult{Revision: rev, Conflicts: []MergeConflict{}}
	if e != nil {
		return out, e
	}
	if c.Expected != rev {
		return out, ErrConflict
	}
	dirty, e := s.git("status", "--porcelain=v1")
	if e != nil {
		return out, e
	}
	if len(dirty) > 0 {
		return out, fmt.Errorf("commit local project changes before reviewing a fetched revision")
	}
	b, e := s.git("show", "FETCH_HEAD:project.head.json")
	if e != nil {
		return out, e
	}
	var head Head
	if e = json.Unmarshal(b, &head); e != nil || !validHash(head.Revision) {
		return out, fmt.Errorf("remote is not a KRIEMHILD repository")
	}
	out.Remote = head.Revision
	if c.Remote != "" && c.Remote != out.Remote {
		return out, ErrConflict
	}
	baseDir, e := os.MkdirTemp(filepath.Join(s.Dir, ".kriemhild"), "merge-")
	if e != nil {
		return out, e
	}
	defer os.RemoveAll(baseDir)
	archive, e := s.git("archive", "--format=zip", "FETCH_HEAD")
	if e != nil {
		return out, e
	}
	z, e := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if e != nil {
		return out, e
	}
	total := uint64(0)
	for _, f := range z.File {
		if f.FileInfo().IsDir() || f.Name == ".gitignore" {
			continue
		}
		if !archivePath(f.Name) || f.Mode()&os.ModeSymlink != 0 {
			return out, fmt.Errorf("remote has unexpected files")
		}
		total += f.UncompressedSize64
		if total > ArchiveLimit {
			return out, fmt.Errorf("remote project exceeds merge budget")
		}
		in, e := f.Open()
		if e != nil {
			return out, e
		}
		data, e := io.ReadAll(io.LimitReader(in, ArchiveLimit+1))
		in.Close()
		if e != nil {
			return out, e
		}
		p := filepath.Join(baseDir, filepath.FromSlash(f.Name))
		if e = os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			return out, e
		}
		if e = os.WriteFile(p, data, 0600); e != nil {
			return out, e
		}
	}
	remote := &Store{Dir: baseDir}
	var incoming Root
	if e = remote.read("revisions", head.Revision, &incoming); e != nil {
		return out, e
	}
	if incoming.World.ID != local.World.ID || incoming.Version != Format {
		return out, fmt.Errorf("remote world identity or format differs")
	}
	if e = remote.validateRoot(incoming); e != nil {
		return out, e
	}
	ancestors, e := s.revisionAncestry(rev, local.World.ID)
	if e != nil {
		return out, e
	}
	remoteAncestors, e := remote.revisionAncestry(head.Revision, local.World.ID)
	if e != nil {
		return out, e
	}
	out.Base, e = mergeBase(ancestors, remoteAncestors)
	if e != nil {
		return out, e
	}
	if out.Base == head.Revision {
		out.AlreadyIncluded = true
		return out, nil
	}
	var base Root
	if e = s.read("revisions", out.Base, &base); e != nil {
		return out, e
	}
	bv, e := s.logical(base)
	if e != nil {
		return out, e
	}
	lv, e := s.logical(local)
	if e != nil {
		return out, e
	}
	rv, e := remote.logical(incoming)
	if e != nil {
		return out, e
	}
	merged := mergeJSON(bv, lv, rv, "", c.Choices, &out.Conflicts)
	if !c.Apply || len(out.Conflicts) > 0 {
		return out, nil
	}
	// Immutable union staging cannot change the active root if validation fails.
	for _, dir := range []string{"objects", "snapshots", "revisions", "assets"} {
		entries, _ := os.ReadDir(filepath.Join(baseDir, dir))
		for _, entry := range entries {
			if !validHash(entry.Name()) {
				return out, fmt.Errorf("invalid remote object")
			}
			data, e := os.ReadFile(filepath.Join(baseDir, dir, entry.Name()))
			if e != nil {
				return out, e
			}
			hash, e := s.putBytes(dir, data)
			if e != nil || hash != entry.Name() {
				return out, fmt.Errorf("remote object integrity failure")
			}
		}
	}
	raw, _ := json.Marshal(merged)
	var result struct {
		Ages map[string]struct {
			Metadata Age               `json:"metadata"`
			Records  map[string]Record `json:"records"`
		} `json:"ages"`
	}
	if e = json.Unmarshal(raw, &result); e != nil {
		return out, e
	}
	root := local
	root.Ages = map[string]Age{}
	root.Message = "Reconciled fetched project revision"
	for id, item := range result.Ages {
		snap := Snapshot{Format, map[string]string{}}
		for key, r := range item.Records {
			hash, e := s.put("objects", r)
			if e != nil {
				return out, e
			}
			snap.Records[key] = hash
		}
		hash, e := s.put("snapshots", snap)
		if e != nil {
			return out, e
		}
		a := item.Metadata
		a.Snapshot = hash
		if prior, ok := local.Ages[id]; ok {
			a.Undo = append(append([]string{}, prior.Undo...), prior.Snapshot)
		} else {
			a.Undo = []string{}
		}
		a.Redo = []string{}
		root.Ages[id] = a
	}
	if _, e = s.commit(rev, root, head.Revision); e != nil {
		return out, e
	}
	out.Applied = true
	out.Revision, _, e = s.root()
	if e != nil {
		return out, e
	}
	if e = s.stageProject(); e != nil {
		return out, e
	}
	tree, e := s.git("write-tree")
	if e != nil {
		return out, e
	}
	commit, e := s.git("-c", "user.name=KRIEMHILD author", "-c", "user.email=author@kriemhild.local", "commit-tree", strings.TrimSpace(string(tree)), "-p", "HEAD", "-p", "FETCH_HEAD", "-m", "Merge KRIEMHILD logical records")
	if e != nil {
		return out, fmt.Errorf("project merged safely; Git merge commit failed: %w", e)
	}
	_, e = s.git("update-ref", "HEAD", strings.TrimSpace(string(commit)))
	return out, e
}
