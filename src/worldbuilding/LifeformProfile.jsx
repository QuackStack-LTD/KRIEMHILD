import useUnsavedChanges from "./ui/useUnsavedChanges.js";
import Dialog from "./ui/Dialog.jsx";
import { useState } from "react";
import { activeEntities, identity, label } from "./model.js";
import { groups, roles, statuses, interactions } from "./biosphere.js";
import {
  Tolerances,
  TraitsEditor,
  DependenciesEditor,
  Choices,
  CSVInput,
} from "./BiosphereEditors.jsx";
import "./biosphere.css";
export default function LifeformProfile({
  doc,
  entity,
  command,
  onProfile,
  onBack,
  onBiosphere,
}) {
  const [draft, setDraft] = useState(structuredClone(entity)),
    [tab, setTab] = useState("Identity"),
    [message, setMessage] = useState(""),
    [occurrence, setOccurrence] = useState(null),
    [connection, setConnection] = useState(null);
  useUnsavedChanges(JSON.stringify(draft) !== JSON.stringify(entity), () =>
    setDraft(structuredClone(entity)),
  );
  const b = draft.biology,
    p = draft.properties || {},
    age = entity.ageId,
    all = activeEntities(doc, age).filter((e) => e.biology),
    states = (doc.biospheres || []).filter((s) => s.ageId === age),
    ranges = states.flatMap((s) =>
      s.occurrences
        .filter((o) => o.speciesId === entity.id)
        .map((o) => ({ s, o })),
    ),
    links = states.flatMap((s) =>
      s.interactions
        .filter((r) => r.source === entity.id || r.destination === entity.id)
        .map((r) => ({ s, r })),
    );
  const bio = (k, v) =>
      setDraft((d) => ({ ...d, biology: { ...d.biology, [k]: v } })),
    prop = (k, v) =>
      setDraft((d) => ({ ...d, properties: { ...d.properties, [k]: v } }));
  async function save(e) {
    e.preventDefault();
    const d = await command({
      kind: "entity",
      ageId: age,
      entity: { ...draft, biology: { ...b, locked: true } },
    });
    if (d)
      setDraft(d.entities.find((x) => x.id === entity.id && x.ageId === age));
    setMessage(
      d
        ? "Saved in this Age. Existing ranges are retained for ecological review."
        : "Could not save. Check the page error.",
    );
  }
  return (
    <main className="world-page lifeform-profile">
      <div className="profile-breadcrumb">
        <button onClick={onBack}>Back to Biosphere</button>
        <span>
          {doc.world.name} /{" "}
          <strong>{doc.ages.find((a) => a.id === age)?.name}</strong>
        </span>
      </div>
      <header className="lifeform-heading">
        {p.portrait ? (
          <img src={p.portrait} alt="" />
        ) : (
          <span className="lifeform-symbol" aria-hidden="true">
            ❧
          </span>
        )}
        <div>
          <span className="eyebrow">
            {entity.biology.group} · {entity.biology.origin}
          </span>
          <h1>{entity.name}</h1>
          <p>{entity.properties?.summary}</p>
          <small>
            {entity.biology.completeness} ·{" "}
            {entity.biology.locked
              ? "Protected authored record"
              : "Generated ecological record"}
          </small>
        </div>
      </header>
      <nav className="institution-tabs">
        {[
          "Identity",
          "Habitat & Range",
          "Traits & Lifecycle",
          "Ecology",
          "Variants & History",
        ].map((t) => (
          <button
            key={t}
            aria-current={tab === t ? "page" : undefined}
            onClick={() => setTab(t)}
          >
            {t}
          </button>
        ))}
      </nav>
      <form onSubmit={save}>
        {tab === "Identity" && (
          <section className="bio-profile-form">
            <label>
              Name
              <input
                type="text"
                required
                maxLength={200}
                value={draft.name}
                onChange={(e) => setDraft({ ...draft, name: e.target.value })}
              />
            </label>
            <label>
              Scientific name (optional)
              <input
                type="text"
                value={b.scientificName || ""}
                onChange={(e) => bio("scientificName", e.target.value)}
              />
            </label>
            <label>
              Lifeform group
              <select
                value={b.group}
                onChange={(e) => bio("group", e.target.value)}
              >
                {groups.map((g) => (
                  <option key={g}>{g}</option>
                ))}
              </select>
            </label>
            {b.group === "custom" && (
              <label>
                Custom group
                <input
                  type="text"
                  value={b.customGroup || ""}
                  onChange={(e) => bio("customGroup", e.target.value)}
                />
              </label>
            )}
            <label>
              Origin
              <select
                value={b.origin}
                onChange={(e) => bio("origin", e.target.value)}
              >
                {[
                  "earth-reference",
                  "fictional",
                  "custom-created",
                  "placeholder",
                ].map((g) => (
                  <option key={g}>{g}</option>
                ))}
              </select>
            </label>
            <label>
              Profile state
              <select
                value={b.completeness}
                onChange={(e) => bio("completeness", e.target.value)}
              >
                {["placeholder", "draft", "complete"].map((g) => (
                  <option key={g}>{g}</option>
                ))}
              </select>
            </label>
            <label>
              Short summary
              <textarea
                value={p.summary || ""}
                onChange={(e) => prop("summary", e.target.value)}
              />
            </label>
            <label>
              Description
              <textarea
                rows="6"
                value={p.description || ""}
                onChange={(e) => prop("description", e.target.value)}
              />
            </label>
            <label>
              Illustration
              <input
                type="file"
                accept="image/png,image/jpeg,image/webp"
                onChange={(e) => {
                  const f = e.target.files[0];
                  if (!f) return;
                  if (f.size > 1500000) {
                    setMessage("Choose an image smaller than 1.5 MB.");
                    return;
                  }
                  const reader = new FileReader();
                  reader.onload = () => prop("portrait", reader.result);
                  reader.readAsDataURL(f);
                }}
              />
            </label>
            <div className="field-pair">
              {["startYear", "endYear"].map((k) => (
                <label key={k}>
                  {k === "startYear"
                    ? "First recorded year"
                    : "Last recorded year"}
                  <input
                    type="text"
                    inputMode="numeric"
                    value={p[k] ?? ""}
                    onChange={(e) => prop(k, e.target.value)}
                  />
                </label>
              ))}
            </div>
          </section>
        )}
        {tab === "Habitat & Range" && (
          <section>
            <h2>Environmental tolerances</h2>
            <Tolerances
              value={b.requirements}
              onChange={(r) => bio("requirements", r)}
            />
            <label>
              Dispersal
              <select
                value={b.dispersal}
                onChange={(e) => bio("dispersal", e.target.value)}
              >
                {["local", "regional", "mobile", "waterborne"].map((k) => (
                  <option key={k}>{k}</option>
                ))}
              </select>
            </label>
            <h2>Actual distributions</h2>
            <p>
              Suitability describes possible survival. Only the saved ranges
              below represent actual occurrences.
            </p>
            {ranges.map(({ s, o }) => (
              <article className="bio-range" key={o.id}>
                <h3>{s.habitats.find((h) => h.id === o.habitatId)?.name}</h3>
                <p>
                  {
                    doc.terrains.find(
                      (t) => t.ageId === age && t.id === s.terrainId,
                    )?.name
                  }{" "}
                  · {o.status} · {Math.round(o.abundance * 100)}% potential
                </p>
                <p>{o.reason}</p>
                <small>
                  {o.seasons.join(", ")}{" "}
                  {o.exception && " · Author exception: " + o.exception}
                </small>
                <button
                  type="button"
                  onClick={() =>
                    setOccurrence({ state: s, value: structuredClone(o) })
                  }
                >
                  Edit range status / protection
                </button>
                <button
                  type="button"
                  onClick={() => onBiosphere(s.terrainId, o.habitatId)}
                >
                  Inspect habitat
                </button>
              </article>
            ))}
            <button
              type="button"
              disabled={!states.length}
              onClick={() =>
                setOccurrence({
                  state: states[0],
                  value: {
                    id: crypto.randomUUID(),
                    speciesId: entity.id,
                    habitatId: "",
                    terrainId: states[0].terrainId,
                    status: "native",
                    abundance: 0.5,
                    suitability: 0,
                    seasons: [],
                    reason: "Authored occurrence",
                    confirmed: true,
                    locked: true,
                  },
                })
              }
            >
              Add authored range
            </button>
            {!states.length && (
              <p>
                Generate habitats on a terrain before defining geographic
                occurrences.
              </p>
            )}
          </section>
        )}
        {tab === "Traits & Lifecycle" && (
          <section>
            <h2>Ecological roles</h2>
            <Choices
              values={roles}
              value={b.roles}
              onChange={(v) => bio("roles", v)}
            />
            <h2>Traits</h2>
            <TraitsEditor value={b.traits} onChange={(v) => bio("traits", v)} />
            <label>
              Lifecycle, physiology & reproduction
              <textarea
                rows="7"
                value={b.lifecycle || ""}
                onChange={(e) => bio("lifecycle", e.target.value)}
              />
            </label>
            <p>
              Add morphology, growth, dormancy, reproduction or custom
              biological mechanisms only where relevant.
            </p>
          </section>
        )}
        {tab === "Ecology" && (
          <section>
            <h2>Required ecological dependencies</h2>
            <DependenciesEditor
              species={all.filter((species) => species.id !== entity.id)}
              value={b.dependencies}
              onChange={(v) => bio("dependencies", v)}
            />
            {b.group === "virus" && (
              <p>
                Keep an “infects” host-group dependency and a transmission
                trait. Infection does not automatically imply disease.
              </p>
            )}
            <h2>Recorded interactions</h2>
            {links.map(({ s, r }) => (
              <article className="bio-range" key={r.id}>
                <button
                  type="button"
                  onClick={() =>
                    onProfile(
                      all.find(
                        (e) =>
                          e.id ===
                          (r.source === entity.id ? r.destination : r.source),
                      ),
                    )
                  }
                >
                  {
                    all.find(
                      (e) =>
                        e.id ===
                        (r.source === entity.id ? r.destination : r.source),
                    )?.name
                  }
                </button>
                <span>
                  {" "}
                  · {label(r.kind)} · {r.authored ? "authored" : "generated"}
                </span>
                <p>
                  {s.habitats.find((h) => h.id === r.habitatId)?.name} ·{" "}
                  {r.description}
                </p>
                <button
                  type="button"
                  onClick={() =>
                    setConnection({ state: s, value: structuredClone(r) })
                  }
                >
                  Edit interaction
                </button>
              </article>
            ))}
            <button
              type="button"
              disabled={!states.length}
              onClick={() =>
                setConnection({
                  state: states[0],
                  value: {
                    id: crypto.randomUUID(),
                    source: entity.id,
                    destination: "",
                    kind: "symbiotic-with",
                    habitatId: "",
                    authored: true,
                  },
                })
              }
            >
              Add ecological connection
            </button>
          </section>
        )}
        {tab === "Variants & History" && (
          <section>
            <label>
              Taxonomy / custom classification
              <textarea
                value={b.taxonomy || ""}
                onChange={(e) => bio("taxonomy", e.target.value)}
              />
            </label>
            <label>
              Parent taxon or evolutionary lineage
              <select
                value={b.parentId || ""}
                onChange={(e) => bio("parentId", e.target.value)}
              >
                <option value="">Independent / unknown</option>
                {all
                  .filter((e) => e.id !== entity.id)
                  .map((e) => (
                    <option key={e.id} value={e.id}>
                      {e.name}
                    </option>
                  ))}
              </select>
            </label>
            <p>
              Catalogue reference: {b.catalogueId || "Author-defined"} ·
              Generator: {b.generator || "Manual"} · Rules:{" "}
              {(b.ruleIds || []).join(", ") || "None"}
            </p>
            <h2>Historical manifestations</h2>
            {doc.entities
              .filter(
                (e) => e.ageId !== age && identity(e) === identity(entity),
              )
              .map((e) => (
                <p key={e.ageId}>
                  <button type="button" onClick={() => onProfile(e)}>
                    {doc.ages.find((a) => a.id === e.ageId)?.name}: {e.name}
                  </button>
                </p>
              ))}
            <h2>Related taxa</h2>
            {all
              .filter((e) => e.biology.parentId === entity.id)
              .map((e) => (
                <p key={e.id}>
                  <button type="button" onClick={() => onProfile(e)}>
                    {e.name}
                  </button>
                </p>
              ))}
            <p>
              Use locally extinct, introduced, migratory or cultivated range
              statuses to record changes independently in each Age.
            </p>
          </section>
        )}
        <div className="history-actions">
          <button className="primary">Save lifeform</button>
          <output role="status">{message}</output>
        </div>
      </form>
      {occurrence && (
        <Dialog
          title={<>Geographical occurrence</>}
          size="form"
          className=""
          onClose={() => setOccurrence(null)}
          onSubmit={async (e) => {
            e.preventDefault();
            const { state: s, value: o } = occurrence;
            const rows = s.occurrences.filter((x) => x.id !== o.id);
            const d = await command({
              kind: "biosphere",
              ageId: age,
              biosphereEdit: { terrainId: s.terrainId, occurrence: o },
            });
            if (d) setOccurrence(null);
          }}
          actions={
            <>
              <button
                type="button"
                onClick={() => setOccurrence(null)}
                data-dialog-dismiss
              >
                Cancel
              </button>
              <button className="primary">Save occurrence</button>
            </>
          }
        >
          <label>
            Terrain
            <select
              value={occurrence.state.id}
              onChange={(e) => {
                const s = states.find((s) => s.id === e.target.value);
                setOccurrence({
                  state: s,
                  value: {
                    ...occurrence.value,
                    id: crypto.randomUUID(),
                    terrainId: s.terrainId,
                    habitatId: "",
                  },
                });
              }}
            >
              {states.map((s) => (
                <option key={s.id} value={s.id}>
                  {
                    doc.terrains.find(
                      (t) => t.id === s.terrainId && t.ageId === age,
                    )?.name
                  }
                </option>
              ))}
            </select>
          </label>
          <label>
            Habitat
            <select
              required
              value={occurrence.value.habitatId}
              onChange={(e) =>
                setOccurrence((x) => ({
                  ...x,
                  value: { ...x.value, habitatId: e.target.value },
                }))
              }
            >
              <option value="">Choose habitat</option>
              {occurrence.state.habitats.map((h) => (
                <option key={h.id} value={h.id}>
                  {h.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Status
            <select
              value={occurrence.value.status}
              onChange={(e) =>
                setOccurrence((x) => ({
                  ...x,
                  value: { ...x.value, status: e.target.value },
                }))
              }
            >
              {statuses.map((s) => (
                <option key={s}>{s}</option>
              ))}
            </select>
          </label>
          <label>
            Abundance
            <input
              type="range"
              min="0"
              max="1"
              step=".05"
              value={occurrence.value.abundance}
              onChange={(e) =>
                setOccurrence((x) => ({
                  ...x,
                  value: { ...x.value, abundance: Number(e.target.value) },
                }))
              }
            />
          </label>
          <label>
            Seasonal appearance
            <CSVInput
              value={occurrence.value.seasons}
              onChange={(seasons) =>
                setOccurrence((x) => ({ ...x, value: { ...x.value, seasons } }))
              }
            />
          </label>
          <label>
            Authored exception / supporting mechanism
            <textarea
              value={occurrence.value.exception || ""}
              onChange={(e) =>
                setOccurrence((x) => ({
                  ...x,
                  value: { ...x.value, exception: e.target.value },
                }))
              }
            />
          </label>
          {["locked", "confirmed"].map((k) => (
            <label key={k}>
              <input
                type="checkbox"
                checked={occurrence.value[k]}
                onChange={(e) =>
                  setOccurrence((x) => ({
                    ...x,
                    value: { ...x.value, [k]: e.target.checked },
                  }))
                }
              />
              {label(k)} — protected from regeneration
            </label>
          ))}
        </Dialog>
      )}
      {connection && (
        <Dialog
          title={<>Ecological interaction</>}
          size="form"
          className=""
          onClose={() => setConnection(null)}
          onSubmit={async (e) => {
            e.preventDefault();
            const { state: s, value: r } = connection;
            const d = await command({
              kind: "biosphere",
              ageId: age,
              biosphereEdit: {
                terrainId: s.terrainId,
                interaction: { ...r, authored: true },
              },
            });
            if (d) setConnection(null);
          }}
          actions={
            <>
              <button
                type="button"
                onClick={() => setConnection(null)}
                data-dialog-dismiss
              >
                Cancel
              </button>
              <button className="primary">Save interaction</button>
            </>
          }
        >
          <label>
            Terrain
            <select
              value={connection.state.id}
              onChange={(e) => {
                const s = states.find((s) => s.id === e.target.value);
                setConnection({
                  state: s,
                  value: {
                    ...connection.value,
                    id: crypto.randomUUID(),
                    habitatId: "",
                  },
                });
              }}
            >
              {states.map((s) => (
                <option key={s.id} value={s.id}>
                  {
                    doc.terrains.find(
                      (t) => t.id === s.terrainId && t.ageId === age,
                    )?.name
                  }
                </option>
              ))}
            </select>
          </label>
          {["source", "destination"].map((k) => (
            <label key={k}>
              {label(k)}
              <select
                required
                value={connection.value[k]}
                onChange={(e) =>
                  setConnection((x) => ({
                    ...x,
                    value: { ...x.value, [k]: e.target.value },
                  }))
                }
              >
                <option value="">Choose lifeform</option>
                {all.map((e) => (
                  <option key={e.id} value={e.id}>
                    {e.name}
                  </option>
                ))}
              </select>
            </label>
          ))}
          <label>
            Type
            <select
              value={connection.value.kind}
              onChange={(e) =>
                setConnection((x) => ({
                  ...x,
                  value: { ...x.value, kind: e.target.value },
                }))
              }
            >
              {interactions.map((k) => (
                <option key={k}>{k}</option>
              ))}
            </select>
          </label>
          <label>
            Habitat
            <select
              required
              value={connection.value.habitatId}
              onChange={(e) =>
                setConnection((x) => ({
                  ...x,
                  value: { ...x.value, habitatId: e.target.value },
                }))
              }
            >
              <option value="">Choose context</option>
              {connection.state.habitats.map((h) => (
                <option key={h.id} value={h.id}>
                  {h.name}
                </option>
              ))}
            </select>
          </label>
          {["description", "season", "lifeStage"].map((k) => (
            <label key={k}>
              {label(k)}
              <input
                type="text"
                value={connection.value[k] || ""}
                onChange={(e) =>
                  setConnection((x) => ({
                    ...x,
                    value: { ...x.value, [k]: e.target.value },
                  }))
                }
              />
            </label>
          ))}
        </Dialog>
      )}
    </main>
  );
}
