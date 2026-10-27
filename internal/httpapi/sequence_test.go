package httpapi

import (
	"encoding/json"
	"kriemhild/internal/languages"
	"os"
	"strings"
	"testing"
)

func TestLanguagesSequenceAPIReadinessPermissionsAndUnicode(t *testing.T) {
	s, db, owner, viewer, doc := secureFixture(t)
	share(t, db, owner, viewer, doc.World.ID, "viewer")
	base := "/api/worlds/" + doc.World.ID + "/languages"
	age := doc.World.CurrentAge
	read := func(name string) []byte {
		t.Helper()
		raw, e := os.ReadFile("../languages/testdata/" + name)
		if e != nil {
			t.Fatal(e)
		}
		return raw
	}
	for _, pair := range [][2]string{{"script", "sequence-script.json"}, {"language", "sequence.json"}} {
		var definition any
		if e := json.Unmarshal(read(pair[1]), &definition); e != nil {
			t.Fatal(e)
		}
		w := secured(s, owner, "POST", base, string(languages.Canonical(map[string]any{"kind": pair[0], "ageId": age, "revision": doc.World.Revision, "definition": definition, "metadata": languages.State{}})))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
		if e := json.Unmarshal(w.Body.Bytes(), &doc); e != nil {
			t.Fatal(e)
		}
	}
	d, err := languages.Decode(read("sequence.json"))
	if err != nil {
		t.Fatal(err)
	}
	path := base + "/" + d.LanguageID
	input := map[string]any{"ageId": age, "revision": doc.World.Revision, "definitionRevision": languages.Hash(d)}
	if w := secured(s, viewer, "POST", path+"/compile", string(languages.Canonical(input))); w.Code != 403 {
		t.Fatal("viewer persisted compilation", w.Code)
	}
	w := secured(s, owner, "POST", path+"/compile", string(languages.Canonical(input)))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"spell":true`) {
		t.Fatal(w.Code, w.Body)
	}
	for _, name := range []string{"sequence-forward", "sequence-inverse"} {
		var q languages.Request
		json.Unmarshal(read(name+".request.json"), &q)
		input["request"] = q
		w = secured(s, viewer, "POST", path+"/run", string(languages.Canonical(input)))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
		var actual struct {
			Result languages.Result `json:"result"`
		}
		json.Unmarshal(w.Body.Bytes(), &actual)
		var expected languages.Result
		json.Unmarshal(read(name+".golden.json"), &expected)
		if string(languages.Canonical(actual.Result)) != string(languages.Canonical(expected)) {
			t.Fatal("API differed from pinned full golden", name)
		}
	}
	malformed := `{"ageId":"` + age + `","request":{"text":"` + string([]byte{0xff}) + `"}}`
	w = secured(s, owner, "POST", path+"/run", malformed)
	if w.Code != 400 {
		t.Fatal("invalid UTF-8 accepted", w.Code)
	}
	w = secured(s, viewer, "GET", base+"/capabilities", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"engine":"languages/4"`) {
		t.Fatal("capability profile missing", w.Code, w.Body)
	}
}
