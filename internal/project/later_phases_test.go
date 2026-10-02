package project

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConnectedDomainsQuantitiesWritingAndAgeIsolation(t *testing.T) {
	s, st := fixture(t)
	records := []Record{}
	for _, d := range DomainCatalog() {
		records = append(records, Record{ID: NewID(), Kind: "entity", Name: d.Name, Type: d.Name, Properties: map[string]any{"_domain": d.ID}})
	}
	city := records[0]
	city.Properties["population"] = map[string]any{"mode": "range", "min": "1000", "max": "1500", "unit": "people"}
	records[0] = city
	for _, r := range append([]Record{}, records[1:]...) {
		records = append(records, Record{ID: NewID(), Kind: "relation", Name: "connected to", From: city.ID, To: r.ID})
	}
	st = apply(t, s, st, Command{Action: "put-many", Records: records})
	scene := simpleScene(st)
	scene.References = []string{city.ID}
	scene.Properties = map[string]any{"locationID": city.ID, "goal": "Reach the port"}
	st = put(t, s, st, scene)
	source := st
	b := apply(t, s, st, Command{Action: "copy-age", Name: "Later society"})
	changed := cloneProperties(b.Records[city.ID])
	changed.Properties["population"] = map[string]any{"mode": "qualitative", "text": "crowded and growing"}
	b = put(t, s, b, changed)
	original, e := s.State(source.Age.ID)
	if e != nil || !reflect.DeepEqual(original.Records, source.Records) {
		t.Fatal("domain history changed", e)
	}
	if b.Records[scene.ID].SettingSnapshot != scene.SettingSnapshot {
		t.Fatal("scene lost setting pin")
	}
	graph := Related(b.Records, city.ID, 8)
	if len(graph.Nodes) != len(DomainCatalog()) || graph.Truncated {
		t.Fatalf("domain graph: %d", len(graph.Nodes))
	}
	changed.Properties["population"] = map[string]any{"mode": "range", "min": "5", "max": "1"}
	if _, e = s.Apply(Command{Expected: b.Revision, Age: b.Age.ID, Action: "put", Record: &changed}); e == nil {
		t.Fatal("reversed quantity range accepted")
	}
	a := Record{ID: NewID(), Kind: "relation", Name: "divine parent of", From: records[1].ID, To: city.ID}
	b = put(t, s, b, a)
	if Related(b.Records, city.ID, 8).Truncated {
		t.Fatal("cycle broke graph traversal")
	}
}
func TestArchivePreviewMigrationAndPublicAllowlist(t *testing.T) {
	s, st := fixture(t)
	public := Record{ID: NewID(), Kind: "entity", Name: "Visible <city>", Type: "Settlement", Notes: "Public history", Properties: map[string]any{"secret": "DO_NOT_EXPORT"}}
	private := Record{ID: NewID(), Kind: "note", Name: "HIDDEN_RECORD", Notes: "Hidden notes"}
	st = put(t, s, st, public)
	st = put(t, s, st, private)
	scene := simpleScene(st)
	st = put(t, s, st, scene)
	st = apply(t, s, st, Command{Action: "copy-age", Name: "Copied"})
	original, _ := os.ReadFile(filepath.Join(s.Dir, "project.head.json"))
	data, e := s.Archive()
	if e != nil {
		t.Fatal(e)
	}
	library := t.TempDir()
	preview, e := PrepareImport(library, data)
	if e != nil {
		t.Fatal(e)
	}
	if preview.World.ID == st.Root.World.ID || preview.Ages != 2 {
		t.Fatal("import did not create independent identity")
	}
	if _, e = os.Stat(filepath.Join(library, preview.World.ID)); !os.IsNotExist(e) {
		t.Fatal("preview made a world visible")
	}
	if e = AcceptImport(library, preview); e != nil {
		t.Fatal(e)
	}
	copy, e := Open(filepath.Join(library, preview.World.ID))
	if e != nil {
		t.Fatal(e)
	}
	defer copy.Close()
	imported, e := copy.State(st.Age.ID)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(imported.Records, st.Records) {
		t.Fatal("native archive lost record content")
	}
	after, _ := os.ReadFile(filepath.Join(s.Dir, "project.head.json"))
	if !bytes.Equal(original, after) {
		t.Fatal("archive changed source")
	}
	for _, format := range []string{"html", "publication", "markdown", "epub", "docx", "fountain"} {
		out, e := s.Export(ExportRequest{Snapshot: st.Age.Snapshot, IDs: []string{public.ID, scene.ID}, Format: format, Title: "A <book>"})
		if e != nil {
			t.Fatal(e)
		}
		if format == "epub" || format == "docx" {
			z, e := zip.NewReader(bytes.NewReader(out.Data), int64(len(out.Data)))
			if e != nil {
				t.Fatal(e)
			}
			if format == "epub" && (z.File[0].Name != "mimetype" || z.File[0].Method != zip.Store) {
				t.Fatal("EPUB mimetype is not first and uncompressed")
			}
			for _, f := range z.File {
				r, _ := f.Open()
				b, _ := io.ReadAll(r)
				r.Close()
				if bytes.Contains(b, []byte("DO_NOT_EXPORT")) || bytes.Contains(b, []byte("HIDDEN_RECORD")) {
					t.Fatal("export leaked unselected content")
				}
				if strings.HasSuffix(f.Name, "xml") || strings.HasSuffix(f.Name, "xhtml") || strings.HasSuffix(f.Name, "opf") {
					d := xml.NewDecoder(bytes.NewReader(b))
					for {
						_, e = d.Token()
						if e == io.EOF {
							break
						}
						if e != nil {
							t.Fatalf("invalid XML %s: %v", f.Name, e)
						}
					}
				}
			}
		} else if bytes.Contains(out.Data, []byte("DO_NOT_EXPORT")) || bytes.Contains(out.Data, []byte("HIDDEN_RECORD")) {
			t.Fatal("allowlist leak")
		}
	}
	bad, e := zipFiles(map[string][]byte{"../escape": []byte("bad")}, false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = PrepareImport(library, bad); e == nil {
		t.Fatal("archive traversal accepted")
	}
	// Legacy root/snapshot migration preserves historical document references.
	legacy := st.Root
	legacy.Version = 1
	hash, e := s.put("revisions", legacy)
	if e != nil {
		t.Fatal(e)
	}
	head, _ := json.Marshal(Head{hash})
	files := map[string][]byte{"project.head.json": head}
	for _, dir := range []string{"objects", "snapshots", "revisions", "assets"} {
		items, _ := os.ReadDir(filepath.Join(s.Dir, dir))
		for _, item := range items {
			b, _ := os.ReadFile(filepath.Join(s.Dir, dir, item.Name()))
			files[dir+"/"+item.Name()] = b
		}
	}
	archive, e := zipFiles(files, false)
	if e != nil {
		t.Fatal(e)
	}
	migrated, e := PrepareImport(library, archive)
	if e != nil {
		t.Fatal(e)
	}
	if migrated.SourceFormat != 1 {
		t.Fatal("migration version")
	}
	if e = AcceptImport(library, migrated); e != nil {
		t.Fatal(e)
	}
}
func TestExperimentsAreDeterministicAndRequireAcceptance(t *testing.T) {
	s, st := fixture(t)
	lex := Record{ID: NewID(), Kind: "entity", Name: "A word", Type: "Lexeme", Properties: map[string]any{"_domain": "lexeme", "phonemes": "p a t a", "form": "pata"}}
	st = put(t, s, st, lex)
	req := ExperimentRequest{Expected: st.Revision, Age: st.Age.ID, Operation: "sound-change", IDs: []string{lex.ID}, Values: map[string]string{"rules": "p > f\nt > θ"}}
	out, e := s.Experiment(req)
	if e != nil {
		t.Fatal(e)
	}
	if len(out.Proposals) != 1 || out.Proposals[0].Properties["phonemes"] != "f a θ a" {
		t.Fatal("token sound changes failed")
	}
	again, _ := s.Experiment(req)
	if !reflect.DeepEqual(out, again) {
		t.Fatal("experiment nondeterminism")
	}
	saved, _ := s.State(st.Age.ID)
	if saved.Revision != st.Revision {
		t.Fatal("preview mutated canon")
	}
	st = apply(t, s, st, Command{Action: "put-many", Records: out.Proposals})
	if _, e = s.Experiment(req); e != ErrConflict {
		t.Fatal("stale preview not rejected")
	}
	p, e := s.Experiment(ExperimentRequest{Expected: st.Revision, Age: st.Age.ID, Operation: "production", Values: map[string]string{"supply": "0.1", "demand": "0.2", "duration": "3", "unit": "coins"}})
	if e != nil || p.Rows[0]["balance (exact)"] != "-3/10" {
		t.Fatal("decimal calculation lost precision", e)
	}
}
func TestSemanticMergeCombinesFieldsAndSurfacesConflicts(t *testing.T) {
	base := map[string]any{"name": "Port", "properties": map[string]any{"a": "1", "b": "2"}, "document": []any{"one"}}
	local := map[string]any{"name": "Local port", "properties": map[string]any{"a": "3", "b": "2"}, "document": []any{"mine"}}
	remote := map[string]any{"name": "Remote port", "properties": map[string]any{"a": "1", "b": "4"}, "document": []any{"theirs"}}
	conflicts := []MergeConflict{}
	out := mergeJSON(base, local, remote, "", nil, &conflicts).(map[string]any)
	if len(conflicts) != 2 || out["properties"].(map[string]any)["b"] != "4" {
		t.Fatal("semantic field merge")
	}
	conflicts = nil
	out = mergeJSON(base, local, remote, "", map[string]string{"/name": "remote", "/document": "local"}, &conflicts).(map[string]any)
	if len(conflicts) != 0 || out["name"] != "Remote port" {
		t.Fatal("explicit conflict resolution")
	}
	if _, e := exec.LookPath("git"); e != nil {
		t.Skip("git unavailable")
	}
	s, st := fixture(t)
	if e := s.GitAction(GitCommand{Action: "init"}); e != nil {
		t.Fatal(e)
	}
	if e := s.GitAction(GitCommand{Action: "commit", Message: "Initial world"}); e != nil {
		t.Fatal(e)
	}
	info, e := s.GitStatus()
	if e != nil || info.Changes != "" || !info.Initialized {
		t.Fatal("local Git integration", e, info)
	}
	st = put(t, s, st, Record{ID: NewID(), Kind: "note", Name: "Changed"})
	info, e = s.GitStatus()
	if e != nil || info.Changes == "" {
		t.Fatal("Git did not detect new project revision")
	}
	if e = s.GitAction(GitCommand{Action: "remote", Remote: "https://token@example.com/repo"}); e == nil {
		t.Fatal("credential URL accepted")
	}
}
func TestLiveDraftDurabilityAndCheckpointConflict(t *testing.T) {
	s, st := fixture(t)
	scene := simpleScene(st)
	st = put(t, s, st, scene)
	request := LiveRequest{Age: st.Age.ID, Scene: scene.ID, Client: NewID()}
	r, e := s.Live(request, "Writer")
	if e != nil || r.Sequence != 0 {
		t.Fatal("open draft", e)
	}
	request.Action = "initialize"
	request.Update = base64.StdEncoding.EncodeToString([]byte{0, 0})
	r, e = s.Live(request, "Writer")
	if e != nil || r.Sequence != 1 {
		t.Fatal("initialize draft", e)
	}
	if _, e = s.Live(request, "Other"); e != ErrConflict {
		t.Fatal("initialization race not rejected")
	}
	dir := s.Dir
	s.Close()
	reopened, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	request.Action = "poll"
	r, e = reopened.Live(request, "Writer")
	if e != nil || r.Sequence != 1 || len(r.Updates) != 1 {
		t.Fatal("shared draft lost on restart", e)
	}
	external, _ := reopened.State(st.Age.ID)
	scene.Document = json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"An external revision."}]}]}`)
	external = put(t, reopened, external, scene)
	request.Action = "checkpoint"
	request.Sequence = 1
	request.Document = json.RawMessage(`{"type":"doc","content":[{"type":"paragraph"}]}`)
	if _, e = reopened.Live(request, "Writer"); e != ErrConflict {
		t.Fatal("shared checkpoint overwrote external scene")
	}
	saved, _ := reopened.State(st.Age.ID)
	if saved.Revision != external.Revision {
		t.Fatal("conflicting checkpoint mutated revision")
	}
}

func TestFetchedGitMergePreservesBothAuthorsAndCancel(t *testing.T) {
	if _, e := exec.LookPath("git"); e != nil {
		t.Skip("git unavailable")
	}
	local, st := fixture(t)
	city := Record{ID: NewID(), Kind: "entity", Name: "Port", Type: "Settlement", Notes: "Original notes"}
	st = put(t, local, st, city)
	for _, c := range []GitCommand{{Action: "init"}, {Action: "commit", Message: "Common world"}} {
		if e := local.GitAction(c); e != nil {
			t.Fatal(e)
		}
	}
	git := func(dir string, args ...string) string {
		t.Helper()
		out, e := exec.Command("git", append([]string{"-C", dir, "-c", "protocol.file.allow=always"}, args...)...).CombinedOutput()
		if e != nil {
			t.Fatalf("git %v: %v %s", args, e, out)
		}
		return strings.TrimSpace(string(out))
	}
	remoteDir := filepath.Join(t.TempDir(), "remote")
	git(t.TempDir(), "clone", "--no-hardlinks", local.Dir, remoteDir)
	remote, e := Open(remoteDir)
	if e != nil {
		t.Fatal(e)
	}
	defer remote.Close()
	remoteState, e := remote.State(st.Age.ID)
	if e != nil {
		t.Fatal(e)
	}
	remoteCity := city
	remoteCity.Notes = "Remote author's history"
	remoteState = put(t, remote, remoteState, remoteCity)
	if e = remote.GitAction(GitCommand{Action: "commit", Message: "History"}); e != nil {
		t.Fatal(e)
	}
	city.Name = "Local harbor"
	st = put(t, local, st, city)
	if e = local.GitAction(GitCommand{Action: "commit", Message: "Rename"}); e != nil {
		t.Fatal(e)
	}
	localParent := git(local.Dir, "rev-parse", "HEAD")
	remoteParent := git(remoteDir, "rev-parse", "HEAD")
	git(local.Dir, "fetch", remoteDir, "main")
	preview, e := local.GitMerge(MergeRequest{Expected: st.Revision})
	if e != nil || len(preview.Conflicts) != 0 {
		t.Fatal("preview", e, preview)
	}
	unchanged, _ := local.State(st.Age.ID)
	if unchanged.Revision != st.Revision || git(local.Dir, "rev-parse", "HEAD") != localParent {
		t.Fatal("merge preview changed active history")
	}
	result, e := local.GitMerge(MergeRequest{Expected: st.Revision, Remote: preview.Remote, Apply: true})
	if e != nil || !result.Applied {
		t.Fatal("apply", e, result)
	}
	merged, e := local.State(st.Age.ID)
	if e != nil || merged.Records[city.ID].Name != city.Name || merged.Records[city.ID].Notes != remoteCity.Notes {
		t.Fatal("lost authored fields", e)
	}
	parents := git(local.Dir, "show", "-s", "--format=%P", "HEAD")
	if parents != localParent+" "+remoteParent {
		t.Fatal("Git ancestry lost", parents)
	}
	info, e := local.GitStatus()
	if e != nil || info.Changes != "" {
		t.Fatal("merged Git tree is dirty", e, info)
	}
	// A second pair of conflicting edits must preserve the local project until resolved.
	localCity := merged.Records[city.ID]
	localCity.Name = "Mine"
	merged = put(t, local, merged, localCity)
	if e = local.GitAction(GitCommand{Action: "commit", Message: "Mine"}); e != nil {
		t.Fatal(e)
	}
	remoteCity.Name = "Theirs"
	remoteState = put(t, remote, remoteState, remoteCity)
	if e = remote.GitAction(GitCommand{Action: "commit", Message: "Theirs"}); e != nil {
		t.Fatal(e)
	}
	git(local.Dir, "fetch", remoteDir, "main")
	conflict, e := local.GitMerge(MergeRequest{Expected: merged.Revision, Apply: true})
	if e != nil || len(conflict.Conflicts) != 1 || conflict.Applied {
		t.Fatal("conflict review", e, conflict)
	}
	choices := map[string]string{conflict.Conflicts[0].Path: "remote"}
	accepted, e := local.GitMerge(MergeRequest{Expected: merged.Revision, Remote: conflict.Remote, Choices: choices, Apply: true})
	if e != nil || !accepted.Applied {
		t.Fatal("conflict acceptance", e, accepted)
	}
	final, _ := local.State(st.Age.ID)
	if final.Records[city.ID].Name != "Theirs" || final.Records[city.ID].Notes != remoteCity.Notes {
		t.Fatal("conflict choice lost content")
	}
	if len(final.Root.MergeParents) != 1 || final.Root.MergeParents[0] != remoteState.Revision {
		t.Fatal("native history lost fetched parent")
	}
	beforeRepeat := final.Revision
	repeated, e := local.GitMerge(MergeRequest{Expected: final.Revision, Apply: true})
	if e != nil || !repeated.AlreadyIncluded || repeated.Applied {
		t.Fatal("repeated fetch created another merge", e, repeated)
	}
	final, _ = local.State(st.Age.ID)
	if final.Revision != beforeRepeat {
		t.Fatal("already-included merge changed project")
	}
	// Remote changes again without first pulling the merged branch. The native
	// second parent is the base; the old pre-merge fork would invent a conflict.
	remoteCity.Name = "Remote harbor after merge"
	remoteState = put(t, remote, remoteState, remoteCity)
	if e = remote.GitAction(GitCommand{Action: "commit", Message: "Later remote name"}); e != nil {
		t.Fatal(e)
	}
	localCity = final.Records[city.ID]
	localCity.Notes = "Local history after merge"
	final = put(t, local, final, localCity)
	if len(final.Root.MergeParents) != 0 {
		t.Fatal("ordinary edit inherited previous merge parents")
	}
	if e = local.GitAction(GitCommand{Action: "commit", Message: "Later local history"}); e != nil {
		t.Fatal(e)
	}
	git(local.Dir, "fetch", remoteDir, "main")
	later, e := local.GitMerge(MergeRequest{Expected: final.Revision, Apply: true})
	if e != nil || len(later.Conflicts) != 0 || !later.Applied {
		t.Fatal("later synchronization used stale base", e, later)
	}
	final, _ = local.State(st.Age.ID)
	if final.Records[city.ID].Name != remoteCity.Name || final.Records[city.ID].Notes != localCity.Notes {
		t.Fatal("later synchronization lost edits")
	}
	archive, e := local.Archive()
	if e != nil {
		t.Fatal(e)
	}
	library := t.TempDir()
	importPreview, e := PrepareImport(library, archive)
	if e != nil {
		t.Fatal(e)
	}
	if e = AcceptImport(library, importPreview); e != nil {
		t.Fatal(e)
	}
	marker := importPreview.World
	imported, e := Open(filepath.Join(library, marker.ID))
	if e != nil {
		t.Fatal(e)
	}
	defer imported.Close()
	migrated, e := imported.State(st.Age.ID)
	if e != nil {
		t.Fatal(e)
	}
	if len(migrated.Root.MergeParents) != 1 || migrated.Root.MergeParents[0] == remoteState.Revision {
		t.Fatal("migration lost or failed to rewrite merge parent")
	}
	if _, e = imported.revisionAncestry(migrated.Revision, marker.ID); e != nil {
		t.Fatal("migrated merge graph is broken", e)
	}
}
