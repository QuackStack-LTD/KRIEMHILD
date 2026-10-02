package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type LiveRequest struct {
	Age      string          `json:"age"`
	Scene    string          `json:"scene"`
	Client   string          `json:"client"`
	Action   string          `json:"action"`
	After    int             `json:"after"`
	Sequence int             `json:"sequence"`
	Update   string          `json:"update"`
	Document json.RawMessage `json:"document"`
}
type LiveResponse struct {
	Sequence int             `json:"sequence"`
	Updates  []string        `json:"updates"`
	Document json.RawMessage `json:"document"`
	Peers    []string        `json:"peers"`
	Conflict bool            `json:"conflict"`
	Revision string          `json:"revision"`
}
type livePeer struct {
	User string
	Seen time.Time
}

func clearStaleCollaboration(r, before Record) Record {
	seed, _ := r.Properties["_collaborationState"].(string)
	prior, _ := before.Properties["_collaborationState"].(string)
	if r.Kind == "scene" && !bytes.Equal(r.Document, before.Document) && seed == prior {
		r = cloneProperties(r)
		delete(r.Properties, "_collaborationState")
		delete(r.Properties, "_collaborationCheckpoint")
	}
	return r
}

type liveRoom struct {
	Offset  int `json:"offset,omitempty"`
	dirty   bool
	Base    json.RawMessage     `json:"base"`
	Updates []string            `json:"updates"`
	Peers   map[string]livePeer `json:"-"`
}

type liveCheckpoint struct {
	Sequence     int    `json:"sequence"`
	PreviousBase string `json:"previousBase"`
}

func documentHash(doc json.RawMessage) string {
	sum := sha256.Sum256(doc)
	return hex.EncodeToString(sum[:])
}
func (r *liveRoom) sequence() int { return r.Offset + len(r.Updates) }
func (r *liveRoom) compact(document json.RawMessage, seed string) {
	sequence := r.sequence()
	r.Base = bytes.Clone(document)
	r.Updates = []string{seed}
	r.Offset = sequence - 1
	r.dirty = true
}

// Polling needs just one scene. Materializing every record in the Age on every
// one-second poll makes collaborative writing scale with unrelated world data.
func (s *Store) liveScene(age, id string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := State{}
	rev, root, e := s.root()
	if e != nil {
		return out, e
	}
	a, ok := root.Ages[age]
	if !ok {
		return out, fmt.Errorf("Age not found")
	}
	snap, e := s.snapshot(a.Snapshot)
	if e != nil {
		return out, e
	}
	hash, ok := snap.Records[id]
	if !ok {
		return out, fmt.Errorf("scene not found")
	}
	var scene Record
	if e = s.read("objects", hash, &scene); e != nil {
		return out, e
	}
	return State{Revision: rev, Root: root, Age: a, Records: map[string]Record{id: scene}}, nil
}

// Transport is self-hosted HTTP polling. Yjs update merging runs in the Next
// client; Go durably stores bounded updates and validates canonical checkpoints.
func (s *Store) Live(c LiveRequest, user string) (LiveResponse, error) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	out := LiveResponse{Updates: []string{}, Peers: []string{}}
	if !validID(c.Age) || !validID(c.Scene) || !validID(c.Client) || c.After < 0 {
		return out, fmt.Errorf("invalid collaboration context")
	}
	st, e := s.liveScene(c.Age, c.Scene)
	if e != nil {
		return out, e
	}
	scene, ok := st.Records[c.Scene]
	if !ok || scene.Kind != "scene" {
		return out, fmt.Errorf("scene not found")
	}
	key := c.Age + "_" + c.Scene
	dir := filepath.Join(s.Dir, ".kriemhild", "live")
	if s.rooms == nil {
		s.rooms = map[string]*liveRoom{}
	}
	room := s.rooms[key]
	if room == nil {
		// Unmarshal may reuse RawMessage capacity. Keep journal bytes separate
		// from the canonical scene, especially when recovering an older journal.
		room = &liveRoom{Base: bytes.Clone(scene.Document), Updates: []string{}, Peers: map[string]livePeer{}}
		b, e := os.ReadFile(filepath.Join(dir, key+".json"))
		if e == nil {
			if len(b) > 32<<20 {
				return out, fmt.Errorf("collaborative draft exceeds budget")
			}
			if e = json.Unmarshal(b, room); e != nil {
				return out, e
			}
			room.Peers = map[string]livePeer{}
			if room.Offset < 0 || room.sequence() > 9007199254740991 {
				return out, fmt.Errorf("invalid shared draft sequence")
			}
		} else if !os.IsNotExist(e) {
			return out, e
		} else if seed, ok := scene.Properties["_collaborationState"].(string); ok && seed != "" {
			room.Updates = append(room.Updates, seed)
		}
		s.rooms[key] = room
	}
	room.Peers[c.Client] = livePeer{user, time.Now()}
	save := func() error {
		if e := os.MkdirAll(dir, 0700); e != nil {
			return e
		}
		b, e := json.Marshal(room)
		if e != nil {
			return e
		}
		if len(b) > 32<<20 {
			return fmt.Errorf("shared draft log exceeds 32 MiB")
		}
		if e = s.checkpoint("live-journal"); e != nil {
			return e
		}
		return atomicWrite(filepath.Join(dir, key+".json"), b)
	}
	// A canonical checkpoint may have committed immediately before a crash or
	// journal write failure. Only its exact previous-base/sequence marker permits
	// recovery; an ordinary scene edit still produces a visible conflict.
	if !bytes.Equal(room.Base, scene.Document) && len(room.Updates) > 0 {
		var marker liveCheckpoint
		raw, _ := json.Marshal(scene.Properties["_collaborationCheckpoint"])
		seed, _ := scene.Properties["_collaborationState"].(string)
		if json.Unmarshal(raw, &marker) == nil && marker.Sequence == room.sequence() && marker.PreviousBase == documentHash(room.Base) && seed != "" {
			room.compact(scene.Document, seed)
		}
	}
	if room.dirty {
		if e = save(); e != nil {
			return out, fmt.Errorf("draft journal recovery failed: %w", e)
		}
		room.dirty = false
	}
	switch c.Action {
	case "", "poll":
	case "initialize", "update":
		if c.Action == "initialize" && len(room.Updates) > 0 {
			return out, ErrConflict
		}
		decoded, e := base64.StdEncoding.DecodeString(c.Update)
		if e != nil || len(decoded) == 0 || len(decoded) > 2<<20 {
			return out, fmt.Errorf("invalid or oversized document update")
		}
		if len(room.Updates) >= 100000 || room.sequence() >= 9007199254740991 {
			return out, fmt.Errorf("shared draft update budget reached")
		}
		room.Updates = append(room.Updates, c.Update)
		if e = save(); e != nil {
			room.Updates = room.Updates[:len(room.Updates)-1]
			return out, e
		}
	case "checkpoint":
		if c.Sequence != room.sequence() || len(room.Updates) == 0 || !bytes.Equal(scene.Document, room.Base) {
			return out, ErrConflict
		}
		if e := validateDocument(c.Document); e != nil {
			return out, e
		}
		decoded, e := base64.StdEncoding.DecodeString(c.Update)
		if e != nil || len(decoded) == 0 || len(decoded) > 2<<20 {
			return out, fmt.Errorf("invalid checkpoint state")
		}
		scene = cloneProperties(scene)
		scene.Document = c.Document
		scene.Properties["_collaborationState"] = c.Update
		scene.Properties["_collaborationCheckpoint"] = liveCheckpoint{Sequence: room.sequence(), PreviousBase: documentHash(room.Base)}
		next, e := s.Apply(Command{Expected: st.Revision, Age: c.Age, Action: "put", Record: &scene})
		if e != nil {
			return out, e
		}
		room.compact(next.Records[c.Scene].Document, c.Update)
		st = next
		if e = save(); e != nil {
			return out, fmt.Errorf("revision saved but draft journal needs recovery: %w", e)
		}
		room.dirty = false
	default:
		return out, fmt.Errorf("unknown collaboration operation")
	}
	if c.After > room.sequence() {
		return out, fmt.Errorf("draft sequence changed; reopen shared writing")
	}
	out.Sequence = room.sequence()
	index := max(0, c.After-room.Offset)
	out.Updates = append(out.Updates, room.Updates[index:]...)
	out.Document = room.Base
	out.Conflict = !bytes.Equal(st.Records[c.Scene].Document, room.Base)
	out.Revision = st.Revision
	for id, p := range room.Peers {
		if time.Since(p.Seen) > 15*time.Second {
			delete(room.Peers, id)
		} else {
			out.Peers = append(out.Peers, p.User)
		}
	}
	sort.Strings(out.Peers)
	return out, nil
}
