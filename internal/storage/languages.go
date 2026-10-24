package storage

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	"kriemhild/internal/languages"
	"strings"
	"time"
)

func linguisticState(e *history.Entity) *languages.State {
	if e == nil {
		return nil
	}
	if e.Kind == "language" {
		return e.Language
	}
	if e.Kind == "script" {
		return e.Script
	}
	return nil
}
func hasLinguisticRecords(d *history.Document) bool {
	for _, e := range d.Entities {
		if e.Kind == "language" || e.Kind == "script" || e.Language != nil || e.Script != nil {
			return true
		}
	}
	return false
}
func hasProductiveRevisions(d *history.Document) bool {
	for _, e := range d.Entities {
		if e.Language != nil {
			for _, ref := range e.Language.Revisions {
				if ref.Schema == languages.ProductiveSchemaVersion {
					return true
				}
			}
		}
	}
	return false
}
func linguisticSchema(raw []byte) int {
	var h struct {
		Schema int `json:"schema"`
	}
	if json.Unmarshal(raw, &h) != nil {
		return 0
	}
	return h.Schema
}
func definitionOwner(raw []byte, kind string) (string, error) {
	if kind == "language" {
		v, e := languages.Decode(raw)
		return v.LanguageID, e
	}
	if kind == "script" {
		v, e := languages.DecodeScript(raw)
		return v.ScriptID, e
	}
	return "", fmt.Errorf("choose a language or script")
}
func canonicalDefinition(raw []byte, kind string) ([]byte, error) {
	if kind == "language" {
		v, err := languages.Decode(raw)
		if err != nil {
			return nil, err
		}
		return languages.Canonical(v), nil
	}
	if kind == "script" {
		v, err := languages.DecodeScript(raw)
		if err != nil {
			return nil, err
		}
		return languages.Canonical(v), nil
	}
	return nil, fmt.Errorf("unsupported linguistic definition type")
}
func validateLinguisticLinks(d *history.Document, age, owner string, links []languages.Link) error {
	for _, l := range links {
		e := d.Entity(age, l.ID)
		if e == nil || e.Deleted || e.ID == owner {
			return fmt.Errorf("relationship target must be another active record in this Age")
		}
		switch l.Kind {
		case "parent", "variety", "cognate", "contact":
			if e.Kind != "language" {
				return fmt.Errorf("language relationships require another language")
			}
		case "script":
			if e.Kind != "script" {
				return fmt.Errorf("script association requires a script")
			}
		case "speaker", "region":
		default:
			return fmt.Errorf("unknown language relationship")
		}
		if l.Revision != "" && linguisticState(e).Ref(l.Revision) == nil {
			return fmt.Errorf("relationship revision is not retained by its target")
		}
	}
	// Parentage is a directed acyclic graph; contact and cognacy may be reciprocal.
	visiting, done := map[string]bool{}, map[string]bool{}
	var walk func(string) error
	walk = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("language ancestry cannot contain a cycle")
		}
		if done[id] {
			return nil
		}
		visiting[id] = true
		edges := links
		if id != owner {
			e := d.Entity(age, id)
			if e == nil || e.Language == nil {
				edges = nil
			} else {
				edges = e.Language.Links
			}
		}
		for _, l := range edges {
			if l.Kind == "parent" {
				if err := walk(l.ID); err != nil {
					return err
				}
			}
		}
		delete(visiting, id)
		done[id] = true
		return nil
	}
	return walk(owner)
}

type LinguisticSaveOptions struct {
	ExpectedDraft *string
	DisplayName   *string
}

// SaveLinguisticDefinition atomically stores the immutable body and advances the
// Age's draft reference. It neither compiles nor activates the draft.
func (s *Store) SaveLinguisticDefinition(ctx context.Context, world, age, kind string, revision int, raw []byte, metadata languages.State, note string, options ...LinguisticSaveOptions) (*history.Document, error) {
	opts := LinguisticSaveOptions{}
	if len(options) > 0 {
		opts = options[0]
	}
	if opts.DisplayName != nil && (strings.TrimSpace(*opts.DisplayName) == "" || len(*opts.DisplayName) > 200) {
		return nil, fmt.Errorf("choose an Age name of at most 200 bytes")
	}
	owner, err := definitionOwner(raw, kind)
	if err != nil {
		return nil, err
	}
	var body []byte
	var name string
	if kind == "language" {
		v, _ := languages.Decode(raw)
		body = languages.Canonical(v)
		name = v.Name
	} else {
		v, _ := languages.DecodeScript(raw)
		body = languages.Canonical(v)
		name = v.Name
	}
	if len(note) > 1000 {
		return nil, fmt.Errorf("revision note exceeds 1,000 bytes")
	}
	tx, err := s.beginHistory(ctx, false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d, err := readHistory(ctx, tx, world)
	if err != nil {
		return nil, err
	}
	if revision != d.World.Revision {
		return nil, fmt.Errorf("World changed; reload before retrying")
	}
	if d.Age(age) == nil {
		return nil, fmt.Errorf("Age not found")
	}
	if err = validateLinguisticLinks(d, age, owner, metadata.Links); err != nil {
		return nil, err
	}
	if kind == "language" {
		v, _ := languages.Decode(body)
		if v.Lexical != nil {
			for _, source := range v.Lexical.Sources {
				e := d.Entity(source.AgeID, source.LanguageID)
				if e == nil || e.Language.Ref(source.Revision) == nil {
					return nil, fmt.Errorf("lexical provenance must pin a retained source language in this World")
				}
				raw, err := linguisticBlob(ctx, tx, source.Revision, "language", source.LanguageID, 0)
				if err != nil {
					return nil, err
				}
				definition, err := languages.Decode(raw)
				if err != nil {
					return nil, err
				}
				found := false
				for _, lex := range definition.Lexicon {
					if lex.ID == source.Lexeme {
						found = true
						break
					}
				}
				if !found {
					return nil, fmt.Errorf("lexical source identity is not present in its pinned definition")
				}
			}
		}
		if o := v.Orthography; o != nil {
			e := d.Entity(age, o.Script.ID)
			if e == nil || e.Kind != "script" || e.Script.Ref(o.Script.Revision) == nil {
				return nil, fmt.Errorf("pin an existing script revision in this Age")
			}
		}
		if h := v.History; h != nil {
			e := d.Entity(h.SourceAge, h.SourceLanguage)
			if e == nil || e.Kind != "language" || e.Language.Ref(h.SourceRevision) == nil {
				return nil, fmt.Errorf("historical source must reference a retained language revision in this World")
			}
		}
	}
	e := d.Entity(age, owner)
	if e != nil && (e.Kind != kind || e.Deleted) {
		return nil, fmt.Errorf("identity already belongs to another or deleted record")
	}
	if e == nil {
		entity := history.Entity{ID: owner, AgeID: age, Name: name, Kind: kind}
		if err = d.Apply(history.Command{Kind: "entity", AgeID: age, Entity: &entity}); err != nil {
			return nil, err
		}
		e = d.Entity(age, owner)
	}
	state := linguisticState(e)
	if opts.ExpectedDraft != nil {
		current := ""
		if state != nil {
			current = state.Draft
		}
		if current != *opts.ExpectedDraft {
			return nil, fmt.Errorf("Language draft changed; review the latest saved definition before retrying")
		}
	}
	if state == nil {
		state = &languages.State{Revisions: []languages.RevisionRef{}}
	} else {
		state = languages.Clone(state)
	}
	h := hash(body)
	if state.Ref(h) == nil {
		if len(state.Revisions) >= languages.MaxRevisions {
			return nil, fmt.Errorf("revision limit reached; export this language before starting a new revision series")
		}
		var header struct {
			Schema int `json:"schema"`
		}
		if err := json.Unmarshal(body, &header); err != nil {
			return nil, err
		}
		state.Revisions = append(state.Revisions, languages.RevisionRef{Hash: h, Schema: header.Schema, Canonicalizer: languages.CanonicalVersion, Size: len(body), Parent: state.Draft, CreatedAt: time.Now().UnixMilli(), Author: Actor(ctx), Note: note})
	}
	state.Draft = h
	state.Kind = metadata.Kind
	state.Status = metadata.Status
	state.Description = metadata.Description
	state.Links = languages.Clone(metadata.Links)
	if kind == "language" {
		e.Language = state
	} else {
		e.Script = state
	}
	if opts.DisplayName != nil {
		e.Name = strings.TrimSpace(*opts.DisplayName)
	}
	e.UpdatedAt = time.Now().UnixMilli()
	if _, err = tx.ExecContext(ctx, "INSERT INTO history_blobs(hash,data) VALUES($1,$2) ON CONFLICT(hash) DO NOTHING", h, body); err != nil {
		return nil, err
	}
	if err = writeHistory(ctx, tx, d, revision, false); err != nil {
		return nil, err
	}
	return d, tx.Commit()
}

// LinguisticDefinition never authorizes by a globally known content hash.
func (s *Store) LinguisticDefinition(ctx context.Context, world, age, id, revision string) ([]byte, error) {
	tx, err := s.beginHistory(ctx, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	d, err := readHistory(ctx, tx, world)
	if err != nil {
		return nil, err
	}
	e := d.Entity(age, id)
	state := linguisticState(e)
	if e == nil || e.Deleted || state == nil {
		return nil, sql.ErrNoRows
	}
	if revision == "" {
		revision = state.Draft
	}
	r := state.Ref(revision)
	if r == nil {
		return nil, sql.ErrNoRows
	}
	raw, err := linguisticBlob(ctx, tx, revision, e.Kind, id, r.Size)
	if err != nil {
		return nil, err
	}
	if linguisticSchema(raw) != r.Schema {
		return nil, fmt.Errorf("definition reference schema does not match its immutable body")
	}
	return raw, tx.Commit()
}
func linguisticBlob(ctx context.Context, q historyQuery, revision, kind, owner string, size int) ([]byte, error) {
	var raw []byte
	if !languages.ValidHash(revision) {
		return nil, fmt.Errorf("invalid definition hash")
	}
	if err := q.QueryRowContext(ctx, "SELECT data FROM history_blobs WHERE hash=$1", revision).Scan(&raw); err != nil {
		return nil, err
	}
	if len(raw) > languages.MaxDefinitionBytes || hash(raw) != revision || size > 0 && size != len(raw) {
		return nil, fmt.Errorf("corrupt definition body")
	}
	id, err := definitionOwner(raw, kind)
	if err != nil || id != owner {
		return nil, fmt.Errorf("definition owner or schema mismatch")
	}
	canonical, err := canonicalDefinition(raw, kind)
	if err != nil || !bytes.Equal(raw, canonical) {
		return nil, fmt.Errorf("stored definition is not canonical for its declared version")
	}
	return raw, nil
}

func (s *Store) LanguageDependencies(ctx context.Context, d languages.Definition) (map[languages.ScriptRef]languages.ScriptDefinition, error) {
	result := map[languages.ScriptRef]languages.ScriptDefinition{}
	if d.Orthography == nil {
		return result, nil
	}
	ref := d.Orthography.Script
	// This internal lookup is reached only after reading an owned language body;
	// Save and import validate that body's pinned dependencies.
	raw, err := linguisticBlob(ctx, s.db, ref.Revision, "script", ref.ID, 0)
	if err != nil {
		return nil, err
	}
	script, err := languages.DecodeScript(raw)
	if err != nil {
		return nil, err
	}
	result[ref] = script
	return result, nil
}

// The caller must first load this target through LinguisticDefinition. Its
// immutable, validated source reference remains reachable after Age deletion.
func (s *Store) LanguageHistoricalSource(ctx context.Context, target languages.Definition) (languages.Definition, error) {
	if target.History == nil {
		return languages.Definition{}, fmt.Errorf("historical source is undecided")
	}
	h := target.History
	raw, err := linguisticBlob(ctx, s.db, h.SourceRevision, "language", h.SourceLanguage, 0)
	if err != nil {
		return languages.Definition{}, err
	}
	return languages.Decode(raw)
}

func (s *Store) RecordLanguageCompile(ctx context.Context, world, age, id string, revision int, report languages.Report) (*history.Document, error) {
	return s.UpdateHistory(ctx, world, revision, func(d *history.Document) error {
		e := d.Entity(age, id)
		if e == nil || e.Deleted || e.Language == nil || e.Language.Ref(report.Revision) == nil {
			return fmt.Errorf("language revision not found")
		}
		refSchema := e.Language.Ref(report.Revision).Schema
		if !languages.CompatibleVersions(refSchema, report.Engine, report.Adapter, report.Compiler) || refSchema == 2 && report.Schema != 2 {
			return fmt.Errorf("unsupported compiler version")
		}
		state := languages.Clone(e.Language)
		ref := languages.CompileRef{Revision: report.Revision, Engine: report.Engine, Adapter: report.Adapter, Compiler: report.Compiler, Dependencies: report.Dependencies, Report: report}
		found := false
		for i, c := range state.Compiled {
			if c.Revision == ref.Revision && c.Engine == ref.Engine && c.Adapter == ref.Adapter && c.Compiler == ref.Compiler {
				state.Compiled[i] = ref
				found = true
				break
			}
		}
		if !found {
			if len(state.Compiled) >= 32 {
				state.Compiled = state.Compiled[1:]
			}
			state.Compiled = append(state.Compiled, ref)
		}
		e.Language = state
		return nil
	})
}
func (s *Store) ActivateLanguage(ctx context.Context, world, age, id, definition string, revision int) (*history.Document, error) {
	return s.UpdateHistory(ctx, world, revision, func(d *history.Document) error {
		e := d.Entity(age, id)
		if e == nil || e.Deleted || e.Language == nil || e.Language.Ref(definition) == nil {
			return fmt.Errorf("language revision not found")
		}
		for _, c := range e.Language.Compiled {
			if c.Revision == definition && languages.CompatibleVersions(e.Language.Ref(definition).Schema, c.Engine, c.Adapter, c.Report.Compiler) && len(languages.Operations(c.Report)) > 0 {
				e.Language = languages.Clone(e.Language)
				e.Language.Active = definition
				return nil
			}
		}
		return fmt.Errorf("compile this revision with at least one ready operation before activation")
	})
}

// Restoration changes a pointer, never an immutable body. Retained provenance
// stays valid even after deletion of its original source Age.
func (s *Store) RestoreLinguisticDraft(ctx context.Context, world, age, id, definition, expectedDraft string, revision int) (*history.Document, error) {
	_, err := s.LinguisticDefinition(ctx, world, age, id, definition)
	if err != nil {
		return nil, err
	}
	return s.UpdateHistory(ctx, world, revision, func(d *history.Document) error {
		e := d.Entity(age, id)
		state := linguisticState(e)
		if e == nil || e.Deleted || state.Ref(definition) == nil {
			return fmt.Errorf("saved language/script revision not found")
		}
		if state.Draft != expectedDraft {
			return fmt.Errorf("Language draft changed; review before restoring")
		}
		state = languages.Clone(state)
		state.Draft = definition
		if e.Kind == "language" {
			e.Language = state
		} else {
			e.Script = state
		}
		e.UpdatedAt = time.Now().UnixMilli()
		return nil
	})
}

// collectLinguisticBodies includes all retained definitions and pinned scripts,
// even when a script's original Age was subsequently removed.
func collectLinguisticBodies(ctx context.Context, q historyQuery, d *history.Document, blobs map[string][]byte) error {
	load := func(ref languages.RevisionRef, kind, owner string) ([]byte, error) {
		return linguisticBlob(ctx, q, ref.Hash, kind, owner, ref.Size)
	}
	return walkLinguisticBodies(d, blobs, load)
}
func validateLinguisticBundle(d *history.Document, blobs map[string][]byte, archiveSchema int) error {
	load := func(ref languages.RevisionRef, kind, owner string) ([]byte, error) {
		raw := blobs[ref.Hash]
		id, err := definitionOwner(raw, kind)
		if err != nil || id != owner || hash(raw) != ref.Hash || ref.Size > 0 && len(raw) != ref.Size {
			return nil, fmt.Errorf("missing, corrupt or incorrectly owned language/script definition")
		}
		canonical, err := canonicalDefinition(raw, kind)
		if err != nil || !bytes.Equal(raw, canonical) {
			return nil, fmt.Errorf("archive definition is not canonical for its declared version")
		}
		if linguisticSchema(raw) == 2 && archiveSchema < 3 {
			return nil, fmt.Errorf("Pinned productive definitions require World archive schema 3")
		}
		return raw, nil
	}
	return walkLinguisticBodies(d, map[string][]byte{}, load)
}

// Follow pinned bodies as well as current Age records: deleting an old Age must
// not make a descendant's reproducible historical source disappear from export.
func walkLinguisticBodies(d *history.Document, blobs map[string][]byte, load func(languages.RevisionRef, string, string) ([]byte, error)) error {
	visited := map[string]bool{}
	var visit func(languages.RevisionRef, string, string, int) error
	visit = func(ref languages.RevisionRef, kind, owner string, depth int) error {
		if depth > 32 || len(visited) >= 4096 {
			return fmt.Errorf("linguistic dependency graph exceeds archive limits")
		}
		key := kind + "/" + owner + "/" + ref.Hash
		if visited[key] {
			return nil
		}
		raw, err := load(ref, kind, owner)
		if err != nil {
			return err
		}
		if ref.Schema != 0 && linguisticSchema(raw) != ref.Schema {
			return fmt.Errorf("definition reference schema does not match its immutable body")
		}
		visited[key] = true
		blobs[ref.Hash] = raw
		if kind == "language" {
			definition, err := languages.Decode(raw)
			if err != nil {
				return err
			}
			if definition.Lexical != nil {
				for _, source := range definition.Lexical.Sources {
					if err := visit(languages.RevisionRef{Hash: source.Revision}, "language", source.LanguageID, depth+1); err != nil {
						return err
					}
				}
			}
			if o := definition.Orthography; o != nil {
				if err := visit(languages.RevisionRef{Hash: o.Script.Revision}, "script", o.Script.ID, depth+1); err != nil {
					return err
				}
			}
			if h := definition.History; h != nil {
				if err := visit(languages.RevisionRef{Hash: h.SourceRevision}, "language", h.SourceLanguage, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, e := range d.Entities {
		for _, state := range []*languages.State{e.Language, e.Script} {
			if state == nil {
				continue
			}
			for _, ref := range state.Revisions {
				if err := visit(ref, e.Kind, e.ID, 0); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
