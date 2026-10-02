# Assessment of all 80 reference repositories

Reviewed 2026-10-01 against the local checkouts, not against assumptions about similarly named upstream projects. Exact origin URLs, commit hashes, file counts, manifest paths and license evidence are in [the inventory](research/repository-inventory.json). The numbering below covers all 80 sibling directories, excluding the KRIEMHILD destination repository.

This is a static product/architecture assessment. README claims were compared with manifests and representative code; larger, more relevant projects received additional domain/storage inspection. No reference application was installed or executed, and no production readiness, test pass, benchmark or comprehensive security audit is claimed. A license observation is a reuse gate, not a complete legal opinion. Nested licenses, data, fonts, assets, copied algorithms and dependencies need individual review before importing code.

**Disposition key:** **R** = candidate for selective code/library reuse after testing and license review; **I** = implement the useful idea natively; **D** = documentation/content/format reference; **X** = exclude the implementation from the product. Multiple dispositions may apply. Every service recommendation below is optional: **none is needed for the default KRIEMHILD installation**. “No service” means no worthwhile reason to operate this repository as a platform dependency, even if it technically can be hosted.

## Principal recommendations

Prioritize Fantasy-Map-Generator, Filrodens-world-map-builder, genworldvoronoi, go_gens, worldengine and Terra for geography; GLX and obsidian-charted-roots for genealogy/provenance; conlang-studio and PolyGlot for linguistics; novelWriter and the non-AI parts of WorldScript-Studio for writing and persistence; scenario-forge for political map editing. These supply complementary lessons, not a single foundation to fork.

Useful standalone components are more attractive than entire products: `milsymbol`, selected permissive phonology/math utilities, and evaluated GLX interoperability code. Kanka, Chronicler and Tree-Monk are useful product references with restrictive licenses. Several other directories have no root license, and `genealogy-projects/LICENSE` is empty. Treat them as uncleared, regardless of README badges.

The smallest and most misleading repositories matter to the assessment too: `open-deity-project` contains only an essay and license; Lore-Flow's backend is starter code; `wyrd-ecs-core` has no inspectable ECS library and an obfuscated HTML entry point. Do not build plans around capabilities these checkouts do not demonstrate.

## Individual assessments

### 01. aifiver

**Evidence:** [README](../../aifiver/README.md), `stats.go`, `skills.go`, `traits.go`, `LICENSE`. Go; root GPL-3.0 text; explicitly bare-bones.

**I/X.** Useful distinction between author-defined traits, skills and conditions for displaying a trait. Implement optional character templates with editable scales and provenance. Reject behavioral AI, personality-driven decisions and random canonical fates: those conflict with author control and are unnecessary for dossiers. The name is not evidence of an LLM, but its behavior-agent purpose still does not belong in the creative workflow. Do not import its opinion/personality model as universal psychology. **Service: no.**

### 02. ancientWorldMap

**Evidence:** [README](../../ancientWorldMap/README.md), `src/components/HexMap.jsx`, `src/utils/mapGenerator.js`, `package.json`. React/Vite/canvas; README claims MIT but referenced root license is absent.

**I.** Useful hex/realistic view switching, settlement selection, minimap, feature filters and canvas performance controls. Rebuild these as Next map components over Go-owned source geography. Its noise and fixed name/biome lists are prototype material; local component state and browser saves do not implement historical snapshots. Prefer stronger terrain references. **Service: no; no backend worth retaining.** Code reuse stays blocked pending license evidence.

### 03. Ars-Magica-Open-License

**Evidence:** [README](../../Ars-Magica-Open-License/README.md), `reviewed/`, `wip/`, `3rd-party/`, `LICENSE.md`. A Markdown corpus and assets, not application code; root CC-BY-SA-4.0.

**D.** Useful stress test for expressive magic rulebooks: capabilities, constraints, traditions, institutions, creatures and cross-referenced source texts. Build neutral schemas that can describe comparable complexity. An optional, separately attributed content pack is conceivable after text/asset-specific review; no rules or setting are mandatory defaults. Machine-extracted/WIP material needs editorial checking. **Service: no.** Do not mistake available RPG content for a reusable rule evaluator.

### 04. Bakhroshungz

**Evidence:** [README](../../Bakhroshungz/README.md), `lexicon.html`, `morphology.html`, `phonology.html`, `LICENSE`. Static conlang site; CC-BY-SA-4.0; README says development moved elsewhere.

**D/I.** Its rich morphology, lexical categories, script and dialect documentation demonstrate why KRIEMHILD must support structured tables alongside freeform language notes. Implement per-language grammar pages and script-aware lexicons. It is a finished-language example, not a language construction engine. Do not transplant its vocabulary, flag or native script into user worlds by default. **Service: no.** Analyze the supplied snapshot without assuming the linked successor has the same content.

### 05. branching-tales-2-0

**Evidence:** [README](../../branching-tales-2-0/readme.md), `faq.php`, `api/story/`, `script/`, `LICENSE`. PHP/JavaScript; MIT; branches/tiles/media and JSON/Markdown packages.

**I/R.** Useful optional interactive-publication structure: authored branches, media tiles, explicit links, and versioned ZIP interchange. Reimplement a Next editor and Go package validator; investigate isolated format utilities if helpful. A reading-choice branch is distinct from an alternate historical Age. Some media features are marked planned, so do not promise full audio/video parity. **Service: technically hostable PHP companion, but not recommended**; native static publication is simpler and keeps one UI.

### 06. chronicler

**Evidence:** [README](../../chronicler/README.md), `src/lib/mapStore.ts`, `src/lib/mapModels.ts`, `LICENSE`. Svelte/Tauri/Rust; PolyForm Shield; README explicitly calls it source-available.

**I/X.** Strong product reference for offline Markdown ownership, linked notes, context panels and tiled image maps. Implement comparable user outcomes independently. Do not adopt its Svelte shell or persistence code as a convenient shortcut under KRIEMHILD's existing license; the restrictive terms require separate consideration. Image-map annotation is not semantic terrain. **Service: no**; optional user-managed Markdown exchange only. No need to reproduce its licensing/subscription machinery.

### 07. Commlink6

**Evidence:** [README](../../Commlink6/README.md), `SR6_Core/`, `SR6_Export_JSON/`, `SR6_Export_FVTT/`, `pom.xml`. Java/JavaFX with external RPG framework dependencies; no root license found.

**I.** Useful separation of character data, rule flags, creation workflows and format-specific export modules. Apply that separation to user-defined ability sheets and rulebooks. Native Go evaluators and Next forms should remain setting-neutral. Shadowrun equipment, rules, graphics and framework dependencies are not a general world model, and copying is uncleared. **Service: no**; possibly future user-imported character exports after a documented mapping/loss report.

### 08. conlang-studio

**Evidence:** [README](../../conlang-studio/README.md), `app/db/schema.ts`, `app/lib/rules.ts`, `app/lib/rule-apply.ts`, `package.json`. Next/TypeScript/PostgreSQL; MIT.

**R/I — high priority.** Phoneme inventories/groups, weighted syllables, contextual sound changes, lexeme senses and phonotactics closely match the language workbench. The pure token-based rule application is a concrete reusable candidate; port to Go or maintain shared deterministic fixtures if a browser preview is retained. Add Age/language revision bindings and historical naming provenance. **Service: no**; do not inherit Clerk/Neon requirements or its single-current-language database model. It explicitly relies on user-defined rules rather than built-in linguistic intelligence.

### 09. CraftSim

**Evidence:** [README](../../CraftSim/README.md), `DB/customerHistoryDB.lua`, `Modules/SimulationMode/`, `CraftSim.toc`, `LICENSE`. Lua World of Warcraft addon; MIT root.

**I.** Useful ideas are recipe inputs/outputs, material quality, cost comparison and a separate simulation mode. Implement a setting-neutral production calculator and economic what-if preview in Go. The UI and runtime call game APIs and cannot be transplanted as a standalone economy service. Its customer/order database is peripheral to fictional world history. **Service: no.** Exclude live auction-house/game integration and audit game data separately from source licensing.

### 10. current-tags

**Evidence:** [README](../../current-tags/README.md), `stage2-essentials.md`, `stage2-essentials-overview.md`, `outdated-files/`. Family History Metadata Working Group documents; no root license.

**D.** Useful for media provenance: people depicted, dates, descriptions, attribution and metadata interoperability. Design asset records to preserve original embedded metadata while distinguishing fictional dates from camera timestamps. Optional future XMP mapping must target a verified version of the recommendations. README references to stage-one files do not match the present root layout; use actual files. **Service: no.** No implementation library here, and no reason to make these recommendations the world's canonical schema.

### 11. deorum

**Evidence:** [README](../../deorum/README.md), `src/lib/data/characters.ts`, `src/lib/utils/characters.ts`, `package.json`, `LICENSE.md`. SvelteKit/PocketBase; MIT root; manifest includes AI SDK/provider dependencies.

**I/R, non-AI subset only.** Useful blank character record, optional physical/background fields, portrait attachments and export UX. Build native dossiers and asset galleries, with portraits supplied or drawn by users. Reject provider-driven biographies/portraits and public-gallery social infrastructure as core dependencies. **Service: no**; a second PocketBase character database would break Age ownership. Evaluate any extracted utility and portrait/content rights separately. Local file inventory used a filesystem fallback because tracked-file enumeration was empty.

### 12. diagrams-for-magic-systems

**Evidence:** [README](../../diagrams-for-magic-systems/README.md), `6-element/`, `8-element/`, `LICENSE-CC-BY`. SVG/PNG diagrams; CC-BY-4.0.

**D/R.** Useful author-controlled visual templates for affinities/oppositions among magical elements. Offer a generic diagram builder whose nodes link to rulebook entities; optional attributed diagram assets are feasible. Implement relationships semantically so a rendered wheel is not the only record of its rules. These drawings contain no rules engine or simulation. **Service: no.** Keep elemental structures optional rather than assuming every magic system uses a fixed number of elements.

### 13. dnd-character-tool

**Evidence:** [README](../../dnd-character-tool/README.md), `app/utils/store.ts`, `app/components/character/ClassEditor.tsx`, `package.json`. Next/PocketBase/Zustand; MIT claimed in README, root license missing.

**I.** Useful multi-panel character editing and separation of a sheet from its visual form. Rebuild as configurable dossiers with typed references and optional rule sheets. Its data is heavily D&D-shaped and component mutations assume a current sheet rather than an Age snapshot. Exclude hardcoded classes, ability formulas and game datasets from core. **Service: no**; PocketBase is not needed. Missing license prevents treating the familiar frontend stack as permission to copy it.

### 14. DungeonsAndDragons-Character-Creator-and-PDF-Generator

**Evidence:** [README](../../DungeonsAndDragons-Character-Creator-and-PDF-Generator/README.md), `D-and-D-Class-maker/createNewCharacter.py`, `editExistingCharacter.py`. Python/Tkinter/PDF forms; no root license.

**I/X.** The relevant outcome is filling a printable character sheet from structured data. Implement native dossier/export templates. The inspected code attempts dependency installation during imports and the README acknowledges incomplete error handling; it is unsuitable as an embedded worker. Fixed D&D forms and desktop UI do not generalize to arbitrary worlds. **Service: no.** A user may attach an exported PDF as an asset without integrating the application.

### 15. EconoSim.jl

**Evidence:** [README](../../EconoSim.jl/README.md), `src/models/`, `src/production/entity.jl`, `src/finance/`, `Project.toml`. Julia/Agents.jl; GPL-3.0 root.

**I; optional research adapter later.** Valuable concepts: production blueprints, input/output stocks, deterioration/repair, currencies and double-entry balances. Implement simpler explicit calculators natively first. Its agent behaviors and particular monetary theories are optional modeling assumptions, not facts of a fictional economy. **Service: possible bounded local Julia worker for author-selected numeric experiments**, never required or allowed to decide canon. Separate packaging and license review are necessary before distribution.

### 16. family-book

**Evidence:** [README](../../family-book/README.md), `app/schemas.py`, `app/models/relationships.py`, `app/models/media.py`, `pyproject.toml`. FastAPI/SQLAlchemy; MIT root.

**I/R.** Useful explicit date precision, raw dates, naming variants, relationship records, source/archive attachments and private self-hosting concepts. Implement Age-aware equivalents in Go; retain unknowns instead of forcing exact dates. Real-family messaging ingestion, social login and contact details are not needed for fiction. **Service: not recommended**, although it is self-hostable. README download instructions are not evidence of a production-ready release and were not executed.

### 17. family-tree-front

**Evidence:** [README](../../family-tree-front/README.md), `src/lib/pedigree/`, `src/components/app/pedigree/`, `src/stores/auth-store.ts`, `package.json`. Next/React Flow/next-intl; separate backend required; no root license.

**I.** Useful pedigree canvas, collapse/focus/export, kinship paths, multilingual/RTL UX and import preview. Build those interactions over KRIEMHILD's graph and Go API. Do not import the tenant/admin/support system or browser token-storage pattern. The associated backend is not part of this repository and was not reviewed. **Service: no**; it is a frontend, not an independent genealogy engine. Code reuse remains uncleared.

### 18. FamilyTreeView

**Evidence:** [README](../../FamilyTreeView/README.md), `src/family_tree_view_tree_builder.py`, `src/family_tree_view_timeline.py`, `COPYING.txt`. Python Gramps/GooCanvas addon; GPL-2.0-or-later indicated in source.

**I.** High-value genealogy UX reference: focused ancestors/descendants, expanders, contextual family/person panels, badges, long-name abbreviation and minimap. Reimplement layouts and interactions in Next with worker computations and accessible lists. It depends on Gramps models and GTK, and its README explicitly describes development instability. **Service: no**; users can exchange genealogy through supported formats rather than installing an addon for core functionality.

### 19. fanlang

**Evidence:** [README](../../fanlang/README.md), `Scripts/Translator/Translator.cs`, `Scripts/Data/TranslateHashData.cs`, `LICENSE`. Unity/C#; MIT.

**R/I.** Useful ordered longest-match replacement with word/prefix/suffix contexts and sequential sheets. Adapt the idea for author-defined orthography/transliteration or simple transformations, preserving unknown tokens. Label it accurately: substitution does not provide semantic translation or a grammar. Use a native Go engine rather than Unity. **Service: no.** Deep linguistic evolution should instead use tokenized phonological rules and etymological relations.

### 20. Fantasy-Map-Generator

**Evidence:** [README](../../Fantasy-Map-Generator/README.md), `CONTEXT.md`, `docs/architecture/data-model.md`, `src/types/GridGraph.ts`, `src/types/PackedGraph.ts`, `src/generators/`. JavaScript/TypeScript, D3/SVG; MIT root.

**R/I — leading atlas reference.** Rich terrain, rivers, settlements, political/cultural/religious layers, routes, labeling, editing and export offer the strongest feature reference. Select pure algorithms/utilities only after isolating globals and checking provenance. Its grid/pack repacking, array-index identities and current-state `.map` format must not become historical identity/storage. Build a versioned importer with loss reporting. **Service: no by default**; launching its editor separately is optional, but an iframe is not a native KRIEMHILD atlas.

### 21. FantasyMapGenerator

**Evidence:** [README](../../FantasyMapGenerator/README.md), `Terrain/Mesh.cs`, `Terrain/MapEdge.cs`, `Terrain/Terrain.cs`, solution/project files. C# desktop/Voronoi; README claims MIT, no root license found.

**I.** Useful staged examples of mesh, slope, coast, erosion, rivers, cities and map labeling. Compare outputs during terrain prototyping, but prefer the Go/TypeScript references for implementation. Its desktop renderer and data structures lack the planned editing/history integration. Check the licenses of credited upstream ports before any reuse. **Service: no**; adding a .NET worker is not justified by capabilities already covered elsewhere.

### 22. Filrodens-world-map-builder

**Evidence:** [README](../../Filrodens-world-map-builder/README.md), `src/tools/TerrainVersion.js`, `TerrainShading.js`, `src/generation/BiomeRuleEngine.js`, `src/applications/MapStateManager.js`. Foundry module/JavaScript; MIT.

**R/I — high priority.** Strong references for deterministic generation, non-destructive brushes, masks, custom biome rules, versioned generators and cheap shader-based style changes. Evaluate standalone math modules; replace Foundry journals/globals with KRIEMHILD snapshot storage. Its regeneration-from-recipe approach needs strengthening by retaining accepted output for durable historical geography. **Service: no**; Foundry must not be required. Useful optional export target if users already own/use it.

### 23. GEDKeeper

**Evidence:** [README](../../GEDKeeper/README.md), `projects/GKCore/GKCore/Validation/`, `scripts/ancestors_map.lua`, `projects/plugins/`, `LICENSE`. C# desktop and plugins; GPL-3.0 root.

**I/D.** Useful source/media records, genealogy validation, calendars, reports, timelines and geographic events. Mine requirements and edge cases; implement native checks and GEDCOM interoperability. Desktop UI, plugin host and medical/genetic assumptions are not foundational. Explicitly exclude its MCP/RAG/LLM integration. **Service: no**; optional file exchange with an independently installed desktop application. Submodule and plugin licenses require their own review.

### 24. genealogical-trees

**Evidence:** [README](../../genealogical-trees/README.md), `infer.py`, `gedcom2ttl.py`, `ttl2json.py`, `index.html`. Legacy Python/RDF/OWL/D3; MIT root.

**I/D.** Useful demonstration that kinship can be derived from typed graph edges and visualized independently of storage. Implement bounded, explainable kinship traversals in Go. Do not require an OWL reasoner: the README explicitly warns that the educational ontology and naive reasoning are impractical at scale. **Service: no.** Gendered/two-parent assumptions and old runtime syntax make wholesale adoption especially unsuitable.

### 25. genealogy-projects

**Evidence:** [README](../../genealogy-projects/README.md), `family-story-ui-main/src/App.tsx`, the three nested UI projects and manifests. React/TypeScript UI collection; root `LICENSE` is empty.

**I/X.** At most a source of layout comparisons for family pages, records and media. The inspected routing/UI does not establish the advertised research backend or real genealogical reasoning. Do not plan DNA testing, AI heritage tools, pricing/community screens or download installers into KRIEMHILD. **Service: no.** Empty licensing and README download links unrelated to normal release packaging reinforce the choice to keep this a read-only visual reference, not a dependency.

### 26. geneweb

**Evidence:** [README](../../geneweb/README.md), `lib/history.ml`, `lib/historyDiff.ml`, `lib/util/calendar.ml`, `dune-project`. OCaml genealogy web application; GPL-2.0-only declared in manifest.

**I/D.** Useful long-lived genealogy concerns: relationship paths, history diffs, alternate dates, privacy and format interchange. Reimplement suitable graph algorithms from general principles and test complex ancestry. README scale claims were not benchmarked locally. Its person/family revision history is editing history, not a complete Age snapshot. **Service: possible external genealogy companion**, but native family records plus GEDCOM exchange avoid an OCaml runtime and second source of truth.

### 27. genideas

**Evidence:** [README](../../genideas/README.md), `simciv/map.go`, `dfstyle/`, `simhydrology/`, `simerosion/`. Go experiments; no root license found; README calls projects half-baked.

**I/X.** Useful algorithm experiments for settlement suitability, hydrology, erosion, vegetation and map tiles. Treat them as exploration notes, not a dependable shared module. Unclear licensing and credited ports need resolution before copying. Autonomous settlers, people and strategy-game AI are outside the creative platform's mandate. **Service: no.** Stronger Go references exist, so this repository should not control backend architecture.

### 28. genworldvoronoi

**Evidence:** [README](../../genworldvoronoi/README.md), `geo/hydrology.go`, `geo/calendar.go`, `history.go`, `civTraderoutes.go`, `cmd/server/`. Go planetary generator; Apache-2.0 root.

**R/I — high priority, experimental.** Relevant terrain graph, wind/rainfall, hydrology, resource, settlement and route code; evaluate headless geographic components and reference outputs. Replace index-based references, mutable simulation state and Earth `time.Time` calendar with native IDs/date logic. The history record contains TODOs and is not an Age engine. **Service: demo server useful for prototyping only**; selectively integrate tested components. Automatic tribes, religions and historical narrative are not adopted.

### 29. Glotbase

**Evidence:** [README](../../Glotbase/README.md), `src/types/schema.ts`, `src/utils/affixEngine.ts`, `src/context/LexiconContext.tsx`. React/TypeScript/localStorage; no root license.

**I.** Useful native-script/romanization/IPA fields, RTL awareness, affix previews, lexicon filters and user-defined grammar categories. Implement Age-aware lexical records and proper persistence. A six-pattern word-order validator is not a universal syntax model; retain freeform grammar and exceptions. Browser localStorage should not own a large conlang. **Service: no.** Ideas are applicable, but license absence prevents assuming component reuse.

### 30. glx

**Evidence:** [README](../../glx/README.md), `go-glx/types.go`, `go-glx/calendar.go`, `go-glx/gedcom_family.go`, `go-glx/diff.go`, `specification/`, `docs/examples/temporal-properties/`. Go, YAML-based archive/specification; Apache-2.0.

**R/I/D — leading genealogy/provenance reference.** Strong candidate for GEDCOM/GLX adapters and lessons on assertions, citations, archive-owned vocabularies, temporal properties and validation. Confirm package boundaries/toolchain and conversion fidelity before reuse. Its known calendar conversions do not implement arbitrary fictional calendars; its archive is not the full world snapshot format. **Service: no**; integrate selected libraries/CLI interchange locally rather than host another datastore. Preserve provenance and unknown extension fields in adapters.

### 31. go_gens

**Evidence:** [README](../../go_gens/README.md), `vmesh/heightmap.go`, `genmapvoronoi/rivers.go`, `genmap2derosion/`, `genlanguage/`, `gendemographics/`, `go.mod`. Go; Apache-2.0 root; README warns not to use yet.

**R/I.** Broad algorithm source for noise/heightfields, erosion, river extraction, city geometry, biomes and table/grammar-based names. Pick bounded pure packages, complete dependencies and add deterministic/edge-case tests. Do not import the whole experimental game/simulation collection or narrative/personality generators. **Service: no**; native Go components fit the backend, but provenance of ports and transitive dependencies still matters. Terrain/route correctness and author control take priority over feature breadth.

### 32. gramps-web

**Evidence:** [README](../../gramps-web/README.md), `src/gcalendar.js`, `src/mapFilters.js`, `src/charts/`, `package.json`. Lit frontend; AGPL-3.0 root; Gramps backend is separate.

**I/D.** Valuable chart variants, family/source navigation, historical map overlays, privacy and event filtering. Use the interactions and edge cases as requirements for native Next views. Rebuild calendar logic for user-defined worlds instead of copying Earth date assumptions. Exclude AI chat and chromosome/clinical tooling from core. **Service: possible existing user companion with Gramps backend**, not this frontend alone; support interchange first rather than a required second genealogy service.

### 33. hexploit

**Evidence:** [README](../../hexploit/README.md), `code_hexploit/worldgen.py`, `simulation.py`, `hexmath.py`, `gui/undo.py`, `pyproject.toml`. Python/PySide6; MIT.

**R/I.** Useful hex coordinates, deterministic terrain pipeline, geological snapshots, campaign annotations, icon packs, print exports and undo concepts. Reimplement appropriate math in Go and authoring in Next. Its simplified flow/erosion explicitly trades realism for accessibility; retain that transparency. README/manifest version disagreement is a reason to pin code, not assume release readiness. **Service: no**; optional map import/export is more useful than a GUI sidecar. Fog-of-war can become an audience map view later.

### 34. kanka

**Evidence:** [README](../../kanka/README.md), `app/Models/Entity.php`, `app/Models/Map.php`, `config/maps.php`, `composer.json`, `LICENSE`. Laravel/PHP with mixed web UI; source-available, Commons Clause restriction.

**I/X — product reference only by default.** Excellent reference for connected entities, campaign context, relations, permissions, maps and discoverability. Build equivalent author journeys natively without adopting its schema/runtime or assuming permission to redistribute. Its campaign-scoped current entities still require a different Age model. **Service: technically self-hostable, not recommended**; an integration would fragment authoring, add infrastructure and need separate terms review. Do not treat it as unrestricted open-source code because it is on GitHub.

### 35. lineage

**Evidence:** [README](../../lineage/README.md), `lineage-class.php`, `LICENSE`. Small PHP kinship class; GPL-3.0.

**I/X.** Useful compact example of relationship labels and import formats. The documented dataset uses a single parent and required root conventions, which are insufficient for multiple parents, adoption, pedigree collapse and disputed ancestry. Implement graph-based kinship with configurable labels and relationship-role filters in Go. **Service: no**; wrapping this tiny class in PHP adds complexity while preserving the wrong constraints.

### 36. logseq

**Evidence:** [README](../../logseq/README.md), `cli/lib/entity.ml`, `cli/lib/graph.ml`, `deps/db/`, `src/main/frontend/handler/history.cljs`, manifests. ClojureScript/React plus supporting modules; AGPL-3.0 root.

**I/D.** Useful block references, backlinks, graph navigation, quick capture and research organization. KRIEMHILD should adopt the interaction principles while keeping scenes/documents and Age-specific entities explicit. Its inspected checkout includes database-oriented graph code; do not describe it simplistically as only a Markdown-folder application. **Service: no required service**; optional note exchange. Do not import the plugin/runtime/synchronization architecture wholesale or turn KRIEMHILD into an outliner.

### 37. Lore-Flow

**Evidence:** [README](../../Lore-Flow/README.md), `app.go`, `main.go`, `frontend/`, `go.mod`. Go/Wails/React; GPL-3.0 root.

**I/X.** Its proposed TOML templates and UUID-linked lore files are useful ideas for schema-driven forms and portability. However, inspected Go application code exposes a starter `Greet` method and window setup, not the advertised indexing/search/entity backend. Do not count its README feature list as implemented infrastructure. **Service: no.** Build the template system natively; Wails is unnecessary for the planned Go-served Next browser UI.

### 38. mil-std-2525

**Evidence:** [README](../../mil-std-2525/README.md), `src/2525b.js`, `src/2525c.js`, `src/2525d.js`, `tsv-tables/`, `LICENSE`. JavaScript/TSV catalogs; MIT.

**R/D.** Useful optional military symbol picker/catalog metadata. Keep a version identifier on any selected symbol and map it to KRIEMHILD unit records. The checkout catalogs B/C/D and points elsewhere for E; do not claim the package itself includes every later standard. **Service: no.** Default fantasy symbols remain simpler and setting-neutral. This is a symbol taxonomy, not a tactical or logistics engine.

### 39. milsymbol

**Evidence:** [README](../../milsymbol/README.md), `index.js`, `index.mjs`, `src/ms.js`, `src/ms/symbol.js`, `LICENSE`. JavaScript symbol renderer; MIT.

**R — strong isolated component candidate.** Render optional military unit symbols as SVG/canvas inside Next. Keep code/version/options in presentation records; symbols reference the same units used by campaigns. Validate/sanitize label inputs and benchmark dense maps. This is a good fit because it does not own world state. **Service: no**; browser rendering avoids an extra server. Standard icons supplement, rather than define, the platform's configurable military vocabulary.

### 40. milsymbol-esm

**Evidence:** [README](../../milsymbol-esm/README.md), `index.esm.js`, `src/ms.js`, `package.json`. MIT fork adding ESM and cyber dimensions; source identifies version 2.0.0.

**D/X by default.** Useful only if a specific cyber-symbol extension is required for a setting. The supplied upstream `milsymbol` already exposes ESM, so ESM alone is not a reason to choose this fork. Prefer one renderer to avoid duplicate catalogs and divergent output. **Service: no.** Re-evaluate only for a documented extension absent from the selected upstream revision.

### 41. milsymbol-server

**Evidence:** [README](../../milsymbol-server/README.md), `index.js`, `package.json`, `Dockerfile`. Node HTTP example using milsymbol/node-canvas; MIT.

**R/X.** Demonstrates symbol-to-SVG/PNG export and size caps. **Service: viable optional headless rendering endpoint**, but unnecessary in the core since symbols can be rendered in the frontend and exported. Do not add an always-running Node service for one function. If later used for server publication, constrain requests, pin fonts/rendering dependencies and run locally; do not depend on its public test server.

### 42. novelWriter

**Evidence:** [README](../../novelWriter/README.md), `novelwriter/core/document.py`, `project.py`, `projectsearch.py`, `novelwriter/editor/history.py`, `pyproject.toml`. Python/PyQt; GPL-3.0-or-later main code, manifest lists additional component licenses.

**I/D — leading writing reference.** Scene-sized documents, synopsis/metadata, cross-references, organization and robust human-readable storage suit long-form authors. Implement these workflows in Next with Go persistence and explicit historical setting references. Keep the writing workspace fast and useful independently of detailed world modeling. **Service: no**; offer Markdown interchange with an external installation. Do not port a desktop GUI or its syntax blindly, and distinguish its source license from bundled assets/components.

### 43. Nyrakai

**Evidence:** [README](../../Nyrakai/README.md), `chapters/`, `tools/sound_map.py`, `tools/validator.py`, `tools/word-generator.py`. Language documentation and Python utilities; no root license.

**D/I.** Useful example of linked phonology, nominal/verbal systems, morphology, lexicon and narrative context. Implement generic rule/lexicon/document structures able to represent unusual word order and author-defined sound symbolism. Its vocabulary and validator assumptions are specific to one conlang; do not present them as general linguistic truth or copy the setting. **Service: no.** Handwritten grammar remains valid when tooling cannot model it.

### 44. obsidian-charted-roots

**Evidence:** [README](../../obsidian-charted-roots/README.md), `src/core/family-graph.ts`, `src/models/place.ts`, `src/dates/types/date-types.ts`, `src/dates/parser/fictional-date-parser.ts`, `package.json`. TypeScript Obsidian plugin; MIT.

**R/I — high priority.** Useful fictional places/universes, sources, custom relationships, family charts, event identity, maps and date-system design. Evaluate pure utilities and interchange; replace Obsidian vault dependencies and current-note state with Age-aware native records. Its date type still has Earth-like month/day bounds in comments, so arbitrary calendars need a fresh contract. **Service: no**; Obsidian is optional interchange, not the runtime. Inspect dependency patches/licenses before extracting chart code.

### 45. online-geopolitical-simulator

**Evidence:** [README](../../online-geopolitical-simulator/README.md), `modules/simulation.js`, `modules/country.js`, `modules/war.js`, `sketch.js`. JavaScript/p5.js; ISC.

**I/X.** Useful visual vocabulary for country nodes, alliances and active conflicts. Rebuild the political network as an inspection/editor view over explicit relationships. The simulation includes automatic country behavior and simplistic war assumptions; do not adopt those as either historical truth or background creativity. **Service: no**; it runs as a browser toy, not a general political backend. Manual dashboards and explicit tension assessments better fit the task.

### 46. open-deity-project

**Evidence:** [README](../../open-deity-project/README.md), `LICENSE`; only two tracked files. Essay/proposal; GPL-2.0 text.

**D/X.** Conceptual metaphor for religions changing, branching and acquiring new interpretations. It supplies no deity schema, pantheon editor, dogma engine or working service. Implement religion and belief lineages from KRIEMHILD requirements, not from nonexistent code. **Service: no.** Beyond the versioned-belief idea, it is not materially relevant to implementation.

### 47. open-historia

**Evidence:** [README](../../open-historia/README.md), `src/Editor/`, `src/runtime/`, `server/mapEditorStore.js`, `server/basemapStore.js`, manifests. React/JavaScript with desktop/mobile/server pieces; AGPL-3.0-or-later.

**I/X.** Useful manually operated map editor concepts: draw/split/merge, ownership painting, scenario packages and cached basemaps. Reimplement those interactions over fictional geography. Its central event generation, diplomacy, advisor and battle resolution depend on AI and are excluded. **Service: no**; even its advertised offline gameplay expects an AI backend. A self-hosted model does not make it compatible with the user's no-AI creative requirement.

### 48. Open-Map-Creator

**Evidence:** [README](../../Open-Map-Creator/README.MD), `js/canvas.js`, `js/storage.js`, `js/hud.js`, `LICENCE`. Browser JavaScript/canvas; MIT.

**R/I.** Useful local/tactical drawing, grids, patterns, layer controls and real-world-size print export. Add a local-map workspace linked to a parent geographic footprint and Age. Drawing pixels alone does not define elevation, hydrology or political history; keep illustration and semantic features separate. **Service: no.** Replace browser-only persistence with the Go project service and bundle export dependencies/fonts for offline use.

### 49. open-pedigree

**Evidence:** [README](../../open-pedigree/README.md), `src/script/model/baseGraph.js`, `dynamicGraph.js`, `import.js`, `export.js`. Prototype/Raphaël browser pedigree tool; LGPL-2.1 root.

**I/R after scoped review.** Strong algorithm/UX reference for complex families, multiple generations, consanguinity and automatic layout. Prefer a modern Next implementation over embedding its legacy DOM framework. Clinical symbols, diagnoses and genetic exchange formats are optional specialized adapters, not universal fictional family semantics. **Service: optional standalone pedigree editor, not recommended**; native charts prevent split identity and historical state. License/linking and dependency terms need review before any extraction.

### 50. opensiddur

**Evidence:** [README](../../opensiddur/README.md), `python/os.py`, `opensiddur-server/src/`, schemas, `LICENSE`. XML/TEI, XQuery/eXist and helpers; mixed LGPL-3.0/GPL-3.0 code/schema and per-text CC licenses.

**I/D.** Valuable concepts: variants, translations, transliterations, commentary, sources and assembling selected text into publications. Apply these to sacred texts, chronicles and conflicting cultural memory. Do not treat a religious publishing toolkit as a generic religion simulator or reuse all text under one license. **Service: technically possible specialized publishing companion**, but native documents/export avoid the XML database and separate authority. No requirement to import real religious content.

### 51. Perplexicon

**Evidence:** [README](../../Perplexicon/README.md), `lexicon.py`, `template.py`, sample JSON lexicons, `LICENSE`. Small Python CLI; Unlicense text.

**R/I.** Useful compact lexical model with multiple senses and parts of speech, plus format templates. Adapt into native dictionary records and simple import/export. The README marks editing GUI, sorting and IPA work incomplete; it is not a full conlang environment. **Service: no**; no reason to introduce Python for a small JSON model. Add stable IDs, Age versions, etymologies, scripts and language-specific collation in KRIEMHILD.

### 52. planet_heightmap_generation

**Evidence:** [README](../../planet_heightmap_generation/README.md), `js/terrain-post.js`, `js/color-map.js`, `js/plates.js`, `js/koppen.js`. World Orogen browser/Three.js generator; GPL-3.0 root.

**I/D.** Useful tectonic editing, globe/heightmap presentation, terrain post-processing and explicit “plausibility rather than precision” product framing. Compare algorithm outputs and design an optional spherical/tectonic workbench later. Do not copy GPL implementation into the existing licensed core without a deliberate decision. **Service: no required service**; imported height/climate outputs can support an external companion workflow. Plate realism must remain optional for magical or nonplanetary worlds.

### 53. polyglot

**Evidence:** [README](../../polyglot/README.md), `src/main/java/org/darisadesigns/polyglotlina/Nodes/ConjugationGenRule.java`, `DomParser/LexiconParser.java`, `ManagersCollections/`, `pom.xml`, `LICENSE.TXT`. Java conlang toolkit; MIT.

**R/I — high priority.** Useful declension/conjugation paradigms, lexicon constraints, IPA conversion, custom scripts, lexical families and CSV/PDF/dictionary export. Port selected rule/format logic to Go rather than running a Java desktop UI inside the platform. Add Age-pinned language evolution and entity-name links. **Service: no**; optional external PolyGlot file exchange. Quiz features are lower priority than authoring and historical linguistics.

### 54. pynames

**Evidence:** [README](../../pynames/README.rst), `pynames/names.py`, `from_list_generator.py`, `from_tables_generator.py`, `pyproject.toml`. Python list/table name generation; BSD-3-Clause.

**R/I.** Useful optional author-owned naming tables, name forms, grammatical variants and templates. Implement seeded equivalents in Go tied to a language/culture revision. Show candidate names and allow accept/edit/reject; never silently name every society or write its history. **Service: no.** Review bundled franchise/culture datasets separately, or ship only original neutral examples and let authors provide their own tables.

### 55. range-family

**Evidence:** [README](../../range-family/README.md), `src/family/src/tree_diff_widget.cpp`, associated merge dialogs, `src/CMakeLists.txt`. C++/Qt with submodules; GPL-3.0 root.

**I/D.** Useful side-by-side tree comparison and explicit add/update/remove reconciliation, directly relevant to semantic Git merge UX. Reimplement conflict review in Next and graph changes in Go. Its desktop runtime/submodules are unnecessary, and AI chat files present in the checkout are outside scope. **Service: no**; external file exchange only if there is user demand and a documented format.

### 56. scenario-forge

**Evidence:** [README](../../scenario-forge/README.md), `map_backend/store.py`, `map_backend/service.py`, `js/core/map_renderer.js`, `js/workers/`, `package.json`. JavaScript map editor/Python tooling; MIT root.

**R/I — high priority for political cartography.** Ownership versus control, frontlines, labels, strategic markings, rendering/export limits and editable project packages are valuable. Adapt small isolated utilities or implement equivalent Next/Go tools. Earth administrative data and HOI4/TNO scenarios are not default fictional geography; audit dataset/game-content rights independently. **Service: optional existing map backend only for experimentation**, not production core. Its SQLite runtime store is not a replacement for Age-owned canonical files.

### 57. SCSG

**Evidence:** [README](../../SCSG/README.md), `scripts/collections/characteristics/`, `scripts/collections/background/`, `scripts/generator/`, `LICENSE.txt`. Browser JavaScript character-sheet generator; GPL-3.0.

**I.** Useful customizable sheet sections and descriptive trait scales. Offer author-created templates and, optionally, transparent random tables, with no assumption that generated traits determine a character's actions. Rebuild native components instead of copying a fixed generator. **Service: no.** Predetermined physical/personality distributions and stock background text are not needed for the general writing platform.

### 58. Simulator

**Evidence:** [README](../../Simulator/README.md), `hooks/useSimulation.ts`, `lib/worldCountries.ts`, `lib/AIEngine.ts`, `package.json`. Next/Three.js/Firebase and generative-AI dependencies; no root license.

**I/X.** Useful only as a visual reference for dependency/cascade inspection and map-linked dashboards. Hardcoded geopolitical power/stability scores and AI emergence/aftermath analysis are inappropriate foundations for author-controlled fiction. A deterministic impact inspector should show actual user-defined links and assumptions. **Service: no.** Reusing its Next stack would import the wrong product model and uncleared code.

### 59. story-forge

**Evidence:** [README](../../story-forge/README.md), `src/ts/characters.ts`, `src/ts/model/`, `src/ts/storage/`, `package.json`. Svelte/Tauri/chat/model infrastructure; MIT root.

**X, limited I.** Important discrepancy: README advertises a plain TypeScript branching-story library, but the supplied source and dependencies include Ollama, provider SDKs, transformers and chat/character-card systems. Do not rely on the README's `StoryRunner` example as an implemented engine. Generic character-card/media serialization might inspire import UX, but better references cover it. **Service: no**; model/chat services are excluded, and stripping them is poor value compared with a native writing engine.

### 60. storyboard

**Evidence:** [README](../../storyboard/README.md), `src/nodeGraph.ts`, `src/state.ts`, `src/predicate.ts`, `package.json`. TypeScript storylet/nonlinear narrative runtime; MIT.

**I/R, later only.** Useful distinction among author-defined story graph, conditions, state and rendering. This is authored interactive fiction, not AI writing. Implement a bounded optional choice/storylet mode without changing historical Age state. Its README explicitly warns it is academic software unsuited to general production use; audit compiler dependency and predicate behavior before considering reuse. **Service: no**; an embedded runtime suffices for static interactive publishing.

### 61. storyboarder

**Evidence:** [README](../../storyboarder/README.md), `src/js/models/board.js`, `scene.js`, `src/js/exporters/`, `package.json`, `build/license_en.txt`. Electron/JavaScript visual storyboarding; no standard root license found, packaged EULA present.

**I/D; reuse uncleared.** Strong optional UX reference for sketch boards, shot order, annotations, timing, animatics and Fountain screenplay exchange. Build lightweight scene boards using uploaded/drawn assets before considering a full drawing/3D suite. Do not infer unrestricted reuse from its open-source positioning; inspect exact code/asset terms. **Service: no**; a separately installed external storyboarder can exchange images and screenplay files. VR and heavy shot generation are outside core scope.

### 62. Terra

**Evidence:** [README](../../Terra/README.md), `crates/terra-gpu/src/graph.rs`, `crates/terra-render/src/clipmap.rs`, `crates/terra-core/src/terrain/`, `terrain_recipe.rs`. Rust terrain editor/renderer; MIT; explicit unfinished/unstable warnings.

**I/R, evaluated selectively.** Valuable layers/masks, compiled computation passes, dirty regions, tiled pyramids, level of detail and progressive preview concepts. Implement the main model in Go; consider isolated Rust/WASM only after benchmarks prove necessity. Do not build on its unstable project format or assume export/viewport features are complete. **Service: not recommended now**; a future local compute worker is possible, but no native desktop GUI dependency is justified.

### 63. the-lamplighter

**Evidence:** [README](../../the-lamplighter/README.md), `index.html`, `templates/quote-card.html`, `references/`, `assets/fonts/`. Static browser newspaper/prop layout; MIT root.

**R/I.** Useful optional in-world publications: newspapers, wanted posters, telegrams and quotation cards with editable text and print/image export. Rebuild as Next publication templates linked to dated lore documents and author-supplied content. Bundle fonts/scripts offline and verify their individual licenses. **Service: no.** Its agent skill and automatic copywriting workflow are not used or incorporated; only manual layout/rendering ideas fit the product.

### 64. TiddlyWiki5

**Evidence:** [README](../../TiddlyWiki5/readme.md), `core/modules/wiki.js`, `core/modules/widgets/`, `plugins/tiddlywiki/geospatial/`, `license`. JavaScript wiki; BSD-style root terms, separately licensed plugins/dependencies.

**I/R.** Useful backlinks, transclusion, filters, reusable templates, portable documents and static publishing. Adopt a safe, bounded subset of those interactions over native entity IDs and Age context. Do not allow arbitrary wiki executable content in world files. **Service: optional Node wiki companion/publication target**, unnecessary because native codex/static export meets the core need. Avoid making TiddlyWiki a second authoring database.

### 65. tidyfamily

**Evidence:** [README](../../tidyfamily/README.md), `R/family_tree.R`, `R/family_get.R`, `R/family_member.R`, `DESCRIPTION`. Small R genealogy package; GPL-3.0.

**I/D.** Useful union-as-node representation and generation-aware genealogical diagrams. Use equivalent n-ary relationships in Go and interactive Next views. The source's father/mother conventions and README's unresolved layout concerns are too narrow to adopt wholesale. **Service: no**; adding an R process brings little value. Sample dynasties can inform synthetic test shapes without copying their data into the product.

### 66. timelines

**Evidence:** [README](../../timelines/README.md), `src/components/MapView.jsx`, `src/utils/dateUtils.js`, `src/utils/timelineUtils.js`, `src/utils/viewerPackageStore.js`. React/Electron; GPL-3.0.

**I/D — high-value timeline UX.** Events, spans, eras, tags/groups, linked notes, spreadsheet editing, geographic selection and viewer packages match historical exploration. Reimplement with fictional calendar coordinates and stable Age/entity references. An era in a visual timeline is not a complete historical snapshot. **Service: no**; optionally import/export documented timeline packages later. Remote wiki retrieval and Earth tile services must not become offline prerequisites.

### 67. Tree-Monk

**Evidence:** [README](../../Tree-Monk/README.md), `src/main/db/schema.ts`, `src/main/db/mapData.ts`, `src/shared/familysearch.ts`, `LICENSE`. Electron/React/SQLite; PolyForm Noncommercial.

**I/X.** Useful integrated research board, sources, genealogy, atlas and historical filtering as product concepts. Implement independently under KRIEMHILD's architecture. Do not adopt its restricted code, external FamilySearch automation, AI connectors or noncommercial product assumptions. A single SQLite user file differs from the required portable Git-aware world snapshots. **Service: no**; GEDCOM/media exchange with user-owned installations is a possible later adapter.

### 68. V20DA-Character-Sheet

**Evidence:** [README](../../V20DA-Character-Sheet/README.md), `index.html`, `index.hostable.html`, Google Apps Script instructions. Single-page game-specific sheet; no root license.

**I/X.** Useful printable ledger, portable JSON character exchange and sheet layout inspiration. Implement neutral dossier templates and exports. Vampire/Dark Ages rules/assets, Google Sheets hosting, password-sharing patterns and fixed desktop layout are not core platform features. **Service: no**; optional Google-backed hosting is exactly the sort of extra dependency the proposed architecture avoids. Treat game material and code permissions as unresolved.

### 69. vassal

**Evidence:** [README](../../vassal/README.md), `vassal-app/src/main/java/VASSAL/build/module/Map.java`, map modules, `pom.xml`, `LICENSE`. Java boardgame engine; LGPL-family terms, root text LGPL-2.1 with differing README wording.

**I/D.** Useful map layers, counters, stacking, authored orders, replay/undo, hidden information and modular scenarios. Implement a later tactical workspace over canonical military records and dated commands. Do not add a complete game engine or assume user-contributed boardgame modules share its license. **Service: no**; optional exports to an external desktop tool could follow demand. Verify exact file terms before extraction given license wording differences.

### 70. webtrees

**Evidence:** [README](../../webtrees/README.md), `app/Family.php`, `app/Schema/`, report/relationship modules, `composer.json`. PHP genealogy web app; GPL-3.0-or-later.

**I/D.** Strong reference for source/media records, tree variants, privacy, multilingual genealogy and GEDCOM interoperability. Reimplement appropriate user journeys and conversion checks in Go/Next; human historical schemas cannot dictate fictional family/calendar restrictions. **Service: viable optional external genealogy archive**, but not recommended as native family storage. File exchange preserves one Age-owned source of truth and avoids a mandatory PHP/database stack.

### 71. world-maker

**Evidence:** [README](../../world-maker/README.md), `singletone/project_setting.gd`, `systems/dependency/map_resource_manager.gd`, map/wiki systems, `project.godot`. Godot/GDScript; MIT root.

**I/R.** Useful approachable local projects, wiki templates, map layers/pins and article IDs. Apply those concepts to quick-start world creation and linked image maps. The inspected map resource is principally image metadata and article links, not semantic terrain or historical inheritance. **Service: no**; the Godot runtime/UI is unnecessary for a Next frontend. Any copied assets/addons need separate license checks.

### 72. Worldbinder

**Evidence:** [README](../../Worldbinder/README.md), `packages/contracts/src/maps.ts`, `packages/contracts/src/calendar.ts`, `apps/api/src/database/schema.ts`, roadmap/decisions. React/NestJS/PostgreSQL/Redis/object storage; no root license.

**I — useful connected-product reference.** Typed API contracts, permission-aware entities, continuity, plot-thread/map links and structured dates inform native design. Borrow architectural lessons, not its infrastructure requirements or uncleared source. Its distributed service stack is oversized for a self-contained first release. **Service: technically deployable, not recommended**; it would duplicate world authority. Hosted permission concepts belong later behind the same Go model.

### 73. worldbuilder

**Evidence:** [README](../../worldbuilder/README.md), `scripts/graph.py`, `scripts/worldbuilder.py`, `webapp/`, `pyproject.toml`. Python/Flask/file-oriented CLI and AI integrations; Apache-2.0 root.

**R/I, non-AI subset only.** Useful typed-reference graph validation, dangling-link detection, neighborhood/path queries and compile-to-document concepts. Implement native Go checks; evaluate isolated permissive utilities only if worth porting. Slug-based identities are unsuitable across renames and Ages. **Service: no**; exclude Claude/MCP creative workflows, image generation and voice generation. README download links were not followed; inspect actual code rather than relying on packaging claims.

### 74. WorldBuilding

**Evidence:** [README](../../WorldBuilding/readme.md), `TerrainGenerator.py`, `NomadModel.py`, `TributeModel.py`, `TributeNarrative.py`. Python/NetworkX/notebook research prototype; no root license.

**D/I/X.** Useful conceptual pipeline connecting terrain, settlement suitability and explicit event logs. Optional author-defined numeric experiments could expose similar intermediate outputs. Reject automatic nomad decisions, tribute-driven wars and generated historical narrative as the authoring model. Its simple wealth/coalition assumptions are not general social science. **Service: no.** Useful to test dependency visualization ideas, but not a backend foundation or a cleared code source.

### 75. worldcraft-codex

**Evidence:** [README](../../worldcraft-codex/README.md), `app/world-planning.ts`, `app/map-export.ts`, `app/project-references.ts`, `package.json`, feature documentation. Next/Electron/SQLite/Tiptap; no root license.

**I — strong UX reference, no code adoption by default.** Relevant integrated manuscript, map canvas, timelines, relations, revision recovery, foreshadowing and character knowledge. Implement equivalent workflows over complete Ages, replacing the current-project model. Exclude AI writing/operators/memory entirely. README states a Windows release-candidate context; do not assume general cross-platform readiness. **Service: no**; the familiar Next stack does not remove license or architectural incompatibilities.

### 76. worldengine

**Evidence:** [README](../../worldengine/README.md), `worldengine/step.py`, `generation.py`, `model/world.py`, `plates.py`, `pyproject.toml`. Python/numeric libraries and plate simulation; MIT root.

**R/I — high priority.** Useful staged physical generation, climate/erosion/biomes and separated world data/export. Port selected algorithms to Go where practical or use its outputs as reference fixtures. Validate native library dependencies, memory use and world format support. **Service: plausible optional local Python terrain worker**, with bounded jobs returning artifacts; the default Go generator must work without it. No automatic civilization history is required.

### 77. WorldGeneratorFinal

**Evidence:** [README](../../WorldGeneratorFinal/README.md), `Assets/Scripts/MapData.cs`, `WrappingWorldGenerator.cs`, `SphericalWorldGenerator.cs`, `Generator.cs`. Unity/C# tutorial; MIT root.

**R/I/D.** Useful examples of seamless wrapping, spherical sampling, heat/moisture fields and biomes. Implement topology/seam tests in the native map-space engine and compare tutorial outputs. Do not carry Unity rendering/runtime into the platform. Credited sphere/noise sources need provenance review before porting. **Service: no.** This is an educational generation reference, not a complete historical cartography editor.

### 78. WorldScript-Studio

**Evidence:** [README](../../WorldScript-Studio/README.md), `services/projectDocument.ts`, `services/fs/projectFsStore.ts`, `snapshotFsStore.ts`, `features/project/`, `app/store.ts`, manifests. React/TypeScript and native/storage modules; MIT root.

**R/I — high priority for non-AI writing/persistence.** Useful scene/plot organization, comments, snapshots, conservative schema admission, preservation of unknown data and platform/storage boundaries. Evaluate isolated editor/export/storage ideas; keep canonical persistence in Go. Do not import its entire Redux/native migration stack, AI-core packages or provider tooling. **Service: no**; a native KRIEMHILD editor should not depend on another writing application. Advertised test counts do not substitute for tests of extracted code.

### 79. wtfamily-be

**Evidence:** [README](../../wtfamily-be/README.rst), `wtfamily/models.py`, `schema.py`, `restful.py`, `etl/gramps_xml_to_mongo.py`, `COPYING`. Python/Flask/MongoDB; GPL-3.0 root versus LGPL wording in inspected source headers.

**I/D.** Useful ETL round-trip concerns, source-rich genealogy and REST/model separation. The README's abandonment of its earlier YAML/Git design is a valuable warning to test editable-file usability and merging rather than assume portability alone solves them. **Service: possible existing genealogy API, not recommended**; avoid MongoDB and a second authority. Resolve license inconsistency before reuse; preserve unsupported import data rather than claiming perfect conversion.

### 80. wyrd-ecs-core

**Evidence:** [README](../../wyrd-ecs-core/README.md), `index.html`, six tracked files; no root license or package manifest.

**X.** The README describes an ambitious AI/ECS world model, but the checkout does not provide a normal inspectable ECS library. Its HTML entry point contains an obfuscated payload. No execution or download was attempted, and this review does not claim to identify what that payload does. Generic entity-component ideas are already covered by better sources; there is no demonstrated component worth adopting here. **Service: none recommended.** Its AI-centered purpose also conflicts with the product requirement.

## Reuse and service policy for implementation

1. Start with native Next/Go modules and the KRIEMHILD contracts. Every imported component must solve a named problem without taking ownership of canonical world state.
2. For a candidate, record exact commit/file, license/NOTICE, transitive dependencies, copied upstream provenance, data/font rights, modifications, maintenance owner and replacement path.
3. Prove isolated correctness, deterministic behavior where required, offline operation, resource bounds and compatible packaging. Root license presence alone is insufficient.
4. Importers and optional workers use versioned interfaces and return previewable data. They do not mutate project files directly.
5. Prefer native features over accompanying applications. Retain services only where a measured capability justifies a separately packaged runtime, with an offline fallback for core work.
6. AI functions remain excluded even in otherwise useful permissive projects. Never import a broad package that quietly downloads a model or sends creative content to a provider.

See [the platform plan](PLATFORM_PLAN.md) for the chosen feature boundaries and implementation sequence.
