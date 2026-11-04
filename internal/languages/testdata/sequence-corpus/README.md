# Brief 6 focused sequence fixtures

Each `.corpus.json` is a dedicated one-case external corpus. Its exact language,
Script, Unicode and renderer pins are authoritative; definitions are explicitly
supplied by the caller. Files are never looked up by paths from corpus JSON.

The author authorized independent derivation and explicitly labelled Codex review
on 2026-10-29. New expectations name **Codex (independent derivation; not human
approval)**. Each carries its derivation note. The NFC and Unicode wrappers reuse
the previously author-approved complete expectations without changing their data.
No execution result was captured as an expectation. Source-control review remains
the author's review mechanism; pending metadata never passes the review gate.

| Files                                                                                                                 | Independent expectation basis                                                                                                                                                                                                                             |
| --------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `literal-contexts`, `left-context-miss`, `right-context-miss`, `word-edges`                                           | Immutable input-snapshot literal matching and virtual word-edge semantics; context rule definitions show why `XAN`, `néN`, `Xéé`, `XN` follow. Rules see original `p` even after it produces `X`.                                                         |
| `priority-overlap`                                                                                                    | Priority 20 consumes `p,a` before priority 10 word-edge `p`; output `QN` merges source provenance `[0,1]`. Array order is deliberately lower-priority first.                                                                                              |
| `no-match`, `wrong-input-layer`                                                                                       | Declared `i` has no mapping; a typed grapheme cannot be consumed as a surface segment. Exact `no_mapping`/`invalid_unit` diagnostics and empty result sets.                                                                                               |
| `invalid-layer`, `invalid-conflict`                                                                                   | Documented adjacent-layer and equal-priority prefix validation. `/2` compares semantic compiler validation with outcome `rejected`; no execution trace is fabricated.                                                                                     |
| `nfc-exact`, `nfd-forward`                                                                                            | Retained NFC golden; independently authored `U+00E9 -> U+0065 U+0301` explicit NFD, exact trace and cluster offsets.                                                                                                                                      |
| `unicode-15-clusters`, `unicode-coordinates`                                                                          | Retained Unicode golden includes combining marks, supplementary scalar, ZWJ, flag and modifier examples. Explicit coordinate table is derived by counting its exact reviewed code points, independently of segmentation execution.                        |
| `deletion-forward`, `deletion-inverse-unsupported`, `nfd-inverse-unsupported`, `inverse-disabled-inverse-unsupported` | Explicit consuming deletion/provenance; documented lossy-stage or authored-policy inverse readiness. No reverse mapping is inferred.                                                                                                                      |
| `missing-script-sign`, `missing-script-reading`                                                                       | Production Script validation and `TestVectorReadingsAndExplicitChoices`: raw `missing_sign` / `unresolved_reading`, not invented status names.                                                                                                            |
| `topology-exact-forward`, `topology-exact-inverse-a2`                                                                 | Exact `p,a,d,\|,a` pre-erasure stream. Full A2 candidates/derivations and counters are copied from retained `sequence-inverse.golden.json`. Only revision-bound orthography/index/request identities and the independently authored forward trace change. |
| `topology-mismatch-forward`, `topology-mismatch-inverse-a2`                                                           | Same surface segments, boundary deliberately shifted from index 3 to 2: raw spelling succeeds; A2 accepts no analysis with that topology.                                                                                                                 |
| `topology-missing-forward`, `topology-missing-inverse-a2`                                                             | Same segments with the boundary omitted: raw spelling succeeds; exact pre-erasure A2 check rejects both historical analyses.                                                                                                                              |

Word boundaries are represented by virtual context edges, not physical tokens.
The morpheme-boundary token remains in typed derivation steps before
`erase-boundaries`, and its removal is explicit in the following step. Malformed
adjacent/leading/trailing morpheme boundaries are separate validation tests.

Operational steps were counted from the documented bounded matching/control flow;
per-case provenance includes the arithmetic. A2 metrics are retained from the
reviewed golden. The A2 index is built before sequence dependencies are installed
by compilation, so its identity hashes the then-empty dependency slice; the outer
result still pins and compares the Script. This ordering is preserved, not repaired.

All arrays, stage spans, offsets, provenance, candidate identities and null/empty
distinctions are exact. No canonical-equivalence, fuzzy trace, final-string-only or
subset comparison is used. `/2` validation records compare all readiness and
diagnostic fields; compile size/capability metadata is deliberately outside that
separately typed record. Existing full execution goldens remain intact.
