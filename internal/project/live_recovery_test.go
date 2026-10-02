package project

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSharedCheckpointCompactionAndJournalFailureRecovery(t *testing.T) {
	s, st := fixture(t)
	scene := Record{ID: NewID(), Kind: "scene", Name: "Shared chapter", SettingAge: st.Age.ID, SettingSnapshot: st.Age.Snapshot, Document: json.RawMessage(`{"type":"doc","content":[{"type":"paragraph"}]}`)}
	st = put(t, s, st, scene)
	encoded := func(text string) string { return base64.StdEncoding.EncodeToString([]byte(text)) }
	request := LiveRequest{Age: st.Age.ID, Scene: scene.ID, Client: NewID(), Action: "initialize", Update: encoded("seed")}
	for i := 0; i < 4; i++ {
		if _, e := s.Live(request, "Writer"); e != nil {
			t.Fatal(e)
		}
		request.Action = "update"
		request.Update = encoded("increment")
	}
	request.Action = "checkpoint"
	request.Sequence = 4
	request.Update = encoded("full-state-one")
	request.Document = json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"First checkpoint"}]}]}`)
	out, e := s.Live(request, "Writer")
	if e != nil || out.Sequence != 4 || len(out.Updates) != 1 || out.Updates[0] != request.Update {
		t.Fatal("checkpoint was not compacted", e, out)
	}
	request.Action = "poll"
	request.After = 3
	out, e = s.Live(request, "Reader")
	if e != nil || len(out.Updates) != 1 {
		t.Fatal("lagging client did not receive compacted state", e)
	}
	request.After = 4
	out, e = s.Live(request, "Reader")
	if e != nil || len(out.Updates) != 0 {
		t.Fatal("caught-up reader replayed seed", e)
	}
	request.Action = "update"
	request.Update = encoded("next increment")
	out, e = s.Live(request, "Writer")
	if e != nil || out.Sequence != 5 {
		t.Fatal("sequence reset on compaction", e)
	}
	// The native checkpoint commits; only the following live journal write fails.
	s.fail = func(stage string) error {
		if stage == "live-journal" {
			return errors.New("injected draft journal failure")
		}
		return nil
	}
	request.Action = "checkpoint"
	request.Sequence = 5
	request.Update = encoded("full-state-two")
	request.Document = json.RawMessage(`{"type":"doc", "content":[{"type":"paragraph","content":[{"type":"text","text":"Second checkpoint"}]}]}`)
	if _, e = s.Live(request, "Writer"); e == nil {
		t.Fatal("journal failure not reported")
	}
	canonical, e := s.State(st.Age.ID)
	if e != nil || textDocument(canonical.Records[scene.ID].Document) != "Second checkpoint" {
		t.Fatal("canonical checkpoint lost", e)
	}
	dir := s.Dir
	s.Close()
	reopened, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	request.Action = "poll"
	request.After = 0
	out, e = reopened.Live(request, "Reader")
	if e != nil || out.Conflict || out.Sequence != 5 || len(out.Updates) != 1 || out.Updates[0] != encoded("full-state-two") {
		t.Fatal("checkpoint journal recovery failed", e, out)
	}
	raw, e := os.ReadFile(filepath.Join(dir, ".kriemhild", "live", st.Age.ID+"_"+scene.ID+".json"))
	if e != nil {
		t.Fatal(e)
	}
	var journal liveRoom
	if json.Unmarshal(raw, &journal) != nil || journal.Offset != 4 || len(journal.Updates) != 1 {
		t.Fatal("recovered journal not durable")
	}
	external, _ := reopened.State(st.Age.ID)
	changed := external.Records[scene.ID]
	changed.Document = json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Ordinary editor changed this."}]}]}`)
	external = put(t, reopened, external, changed)
	if _, ok := external.Records[scene.ID].Properties["_collaborationCheckpoint"]; ok {
		t.Fatal("ordinary edit retained checkpoint recovery marker")
	}
	out, e = reopened.Live(request, "Reader")
	if e != nil || !out.Conflict {
		t.Fatal("compaction recovery hid an ordinary edit conflict", e)
	}
}
