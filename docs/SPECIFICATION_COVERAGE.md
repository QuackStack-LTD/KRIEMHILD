# Specification coverage and scope decisions

This maps [the supplied specification](../../KRIEMHILD_PLATFORM_SPECIFICATION_REVISED.md) to [the implementation plan](PLATFORM_PLAN.md). Plan section numbers below refer to that document; P0–P6 are its delivery phases, not release dates. This is planned coverage, not implemented functionality. The original specification remains unchanged. The source link and repository evidence links require the surrounding worldbuilding collection; the plan itself is self-contained.

The user's explicit prohibition on AI/LLM participation overrides specification §60 and the AI item in §59. Those are excluded, not deferred. Deterministic terrain algorithms, explicit formulas, authored grammar/name tables and structured consistency checks remain author-controlled tools.

## All specification sections

| Spec | Subject | Plan sections | Delivery and interpretation |
|---|---|---|---|
| 1 | Purpose | 1–3, 26 | User-oriented connected worldbuilding and writing; P1 through P4 core |
| 2 | Foundational model | 4–6 | P1 independent snapshots and stable identity; P2 temporal details |
| 3 | Primary experience | 3, 24 | P1 contextual shell, progressive disclosure, keyboard access |
| 4 | Storage and Git | 6–7, 22 | P1 local; P4 Git/Docker; P6 hosted mode |
| 5 | Terrain and regional cartography | 8 | P1 image maps; P2 structured terrain; P5 advanced geography |
| 6 | Chronology and timelines | 4, 9 | P2 events, custom calendars, concurrent tracks and causality |
| 7 | Geopolitics and tensions | 11, 21 | P3 relations, claims, internal factions; explicit inputs for dashboards |
| 8 | Characters and entities | 5, 10, 19 | P1 generic entities; P3 dossiers and historical relationships |
| 9 | Pantheons and dogma | 12 | P3 deity/religion models, schisms, beliefs and influence |
| 10 | Magic and technology | 13 | P3 rulebooks; P5 richer constraints and experiments |
| 11 | Linguistics and names | 14 | P3 language families/lexicons/names; P5 sound changes and morphology |
| 12 | Economics and trade | 15 | P3 resources/routes/dependencies; P5 explicit numerical experiments |
| 13 | Families and bloodlines | 16 | P3 flexible kinship; P5 succession and fictional inheritance rules |
| 14 | Warfare and logistics | 17 | P3 forces/campaigns/routes; P5 tactical tools |
| 15 | Ecology and climate | 18, 8 | P2 terrain-derived layers; P3 authored ecology; P5 advanced models |
| 16 | Institutions, law and classes | 11 | P3 jurisdictions, offices, legal rules and overlapping class structures |
| 17 | Cross-system dependencies | 5, 20–21 | P3 typed dependency relations; P4 impact review and checks |
| 18 | Historical change engine | 4, 9, 21 | P2 authored change sets; P4 reviewed dependency consequences |
| 19 | Perspectives and uncertainty | 5, 9, 12 | P2 uncertain dates; P3 attributed and disputed assertions |
| 20 | Lore and cultural memory | 12, 19–20 | P3 variants and source links; lore never silently becomes fact |
| 21 | Population and migration | 10, 15, 18 | P3 qualitative or numeric demographic distributions and routes |
| 22 | Settlements | 8, 10 | P2 geographic identity; P3 urban functions and connected dashboards |
| 23 | Cultures | 10, 14 | P3 overlapping culture/language/identity, no one-state-one-culture rule |
| 24 | Lifecycle and identity | 4–5, 10 | P1 stable IDs; P2 renames, lifecycle, splits/mergers and continuity |
| 25 | Story architecture and writing | 19 | P1 editor; P3 scenes, arcs, character studies, outline and context |
| 26 | Story/world consistency | 19, 21 | P4 checks of structured scene references; no prose understanding claims |
| 27 | World validation | 21 | P1 integrity; P4 author-overridable consistency and explanations |
| 28 | Dashboards | 3, 10–18, 20 | P3 domain dashboards from shared queries; unknown inputs stay unknown |
| 29 | Search and discovery | 20 | P1 search; P2 temporal; P3 structured/geographic reverse discovery |
| 30 | Knowledge graph | 5, 20 | P3 filtered view of canonical relations, not a separate source of truth |
| 31 | Assets and media | 6, 20, 24 | P1 attachments; P4 provenance, offline bundling and safe import/export |
| 32 | Draft/canonical/alternate | 4–5, 19 | P2 explicit status and branch context; publication filters in P4 |
| 33 | Branching Ages | 4 | P2 branches use independent snapshots; explicit reconciliation only |
| 34 | Age creation | 3–4, 6 | P1 exact-source copy; P2 complete guided workflow and provenance |
| 35 | Historical transformation | 4, 8–9, 21 | P2 grouped map/entity edits; P4 impact review before acceptance |
| 36 | What changed | 4, 8–9, 22 | P1 record differences; P2 maps/events; P4 semantic Git differences |
| 37 | Historical object view | 4, 10, 20 | P2 identity history with Age/date/branch and source revision |
| 38 | Map/Age/entity integration | 3–5, 8 | P1 context-preserving selection; P2 historical overlays and comparison |
| 39 | Story bible | 19–20 | P4 generated views of linked records, with pinned story context |
| 40 | Encyclopedia/codex | 19–20 | P4 filtered linked views and export, no duplicated canonical database |
| 41 | Publishing | 19, 22, 24 | P4 native/Markdown/HTML/map output; P5 book formats; P6 interactive sharing |
| 42 | Undo/versioning/safety | 6, 19, 22, 25 | P1 durable saves/undo; P4 backups, migration and Git recovery |
| 43 | Templates/custom entities | 5, 10, 13 | P1 generic schemas; P3 templates; version-pinned and safely migrated |
| 44 | Notes/research | 20 | P1 notes; P3 citations, research inbox and promotion with backlinks |
| 45 | Performance | 7, 25 | P0 benchmarks; budgets verified at each release gate |
| 46 | File philosophy | 6, 22 | P0 format contract; P1 inspectable immutable objects; editable materialized exchange |
| 47 | Logical map data | 5, 8 | P2 separate semantic features, terrain chunks and presentation |
| 48 | Canonical geography | 8, 18 | P2 reproducible accepted outputs; rendering and recalculation never define canon |
| 49 | Terrain history | 8–9, 18 | P2 changes/events; P5 richer erosion and geological operations |
| 50 | Geographic scale | 8 | P2 map-space contracts; P5 deeper regional editing and advanced projections |
| 51 | Domain dashboards | 3, 10–18 | P3 common layout, author-selected detail and query-derived summaries |
| 52 | New world workflow | 3, 26 | P1 minimal world; P2 generation; P3 optional domain templates |
| 53 | New Age workflow | 3–4, 26 | P1 copy/edit/isolation; P2 explanations and historical comparison |
| 54 | Story workflow | 3, 19 | P1 writing; P3 plotting/context; P4 consistency and compilation |
| 55 | Main UI areas | 3, 7 | P1 shared shell; workspaces introduced with their domain phases |
| 56 | Age-centered navigation | 3–5, 19–20 | P1 mandatory contextual references; explicit cross-Age modes |
| 57 | Product exclusions | 1, 21, 23 | No autonomous creative simulation, generic wiki replacement or AI author |
| 58 | Core distinctions | 1, 4, 8, 19 | Verified by P1 vertical slice and P4 connected reference world |
| 59 | First serious version | 26; matrix below | P4 release gate includes all 28 essential items |
| 60 | Future AI | 1, 21, 23 | Excluded by current user instruction, including local models |
| 61 | Finished Age | 3–4, 21 | Author decides completion; warnings and missing domains do not block it |
| 62 | Finished world | 3–4, 20 | Author chooses historical scope; no algorithmic completeness score |
| 63 | Emotional experience | 3, 19, 24 | Progressive disclosure, creative control, reversible tools and ownership |
| 64 | Core concept | 1, 4 | Independent historical states connected into an authored world |
| 65 | Final domain model | 5–6, 8–20 | Shared typed identities/relations, snapshots, maps, events and stories |

## First serious version: all essential items

These are minimum user-visible capabilities by P4. A phase indicates introduction; hardening continues through the release gate. Advanced depth may come later, but the named core cannot be deferred while calling the release complete.

| §59 essential | Introduction | Minimum acceptance evidence |
|---|---|---|
| Local project creation | P1 | Create, close, reopen and edit without network or account |
| GitHub/local repository mode | P4 | Clone/init, review, commit, sync and resolve a semantic conflict safely |
| World and Age model | P1 | Navigate two Ages with the same identity and different states |
| Age inheritance | P1 | Copy all content categories; editing either Age never changes the other |
| Structured entities | P1 | Create custom typed records and linked relationships |
| Map generation | P2 | Repeat seeded generation with a pinned algorithm version |
| Structured terrain | P2 | Terrain source values remain editable independently of rendered images |
| Map editing | P2 | Preview, apply, undo and save a geographic change |
| Rivers and water | P2 | Edit water features and review hydrological consequences |
| Settlements | P2 | Link map location and dossier through stable identity |
| Political geography | P2–P3 | Show authored borders, overlapping claims and control |
| Timeline | P2 | Display custom-calendar dates and uncertain intervals |
| Historical events | P2 | Link event, participants, locations and source perspective |
| Historical changes | P2 | Explain and compare an applied change set |
| Characters | P3 | Dossier, relationships and different states across Ages |
| Cultures | P3 | Overlapping cultural presence and authored demographic descriptions |
| Religions | P3 | Pantheon, dogma, institutions and a historical schism |
| Languages | P3 | Family, lexicon, multilingual names and historical naming |
| Basic economics and routes | P3 | Resource/production relationships and an editable trade route |
| Family trees | P3 | Multiple kinds of parentage, unions and disputed ancestry |
| Basic warfare | P3 | Forces, campaign events, control and logistical routes |
| Ecological/climate layers | P2–P3 | View/edit climate and biome state and compare Ages |
| Institutions and social classes | P3 | Offices, jurisdictions, social relations and historical membership |
| Search | P1–P3 | Find names, aliases and text with explicit Age scope |
| Age comparison | P1–P2 | See entity and geographic differences without mutating either side |
| World consistency checks | P4 | Explain a structured contradiction and record an author exception |
| Story architecture | P3 | Organize acts/chapters/scenes/arcs with world references |
| Manuscript editor | P1–P3 | Write, reorder, undo, autosave/recover and compile linked scenes |

The later list maps to P5 for advanced magic, technology, linguistics, numerical economics, tactical visualization, fictional inheritance and ecology; P6 for collaborative editing, hosting and interactive publication. These remain explicit tools rather than autonomous simulations. AI-assisted worldbuilding is removed from that list.

## Resolved ambiguities and added requirements

| Open issue or missing requirement | Decision and consequence |
|---|---|
| What exactly is copied into a new Age? | A complete manifest from a saved source revision, including stories, schemas, assets and map state. Immutable bytes may be shared physically; mutable parent state is never consulted. |
| What happens when an earlier Age is corrected later? | Existing descendants remain unchanged. Explicit three-way propagation previews field-level changes and conflicts. |
| Does an Age represent a moment or a period? | Its overview is a declared checkpoint, normally the end; validity intervals represent authored internal changes. Earlier unknown state is not inferred from undated edits. |
| Do story settings move when an Age is copied? | The story version is copied, while original scene setting references remain explicit; retargeting is an author action with review. |
| Is Git history fictional history? | No. Edit revisions, historical events, lifecycle and Age branches are separate concepts. |
| Can nonhuman or impossible worlds use the tools? | Custom units, calendars, map spaces, parentage, rules and exceptions; fantasy constraints are not storage corruption. |
| Are cultures, territories and populations exclusive? | No. Overlapping affiliations, jurisdictions and uncertain estimates are first-class data. |
| How are subjective beliefs represented? | Attributed assertions and source variants, separately from author-established facts. |
| Are file saves safe across many JSON/media files? | Immutable objects plus an atomic root commit, recovery journal, single writer and rebuildable indexes; fault injection before release. |
| How can users edit portable files? | Native canonical objects are inspectable; an editable materialized view has validated import and a change preview. Markdown is an exchange format, not a promise of lossless rich text. |
| Can browser-only work use arbitrary filesystem paths? | The local Go service owns approved project roots; the browser uses scoped APIs. Hosted mode has separate permissions. |
| Is a Node server necessary for Next? | No for the chosen static shell; arbitrary project IDs use query parameters. P0 proves deep links and the production export. |
| Can prose checks find every contradiction? | No. Checks use authored structured references and explain their limits. No hidden NLP/LLM dependency. |
| What prevents publishing private lore? | Explicit publication allowlists applied to assets, links, search indexes and metadata, verified with exclusion tests. |
| What enables durable ownership? | Versioned formats, unknown-field preservation, migration recovery, closure-complete archives, actual media bytes and offline operation. |
| What keeps the product usable at scale? | Progressive loading, bounded graph queries, tiled terrain, background jobs, cancellation, stale-result checks and measured budgets. |
| How do more people collaborate later? | Start with one canonical writer and optimistic revisions. Add hosted permissions and comments before concurrent text editing; preserve the same interchange model. |

The remaining P0 questions concern implementation measurements and format details, not whether to preserve history or author control. See plan §27 for risks and §25 for verification.
