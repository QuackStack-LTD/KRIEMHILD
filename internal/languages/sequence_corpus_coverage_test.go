package languages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// All expectations are external. Tests never capture Execute output as expected
// data, promote review status, or write fixture files.
func coverageCorpus(t testing.TB, name string) (SequenceCorpus, CorpusResolver) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "sequence-corpus", name+".corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := DecodeSequenceCorpus(raw)
	if err != nil {
		t.Fatal(name, err)
	}
	_, old := reviewedSequenceCorpus(t)
	files, err := filepath.Glob("testdata/sequence-corpus/*.definition.json")
	if err != nil {
		t.Fatal(err)
	}
	bodies := map[string][]byte{}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		d, err := Decode(raw)
		if err != nil {
			t.Fatal(file, err)
		}
		bodies[d.LanguageID+Hash(d)] = Canonical(d)
	}
	return c, func(ctx context.Context, kind, id, revision string) ([]byte, error) {
		if kind == "language" {
			if raw, ok := bodies[id+revision]; ok {
				return append([]byte{}, raw...), nil
			}
		}
		return old(ctx, kind, id, revision)
	}
}

func TestSequenceCorpusDedicatedCoverageAndReviewGate(t *testing.T) {
	files, err := filepath.Glob("testdata/sequence-corpus/*.corpus.json")
	if err != nil || len(files) != 25 {
		t.Fatal("dedicated external fixtures missing", len(files), err)
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".corpus.json")
		t.Run(name, func(t *testing.T) {
			c, resolve := coverageCorpus(t, name)
			v, err := RunSequenceCorpus(context.Background(), c, resolve, true)
			if err != nil || v.Status != "success" || v.Calls != 0 || len(v.Cases) != 0 || v.Version != c.Version {
				t.Fatal("preflight", v.Status, err)
			}
			first, err := RunSequenceCorpus(context.Background(), c, resolve, false)
			if err != nil || len(first.Cases) != 1 {
				t.Fatal("execute", err, first.Status)
			}
			r := first.Cases[0]
			if !r.Complete || len(r.Diffs) != 0 || r.ActualOutcome != c.Cases[0].Outcome {
				t.Fatal(CorpusSummary(first))
			}
			if c.Cases[0].Review.Status == "reviewed" {
				if !r.Passed || first.Status != "success" {
					t.Fatal("reviewed fixture did not pass", CorpusSummary(first))
				}
			} else if r.Passed || r.Status != "review_required" || first.Status != "failed" {
				t.Fatal("pending expectation counted as reviewed", CorpusSummary(first))
			}
			second, err := RunSequenceCorpus(context.Background(), c, resolve, false)
			if err != nil || string(Canonical(first)) != string(Canonical(second)) {
				t.Fatal("rebuild/process-independent transport changed", err)
			}
		})
	}
}

func TestSequenceCorpusTypedCoordinatesCannotBeInterchanged(t *testing.T) {
	c, _ := coverageCorpus(t, "unicode-coordinates")
	for i := range c.Cases[0].Coordinates {
		bad := Clone(c)
		bad.Cases[0].Coordinates[i].Spans[1].End++
		if ValidateSequenceCorpus(bad) == nil {
			t.Fatal("inexact typed coordinate accepted", i)
		}
	}
	bad := Clone(c)
	bad.Cases[0].Coordinates[0], bad.Cases[0].Coordinates[1] = bad.Cases[0].Coordinates[1], bad.Cases[0].Coordinates[0]
	if ValidateSequenceCorpus(bad) == nil {
		t.Fatal("coordinate kinds silently interchanged")
	}
	c.Version = SequenceCorpusVersion
	if ValidateSequenceCorpus(c) == nil {
		t.Fatal("old schema silently accepts new coordinate expectations")
	}
}

func TestSequenceCorpusRejectsMalformedBoundaryTopology(t *testing.T) {
	c, _ := coverageCorpus(t, "topology-exact-forward")
	p := OrthoUnit{Kind: "segment", Value: "p"}
	b := OrthoUnit{Kind: "boundary", Value: "morpheme"}
	for _, units := range [][]OrthoUnit{{b, p}, {p, b}, {p, b, b, p}} {
		bad := Clone(c)
		bad.Cases[0].Request.Spelling.Units = units
		bad.Cases[0].Expected.Input = bad.Cases[0].Request
		if err := ValidateSequenceCorpus(bad); err == nil || !strings.Contains(err.Error(), "boundary topology") {
			t.Fatal("boundary shifted/merged instead of rejected", units, err)
		}
	}
}

func TestSequenceCorpusDefinitionValidationStrictShapesAndReviewGate(t *testing.T) {
	c, resolve := coverageCorpus(t, "invalid-layer")
	for _, edit := range []func(*SequenceCase){
		func(c *SequenceCase) { c.Phase = "execution" },
		func(c *SequenceCase) { c.Request.Operation = "spell" },
		func(c *SequenceCase) { c.Validation = nil },
		func(c *SequenceCase) { c.Expected = &Result{} },
		func(c *SequenceCase) { c.Outcome = "success" },
		func(c *SequenceCase) { c.Validation.Status = "compiled" },
		func(c *SequenceCase) { c.Validation.Ready["spell"] = true },
		func(c *SequenceCase) { c.Validation.Diagnostics = nil },
		func(c *SequenceCase) { c.Validation.Diagnostics[0].Severity = "warning" },
		func(c *SequenceCase) { c.Validation.DiagnosticsOmitted = 1 },
		func(c *SequenceCase) { c.Request.Segments = []string{"p"} },
		func(c *SequenceCase) { c.Domain = &CorpusDomain{} },
	} {
		bad := Clone(c)
		edit(&bad.Cases[0])
		if ValidateSequenceCorpus(bad) == nil {
			t.Fatal("invalid validation shape accepted")
		}
	}
	bad := Clone(c)
	bad.Cases[0].Validation.Diagnostics[0].RuleID = "wrong-stage"
	r, err := RunSequenceCorpus(context.Background(), bad, resolve, false)
	if err != nil || r.Cases[0].Passed || len(r.Cases[0].Diffs) == 0 || !strings.Contains(r.Cases[0].Diffs[0].Path, "validation.diagnostics") {
		t.Fatal("validation diagnostics were not compared", err, CorpusSummary(r))
	}
	bad = Clone(c)
	delete(bad.Cases[0].Validation.Ready, "generate")
	r, err = RunSequenceCorpus(context.Background(), bad, resolve, false)
	if err != nil || r.Cases[0].Passed || len(r.Cases[0].Diffs) == 0 {
		t.Fatal("readiness omission ignored", err)
	}
	bad = Clone(c)
	bad.Cases[0].Review.Status = "pending"
	r, err = RunSequenceCorpus(context.Background(), bad, resolve, false)
	if err != nil || r.Cases[0].Passed || r.Cases[0].Status != "review_required" || r.Cases[0].Actual != nil || r.Cases[0].Validation == nil {
		t.Fatal("definition validation bypassed review or executed a request", err)
	}
	old, _ := reviewedSequenceCorpus(t)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(Canonical(old.Cases[0]), &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"phase", "validation", "coordinates"} {
		for _, value := range []string{"null", "[]", `""`} {
			raw[key] = json.RawMessage(value)
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(Canonical(old), &envelope); err != nil {
				t.Fatal(err)
			}
			envelope["cases"] = Canonical([]any{raw})
			if _, err := DecodeSequenceCorpus(Canonical(envelope)); err == nil {
				t.Fatal("/1 accepted a /2 field", key, value)
			}
		}
		delete(raw, key)
	}
}

func TestSequenceCorpusExactBoundaryA2RetainsReviewedDerivations(t *testing.T) {
	c, resolve := coverageCorpus(t, "topology-exact-inverse-a2")
	old, _ := reviewedSequenceCorpus(t)
	e := c.Cases[0].Expected.Spelling.Preimages[0]
	prior := old.Cases[1].Expected.Spelling.Preimages[0]
	if string(Canonical(e.Analyses)) != string(Canonical(prior.Analyses)) || string(Canonical(e.Derivations)) != string(Canonical(prior.Derivations)) {
		t.Fatal("historical A2 candidate/derivation golden bytes changed")
	}
	for _, d := range e.Derivations {
		step := d.Steps[len(d.Steps)-1]
		if step.Stage != "erase-boundaries" || len(step.Before) != 5 || step.Before[3].Boundary != "morpheme" || len(step.After) != 4 {
			t.Fatal("missing exact pre/post-erasure topology", d)
		}
	}
	wrong := Clone(c)
	p := &wrong.Cases[0].Expected.Spelling.Preimages[0]
	p.Units[2], p.Units[3] = p.Units[3], p.Units[2]
	r, err := RunSequenceCorpus(context.Background(), wrong, resolve, false)
	if err != nil || len(r.Cases) != 1 || len(r.Cases[0].Diffs) == 0 {
		t.Fatal("shifted boundary accepted", err)
	}
	for _, name := range []string{"topology-mismatch-forward", "topology-mismatch-inverse-a2", "topology-missing-forward", "topology-missing-inverse-a2"} {
		c, resolve = coverageCorpus(t, name)
		r, err = RunSequenceCorpus(context.Background(), c, resolve, false)
		if err != nil || len(r.Cases[0].Diffs) != 0 {
			t.Fatal("negative topology differs", err, CorpusSummary(r))
		}
	}
}

func TestSequenceCorpusVersion2IsAdditiveAndStrict(t *testing.T) {
	for _, name := range []string{"deletion-inverse-unsupported", "nfd-inverse-unsupported", "inverse-disabled-inverse-unsupported", "missing-script-sign", "missing-script-reading"} {
		t.Run(name, func(t *testing.T) {
			c, resolve := coverageCorpus(t, name)
			if c.Version != SequenceCorpusVersion2 {
				t.Fatal("extension must opt in to version 2")
			}
			old := Clone(c)
			old.Version = SequenceCorpusVersion
			if ValidateSequenceCorpus(old) == nil {
				t.Fatal("version 1 silently reinterpreted")
			}
			if c.Cases[0].Expected.Status != "unsupported_inverse" {
				return
			}
			for _, edit := range []func(*SequenceCase){
				func(c *SequenceCase) { c.Expected.Spelling.Script = c.Pin.Script },
				func(c *SequenceCase) { c.Expected.Spelling.Steps++ },
				func(c *SequenceCase) { c.Expected.Diagnostics[0].Message = "invented" },
				func(c *SequenceCase) { c.Expected.Spelling.Clusters = nil },
			} {
				bad := Clone(c)
				edit(&bad.Cases[0])
				if ValidateSequenceCorpus(bad) == nil {
					t.Fatal("inexact early-rejection shape accepted")
				}
			}
			_, err := RunSequenceCorpus(context.Background(), c, func(ctx context.Context, kind, id, revision string) ([]byte, error) {
				if kind == "script" {
					return nil, fmt.Errorf("removed pin")
				}
				return resolve(ctx, kind, id, revision)
			}, false)
			if err == nil {
				t.Fatal("early rejection bypassed dependency preflight")
			}
		})
	}
	// Eligible source cannot claim an early inverse rejection, even in /2.
	c, _ := coverageCorpus(t, "inverse-disabled-inverse-unsupported")
	_, resolve := reviewedSequenceCorpus(t)
	d, _ := sequenceFixture(t)
	test := &c.Cases[0]
	test.Pin.Revision = Hash(d)
	test.Request.Revision = Hash(d)
	test.Expected.Input = test.Request
	test.Expected.Request = test.Request.Request
	test.Expected.Revision = Hash(d)
	if _, err := RunSequenceCorpus(context.Background(), c, resolve, true); err == nil || !strings.Contains(err.Error(), "early inverse rejection requires") {
		t.Fatal("eligible source labelled lossy", err)
	}
	// Existing reviewed corpus reads and reports exactly the old version.
	old, resolve := reviewedSequenceCorpus(t)
	r, err := RunSequenceCorpus(context.Background(), old, resolve, false)
	if err != nil || r.Version != SequenceCorpusVersion || r.Status != "success" {
		t.Fatal("old corpus compatibility", err, r.Status)
	}
}

func TestSequenceExternalInvalidDefinitionsAreValidationFailures(t *testing.T) {
	_, script := sequenceFixture(t)
	for _, tc := range []struct{ file, code, rule, message string }{
		{"invalid-layer", "incompatible_layer", "sign-selection", "Each stage must consume the preceding stage's output layer."},
		{"invalid-conflict", "rule_conflict", "equal-priority-conflict", "Equal-priority overlapping patterns require different explicit priorities (including context-dependent rules)."},
	} {
		t.Run(tc.file, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/sequence-corpus/" + tc.file + ".definition.json")
			if err != nil {
				t.Fatal(err)
			}
			d, err := Decode(raw)
			if err != nil {
				t.Fatal(err)
			}
			a := sequenceArtifact(t, d, script)
			if a.report.Ready["spell"] || a.report.Ready["unspell"] || !a.report.Ready["inflect"] || len(a.report.Diagnostics) != 1 {
				t.Fatal("invalid definition executed or unrelated readiness lost", a.report)
			}
			diag := a.report.Diagnostics[0]
			if diag.Code != tc.code || diag.Severity != "error" || diag.Module != "sequenceWriting" || diag.RuleID != tc.rule || diag.Message != tc.message {
				t.Fatal("wrong validation failure", diag)
			}
		})
	}
}
