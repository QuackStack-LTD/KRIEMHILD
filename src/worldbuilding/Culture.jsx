import Dialog from "./ui/Dialog.jsx";
import { useState } from "react";
import {
  activeEntities,
  entityIn,
  identity,
  label,
  registry,
} from "./model.js";
import { matches } from "./connections.js";
import {
  componentTypes,
  conceptTypes,
  suggestedSections,
  emptyContent,
  isConcept,
  culturalKinds,
  collections,
  conceptCollection,
  conceptSearch,
  conceptContexts,
  compareConcept,
} from "./concepts.js";
import {
  ClaimEditor,
  ClaimView,
  FieldsEditor,
  FieldsView,
  VariationsEditor,
  VariationsView,
  ComponentLinksEditor,
} from "./ConceptEditors.jsx";
import ConnectionEditor from "./ConnectionEditor.jsx";
import "./culture.css";
const uid = () => crypto.randomUUID();
function Dates({ value, onChange }) {
  return (
    <div className="field-pair">
      {["startYear", "endYear"].map((k) => (
        <label key={k}>
          {k === "startYear" ? "Start year" : "End year"}
          <input
            type="number"
            step="1"
            value={value[k] ?? ""}
            onChange={(e) =>
              onChange({
                ...value,
                [k]: e.target.value === "" ? null : Number(e.target.value),
              })
            }
          />
        </label>
      ))}
    </div>
  );
}
function CreateConcept({ doc, age, command, onClose, onProfile, parent }) {
  const [name, setName] = useState(""),
    [kind, setKind] = useState("religion"),
    [summary, setSummary] = useState(""),
    [custom, setCustom] = useState(""),
    [reuse, setReuse] = useState(""),
    [relation, setRelation] = useState("derives-from"),
    [busy, setBusy] = useState(false);
  const options = culturalKinds.filter(
    (k) =>
      matches({ kind }, registry.relations[k].sources) &&
      (!parent || matches(parent, registry.relations[k].destinations)),
  );
  return (
    <Dialog
      title={
        <>{parent ? "Create or link related concept" : "Create concept"}</>
      }
      size="form"
      className=""
      onClose={onClose}
      busy={busy}
      onSubmit={async (e) => {
        e.preventDefault();
        setBusy(true);
        try {
          const id = reuse || uid(),
            commands = [];
          if (!reuse)
            commands.push({
              kind: "entity",
              entity: {
                id,
                name,
                kind,
                properties: { summary },
                content: {
                  ...emptyContent(),
                  template: kind,
                  customType: custom,
                },
              },
            });
          if (parent)
            commands.push({
              kind: "relation",
              relation: {
                source: id,
                destination: parent.id,
                kind: options.includes(relation) ? relation : options[0],
              },
            });
          const d = await command({ kind: "batch", ageId: age, commands });
          if (d) {
            onClose();
            onProfile(entityIn(d, age, id));
          }
        } finally {
          setBusy(false);
        }
      }}
      actions={
        <>
          <button type="button" onClick={onClose} data-dialog-dismiss>
            Cancel
          </button>
          <button
            className="primary"
            disabled={busy || (parent && !options.length)}
          >
            Save concept
          </button>
        </>
      }
    >
      <label>
        Type / template
        <select
          value={kind}
          onChange={(e) => {
            setKind(e.target.value);
            setReuse("");
          }}
        >
          {conceptTypes.map((t) => (
            <option key={t} value={t}>
              {t === "concept" ? "Blank / custom concept" : label(t)}
            </option>
          ))}
        </select>
      </label>
      {parent && (
        <label>
          Reuse existing concept
          <select value={reuse} onChange={(e) => setReuse(e.target.value)}>
            <option value="">Create a new concept</option>
            {activeEntities(doc, age)
              .filter((e) => e.kind === kind && e.id !== parent.id)
              .map((e) => (
                <option key={e.id} value={e.id}>
                  {e.name}
                </option>
              ))}
          </select>
        </label>
      )}
      {!reuse && (
        <>
          <label>
            Name
            <input
              type="text"
              required
              maxLength={200}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          {kind === "concept" && (
            <label>
              Custom type
              <input
                type="text"
                maxLength={200}
                value={custom}
                onChange={(e) => setCustom(e.target.value)}
              />
            </label>
          )}
          <label>
            Short description
            <textarea
              value={summary}
              onChange={(e) => setSummary(e.target.value)}
            />
          </label>
        </>
      )}
      {parent && (
        <label>
          Relationship to {parent.name}
          <select
            value={options.includes(relation) ? relation : options[0]}
            onChange={(e) => setRelation(e.target.value)}
          >
            {options.map((k) => (
              <option key={k} value={k}>
                {label(k)}
              </option>
            ))}
          </select>
        </label>
      )}
    </Dialog>
  );
}
export default function Culture({ doc, age, command, onProfile }) {
  const [search, setSearch] = useState(""),
    [collection, setCollection] = useState("all"),
    [context, setContext] = useState(""),
    [related, setRelated] = useState(""),
    [status, setStatus] = useState(""),
    [sort, setSort] = useState("name"),
    [create, setCreate] = useState(false);
  const all = activeEntities(doc, age.id),
    concepts = all.filter(isConcept),
    relations = doc.relations.filter((r) => r.ageId === age.id);
  const rows = concepts
    .filter(
      (e) =>
        (collection === "all" || conceptCollection(e) === collection) &&
        conceptSearch(e).includes(search.toLowerCase()) &&
        (!status || e.properties?.status === status) &&
        (!context ||
          conceptContexts(e).includes(context) ||
          relations.some(
            (r) =>
              (r.source === e.id && r.destination === context) ||
              (r.destination === e.id && r.source === context),
          )) &&
        (!related ||
          relations.some(
            (r) =>
              (r.source === e.id && r.destination === related) ||
              (r.destination === e.id && r.source === related),
          )),
    )
    .sort((a, b) =>
      sort === "name"
        ? a.name.localeCompare(b.name)
        : (b[sort] || 0) - (a[sort] || 0),
    );
  return (
    <main className="world-page concept-workspace">
      <div className="workspace-heading">
        <div>
          <span className="eyebrow">
            {doc.world.name} / {age.name}
          </span>
          <h1>Culture & Religion</h1>
          <p>
            Traditions, ideas, shared practices and differing interpretations.
          </p>
        </div>
        <button className="primary" onClick={() => setCreate(true)}>
          Create concept
        </button>
      </div>
      <div className="collection-toolbar">
        <input
          type="text"
          aria-label="Search concepts"
          placeholder="Search names, beliefs, practices or content"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <label>
          Collection
          <select
            value={collection}
            onChange={(e) => setCollection(e.target.value)}
          >
            {Object.entries(collections).map(([k, v]) => (
              <option key={k} value={k}>
                {v}
              </option>
            ))}
          </select>
        </label>
        <label>
          Context / region
          <select value={context} onChange={(e) => setContext(e.target.value)}>
            <option value="">Any context</option>
            {all.map((e) => (
              <option key={e.id} value={e.id}>
                {e.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Related tradition
          <select value={related} onChange={(e) => setRelated(e.target.value)}>
            <option value="">All</option>
            {concepts.map((e) => (
              <option key={e.id} value={e.id}>
                {e.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Status
          <select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="">All</option>
            {[
              ...new Set(
                concepts.map((e) => e.properties?.status).filter(Boolean),
              ),
            ].map((s) => (
              <option key={s}>{s}</option>
            ))}
          </select>
        </label>
        <label>
          Order
          <select value={sort} onChange={(e) => setSort(e.target.value)}>
            <option value="name">Name</option>
            <option value="createdAt">Recently created</option>
            <option value="updatedAt">Recently edited</option>
          </select>
        </label>
      </div>
      <div className="concept-directory">
        {rows.map((e) => (
          <article key={e.id}>
            <div>
              <button className="concept-title" onClick={() => onProfile(e)}>
                {e.name}
              </button>
              <small>
                {e.content?.customType || label(e.kind)}
                {e.properties?.status && " · " + e.properties.status}
              </small>
            </div>
            <p>
              {e.properties?.summary ||
                e.content?.claim?.statement ||
                "No summary yet."}
            </p>
            <span>
              {(e.content?.tags || []).join(" · ")}
              {conceptContexts(e)
                .slice(0, 3)
                .map((id) => (
                  <small key={id}>{entityIn(doc, age.id, id)?.name}</small>
                ))}
            </span>
          </article>
        ))}
      </div>
      {!rows.length && (
        <p>
          No concepts match this collection. Create a concept with just its name
          and type.
        </p>
      )}
      {create && (
        <CreateConcept
          doc={doc}
          age={age.id}
          command={command}
          onProfile={onProfile}
          onClose={() => setCreate(false)}
        />
      )}
    </main>
  );
}
function ContentEditor({ value, component, doc, age, onSave, onClose }) {
  const [v, set] = useState(structuredClone(value)),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const change = (k, val) => set((v) => ({ ...v, [k]: val }));
  return (
    <Dialog
      title={
        <>
          {component
            ? "Content section"
            : "Custom fields & contextual variation"}
        </>
      }
      size="editor"
      className="concept-editor"
      onClose={onClose}
      busy={busy}
      onSubmit={async (e) => {
        e.preventDefault();
        setBusy(true);
        try {
          if (await onSave(v)) onClose();
          else
            setError(
              "Unable to save. Check field values and the error message on the page.",
            );
        } finally {
          setBusy(false);
        }
      }}
      actions={
        <>
          <button type="button" onClick={onClose} data-dialog-dismiss>
            Cancel
          </button>
          <button className="primary" disabled={busy}>
            Save content
          </button>
        </>
      }
    >
      {component && (
        <>
          <label>
            Section type
            <select
              value={v.type}
              onChange={(e) => change("type", e.target.value)}
            >
              {Object.entries(componentTypes).map(([k, n]) => (
                <option key={k} value={k}>
                  {n}
                </option>
              ))}
            </select>
          </label>
          <label>
            Title
            <input
              type="text"
              required
              maxLength={200}
              value={v.title}
              onChange={(e) => change("title", e.target.value)}
            />
          </label>
          <label>
            Description
            <textarea
              rows="5"
              value={v.description || ""}
              onChange={(e) => change("description", e.target.value)}
            />
          </label>
          <Dates value={v} onChange={set} />
        </>
      )}
      {v.claim || v.type === "belief" ? (
        <>
          <ClaimEditor value={v.claim} onChange={(c) => change("claim", c)} />
          {v.type !== "belief" && (
            <button type="button" onClick={() => change("claim", null)}>
              Remove claim
            </button>
          )}
        </>
      ) : (
        <button
          type="button"
          onClick={() => change("claim", { statement: "" })}
        >
          Add claim
        </button>
      )}
      <h3>Typed fields</h3>
      <FieldsEditor
        fields={v.fields}
        onChange={(f) => change("fields", f)}
        doc={doc}
        age={age}
      />
      <h3>Contextual variations</h3>
      <VariationsEditor
        value={v.contexts}
        onChange={(c) => change("contexts", c)}
        doc={doc}
        age={age}
      />
      {component && (
        <>
          <h3>Related content</h3>
          <ComponentLinksEditor
            type={v.type}
            value={v.links}
            onChange={(c) => change("links", c)}
            doc={doc}
            age={age}
          />
        </>
      )}
      <p role="alert">{error}</p>
    </Dialog>
  );
}
function OverviewEditor({ entity, doc, age, onSave, onClose }) {
  const [v, set] = useState(structuredClone(entity)),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const p = v.properties || {},
    c = v.content || emptyContent(),
    prop = (k, x) =>
      set((v) => ({ ...v, properties: { ...v.properties, [k]: x } }));
  return (
    <Dialog
      title={<>Edit overview</>}
      size="editor"
      className="concept-editor"
      onClose={onClose}
      busy={busy}
      onSubmit={async (e) => {
        e.preventDefault();
        setBusy(true);
        try {
          if (await onSave(v)) onClose();
          else
            setError(
              "Unable to save. Check field values and the error message on the page.",
            );
        } finally {
          setBusy(false);
        }
      }}
      actions={
        <>
          <button type="button" onClick={onClose} data-dialog-dismiss>
            Cancel
          </button>
          <button className="primary" disabled={busy}>
            Save overview
          </button>
        </>
      }
    >
      <label>
        Name
        <input
          type="text"
          required
          maxLength={200}
          value={v.name}
          onChange={(e) => set({ ...v, name: e.target.value })}
        />
      </label>
      <label>
        Type
        <select disabled value={v.kind}>
          {conceptTypes.map((t) => (
            <option key={t} value={t}>
              {label(t)}
            </option>
          ))}
        </select>
      </label>
      {["summary", "description", "status"].map((k) => (
        <label key={k}>
          {label(k)}
          <textarea
            value={p[k] || ""}
            onChange={(e) => prop(k, e.target.value)}
          />
        </label>
      ))}
      <label>
        Custom classification
        <input
          type="text"
          maxLength={200}
          value={c.customType || ""}
          onChange={(e) =>
            set({ ...v, content: { ...c, customType: e.target.value } })
          }
        />
      </label>
      <label>
        Tags (comma separated)
        <input
          type="text"
          value={(c.tags || []).join(",")}
          onChange={(e) =>
            set({ ...v, content: { ...c, tags: e.target.value.split(",") } })
          }
        />
      </label>
      <Dates value={p} onChange={(properties) => set({ ...v, properties })} />
      <h3>Alternate & contextual names</h3>
      {(v.names || []).map((n, i) => (
        <section key={i}>
          {["name", "language", "script", "validFrom", "validTo"].map((k) => (
            <label key={k}>
              {label(k)}
              <input
                type="text"
                required={k === "name"}
                value={n[k] || ""}
                onChange={(e) =>
                  set({
                    ...v,
                    names: v.names.map((x, j) =>
                      j === i ? { ...x, [k]: e.target.value } : x,
                    ),
                  })
                }
              />
            </label>
          ))}
          <label>
            Name type
            <select
              value={n.type}
              onChange={(e) =>
                set({
                  ...v,
                  names: v.names.map((x, j) =>
                    j === i ? { ...x, type: e.target.value } : x,
                  ),
                })
              }
            >
              {["canonical", "historical", "local", "alternate", "exonym"].map(
                (t) => (
                  <option key={t}>{t}</option>
                ),
              )}
            </select>
          </label>
          <label>
            Used by
            <select
              value={n.contextId || ""}
              onChange={(e) =>
                set({
                  ...v,
                  names: v.names.map((x, j) =>
                    j === i ? { ...x, contextId: e.target.value } : x,
                  ),
                })
              }
            >
              <option value="">No specific context</option>
              {activeEntities(doc, age).map((e) => (
                <option key={e.id} value={e.id}>
                  {e.name}
                </option>
              ))}
            </select>
          </label>
          <button
            type="button"
            onClick={() =>
              set({ ...v, names: v.names.filter((_, j) => i !== j) })
            }
          >
            Remove name
          </button>
        </section>
      ))}
      <button
        type="button"
        onClick={() =>
          set({
            ...v,
            names: [...(v.names || []), { name: "", type: "alternate" }],
          })
        }
      >
        Add name
      </button>
      <p role="alert">{error}</p>
    </Dialog>
  );
}
export function ConceptProfile({ doc, entity, command, onProfile, onBack }) {
  const [editor, setEditor] = useState(null),
    [connection, setConnection] = useState(null),
    [related, setRelated] = useState(false),
    [tab, setTab] = useState("Article"),
    [compare, setCompare] = useState("");
  const age = entity.ageId,
    c = entity.content || emptyContent(),
    p = entity.properties || {},
    versions = doc.entities.filter(
      (e) => e.ageId !== age && identity(e) === identity(entity),
    ),
    relations = doc.relations.filter(
      (r) =>
        r.ageId === age &&
        (r.source === entity.id || r.destination === entity.id),
    ),
    previous = versions.find((e) => e.ageId === compare);
  const save = (e) => command({ kind: "entity", ageId: age, entity: e }),
    saveContent = (content) => save({ ...entity, content });
  const add = (type = "section", title = "") =>
    setEditor({ id: uid(), type, title, fields: [], contexts: [], links: [] });
  const viewContent = (x) => (
    <>
      <ClaimView claim={x.claim} />
      <FieldsView fields={x.fields} doc={doc} age={age} onProfile={onProfile} />
      <VariationsView
        value={x.contexts}
        doc={doc}
        age={age}
        onProfile={onProfile}
      />
    </>
  );
  return (
    <main className="world-page concept-profile">
      <div className="profile-breadcrumb">
        <button onClick={onBack}>Back to Culture & Religion</button>
        <span>
          {doc.world.name} /{" "}
          <strong>{doc.ages.find((a) => a.id === age)?.name}</strong>
        </span>
      </div>
      <header>
        <span className="eyebrow">
          {c.customType || label(entity.kind)} ·{" "}
          {p.status || "No status specified"}
        </span>
        <h1>{entity.name}</h1>
        <p className="concept-summary">{p.summary}</p>
        {entity.names?.length > 0 && (
          <p>Also known as: {entity.names.map((n) => n.name).join(", ")}</p>
        )}
        <small>{(c.tags || []).join(" · ")}</small>
        <div className="history-actions">
          <button onClick={() => setEditor("overview")}>Edit overview</button>
          <button onClick={() => add()}>Add Content</button>
          <button onClick={() => setRelated(true)}>
            Create / reuse related concept
          </button>
        </div>
      </header>
      <nav className="institution-tabs">
        {["Article", "Connections", "History"].map((t) => (
          <button
            key={t}
            aria-current={tab === t ? "page" : undefined}
            onClick={() => setTab(t)}
          >
            {t}
          </button>
        ))}
      </nav>
      {tab === "Article" && (
        <article className="concept-article">
          <p className="prose">{p.description}</p>
          {viewContent(c)}
          <button onClick={() => setEditor("fields")}>
            Edit claims, fields & variations
          </button>
          {(c.components || []).map((part) => (
            <section className="concept-section" key={part.id}>
              <div className="section-heading">
                <h2>{part.title}</h2>
                {!part.entityId && (
                  <button onClick={() => setEditor(part)}>Edit</button>
                )}
              </div>
              {part.entityId ? (
                <button
                  onClick={() => onProfile(entityIn(doc, age, part.entityId))}
                >
                  Open {entityIn(doc, age, part.entityId)?.name || part.title}
                </button>
              ) : (
                <>
                  <p className="prose">{part.description}</p>
                  {viewContent(part)}
                  {(part.startYear != null || part.endYear != null) && (
                    <small>
                      Years: {part.startYear ?? "unspecified"} —{" "}
                      {part.endYear ?? "ongoing"}
                    </small>
                  )}
                  {(part.links || []).map((l) => (
                    <p key={l.id}>
                      {label(l.kind)}{" "}
                      <button
                        onClick={() =>
                          onProfile(entityIn(doc, age, l.targetId))
                        }
                      >
                        {entityIn(doc, age, l.targetId)?.name}
                      </button>{" "}
                      {l.stance} {l.description}
                    </p>
                  ))}
                  <div className="history-actions">
                    <button
                      onClick={() =>
                        command({
                          kind: "promote-component",
                          ageId: age,
                          entityId: entity.id,
                          componentId: part.id,
                        })
                      }
                    >
                      Make {part.name || part.title || "this component"}{" "}
                      independent
                    </button>
                    <button
                      onClick={() =>
                        saveContent({
                          ...c,
                          components: c.components.filter(
                            (x) => x.id !== part.id,
                          ),
                        })
                      }
                    >
                      Remove section
                    </button>
                  </div>
                </>
              )}
            </section>
          ))}
          <details className="section-suggestions">
            <summary>Suggested content for this type</summary>
            {(
              suggestedSections[c.template || entity.kind] ||
              suggestedSections.worldview
            ).map(([type, title]) => (
              <button key={title} onClick={() => add(type, title)}>
                {title}
              </button>
            ))}
          </details>
        </article>
      )}
      {tab === "Connections" && (
        <section>
          <button
            onClick={() =>
              setConnection({
                source: entity.id,
                destination: "",
                kind: "associated-with",
              })
            }
          >
            Add connection
          </button>
          {[...new Set(relations.map((r) => r.kind))].map((k) => (
            <section key={k}>
              <h2>{label(k)}</h2>
              {relations
                .filter((r) => r.kind === k)
                .map((r) => {
                  const from = entityIn(doc, age, r.source),
                    to = entityIn(doc, age, r.destination);
                  return (
                    <article className="concept-connection" key={r.id}>
                      <div>
                        <button onClick={() => onProfile(from)}>
                          {from?.name}
                        </button>{" "}
                        →{" "}
                        <button onClick={() => onProfile(to)}>
                          {to?.name}
                        </button>
                      </div>
                      <p>{r.description}</p>
                      <small>
                        {[
                          r.stance,
                          r.authority,
                          r.prevalence,
                          r.automatic ? "Derived" : "Authored",
                        ]
                          .filter(Boolean)
                          .join(" · ")}
                      </small>
                      <button onClick={() => setConnection(r)}>
                        Details / Edit
                      </button>
                    </article>
                  );
                })}
            </section>
          ))}
          {!relations.length && (
            <p>
              No connections recorded. Shared traditions do not automatically
              assign beliefs to their members.
            </p>
          )}
        </section>
      )}
      {tab === "History" && (
        <section>
          <h2>Historical state</h2>
          <p>
            Start year: {p.startYear ?? "not recorded"} · End year:{" "}
            {p.endYear ?? "not recorded"}
          </p>
          <p>
            Saved changes apply only to{" "}
            {doc.ages.find((a) => a.id === age)?.name}.
          </p>
          <label>
            Compare historical version
            <select
              value={compare}
              onChange={(e) => setCompare(e.target.value)}
            >
              <option value="">Choose an Age</option>
              {versions.map((e) => (
                <option key={e.ageId} value={e.ageId}>
                  {doc.ages.find((a) => a.id === e.ageId)?.name}
                </option>
              ))}
            </select>
          </label>
          {previous && (
            <>
              <button onClick={() => onProfile(previous)}>
                Open {previous.name} in its Age
              </button>
              <p>
                Name: {previous.name} → {entity.name}
              </p>
              <p>
                Overview, claims and fields:{" "}
                {JSON.stringify({
                  ...previous.properties,
                  content: { ...previous.content, components: [] },
                }) ===
                JSON.stringify({ ...p, content: { ...c, components: [] } })
                  ? "unchanged"
                  : "modified"}
              </p>
              <ul>
                {compareConcept(entity, previous).map((r, i) => (
                  <li key={i}>
                    {r.title} — {r.status}
                  </li>
                ))}
              </ul>
            </>
          )}
          {doc.years
            ?.filter((y) => y.ageId === age)
            .flatMap((y) =>
              (y.events || [])
                .filter((ev) => ev.entityId === entity.id)
                .map((ev) => (
                  <p key={ev.id}>
                    Year {y.number}: {ev.title} {ev.description}
                  </p>
                )),
            )}
        </section>
      )}
      {editor === "overview" && (
        <OverviewEditor
          entity={entity}
          doc={doc}
          age={age}
          onSave={save}
          onClose={() => setEditor(null)}
        />
      )}{" "}
      {editor === "fields" && (
        <ContentEditor
          value={c}
          doc={doc}
          age={age}
          onSave={saveContent}
          onClose={() => setEditor(null)}
        />
      )}{" "}
      {editor && typeof editor === "object" && (
        <ContentEditor
          value={editor}
          component
          doc={doc}
          age={age}
          onSave={(part) =>
            saveContent({
              ...c,
              components: (c.components || []).some((x) => x.id === part.id)
                ? c.components.map((x) => (x.id === part.id ? part : x))
                : [...(c.components || []), part],
            })
          }
          onClose={() => setEditor(null)}
        />
      )}
      {connection && (
        <ConnectionEditor
          doc={doc}
          age={age}
          relation={connection}
          kinds={[...new Set([...culturalKinds, connection.kind])]}
          command={command}
          onClose={() => setConnection(null)}
        />
      )}{" "}
      {related && (
        <CreateConcept
          doc={doc}
          age={age}
          parent={entity}
          command={command}
          onProfile={onProfile}
          onClose={() => setRelated(false)}
        />
      )}
    </main>
  );
}
