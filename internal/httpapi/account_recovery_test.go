package httpapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestOfflineAccountRecoveryRequiresStoppedWriterAndPreservesRoles(t *testing.T) {
	directory := t.TempDir()
	s, e := New(directory, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.EnableHosting("https://world.test", "original-admin-password"); e != nil {
		t.Fatal(e)
	}
	other, e := passwordAccount("another-author-password")
	if e != nil {
		t.Fatal(e)
	}
	s.access.data.Users["author"] = other
	s.access.data.Roles["world-fixture"] = map[string]string{"author": "editor"}
	if e = s.access.save(); e != nil {
		t.Fatal(e)
	}
	if e = ResetHostedPassword(directory, "admin", "replacement-admin-password"); e == nil {
		t.Fatal("reset bypassed running server lock")
	}
	s.Close()
	path := filepath.Join(directory, ".access.json")
	before, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = ResetHostedPassword(directory, "missing", "replacement-admin-password"); e == nil {
		t.Fatal("reset created an unknown account")
	}
	if e = ResetHostedPassword(directory, "admin", "short"); e == nil {
		t.Fatal("weak reset accepted")
	}
	unchanged, _ := os.ReadFile(path)
	if string(before) != string(unchanged) {
		t.Fatal("failed reset changed account store")
	}
	if e = ResetHostedPassword(directory, "admin", "replacement-admin-password"); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var after accessData
	if e = json.Unmarshal(raw, &after); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(after.Roles, s.access.data.Roles) || after.Users["author"] != other {
		t.Fatal("recovery changed unrelated accounts/memberships")
	}
	if !passwordMatches(after.Users["admin"], "replacement-admin-password") || passwordMatches(after.Users["admin"], "original-admin-password") {
		t.Fatal("reset credentials not replaced")
	}
	if e = ResetHostedPassword(t.TempDir(), "admin", "replacement-admin-password"); e == nil {
		t.Fatal("reset bootstrapped a missing account store")
	}
}
