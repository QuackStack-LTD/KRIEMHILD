package languages

import (
	"context"
	"fmt"
	"testing"
)

// Cold includes decoding fixture/source files. Warm reuses decoded transport
// snapshots, but deliberately recompiles derived artifacts on every run. These
// allocation totals are not retained heap, peak heap or RSS measurements.
func BenchmarkSequenceCorpus(b *testing.B) {
	c, resolve := reviewedSequenceCorpus(b)
	for _, cold := range []bool{false, true} {
		b.Run(fmt.Sprintf("cold-%v", cold), func(b *testing.B) {
			r, err := RunSequenceCorpus(context.Background(), c, resolve, false)
			if err != nil || r.Status != "success" {
				b.Fatal(err, r.Status)
			}
			b.Logf("cases=%d corpusBytes=%d calls=%d chargedWork=%d outcomes=success:3,ambiguous:1 stop=completed; warm recompiles artifacts", len(c.Cases), len(Canonical(c)), r.Calls, r.ChargedWork)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				current, resolver := c, resolve
				if cold {
					current, resolver = reviewedSequenceCorpus(b)
				}
				r, err := RunSequenceCorpus(context.Background(), current, resolver, false)
				if err != nil || r.Status != "success" {
					b.Fatal(err, r.Status)
				}
			}
		})
	}
}
