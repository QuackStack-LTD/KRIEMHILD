# Atlas and historical entities

The World is the persistent root. Terrain and Map remain specialized records; people,
politics and other logical entities do not need a terrain or home map.

## Historical state

Entity and relationship records are keyed by World, Age and ID. `continuityId` links
historical manifestations; older records fall back to their existing ID. Creating an
Age copies mutable properties, names, representations and relationships while sharing
immutable terrain snapshots. The creation dialog reviews inherited entities and allows
bulk inclusion/exclusion and name changes. Profile changes and deletions affect only the specified Age.
Atlas and Characters display the active Age without comparison controls. Historical
records and profile version links retain access to earlier states.

`internal/history/registry.json` is the shared entity/relationship registry consumed by
Go and React. Endpoint categories, reciprocal relationships and direction are validated
on writes and import. `Relation` stores description, notes, context and optional validity
strings; numeric overlapping attendance years can derive schoolmate connections in graph
views. Derived connections are clearly distinguished from authored records. Names store
type, language, script, cultural/geographical context and validity without new identities. Builder map labels can select a saved name variant or a local label without changing canonical identity or labels on other maps.

## Workspaces

- **Atlas:** terrains, countries, empires, cities, villages, capitals and natural
  features. Create spatial entities through Map Builder, then name and save the drawn
  geometry. Countries and empires can acquire additional disconnected regions under one
  logical identity. Capitals are cities linked to a political entity by `capital-of`.
- **Characters:** one combined graph of characters, families, social connections and
  linked places/institutions. Search, focus, endpoint selection and connection-type filters
  hide presentation details without deleting underlying relationships. Drag nodes to arrange
  them; positions are persisted in each entity's Age-scoped `graphPosition`. Click a node
  for its profile; drag any of its eight ports to another node to create a typed connection.
  Nodes show uploaded portraits or a compact placeholder. Family connections preserve the
  shared parental junction and single descent line per child. Focused Family, Relationship
  and Places & Affiliations graphs open from the character profile. Endpoint-aware choices
  prevent location or institutional types between two characters.
- **Organizations:** searchable directory and dedicated institutional profiles; see below.
- **Culture & Religion and Library:** reserved workspace pages using the shared registry.
- **Writing:** independent creator drafts in `writing_documents`, with explicit saving,
  optimistic revisions, text import/download and deletion. They are not fictional entities
  and are deliberately excluded from World ZIPs.

The main navigation and Age context are shared. Builders/generators hide the worldbuilding
sidebar and timeline. Profiles show their own Age indicator without the timeline. Time
Graph and Writing also hide the global timeline.

## Representations and persistence

A drawable's optional `logicalId` connects additional territorial regions to an existing
entity. Legacy drawables use their own IDs. Multiple map representations therefore remain
separate geometries without duplicating the political identity. Spatial containment never
implies governance. Deleting a map removes its representations while keeping independent
logical identities and their non-spatial relationships. Builder synchronization preserves profile properties and name variants.

Existing JSON-backed database rows and the versioned World ZIP retain all new optional
fields. Existing archives without those fields remain valid. Entity deletion uses historical
tombstones and removes affected representations and relationships in that Age only.

## Terrain relief

3D surface selection is independent of longitude wrapping. Flat terrain relief is the
default, with a separate globe option. Both use the same physical elevation, bathymetry,
detail tiles and draped entities. Settlement Builder uses flat relief and clips its child
extent. View preferences are saved with the map or generator project settings.

## Place maps and immediate entity persistence

Atlas lists terrains independently. Cities, villages and political places use their
linked child map as the tile preview and profile banner; an undrawn place uses the
shared placeholder. The profile opens or creates that entity's Settlement Builder
map, preserving its existing footprint and parent terrain coordinates. The builder
provides links back to the parent map and place profile.

Saving an authored name immediately publishes its logical entity, representation and
spatial relationships to the active Age; undo and redo update that identity as well.
Unnamed shapes receive unique `example-<uuid>` names. They remain map drawables, are
excluded from logical entity tables and have no illustrated labels. Full terrain changes,
explored detail and artwork still require Save. World archives include those immediate
entity records even if the terrain snapshot has not changed. Unnamed objects persist in map snapshots and World ZIPs and can be named after reopening.
Map Builder manages geography and whole-map places; Settlement Builder offers settlement
and infrastructure tools, without geological creation or terrain sculpting.

Collection grids have four equal columns at desktop sizes, with consistent card heights
and responsive two/one-column layouts. Place profiles group relationships by counterpart,
combining equivalent spatial facts without inferring political authority.

## Years within an Age

The **Years** action in the bottom bar opens the active Age's Year Timeline. Profiles
also link to it. Saving start/end years creates persisted `history_years` records,
keyed by Age and normalized whole year; year zero and negative years are supported.
Each record groups linked beginning/ending events from all entities with that date.
Descriptions use the source profile's summary, biography or notes. Years have no ordinary
entity profile and do not appear in entity collections or relationship endpoint pickers.

Events use stable source-entity/endpoint keys. Renaming or editing descriptions updates
the event; changing a year moves it; clearing dates or removing an entity removes its
active events. Empty year anchors remain available, particularly when an Age was started
from one. Legacy numeric profile/builder dates are indexed automatically. Legacy textual
dates remain readable but do not become numbered years until replaced by a whole year.

**Start new Age here** supports linear continuation, branching and parallel timelines.
It stores `startedFromYear` provenance and immediately opens the main Time Graph after
creation. The new Age inherits the current saved state; this does not reconstruct an
unrecorded snapshot from the selected year. Events inherited into another Age are
independent. Year records, source event references and Age origins survive database
restarts and World ZIP export/import. Existing archives without years remain supported.

## Organizations

`organization` (and legacy organizational types) uses the shared entity model.
`organization-unit` and `organization-position` are specialized Age-scoped records with
`organizationId`, optional `parentUnitId` / `unitId`, order, description and custom properties.
Units form an optional outline; same-organization ownership and cycle checks are enforced.
Independent organizations can join a federation through a membership without becoming units.

Memberships use `member-of` relationships; appointments use `appointed-to` relationships
pointing to reusable office definitions. Both carry role, rank, status, unit, appointment
source, validity, description and notes. People can have several memberships and offices.
Offices can be vacant or shared; overlapping active appointments to an unshared office
are rejected. Transfer or close memberships without erasing their historical records.

The directory filters by type, scope, status, location and affiliation. Profile tabs cover
Overview, Structure, Members & Offices, Presence & Operations, Connections and History.
Presence is represented by typed links to existing places, separately from authority and
unit hierarchy. No headquarters, leader, parent organization or membership is mandatory.
New institutional events and their links are saved atomically and feed the Age's Year
Timeline when dated. Founding/dissolution dates use the same year indexing as other profiles.

All records, relationship metadata, portraits and graph positions use the existing
Age inheritance, database and World ZIP path. New Ages copy their mutable state and retain
continuity IDs; no institutional schema or data outside the World archive is required.
The Time Graph inspector lists only newly introduced political entities, characters and
organizations relative to the selected Age's inherited source.

## Culture & Religion

Concepts reuse Age-scoped logical entities and continuity IDs. The shared registry includes
religions, cultures, worldviews, philosophies, ideologies, movements, practices, doctrines,
narratives and a blank `concept` type with a custom classification. Types suggest content;
no hierarchy, deity, institution, scripture or uniform observance is required.

The directory searches names, summaries and structured content; collection, context,
related-tradition and status filters use the active Age. Sort by name, creation or editing
time. Profiles are articles with optional content sections, typed fields, contextual
variations, relationship lists and historical-version comparisons. Main workspaces retain
the Age timeline; profiles show their own Age context without the global bar.

`Entity.content` uses composition schema version 1 (`internal/history/concepts.go`).
Fields carry validated definitions (text, long text, number, Boolean, whole-year date,
Age/entity reference, single/multiple choice and repeatable typed sections). Definitions
can be reused from another concept. Invalid values, unknown schema versions, broken
references and excessive nesting are rejected. Dates currently use whole years because
there is no calendar model. Old Worlds without content remain valid.

Claims separate their statement and interpretation from author-established `worldTruth`.
Unset truth differs from unknown, disputed or undecided. Acceptance/rejection, authority,
prevalence, context, validity and provenance belong to parties' typed relationships or
contextual variations; the engine never assigns a belief to everyone in a culture.

Promoting embedded content atomically creates a logical entity, transfers fields, claims,
contexts and links, and leaves a reference in the original article. The same practice or
claim can then be linked from multiple traditions. Referenced records must be unlinked
before deletion to avoid broken content. Age inheritance deep-copies mutable composition
records while retaining continuity; database and World ZIP persistence include the entire
schema and relationship metadata. Loading does not recreate or reinterpret claims.

All logical entity start/end dates, including organization/unit/office dates and embedded
concept section dates, feed the Age's Year events. Legacy `born`/`died`, `startDate`/`endDate`
and founding/dissolution aliases normalize to `startYear`/`endYear` on edits. Renames, date
changes, clearing dates and section promotion reconcile event identities without duplicates.
Terrains, maps and the separate Writing workspace do not create Year events.

## Navigation and connection editing

Navigation order is Atlas, Biosphere, Magic & Technology, Culture & Religion,
Characters, Organizations, Library; Writing and World export remain at the bottom.
Biosphere provides habitat-based generation, species profiles and ecological rules;
`flora` remains a compatibility route. Magic & Technology is the System Designer
workbench for rules, capabilities, dependencies and consistency analysis. Library
provides in-world archives, versioned files and format-specific editors.

Character graph nodes are 160 units wide. Drag an occupied connection dot to another dot
to transfer that endpoint while preserving relationship ID and metadata. When several
connections share the dot, choose which to move; Shift-drag creates a fresh connection.
Two-parent child connections move together atomically. Incompatible endpoints, self-links
and contradictory parentage are rejected without altering saved connections.


## Biosphere (version 1)

The Biosphere navigation replaces Flora & Fauna. Its Overview, Habitats, Species
Catalogue and Generation Rules views use the active World and Age, with a terrain
selector. Lifeform profiles hide the global timeline while retaining the Age label.
The legacy `/flora` route still opens Biosphere.

Accept a physical terrain in Atlas first. Guided generation defaults to a balanced
familiarity preference. Review executable biological rules, generate a habitat-grouped
preview, then accept selected populations. Quick mode produces expandable archetypes;
Advanced mode shares exactly the same records, with editable tolerances, taxonomy,
traits, lifecycle, requirements, range statuses and ecological connections. A species
can also be created before any terrain exists. Nothing in a preview is persisted until
acceptance. Species names, traits and range records remain editable afterward.

Habitats use connected parent-grid cell sets from the saved terrain and its authored
surface changes, final category, seasonal climate, elevation, hydrology and geological
substrate. Forest canopy, understory and floor share spatial footprints but differ in
light. Ocean, inland water and terrestrial habitats remain separate. Geothermal surface
habitats require supplied hot springs, not merely regional underground heat potential.
Habitats carry a stable region identity plus a revision ID derived from their actual
conditions and cells. Changed regions receive new revision IDs, retaining the exact
geometry of previously confirmed occurrences; the workspace flags them for review.

Generation applies medium, thermal, elevation, depth, salinity, substrate, native
habitat and sunlight requirements before preference scoring. It builds producers and
decomposers before consumers and host-dependent viruses. Food/host links reference
actual populations in the same habitat. Rejecting a food or host population also rejects
unsupported new dependent populations at acceptance. A virus has explicit host and
transmission records; infection does not automatically mean disease. Reference native
ranges are limited to suitable regions around deterministic refugia. Ranges are
habitat populations, never one individual entity for every map cell.

The core reference catalogue consists of ecological archetypes and a few named species.
Numerical tolerance envelopes are broad **game-model approximations**, not scientific
species-survival datasets. Reference grounding includes the National Park Service's
[red fox ecology](https://www.nps.gov/yell/learn/nature/red-fox.htm) and
[thermophilic archaea](https://www.nps.gov/yell/learn/nature/thermophilic-archaea.htm).
Curated Earth references remain distinguished from fictional, authored and placeholder
lifeforms. Comparable fictional ecological niches share species identities across
habitats. JSON reference/template packs use the same validated Lifeform schema.

The idea interpreter is a limited, local preset interpreter (luminous fungi,
heat-adapted plants, fungivory and canopy gliding), not an AI service. It reports its
limitations and requires review. The structured editor is authoritative: habitat and
substrate conditions, role/group/species targets, traits, tolerances, food dependencies,
lifecycle, abundance, priority and conflict policy. Specific-species rules produce an
adapted variant rather than replacing the author's original. Unrecognized prose never
silently introduces extra biological properties.

Persistence extends the existing Age-scoped `Entity` with a versioned `biology` profile.
Species are independent of terrain ownership. Each terrain/Age's versioned `Biosphere`
state stores habitats, occurrences, interactions, settings, rule definitions, reference
packs, source signature and diagnostics in `history_biospheres`. Existing SQLite and
PostgreSQL storage paths use the same model. World ZIP schema 1 includes the new optional
records alongside saved terrain snapshots and explored tiles; older Worlds without
Biosphere data remain valid. Loading never regenerates saved ecological state.

New Ages deep-copy biological profiles and ecological state while preserving species
continuity. Accepted populations are confirmed by default; locked/confirmed ranges and
existing species take precedence over generation. Regeneration adds compatible records
within the chosen scope. Editing a saved species or population flags dependent ranges
for review rather than deleting or moving them. Per-habitat/role generation variants
allow preview regeneration without perturbing unrelated habitats. Historical extinction
is a range status, preserving the species identity and earlier Ages.

API: `GET /api/biosphere/catalogue`, `GET /api/worlds/{world}/biosphere/{terrain}?age=...`,
and `POST` to the latter route for deterministic preview/acceptance (with World revision
and source checks). The existing historical `entity` and `biosphere` commands edit saved
records; `biosphereEdit` updates a single occurrence, interaction or settings record
without uploading an entire ecological state. Acceptance supports all-with-exclusions
for large previews. Atlas's existing preview endpoint accepts a read-only habitat highlight.

This release is a lightweight ecological foundation. It does not simulate population
dynamics or evolution, infer arbitrary biological prose, resolve migration paths at
sub-cell detail, or fabricate caves without underlying environmental support. Distribution
abundance is a relative habitat potential, not a census. Manual lifecycle, lineage and
seasonal records support further worldbuilding without making those simulation claims.
