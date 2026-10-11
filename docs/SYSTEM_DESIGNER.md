# Magic & Technology — System Designer

Open **Magic & Technology** in a World to create a magic, technology, hybrid, or custom system. Quick creation needs only a name and category. Guided templates ask optional questions; advanced mode exposes typed custom fields. Templates do not assert fictional facts.

The workbench groups independently saved records into foundations, mechanics, capabilities, methods, resources, components, implementations, applications, and developments. Use the outline to switch records. Draft edits are local until **Review rule changes**, then **Save reviewed changes**. Review is read-only and saving remains revision-checked. The Age bar controls the historical state.

## Model and compatibility

System records use the existing `Entity` table, identity, continuity, names, properties, and Age machinery. `Entity.system` is a versioned, validated `SystemRecord` (version 1). A root has kind `system`; sections use `system-principle`, `system-rule`, and the other registered kinds. Sections reference their root with `systemId`. This outline membership does not transfer ownership of referenced characters, organisms, places, or organizations.

Mechanics have optional preconditions, inputs, operation, outputs, costs, failures, exceptions, declared effect keys, structured constraints, unresolved prerequisites, and typed custom fields. Freeform equations remain documentation, not executable code. Custom fields reuse the existing definition/value validation for text, numbers, Boolean values, selections, dates, Age/entity references, and repeatable sections.

System connections are normal Age-scoped `Relation` records, registered in the `systems` family:

- `system-requires`, `system-consumes`, and `system-derived-from`: dependent → prerequisite.
- `system-enables` and `system-produces`: provider → dependent/output.
- Restrictions, prohibitions, countermeasures, conflicts, supersession, independent discovery, and explicit exceptions remain separately typed.
- `system-knowledge` and `system-access`: holder → system record. Understanding may be incorrect, disputed, secret, or lost; access may be available, restricted, or unavailable.
- `system-implemented-in`: implementation/application → existing place.
- `system-supply`: resource → destination with an authored supply explanation.
- `system-developed-by`, `system-uses`, and `system-documented-in`: contextual references to existing world entities.

Underlying possibility, development stage, personal knowledge, and practical access are separate values. An invented or understood capability does not create a device or automatically grant availability. Writing files are never included as entities.

## Advisory analysis and change impact

`POST /api/worlds/{world}/systems/analyze` accepts `ageId`, `systemId`, `changedId`, the current World `revision`, and an optional entity/batch `draft`. It loads the saved document, applies the draft to an isolated in-memory copy, and returns before/after analysis, affected dependent records with explanation paths, and supply context. No save or terrain regeneration occurs. Stale revisions return HTTP 409.

The checker identifies unresolved prerequisites, unsupported capabilities, startup dependency cycles without an explicitly declared starting stock, impossible prerequisites, declared incompatibilities, prohibited effects, and inconsistent numeric limits. Exact author-defined keys and matching units are used; units are not silently converted. Root constraints apply to their sections and propagate through prerequisite context. Hypotheses and assumptions are distinguished from hard rules. An `Exception to` relation records a deliberate exemption. Plain exception prose alone does not suppress a structured conflict.

Prose matching is deliberately limited: mentioning a prohibited effect can raise a **possible** inconsistency. This is not semantic verification of arbitrary prose or formulas. Findings are advisory; even contradictions can be saved deliberately. Nothing is rewritten automatically.

Changing a rule lists transitive dependents, including records in other systems, plus their linked knowledge holders, institutions, documents, and places. Removing a dependency retains the former impact path in the review. Changing root limits highlights its sections. New and resolved issues are shown separately.

## Sources and regional feasibility

Resource-role records can reference existing entities (including lifeforms) or actual generated natural-resource occurrences using `terrainId` and `occurrenceId`. `GET /api/worlds/{world}/systems/sources?age=…&terrain=…` lists stored occurrences from the saved environmental layer. It does not infer a mineral from terrain appearance. Source links open a read-only Atlas highlight of the stored occurrence.

For a placed implementation, analysis follows its transitive resource dependencies and compares recorded sources with its location's representation. Biological sources use persisted Biosphere distributions; absent/extinct populations and stale environmental distributions do not establish supply. Natural sources are read from immutable terrain snapshots, without a live generator session or explored render tiles.

Supply findings distinguish a known local source, an explicit authored supply arrangement, a gap between known source and destination, and unverified data. A local deposit does not guarantee access, extraction, sufficient quantity, or transportation. A missing local occurrence does not prove global absence. This release does not simulate trade, depletion, manufacturing throughput, or societal consequences.

## Historical state and persistence

Creating an Age deep-copies mutable system data and inherits relationships while retaining continuity IDs. Excluded prerequisites become unresolved questions rather than disappearing from the historical design without explanation. Root sections are excluded with an excluded root. Start/end years use the shared Year event index.

Records and relations are stored through the existing transactional SQLite/PostgreSQL persistence and included in World ZIP exports. Natural resource references continue to resolve from the exported terrain snapshots. No new external service is required. Worlds without system records remain valid; the optional system payload has its own version and unknown payload versions are rejected.

Tests cover typed validation, cross-system dependency paths, deliberate exceptions, startup cycles and bootstrap stocks, numeric and prose conflicts, Age isolation and exclusions, year indexing, read-only draft analysis, geographical source queries, database restart, and ZIP restoration into an empty database.
