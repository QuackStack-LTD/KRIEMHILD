import useUnsavedChanges from "./ui/useUnsavedChanges.js";
import { confirmAction } from "./ui/feedback.js";
import Dialog from "./ui/Dialog.jsx";
import { useEffect, useState, useMemo } from "react";
import { label, activeEntities } from "./model.js";
import {
  groups,
  roles,
  statuses,
  blankLifeform,
  interpretIdeas,
  catalogueRows,
  habitatStats,
  groupOccurrences,
  bioRequest,
} from "./biosphere.js";
import { RulesEditor } from "./BiosphereEditors.jsx";
import "./biosphere.css";
function Select({ title, value, onChange, options, any = false }) {
  return (
    <label>
      {title}
      <select value={value} onChange={(e) => onChange(e.target.value)}>
        {any && <option value="">All</option>}
        {options.map((v) => (
          <option key={v.id || v} value={v.id || v}>
            {v.name || label(v)}
          </option>
        ))}
      </select>
    </label>
  );
}
export function Preview({
  preview,
  accepted,
  setAccepted,
  locked,
  setLocked,
  names,
  setNames,
  onRegenerate,
}) {
  const [page, setPage] = useState(0),
    [filter, setFilter] = useState("");
  const byID = useMemo(
      () => new Map(preview.species.map((e) => [e.id, e])),
      [preview],
    ),
    byHabitat = useMemo(() => groupOccurrences(preview.state), [preview]),
    hs = preview.state.habitats.filter(
      (h) =>
        byHabitat.has(h.id) &&
        (!filter || h.name.toLowerCase().includes(filter.toLowerCase())),
    );
  const pageCount = Math.ceil(hs.length / 8),
    current = hs.slice(
      Math.min(page, Math.max(0, pageCount - 1)) * 8,
      (Math.min(page, Math.max(0, pageCount - 1)) + 1) * 8,
    );
  return (
    <div>
      <div className="bio-summary">
        {preview.species.length} lifeforms · {preview.state.occurrences.length}{" "}
        habitat occurrences
      </div>
      <label>
        Find preview habitat
        <input
          type="text"
          value={filter}
          onChange={(e) => {
            setFilter(e.target.value);
            setPage(0);
          }}
        />
      </label>
      {current.map((h) => {
        const rows = byHabitat.get(h.id) || [];
        return (
          <section className="bio-preview-group" key={h.id}>
            <div className="section-heading">
              <h3>{h.name}</h3>
              <button
                type="button"
                onClick={() =>
                  setAccepted([
                    ...new Set([...accepted, ...rows.map((o) => o.id)]),
                  ])
                }
              >
                Accept group
              </button>
              <button
                type="button"
                onClick={() =>
                  setAccepted(
                    accepted.filter((id) => !rows.some((o) => o.id === id)),
                  )
                }
              >
                Discard group
              </button>
              <button type="button" onClick={() => onRegenerate(h.id)}>
                Regenerate group
              </button>
            </div>
            <p>{h.description}</p>
            {rows.map((o) => {
              const e = byID.get(o.speciesId);
              return (
                <div className="bio-preview-entry" key={o.id}>
                  <label>
                    <input
                      type="checkbox"
                      aria-label={`Include ${e.name}`}
                      checked={accepted.includes(o.id)}
                      onChange={(ev) =>
                        setAccepted(
                          ev.target.checked
                            ? [...accepted, o.id]
                            : accepted.filter((x) => x !== o.id),
                        )
                      }
                    />
                    <input
                      type="text"
                      aria-label="Lifeform name"
                      maxLength={200}
                      value={names[e.id] ?? e.name}
                      onChange={(ev) =>
                        setNames({ ...names, [e.id]: ev.target.value })
                      }
                    />
                  </label>
                  <small>
                    {e.biology.group} · {e.biology.origin} ·{" "}
                    {e.biology.roles.join(", ")} ·{" "}
                    {Math.round(o.abundance * 100)}% potential
                  </small>
                  <p>{o.reason}</p>
                  {Object.entries(e.biology.traits || {})
                    .filter(([k]) => k !== "reference")
                    .map(([k, v]) => (
                      <small key={k}>
                        {label(k)}: {v}
                      </small>
                    ))}
                  <label>
                    <input
                      type="checkbox"
                      checked={locked.includes(o.id)}
                      onChange={(ev) =>
                        setLocked(
                          ev.target.checked
                            ? [...locked, o.id]
                            : locked.filter((id) => id !== o.id),
                        )
                      }
                    />{" "}
                    Lock this range
                  </label>
                  <button
                    type="button"
                    onClick={() => onRegenerate(h.id, o.speciesId)}
                  >
                    Regenerate entry
                  </button>
                </div>
              );
            })}{" "}
          </section>
        );
      })}
      <div className="history-actions">
        <button disabled={page === 0} onClick={() => setPage((p) => p - 1)}>
          Previous habitats
        </button>
        <span>
          {Math.min(page + 1, pageCount) || 0} / {pageCount}
        </span>
        <button
          disabled={page + 1 >= pageCount}
          onClick={() => setPage((p) => p + 1)}
        >
          Next habitats
        </button>
      </div>
      {preview.exclusions?.length > 0 && (
        <details>
          <summary>Dependency exclusions ({preview.exclusions.length})</summary>
          <ul>
            {preview.exclusions.map((s, i) => (
              <li key={i}>{s}</li>
            ))}
          </ul>
        </details>
      )}
    </div>
  );
}
export default function Biosphere({
  doc,
  age,
  command,
  onDocument,
  onProfile,
  onAtlas,
  query = new URLSearchParams(),
}) {
  const terrains = doc.terrains.filter((t) => t.ageId === age.id),
    [terrainID, setTerrainID] = useState(
      query.get("terrain") || terrains[0]?.id || "",
    ),
    [tab, setTab] = useState(query.get("habitat") ? "Habitats" : "Overview"),
    [loaded, setLoaded] = useState(null),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [settings, setSettings] = useState(null),
    [wizard, setWizard] = useState(null),
    [preview, setPreview] = useState(null),
    [accepted, setAccepted] = useState([]),
    [locked, setLocked] = useState([]),
    [names, setNames] = useState({}),
    [notice, setNotice] = useState(""),
    [confirmed, setConfirmed] = useState(false),
    [filters, setFilters] = useState({}),
    [habitat, setHabitat] = useState(query.get("habitat") || ""),
    [habitatSearch, setHabitatSearch] = useState(""),
    [cataloguePage, setCataloguePage] = useState(0),
    [habitatPage, setHabitatPage] = useState(0),
    [create, setCreate] = useState(false),
    [newName, setNewName] = useState(""),
    [newGroup, setNewGroup] = useState("plant");
  const saved = doc.biospheres?.find(
      (b) => b.ageId === age.id && b.terrainId === terrainID,
    ),
    state = saved || loaded?.environment,
    allSpecies = activeEntities(doc, age.id).filter((e) => e.biology),
    species = new Map(allSpecies.map((e) => [e.id, e])),
    stats = habitatStats(state),
    byHabitat = useMemo(() => groupOccurrences(state), [state]);
  useEffect(() => {
    if (!terrainID) return;
    let live = true;
    setError("");
    setLoaded(null);
    bioRequest(
      `/api/worlds/${doc.world.id}/biosphere/${terrainID}?age=${age.id}`,
    )
      .then((data) => {
        if (live) {
          setLoaded(data);
          setSettings(data.saved?.settings || data.environment.settings);
        }
      })
      .catch((e) => live && setError(e.message));
    return () => {
      live = false;
    };
  }, [doc.world.id, doc.world.revision, age.id, terrainID]);
  const settingsDirty =
    !!saved &&
    !!settings &&
    JSON.stringify(settings) !== JSON.stringify(saved.settings);
  useUnsavedChanges(settingsDirty, () => setSettings(saved?.settings));
  const edit = (k, v) => {
    setSettings((s) => ({ ...s, [k]: v }));
    setConfirmed(false);
  };
  async function work(fn) {
    setError("");
    setBusy(true);
    try {
      return await fn();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  async function generate(
    options = settings,
    replaceHabitat = "",
    replaceSpecies = "",
  ) {
    if (options.scope === "selected" && !options.habitatIds?.length) {
      setError("Select at least one habitat before generating.");
      return;
    }
    await work(async () => {
      const p = await bioRequest(
        `/api/worlds/${doc.world.id}/biosphere/${terrainID}`,
        { revision: doc.world.revision, ageId: age.id, settings: options },
      );
      if (replaceHabitat && preview) {
        const protectedIDs = new Set(locked),
          replaceable = new Set(
            preview.state.occurrences
              .filter(
                (o) =>
                  o.habitatId === replaceHabitat &&
                  (!replaceSpecies || o.speciesId === replaceSpecies) &&
                  !protectedIDs.has(o.id),
              )
              .map((o) => o.id),
          );
        const keep = preview.state.occurrences.filter(
          (o) => !replaceable.has(o.id),
        );
        const priorIDs = new Set(preview.state.occurrences.map((o) => o.id));
        const replacements = p.state.occurrences.filter(
          (o) =>
            o.habitatId === replaceHabitat &&
            !protectedIDs.has(o.id) &&
            (!replaceSpecies || !priorIDs.has(o.id) || replaceable.has(o.id)),
        );
        setPreview(p);
        setAccepted(
          p.state.occurrences
            .filter(
              (o) =>
                protectedIDs.has(o.id) ||
                replacements.some((x) => x.id === o.id) ||
                (keep.some((x) => x.id === o.id) && accepted.includes(o.id)),
            )
            .map((o) => o.id),
        );
      } else {
        setPreview(p);
        setAccepted(p.state.occurrences.map((o) => o.id));
        setLocked([]);
        setNames({});
      }
      setSettings(options);
      setWizard(3);
    });
  }
  // A preview is reproducible from its settings. Regeneration keeps the same
  // configured world seed, with explicit per-habitat/role variants for local regeneration.
  async function accept() {
    await work(async () => {
      const selectedRangeIDs = new Set(accepted);
      const d = await bioRequest(
        `/api/worlds/${doc.world.id}/biosphere/${terrainID}`,
        {
          revision: doc.world.revision,
          ageId: age.id,
          source: preview.state.source,
          settings,
          accept: true,
          ...(accepted.length > preview.state.occurrences.length / 2
            ? {
                acceptAll: true,
                excludeIds: preview.state.occurrences
                  .filter((o) => !selectedRangeIDs.has(o.id))
                  .map((o) => o.id),
              }
            : { acceptIds: accepted }),
          lockIds: locked,
          names,
        },
      );
      onDocument(d);
      setPreview(null);
      setWizard(null);
      setTab("Habitats");
    });
  }
  async function saveRules() {
    if (!saved || busy) return;
    await work(async () => {
      if (
        await command({
          kind: "biosphere",
          ageId: age.id,
          biosphereEdit: { terrainId: saved.terrainId, settings },
        })
      )
        setNotice("Generation rules saved.");
      else
        setError(
          "Unable to save generation rules. Your changes are still available.",
        );
    });
  }
  const rows = catalogueRows(doc, age.id, filters, state),
    h = (state?.habitats || []).find((x) => x.id === habitat),
    rangeRows = (state?.occurrences || []).filter((o) => o.habitatId === h?.id);
  const setFilter = (key, value) => {
    setCataloguePage(0);
    setFilters((f) => ({ ...f, [key]: value }));
  };
  const habitatMatches = (state?.habitats || []).filter((h) =>
    h.name.toLowerCase().includes(habitatSearch.toLowerCase()),
  );
  const visibleHabitats = habitatMatches.slice(
    habitatPage * 80,
    (habitatPage + 1) * 80,
  );
  return (
    <main className="world-page biosphere-workspace">
      <div className="workspace-heading">
        <div>
          <span className="eyebrow">
            {doc.world.name} / {age.name}
          </span>
          <h1>Biosphere</h1>
          <p>Living communities shaped by the world around them.</p>
        </div>
        <div className="history-actions">
          <button
            disabled={!loaded || busy}
            className="primary"
            onClick={() => {
              setWizard(0);
              setConfirmed(false);
            }}
          >
            Generate Biosphere
          </button>
          <button
            onClick={() => {
              setNewName("");
              setCreate(true);
            }}
          >
            Create lifeform
          </button>
        </div>
      </div>
      <div className="collection-toolbar">
        <Select
          title="Terrain"
          value={terrainID}
          onChange={async (v) => {
            if (
              settingsDirty &&
              !(await confirmAction(
                "Discard unsaved generation rules before changing terrain?",
              ))
            )
              return;
            setTerrainID(v);
            setPreview(null);
            setWizard(null);
            setHabitat("");
          }}
          options={terrains}
        />
      </div>
      <nav className="institution-tabs">
        {["Overview", "Habitats", "Species Catalogue", "Generation Rules"].map(
          (t) => (
            <button
              key={t}
              aria-current={tab === t ? "page" : undefined}
              onClick={() => setTab(t)}
            >
              {t}
            </button>
          ),
        )}
      </nav>
      {error && (
        <p className="bio-error" role="alert">
          {error}
        </p>
      )}
      {busy && <p role="status">Working on ecological records…</p>}
      {!terrains.length && (
        <p>
          Accept a physical terrain in Atlas to generate habitats. You can
          create independent lifeforms now.{" "}
          <button onClick={() => onAtlas()}>Open Atlas</button>
        </p>
      )}
      {loaded?.stale && (
        <aside className="bio-warning">
          Terrain or biological records changed. Saved ranges are retained and
          need review; generation will not silently move confirmed populations.
        </aside>
      )}
      {tab === "Overview" && (
        <>
          <div className="bio-metrics">
            {Object.entries(stats).map(([k, v]) => (
              <div key={k}>
                <strong>{v}</strong>
                <span>{label(k)}</span>
              </div>
            ))}
          </div>
          <p>
            {saved
              ? "A saved ecological foundation for this terrain."
              : "Habitats are derived from the complete saved terrain. Generate and accept a community to save the first biosphere."}
          </p>
          <div className="bio-habitat-bars">
            {[...new Set(state?.habitats.map((h) => h.kind))].map((kind) => {
              const hs = state.habitats.filter((h) => h.kind === kind);
              return (
                <button
                  key={kind}
                  onClick={() => {
                    setHabitatPage(0);
                    setHabitatSearch(kind.replaceAll("-", " "));
                    setTab("Habitats");
                  }}
                >
                  <span>{label(kind)}</span>
                  <meter value={hs.length} max={stats.habitats || 1} />
                  <small>{hs.length} regions</small>
                </button>
              );
            })}
          </div>
          {state?.diagnostics?.length > 0 && (
            <details>
              <summary>Ecological diagnostics</summary>
              {state.diagnostics.map((x, i) => (
                <p key={i}>{x}</p>
              ))}
            </details>
          )}
        </>
      )}
      {tab === "Habitats" && (
        <div className="bio-habitats">
          <aside>
            <input
              type="text"
              aria-label="Find habitats"
              placeholder="Find habitat"
              value={habitatSearch}
              onChange={(e) => {
                setHabitatPage(0);
                setHabitatSearch(e.target.value);
              }}
            />
            <div className="bio-region-list">
              {visibleHabitats.map((h) => (
                <button
                  key={h.id}
                  aria-current={habitat === h.id ? "true" : undefined}
                  onClick={() => setHabitat(h.id)}
                >
                  {h.name}
                  <small>
                    {h.cells.length} cells ·{" "}
                    {(byHabitat.get(h.id) || []).length} occurrences
                  </small>
                </button>
              ))}
            </div>
            <div className="history-actions">
              <button
                disabled={!habitatPage}
                onClick={() => setHabitatPage((p) => p - 1)}
              >
                Previous
              </button>
              <button
                disabled={(habitatPage + 1) * 80 >= habitatMatches.length}
                onClick={() => setHabitatPage((p) => p + 1)}
              >
                Next
              </button>
            </div>
          </aside>
          <section>
            {h ? (
              <>
                <span className="eyebrow">
                  {h.kind} / {h.stratum}
                </span>
                <h2>{h.name}</h2>
                <p>{h.description}</p>
                {h.baseBiome && <p>Base cover: {label(h.baseBiome)}{h.overlays?.length ? ` · ${h.overlays.map(label).join(", ")}` : ""}</p>}
                <p>
                  Climate:{" "}
                  {h.conditions.climateZones.join(", ") ||
                    "Approximate terrain climate"}{" "}
                  · Seasons:{" "}
                  {h.conditions.seasons.join(", ") || "Annual average"}
                </p>
                <p>
                  Substrate: {h.conditions.substrates.join(", ")} · Light:{" "}
                  {Math.round(h.conditions.light * 100)}% · Salinity:{" "}
                  {h.conditions.salinity.min.toFixed(1)}–
                  {h.conditions.salinity.max.toFixed(1)}‰
                </p>
                {h.conditions.medium === "terrestrial" && h.conditions.soilPH != null && <p>
                  Soil pH: {h.conditions.soilPH.toFixed(1)} · Soil depth: {(h.conditions.soilDepth || 0).toFixed(2)} m · Drainage: {Math.round((h.conditions.drainage || 0) * 100)}%
                  {h.conditions.seasonalKnown && <> · Drought: {Math.round(h.conditions.droughtMonths || 0)} months · Growing season: {Math.round(h.conditions.growingMonths || 0)} months</>}
                </p>}
                {["photic", "twilight", "aphotic", "benthic"].includes(h.stratum) && h.conditions.bathymetry && <p>
                  Organism depth: {h.conditions.depth.min.toFixed(0)}–{h.conditions.depth.max.toFixed(0)} m · Seabed depth: {h.conditions.bathymetry.min.toFixed(0)}–{h.conditions.bathymetry.max.toFixed(0)} m
                </p>}
                <button
                  disabled={!saved}
                  onClick={() => onAtlas(terrainID, h.id)}
                >
                  Highlight habitat in Atlas
                </button>
                <button
                  disabled={!loaded || busy}
                  onClick={() => {
                    setSettings({
                      ...settings,
                      scope: "selected",
                      habitatIds: [h.id],
                    });
                    setWizard(1);
                  }}
                >
                  Generate this habitat
                </button>
                <h3>Community</h3>
                {rangeRows.map((o) => (
                  <div className="bio-community-row" key={o.id}>
                    <button onClick={() => onProfile(species.get(o.speciesId))}>
                      {species.get(o.speciesId)?.name}
                    </button>
                    <span>
                      {species.get(o.speciesId)?.biology.roles.join(", ")} ·{" "}
                      {o.status}
                    </span>
                    <small>
                      {Math.round(o.abundance * 100)}% abundance potential ·{" "}
                      {o.confirmed ? "Confirmed" : ""}{" "}
                      {o.locked ? "Locked" : ""}
                    </small>
                  </div>
                ))}
                {!rangeRows.length && (
                  <p>No accepted lifeforms in this habitat.</p>
                )}
                <h3>Active rules</h3>
                {(settings?.rules || [])
                  .filter(
                    (r) =>
                      r.enabled &&
                      (!r.habitats?.length || r.habitats.includes(h.kind)),
                  )
                  .map((r) => (
                    <p key={r.id}>
                      {r.name} · {r.strict ? "requirement" : "preference"}
                    </p>
                  ))}
              </>
            ) : (
              <p>Select a habitat to inspect its conditions and community.</p>
            )}
          </section>
        </div>
      )}
      {tab === "Species Catalogue" && (
        <>
          <div className="collection-toolbar">
            <input
              type="text"
              placeholder="Search lifeforms"
              value={filters.search || ""}
              onChange={(e) => setFilter("search", e.target.value)}
            />
            {[
              ["group", groups],
              [
                "origin",
                [
                  "earth-reference",
                  "fictional",
                  "custom-created",
                  "placeholder",
                ],
              ],
              ["role", roles],
              ["status", statuses],
              ["completeness", ["placeholder", "draft", "complete"]],
            ].map(([k, opts]) => (
              <Select
                key={k}
                title={label(k)}
                value={filters[k] || ""}
                onChange={(v) => setFilter(k, v)}
                options={opts}
                any
              />
            ))}
            <Select
              title="Habitat"
              value={filters.habitat || ""}
              onChange={(v) => setFilter("habitat", v)}
              options={state?.habitats || []}
              any
            />
          </div>
          <div
            className="bio-catalogue-scroll"
            tabIndex={0}
            role="region"
            aria-label="Species catalogue"
          >
            <table className="bio-catalogue">
              <thead>
                <tr>
                  <th>Lifeform</th>
                  <th>Group & origin</th>
                  <th>Ecological roles</th>
                  <th>Profile</th>
                </tr>
              </thead>
              <tbody>
                {rows
                  .slice(cataloguePage * 80, (cataloguePage + 1) * 80)
                  .map((e) => (
                    <tr key={e.id}>
                      <td>
                        <button onClick={() => onProfile(e)}>{e.name}</button>
                        <small>{e.properties?.summary}</small>
                      </td>
                      <td>
                        {e.biology.group}
                        <small>{e.biology.origin}</small>
                      </td>
                      <td>{e.biology.roles.join(", ")}</td>
                      <td>
                        {e.biology.completeness}
                        {e.biology.locked ? " · protected" : ""}
                      </td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </div>
          <div className="history-actions">
            <button
              disabled={!cataloguePage}
              onClick={() => setCataloguePage((p) => p - 1)}
            >
              Previous species
            </button>
            <span>{rows.length} lifeforms</span>
            <button
              disabled={(cataloguePage + 1) * 80 >= rows.length}
              onClick={() => setCataloguePage((p) => p + 1)}
            >
              Next species
            </button>
          </div>
          {!rows.length && <p>No matching lifeforms in this Age.</p>}
        </>
      )}
      {tab === "Generation Rules" && settings && (
        <section>
          <p>
            Rules affect traits, environmental requirements and ecological
            dependencies. Confirmed records remain protected.
          </p>
          <RulesEditor
            value={settings.rules}
            onChange={(v) => edit("rules", v)}
            habitats={state?.habitats}
            species={allSpecies}
          />
          <div className="history-actions">
            <button disabled={!saved || busy} onClick={saveRules}>
              Save rules
            </button>
            <button
              disabled={!loaded || busy}
              onClick={() => {
                setWizard(2);
                setConfirmed(false);
              }}
            >
              Review & generate affected habitats
            </button>
          </div>
          <details>
            <summary>Reusable reference packs</summary>
            <p>
              Import a JSON array of lifeform templates with id, name, summary
              and biology. All values are validated by the same species schema.
              Packs remain inside the World archive.
            </p>
            <input
              type="file"
              accept="application/json,.json"
              onChange={async (e) => {
                const file = e.target.files[0];
                if (!file) return;
                await work(async () => {
                  if (file.size > 1500000)
                    throw Error("Reference packs must be smaller than 1.5 MB");
                  const data = JSON.parse(await file.text());
                  if (!Array.isArray(data) || data.length > 150)
                    throw Error(
                      "Expected an array of at most 150 species templates",
                    );
                  setSettings((s) => ({ ...s, catalogue: data }));
                  setNotice(
                    "Imported templates are validated when saved or generated.",
                  );
                });
              }}
            />
            <button
              onClick={() => {
                const blob = new Blob(
                    [
                      JSON.stringify(
                        allSpecies.map((e) => ({
                          id: e.id,
                          name: e.name,
                          summary: e.properties?.summary || "",
                          biology: e.biology,
                        })),
                        null,
                        2,
                      ),
                    ],
                    { type: "application/json" },
                  ),
                  url = URL.createObjectURL(blob),
                  a = document.createElement("a");
                a.href = url;
                a.download = "lifeform-templates.json";
                a.click();
                URL.revokeObjectURL(url);
              }}
            >
              Export lifeform templates
            </button>
            <p>{settings.catalogue?.length || 0} imported templates</p>
          </details>
        </section>
      )}
      {wizard !== null && settings && (
        <Dialog
          title={
            <>{wizard === 3 ? "Community preview" : "Generate Biosphere"}</>
          }
          size="wizard"
          className="bio-wizard"
          onClose={() => {
            setSettings(saved?.settings || loaded?.environment.settings);
            setWizard(null);
            setPreview(null);
          }}
          busy={busy}
          actions={
            <>
              {wizard < 3 && (
                <button data-dialog-dismiss disabled={busy}>
                  Cancel
                </button>
              )}
              {wizard > 0 && wizard < 3 && (
                <button disabled={busy} onClick={() => setWizard((w) => w - 1)}>
                  Back
                </button>
              )}
              {wizard < 2 && (
                <button
                  disabled={
                    busy ||
                    (wizard === 0 &&
                      settings.scope === "selected" &&
                      !settings.habitatIds?.length)
                  }
                  onClick={() => setWizard((w) => w + 1)}
                >
                  Next
                </button>
              )}
              {wizard === 0 && settings.mode === "quick" && (
                <button disabled={busy} onClick={() => generate()}>
                  Generate quick preview
                </button>
              )}
              {wizard === 2 && (
                <button
                  disabled={busy || !confirmed}
                  onClick={() => generate()}
                >
                  Generate preview
                </button>
              )}
              {wizard === 3 && (
                <>
                  <button
                    disabled={busy}
                    onClick={() => {
                      setWizard(1);
                      setPreview(null);
                    }}
                  >
                    Adjust generation
                  </button>
                  <button disabled={busy} className="primary" onClick={accept}>
                    Accept {accepted.length} selected ranges
                  </button>
                  <button
                    disabled={busy}
                    onClick={() => {
                      setWizard(null);
                      setPreview(null);
                    }}
                    data-dialog-dismiss
                  >
                    Discard preview
                  </button>
                </>
              )}
            </>
          }
        >
          {error && <p role="alert">{error}</p>}
          <ol className="ui-steps" aria-label="Generation progress">
            {["Terrain & scope", "Community", "Rules", "Preview"].map(
              (step, i) => (
                <li key={step} aria-current={wizard === i ? "step" : undefined}>
                  {step}
                </li>
              ),
            )}
          </ol>
          {wizard === 0 && (
            <>
              <p>Terrain: {terrains.find((t) => t.id === terrainID)?.name}</p>
              <Select
                title="Workflow"
                value={settings.mode}
                onChange={(v) => edit("mode", v)}
                options={["quick", "guided", "advanced"]}
              />
              <Select
                title="Preset"
                value={settings.preset}
                onChange={(v) => {
                  edit("preset", v);
                  setSettings((s) => ({
                    ...s,
                    preset: v,
                    richness:
                      v === "cold-sparse" || v === "dry-desert"
                        ? "sparse"
                        : v === "tropical-biodiversity"
                          ? "diverse"
                          : "moderate",
                    familiarity:
                      v === "highly-unusual"
                        ? 0.15
                        : v === "earth-like"
                          ? 0.8
                          : 0.5,
                  }));
                }}
                options={[
                  "earth-like",
                  "tropical-biodiversity",
                  "cold-sparse",
                  "dry-desert",
                  "highly-unusual",
                ]}
              />
              <Select
                title="Generation scope"
                value={settings.scope}
                onChange={(v) => edit("scope", v)}
                options={["entire", "selected", "missing"]}
              />
              {settings.scope === "selected" && (
                <div className="bio-scope-list">
                  {(loaded?.environment.habitats || []).map((h) => (
                    <label key={h.id}>
                      <input
                        type="checkbox"
                        checked={settings.habitatIds?.includes(h.id) || false}
                        onChange={(e) =>
                          edit(
                            "habitatIds",
                            e.target.checked
                              ? [...(settings.habitatIds || []), h.id]
                              : (settings.habitatIds || []).filter(
                                  (id) => id !== h.id,
                                ),
                          )
                        }
                      />
                      {h.name}
                    </label>
                  ))}
                </div>
              )}
              <p>
                Quick mode uses expandable ecological placeholders. Advanced
                mode also accepts your reusable catalogue templates and rules.
              </p>
            </>
          )}
          {wizard === 1 && (
            <>
              <label>
                Familiarity · {Math.round(settings.familiarity * 100)}%
                Earth-reference preference
                <input
                  type="range"
                  min="0"
                  max="1"
                  step=".05"
                  value={settings.familiarity}
                  onChange={(e) => edit("familiarity", Number(e.target.value))}
                />
                <small>
                  Fictional ← balanced → familiar. This is a selection
                  preference, not a worldwide quota.
                </small>
              </label>
              <Select
                title="Ecological richness"
                value={settings.richness}
                onChange={(v) => edit("richness", v)}
                options={["sparse", "moderate", "diverse"]}
              />
              <Select
                title="Ecological coherence"
                value={settings.coherence}
                onChange={(v) => edit("coherence", v)}
                options={["realistic", "flexible", "speculative"]}
              />
              <label>
                Distinctive biological ideas
                <textarea
                  rows="5"
                  value={settings.ideas || ""}
                  onChange={(e) => edit("ideas", e.target.value)}
                  placeholder="Luminous fungi in geothermal valleys; herbivores that feed on them…"
                />
              </label>
              <button
                onClick={() => {
                  const v = interpretIdeas(settings.ideas || "");
                  edit("rules", [...settings.rules, ...v.rules]);
                  setNotice(v.notice);
                  setWizard(2);
                }}
              >
                Interpret supported ideas & review rules
              </button>
              <p>
                No AI service is required. The structured editor below is
                authoritative.
              </p>
              <label>
                Generation seed
                <input
                  type="text"
                  value={settings.seed}
                  onChange={(e) => edit("seed", e.target.value)}
                />
              </label>
            </>
          )}
          {wizard === 2 && (
            <>
              <p>
                {settings.mode} · {settings.richness} · {settings.coherence} ·{" "}
                {settings.scope} terrain scope
              </p>
              {notice && <p>{notice}</p>}
              <RulesEditor
                value={settings.rules}
                onChange={(v) => edit("rules", v)}
                habitats={loaded?.environment.habitats}
                species={allSpecies}
              />
              <label>
                <input
                  type="checkbox"
                  checked={confirmed}
                  onChange={(e) => setConfirmed(e.target.checked)}
                />{" "}
                I reviewed the structured rules. Only these rules will affect
                generation.
              </label>
              <p>
                Hard survival limits and host dependencies remain enforced.
                Deliberate exceptions must be defined in species tolerances or
                authored occurrence notes.
              </p>
            </>
          )}
          {wizard === 3 && preview && (
            <Preview
              preview={preview}
              accepted={accepted}
              setAccepted={setAccepted}
              locked={locked}
              setLocked={setLocked}
              names={names}
              setNames={setNames}
              onRegenerate={(h, s) => {
                const targetRoles = s
                  ? preview.species.find((x) => x.id === s)?.biology.roles || []
                  : roles;
                const variants = { ...settings.variants };
                let changed = false;
                for (const role of targetRoles) {
                  const protectedRole = preview.state.occurrences.some(
                    (o) =>
                      o.habitatId === h &&
                      locked.includes(o.id) &&
                      preview.species
                        .find((x) => x.id === o.speciesId)
                        ?.biology.roles.includes(role),
                  );
                  if (!protectedRole) {
                    const key = h + "/" + role;
                    variants[key] = (variants[key] || 0) + 1;
                    changed = true;
                  }
                }
                if (changed) generate({ ...settings, variants }, h, s);
                else
                  setError(
                    "These ecological entries are locked. Unlock them before regenerating.",
                  );
              }}
            />
          )}
        </Dialog>
      )}
      {create && (
        <Dialog
          title={<>Create lifeform</>}
          size="form"
          className=""
          onClose={() => setCreate(false)}
          busy={busy}
          onSubmit={async (e) => {
            e.preventDefault();
            const id = crypto.randomUUID(),
              biology = blankLifeform();
            biology.group = newGroup;
            biology.roles = [
              {
                animal: "herbivore",
                fungus: "decomposer",
                bacterium: "decomposer",
                protist: "omnivore",
                virus: "host-dependent",
              }[newGroup] || "producer",
            ];
            if (["animal", "fungus", "bacterium", "protist"].includes(newGroup))
              biology.dependencies = [
                {
                  role: "producer",
                  kind:
                    newGroup === "fungus" || newGroup === "bacterium"
                      ? "decomposes"
                      : "feeds-on",
                },
              ];
            if (newGroup === "virus") {
              biology.roles = ["host-dependent"];
              biology.dependencies = [{ group: "bacterium", kind: "infects" }];
              biology.traits.transmission =
                "Specify compatible host contact mechanism";
            }
            const d = await command({
              kind: "entity",
              ageId: age.id,
              entity: {
                id,
                kind: "lifeform",
                name: newName,
                properties: {},
                biology,
              },
            });
            if (d) {
              setCreate(false);
              onProfile(
                d.entities.find((e) => e.id === id && e.ageId === age.id),
              );
            }
          }}
          actions={
            <>
              <button
                type="button"
                onClick={() => setCreate(false)}
                data-dialog-dismiss
              >
                Cancel
              </button>
              <button className="primary">Create profile</button>
            </>
          }
        >
          <label>
            Name
            <input
              type="text"
              required
              maxLength={200}
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
            />
          </label>
          <Select
            title="Browsing group"
            value={newGroup}
            onChange={setNewGroup}
            options={groups}
          />
          <p>
            Identity is independent of any terrain. Define its range after its
            ecological requirements.
          </p>
        </Dialog>
      )}
    </main>
  );
}
