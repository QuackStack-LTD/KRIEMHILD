import { useState } from "react";
import { continuity } from "./model.js";
export default function WriterContext({
  node,
  doc,
  project,
  onChange,
  editor,
  onClose,
}) {
  const [tab, setTab] = useState("scene"),
    [search, setSearch] = useState(""),
    [warnings, setWarnings] = useState(null);
  const age =
    node.storyAgeId || project.settings.defaultAge || doc.world.currentAge;
  const entities = doc.entities.filter((e) => e.ageId === age),
    refs = node.references || [];
  const link = (e) =>
    `/worlds/${encodeURIComponent(doc.world.id)}/profile?entity=${encodeURIComponent(e.id)}&age=${encodeURIComponent(e.ageId)}`;
  function attach(e, inline = false) {
    const r = {
      worldId: doc.world.id,
      entityId: e.id,
      ageId: e.ageId,
      type: inline ? "inline" : "metadata",
      wording: e.name,
    };
    if (inline && editor)
      editor
        .chain()
        .focus()
        .insertContent({
          type: "text",
          text: e.name,
          marks: [
            {
              type: "worldReference",
              attrs: {
                worldId: r.worldId,
                entityId: r.entityId,
                ageId: r.ageId,
              },
            },
          ],
        })
        .run();
    onChange({
      storyAgeId: node.storyAgeId || age,
      references: [
        ...refs.filter(
          (x) =>
            !(
              x.entityId === r.entityId &&
              x.ageId === r.ageId &&
              x.type === r.type
            ),
        ),
        r,
      ],
    });
  }
  return (
    <aside className="writer-context" aria-label="Context and notes">
      <div className="writer-panel-heading">
        <strong>Context & notes</strong>
        <button onClick={onClose}>Close</button>
      </div>
      <div className="writer-tabs">
        {["world", "scene", "notes"].map((t) => (
          <button key={t} aria-pressed={tab === t} onClick={() => setTab(t)}>
            {t}
          </button>
        ))}
      </div>
      {tab === "scene" && (
        <>
          <label>
            Story Age
            <select
              value={node.storyAgeId || ""}
              onChange={(e) => {
                onChange({ storyAgeId: e.target.value });
                setWarnings(null);
              }}
            >
              <option value="">Choose Age</option>
              {doc.ages.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Story year
            <input
              type="number"
              value={node.storyYear ?? ""}
              onChange={(e) =>
                onChange({
                  storyYear:
                    e.target.value === "" ? null : Number(e.target.value),
                })
              }
            />
          </label>
          <p className="writer-muted">
            Story time is independent of manuscript order and the World’s
            selected Age.
          </p>
          {[
            ["povEntityId", "Point of view", (e) => e.kind === "character"],
            [
              "locationEntityId",
              "Location",
              (e) =>
                [
                  "continent",
                  "island",
                  "country",
                  "state",
                  "empire",
                  "city",
                  "village",
                  "district",
                  "building",
                  "region",
                  "forest",
                  "mountain",
                  "river",
                  "lake",
                ].includes(e.kind),
            ],
          ].map(([key, label, filter]) => (
            <label key={key}>
              {label}
              <select
                value={node[key] || ""}
                onChange={(e) => onChange({ [key]: e.target.value })}
              >
                <option value="">Unspecified</option>
                {entities.filter(filter).map((e) => (
                  <option key={e.id} value={e.id}>
                    {e.name}
                  </option>
                ))}
                {node[key] && !entities.some((e) => e.id === node[key]) && (
                  <option value={node[key]}>Missing in this Age</option>
                )}
              </select>
            </label>
          ))}
          {[
            ["purpose", "Purpose"],
            ["conflict", "Conflict"],
            ["outcome", "Outcome"],
            ["tags", "Plotlines / tags"],
          ].map(([key, title]) => (
            <label key={key}>
              {title}
              <textarea
                value={node[key] || ""}
                onChange={(e) => onChange({ [key]: e.target.value })}
              />
            </label>
          ))}
          <button
            onClick={() => setWarnings(continuity(node, doc, doc.world.id))}
          >
            Check scene continuity
          </button>
          {warnings && (
            <div role="status">
              {warnings.length ? (
                warnings.map((w) => (
                  <p key={w.id}>
                    <a
                      href={link({ id: w.id, ageId: w.ageId })}
                      target="_blank"
                      rel="noreferrer"
                    >
                      {w.name}
                    </a>
                    : {w.reason}
                  </p>
                ))
              ) : (
                <p>
                  No warnings in the linked records. This is an advisory check,
                  not a plot review.
                </p>
              )}
            </div>
          )}
        </>
      )}
      {tab === "world" && (
        <>
          <label>
            Find world references
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Characters, places, institutions…"
            />
          </label>
          <p className="writer-muted">
            Showing {doc.ages.find((a) => a.id === age)?.name}. Choose a
            different story Age in Scene.
          </p>
          {entities
            .filter((e) =>
              (e.name + " " + e.kind)
                .toLocaleLowerCase()
                .includes(search.toLocaleLowerCase()),
            )
            .slice(0, 40)
            .map((e) => (
              <div className="writer-world-entry" key={e.id}>
                <strong>{e.name}</strong>
                <small>{e.kind}</small>
                <p>
                  {String(
                    e.properties?.summary || e.properties?.description || "",
                  ).slice(0, 220)}
                </p>
                <div>
                  <button disabled={!project.worldId} onClick={() => attach(e)}>
                    Pin
                  </button>
                  <button
                    onClick={() => attach(e, true)}
                    disabled={!editor || !project.worldId}
                  >
                    Insert reference
                  </button>
                  <a href={link(e)} target="_blank" rel="noreferrer">
                    Profile ↗
                  </a>
                </div>
                <small>
                  {doc.relations
                    .filter(
                      (r) =>
                        r.ageId === age &&
                        (r.source === e.id || r.destination === e.id),
                    )
                    .slice(0, 3)
                    .map((r) => r.kind)
                    .join(" · ")}
                </small>
              </div>
            ))}
        </>
      )}
      {tab === "notes" && (
        <>
          <label>
            Private scene notes
            <textarea
              rows={14}
              value={node.notes || ""}
              onChange={(e) => onChange({ notes: e.target.value })}
            />
          </label>
          <p className="writer-muted">
            Notes, research and outline nodes in the navigator stay outside
            manuscript exports unless explicitly included.
          </p>
          <a
            href={`/worlds/${doc.world.id}/library`}
            target="_blank"
            rel="noreferrer"
          >
            Consult the in-world Library ↗
          </a>
        </>
      )}
      {!project.worldId && (
        <p>
          Associate this project with the World from More before attaching
          references.
        </p>
      )}
      <section className="writer-pins">
        <h3>Pinned references</h3>
        {refs.length === 0 ? (
          <p className="writer-muted">
            Pin a reference from World to keep it nearby.
          </p>
        ) : (
          refs.map((r, i) => {
            const e = doc.entities.find(
              (e) => e.id === r.entityId && e.ageId === r.ageId,
            );
            return (
              <div key={i}>
                {e ? (
                  <a href={link(e)} target="_blank" rel="noreferrer">
                    {e.name}
                  </a>
                ) : (
                  <strong>Missing: {r.wording || r.entityId}</strong>
                )}
                <small>
                  {doc.ages.find((a) => a.id === r.ageId)?.name ||
                    "Missing Age"}{" "}
                  · {r.type}
                </small>
                {r.type !== "inline" && (
                  <button
                    aria-label={`Unpin ${e?.name || r.wording}`}
                    onClick={() =>
                      onChange({ references: refs.filter((_, j) => j !== i) })
                    }
                  >
                    Unpin
                  </button>
                )}
              </div>
            );
          })
        )}
      </section>
    </aside>
  );
}
