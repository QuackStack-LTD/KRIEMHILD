package httpapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

// ResetHostedPassword is an offline operator operation. It cannot run while a
// hosted server owns this library, and preserves every account and world role.
func ResetHostedPassword(library, user, password string) error {
	directory, e := filepath.Abs(library)
	if e != nil {
		return e
	}
	info, e := os.Stat(directory)
	if e != nil {
		return e
	}
	if !info.IsDir() {
		return fmt.Errorf("library must be a directory")
	}
	lock := flock.New(filepath.Join(directory, ".access.lock"))
	locked, e := lock.TryLock()
	if e != nil {
		return e
	}
	if !locked {
		return fmt.Errorf("stop the hosted server before resetting a password")
	}
	defer lock.Unlock()
	a := &Access{path: filepath.Join(directory, ".access.json")}
	raw, e := os.ReadFile(a.path)
	if e != nil {
		return e
	}
	if e = json.Unmarshal(raw, &a.data); e != nil {
		return fmt.Errorf("cannot read account store: %w", e)
	}
	if a.data.Users["admin"].Hash == "" || a.data.Roles == nil {
		return fmt.Errorf("invalid account store")
	}
	if _, ok := a.data.Users[user]; !ok {
		return fmt.Errorf("account not found")
	}
	next, e := passwordAccount(password)
	if e != nil {
		return e
	}
	a.data.Users[user] = next
	return a.save()
}
