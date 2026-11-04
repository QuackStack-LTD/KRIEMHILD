package languages

import (
	"context"
	"golang.org/x/text/unicode/norm"
	"kriemhild/internal/languages/spike"
	"sort"
	"unicode"
)

const InverseIndexVersion = "survivor-trie/1"
const inverseNodes = 32768
const inverseRecords = 64000
const inverseCandidates = 8192
const inverseBuildSteps = 200000

type inverseRecord struct{ lexeme, cell int }
type inverseEdge struct {
	token string
	child int
}
type inverseNode struct {
	edges []inverseEdge
	refs  []int
}
type inverseTrie struct{ nodes []inverseNode }
type inverseIndex struct {
	segments, written inverseTrie
	records           []inverseRecord
	identity          string
	bytes, buildSteps int
}
type InverseMetrics struct {
	Version      string `json:"version"`
	Identity     string `json:"identity"`
	Request      string `json:"request"`
	Units        string `json:"units"`
	Records      int    `json:"records"`
	Eligible     int    `json:"eligible"`
	Pruned       int    `json:"pruned"`
	NodesVisited int    `json:"nodesVisited"`
	Steps        int    `json:"steps"`
	PayloadBytes int    `json:"payloadBytes"`
	Stop         string `json:"stop"`
}

func spellingBases(text string) []string {
	out := []string{}
	for _, r := range norm.NFD.String(text) {
		if !unicode.IsMark(r) {
			out = append(out, string(r))
		}
	}
	return out
}
func (idx *inverseIndex) insert(ctx context.Context, trie *inverseTrie, tokens []string, ref int) error {
	node := 0
	for _, token := range tokens {
		idx.buildSteps++
		if err := ctx.Err(); err != nil {
			return productiveError("cancelled", "", "inverse.index", err.Error())
		}
		if idx.buildSteps > inverseBuildSteps {
			return productiveError("index_limit", "", "inverse.index", "Index construction exceeded 200,000 steps. Forward composition remains available.")
		}
		next := -1
		for _, e := range trie.nodes[node].edges {
			idx.buildSteps++
			if idx.buildSteps > inverseBuildSteps {
				return productiveError("index_limit", "", "inverse.index", "Index construction exceeded 200,000 steps.")
			}
			if e.token == token {
				next = e.child
				break
			}
		}
		if next < 0 {
			if len(idx.segments.nodes)+len(idx.written.nodes) >= inverseNodes || idx.bytes+len(token)+384 > productiveBytes {
				return productiveError("index_limit", "", "inverse.index", "Index exceeded 32,768 nodes or 8 MiB of accounted storage. Reduce the lexical domain; forward composition remains available.")
			}
			next = len(trie.nodes)
			trie.nodes = append(trie.nodes, inverseNode{})
			trie.nodes[node].edges = append(trie.nodes[node].edges, inverseEdge{token, next})
			idx.bytes += len(token) + 384
		}
		node = next
	}
	trie.nodes[node].refs = append(trie.nodes[node].refs, ref)
	idx.bytes += 8
	return nil
}

func (a *Artifact) buildInverse(ctx context.Context) error {
	idx := &inverseIndex{segments: inverseTrie{nodes: []inverseNode{{}}}, written: inverseTrie{nodes: []inverseNode{{}}}}
	idx.identity = Hash(struct {
		Revision, Engine, Adapter, Compiler, Index string
		Schema                                     int
		Dependencies                               []ScriptRef
	}{a.report.Revision, a.report.Engine, a.report.Adapter, a.report.Compiler, InverseIndexVersion, a.definition.Schema, a.report.Dependencies})
	mutable := map[string]bool{}
	for _, rule := range a.definition.Productive.Stratum.Rules {
		for _, match := range rule.Input {
			for _, s := range a.definition.Phonology.Segments {
				if err := ctx.Err(); err != nil {
					return productiveError("cancelled", "", "inverse.index", err.Error())
				}
				if a.productive.match(match, ProductiveToken{Segment: s.ID}) {
					mutable[s.ID] = true
				}
			}
		}
	}
	mapping := map[string]string{}
	if a.report.Ready["write"] {
		for _, m := range a.definition.Orthography.Mappings {
			mapping[m.Segment] = m.Grapheme
		}
	}
	for li, lex := range a.definition.Lexicon {
		for ci, cell := range a.productive.paradigms[lex.Paradigm].Cells {
			idx.buildSteps++
			if err := ctx.Err(); err != nil {
				return productiveError("cancelled", "", "inverse.index", err.Error())
			}
			if _, ok := spike.Unify(lex.Features, cell.Features); !ok {
				continue
			}
			if len(idx.records) >= inverseRecords || idx.buildSteps > inverseBuildSteps {
				return productiveError("index_limit", "", "inverse.index", "Index construction exceeded 64,000 lexical cells or 200,000 steps.")
			}
			stem := lex.Underlying
			for _, override := range lex.Overrides {
				if override.Cell == cell.ID {
					stem = override.Segments
					break
				}
			}
			stable := []string{}
			written := []string{}
			for _, token := range stem {
				if !mutable[token] {
					stable = append(stable, token)
					if a.report.Ready["write"] {
						written = append(written, spellingBases(mapping[token])...)
					}
				}
			}
			ref := len(idx.records)
			idx.records = append(idx.records, inverseRecord{li, ci})
			idx.bytes += 16
			if err := idx.insert(ctx, &idx.segments, stable, ref); err != nil {
				return err
			}
			if a.report.Ready["write"] {
				if err := idx.insert(ctx, &idx.written, written, ref); err != nil {
					return err
				}
			}
		}
	}
	a.productive.inverse = idx
	a.report.States += len(idx.segments.nodes) + len(idx.written.nodes)
	return nil
}

func (idx *inverseIndex) possible(q Request, budget *productiveBudget, metrics *InverseMetrics) (map[inverseRecord]bool, error) {
	trie := &idx.segments
	tokens := q.Segments
	metrics.Units = "phonological-segment-ids"
	if q.Operation == "analyze-written" {
		trie = &idx.written
		tokens = spellingBases(q.Text)
		metrics.Units = "NFD-nonmark-code-points"
	}
	metrics.Version = InverseIndexVersion
	metrics.Identity = idx.identity
	metrics.Request = Hash(struct {
		Index string
		Input Request
	}{idx.identity, q})
	metrics.Records = len(idx.records)
	// A node represents one fixed ordered survivor prefix. Its earliest match
	// dominates every later match, so activate each node once, preserving all
	// terminal lexical/cell references including epsilon signatures at the root.
	waiting := map[string][]int{}
	refs := append([]int{}, trie.nodes[0].refs...)
	for _, e := range trie.nodes[0].edges {
		waiting[e.token] = append(waiting[e.token], e.child)
	}
	for _, token := range tokens {
		if err := budget.charge(1, 0); err != nil {
			return nil, err
		}
		nodes := waiting[token]
		delete(waiting, token)
		for _, n := range nodes {
			metrics.NodesVisited++
			if err := budget.charge(1, 0); err != nil {
				return nil, err
			}
			refs = append(refs, trie.nodes[n].refs...)
			for _, e := range trie.nodes[n].edges {
				if err := budget.charge(1, 0); err != nil {
					return nil, err
				}
				waiting[e.token] = append(waiting[e.token], e.child)
			}
		}
	}
	metrics.Eligible = len(refs)
	metrics.Pruned = len(idx.records) - len(refs)
	if len(refs) > inverseCandidates {
		return nil, productiveError("execution_limit", "", "inverse.candidates", "Inverse index matched more than 8,192 lexical cells. No partial analyses were returned.")
	}
	sort.Ints(refs)
	out := map[inverseRecord]bool{}
	for _, ref := range refs {
		if err := budget.charge(1, 16); err != nil {
			return nil, err
		}
		out[idx.records[ref]] = true
	}
	return out, nil
}
