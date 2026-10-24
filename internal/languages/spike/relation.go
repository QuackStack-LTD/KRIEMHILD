package spike

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// RelationBackend isolates the application from any native toolkit API. This
// experimental adapter compiles finite pairs; productive composition remains
// unimplemented, not silently approximated by a string generator.
type RelationBackend interface {
	Build(context.Context, []Pair, int) (Relation, error)
}
type Relation interface {
	Lookup(context.Context, bool, []string, int) ([]int, bool, error)
	InverseText(context.Context, string, int) ([]int, bool, error)
	States() int
}
type Pair struct {
	Upper, Lower []string
	Payload      int
}
type arc struct {
	upper, lower string
	next         int
}
type node struct {
	arcs   []arc
	finals []int
}
type TokenTrieBackend struct{}
type tokenRelation struct{ nodes []node }

func (TokenTrieBackend) Build(ctx context.Context, pairs []Pair, budget int) (Relation, error) {
	r := &tokenRelation{nodes: []node{{}}}
	for _, p := range pairs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(p.Upper) == 0 {
			return nil, fmt.Errorf("analysis tape must be nonempty")
		}
		state := 0
		for i := 0; i < max(len(p.Upper), len(p.Lower)); i++ {
			u, l := "", ""
			if i < len(p.Upper) {
				u = p.Upper[i]
			}
			if i < len(p.Lower) {
				l = p.Lower[i]
			}
			if u == "" && l == "" {
				return nil, fmt.Errorf("empty transition")
			}
			next := -1
			for _, a := range r.nodes[state].arcs {
				if a.upper == u && a.lower == l {
					next = a.next
					break
				}
			}
			if next < 0 {
				if len(r.nodes) >= budget {
					return nil, fmt.Errorf("transducer state limit exceeded")
				}
				next = len(r.nodes)
				r.nodes = append(r.nodes, node{})
				r.nodes[state].arcs = append(r.nodes[state].arcs, arc{u, l, next})
			}
			state = next
		}
		r.nodes[state].finals = append(r.nodes[state].finals, p.Payload)
	}
	for i := range r.nodes {
		sort.Slice(r.nodes[i].arcs, func(a, b int) bool {
			x, y := r.nodes[i].arcs[a], r.nodes[i].arcs[b]
			if x.upper != y.upper {
				return x.upper < y.upper
			}
			return x.lower < y.lower
		})
		sort.Ints(r.nodes[i].finals)
	}
	return r, nil
}
func (r *tokenRelation) States() int { return len(r.nodes) }
func (r *tokenRelation) walk(ctx context.Context, length, limit int, advance func(arc, int) (int, bool)) ([]int, bool, error) {
	type frame struct{ node, pos int }
	stack := []frame{{0, 0}}
	found := []int{}
	steps := 0
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		steps++
		if steps > 100000 {
			return nil, false, fmt.Errorf("transducer traversal limit exceeded")
		}
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if f.pos == length {
			for _, id := range r.nodes[f.node].finals {
				if len(found) == limit {
					sort.Ints(found)
					return found, true, nil
				}
				found = append(found, id)
			}
		}
		for i := len(r.nodes[f.node].arcs) - 1; i >= 0; i-- {
			a := r.nodes[f.node].arcs[i]
			if next, ok := advance(a, f.pos); ok {
				stack = append(stack, frame{a.next, next})
			}
		}
	}
	sort.Ints(found)
	return found, false, nil
}
func (r *tokenRelation) Lookup(ctx context.Context, inverse bool, input []string, limit int) ([]int, bool, error) {
	return r.walk(ctx, len(input), limit, func(a arc, pos int) (int, bool) {
		symbol := a.upper
		if inverse {
			symbol = a.lower
		}
		if symbol == "" {
			return pos, true
		}
		return pos + 1, pos < len(input) && input[pos] == symbol
	})
}

// Consume complete authored mapping strings, not code points. Alternative
// segmentations (a+bc versus ab+c) remain distinct transducer paths/analyses.
func (r *tokenRelation) InverseText(ctx context.Context, text string, limit int) ([]int, bool, error) {
	return r.walk(ctx, len(text), limit, func(a arc, pos int) (int, bool) { return pos + len(a.lower), strings.HasPrefix(text[pos:], a.lower) })
}
