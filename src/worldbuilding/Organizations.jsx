import useUnsavedChanges from "./ui/useUnsavedChanges.js";
import { confirmAction } from "./ui/feedback.js";
import Dialog from "./ui/Dialog.jsx";
import { useState } from "react";
import {
  activeEntities,
  entityIn,
  identity,
  label,
  registry,
} from "./model.js";
import ConnectionEditor from "./ConnectionEditor.jsx";
import "./organizations.css";

export const organizationTypes = [
  "Political institution",
  "Political party",
  "Company",
  "Trade guild",
  "Military organization",
  "Religious order",
  "Academic institution",
  "Professional association",
  "Criminal network",
  "Secret society",
  "Resistance movement",
  "Humanitarian organization",
  "Federation",
  "Coalition",
];
const presenceKinds = [
  "headquartered-in",
  "branch-site",
  "operates-in",
  "mobile-base",
  "owns-site",
];
const externalKinds = [
  "system-knowledge",
  "system-access",
  "system-developed-by",
  "system-uses",
  "institution-rival",
  "contractor-for",
  "client-of",
  "informs",
  "founder-of",
  "institution-alliance",
  "competes-with",
  "cooperates-with",
  "affiliate-of",
  "branch-of",
  "subordinate-to",
  "founded-by",
  "sponsors",
  "funds",
  "controls",
  "serves",
  "represents",
  "advises",
  "protects",
  "opposes",
  "infiltrates",
  "influences",
  "successor-of",
  "predecessor-of",
  "split-from",
  "affirms",
  "rejects",
  "teaches",
  "interprets",
  "disputes",
  "transmits",
  "follows",
  "practices",
];
const statuses = [
  "active",
  "inactive",
  "dissolved",
  "secret",
  "exiled",
  "underground",
];
const scopes = [
  "local",
  "regional",
  "national",
  "international",
  "continental",
  "planetary",
  "mobile",
  "unknown",
];
function Emblem({ entity }) {
  const src = entity.properties?.emblem || entity.properties?.portrait;
  return src ? (
    <img className="organization-emblem" src={src} alt="" />
  ) : (
    <span className="organization-emblem placeholder" aria-hidden="true">
      ◈
    </span>
  );
}
function Suggestions({ id, values }) {
  return (
    <datalist id={id}>
      {values.map((v) => (
        <option key={v} value={v} />
      ))}
    </datalist>
  );
}

export default function Organizations({ doc, age, command, onProfile }) {
  const [search, setSearch] = useState(""),
    [type, setType] = useState(""),
    [status, setStatus] = useState(""),
    [scope, setScope] = useState(""),
    [location, setLocation] = useState(""),
    [affiliation, setAffiliation] = useState(""),
    [sort, setSort] = useState("name"),
    [create, setCreate] = useState(false),
    [name, setName] = useState(""),
    [category, setCategory] = useState(""),
    [summary, setSummary] = useState(""),
    [model, setModel] = useState(""),
    [busy, setBusy] = useState(false);
  const all = activeEntities(doc, age.id),
    organizations = all.filter(
      (e) => registry.types[e.kind]?.category === "organizations",
    ),
    relations = doc.relations.filter((r) => r.ageId === age.id);
  const rows = organizations
    .filter(
      (e) =>
        `${e.name} ${e.properties?.summary || ""}`
          .toLowerCase()
          .includes(search.toLowerCase()) &&
        (!type || e.properties?.organizationType === type) &&
        (!status || (e.properties?.status || "active") === status) &&
        (!scope || e.properties?.scope === scope) &&
        (!location ||
          relations.some(
            (r) =>
              r.source === e.id &&
              r.destination === location &&
              presenceKinds.includes(r.kind),
          )) &&
        (!affiliation ||
          relations.some(
            (r) =>
              ((r.source === e.id && r.destination === affiliation) ||
                (r.destination === e.id && r.source === affiliation)) &&
              ["member-of", ...externalKinds].includes(r.kind),
          )),
    )
    .sort((a, b) =>
      String(
        sort === "name" ? a.name : a.properties?.[sort] || "",
      ).localeCompare(
        String(sort === "name" ? b.name : b.properties?.[sort] || ""),
      ),
    );
  const options = (items, value, setter, title) => (
    <label>
      {title}
      <select value={value} onChange={(e) => setter(e.target.value)}>
        <option value="">All</option>
        {[...new Set(items.filter(Boolean))].sort().map((v) => (
          <option key={v}>{v}</option>
        ))}
      </select>
    </label>
  );
  return (
    <main className="world-page workspace organizations-directory">
      <div className="workspace-heading">
        <div>
          <span className="eyebrow">
            {doc.world.name} / {age.name}
          </span>
          <h1>Organizations</h1>
          <p>Institutions, associations and independent groups.</p>
        </div>
        <button className="primary" onClick={() => setCreate(true)}>
          Create Organization
        </button>
      </div>
      <div className="collection-toolbar">
        <input
          type="text"
          aria-label="Search organizations"
          placeholder="Search name or purpose"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        {options(
          organizations.map((e) => e.properties?.organizationType),
          type,
          setType,
          "Type",
        )}
        {options(
          organizations.map((e) => e.properties?.status || "active"),
          status,
          setStatus,
          "Status",
        )}
        {options(
          organizations.map((e) => e.properties?.scope),
          scope,
          setScope,
          "Scope",
        )}
        <label>
          Location
          <select
            value={location}
            onChange={(e) => setLocation(e.target.value)}
          >
            <option value="">All places</option>
            {all
              .filter((e) =>
                ["geography", "settlements", "politics"].includes(
                  registry.types[e.kind]?.category,
                ),
              )
              .map((e) => (
                <option key={e.id} value={e.id}>
                  {e.name}
                </option>
              ))}
          </select>
        </label>
        <label>
          Affiliation
          <select
            value={affiliation}
            onChange={(e) => setAffiliation(e.target.value)}
          >
            <option value="">All affiliations</option>
            {organizations.map((e) => (
              <option key={e.id} value={e.id}>
                {e.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Sort
          <select value={sort} onChange={(e) => setSort(e.target.value)}>
            {["name", "organizationType", "scope", "status"].map((k) => (
              <option key={k} value={k}>
                {label(k)}
              </option>
            ))}
          </select>
        </label>
      </div>
      <div className="institution-table-scroll">
        <table className="institution-table">
          <thead>
            <tr>
              {[
                "Organization",
                "Type",
                "Purpose",
                "Scope",
                "Headquarters / location",
                "Status",
              ].map((s) => (
                <th key={s}>{s}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((e) => {
              const place =
                relations.find(
                  (r) => r.source === e.id && r.kind === "headquartered-in",
                ) ||
                relations.find(
                  (r) => r.source === e.id && presenceKinds.includes(r.kind),
                );
              return (
                <tr key={e.id}>
                  <td>
                    <button
                      className="directory-name"
                      onClick={() => onProfile(e)}
                    >
                      <Emblem entity={e} />
                      {e.name}
                    </button>
                  </td>
                  <td>{e.properties?.organizationType || label(e.kind)}</td>
                  <td>{e.properties?.summary || "—"}</td>
                  <td>{e.properties?.scope || "Unspecified"}</td>
                  <td>
                    {place ? (
                      <button
                        onClick={() =>
                          onProfile(entityIn(doc, age.id, place.destination))
                        }
                      >
                        {entityIn(doc, age.id, place.destination)?.name}
                      </button>
                    ) : (
                      e.properties?.locationPolicy || "No recorded location"
                    )}
                  </td>
                  <td>{e.properties?.status || "active"}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      {!rows.length && (
        <p className="workspace-empty">
          No organizations match this directory. Create one with just a name.
        </p>
      )}
      {create && (
        <Dialog
          title={<>Create Organization</>}
          size="form"
          className=""
          onClose={() => setCreate(false)}
          busy={busy}
          onSubmit={async (e) => {
            e.preventDefault();
            setBusy(true);
            const entity = {
              id: crypto.randomUUID(),
              kind: "organization",
              name,
              properties: {
                organizationType: category,
                summary,
                governance: model,
                status: "active",
              },
            };
            try {
              if (await command({ kind: "entity", ageId: age.id, entity })) {
                setCreate(false);
                onProfile({ ...entity, ageId: age.id });
              }
            } finally {
              setBusy(false);
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
              <button className="primary" disabled={busy}>
                Create
              </button>
            </>
          }
        >
          <label>
            Name
            <input
              type="text"
              required
              autoFocus
              maxLength={200}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <label>
            Type (optional, or enter your own)
            <input
              type="text"
              list="organization-types"
              value={category}
              onChange={(e) => setCategory(e.target.value)}
            />
          </label>
          <Suggestions id="organization-types" values={organizationTypes} />
          <label>
            Summary
            <textarea
              value={summary}
              onChange={(e) => setSummary(e.target.value)}
            />
          </label>
          <label>
            Organizational model
            <input
              type="text"
              placeholder="Flat, federated, hierarchical, decentralized…"
              value={model}
              onChange={(e) => setModel(e.target.value)}
            />
          </label>
        </Dialog>
      )}
    </main>
  );
}

function RecordEditor({
  record,
  doc,
  age,
  organization,
  command,
  onClose,
  onProfile,
}) {
  const [e, set] = useState(record),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    p = e.properties || {},
    all = activeEntities(doc, age),
    units = all.filter(
      (x) =>
        x.kind === "organization-unit" &&
        x.properties?.organizationId === organization.id,
    ),
    office = e.kind === "organization-position";
  const prop = (k, v) =>
    set((e) => ({ ...e, properties: { ...e.properties, [k]: v } }));
  const descendants = new Set([e.id]);
  let changed = true;
  while (changed) {
    changed = false;
    for (const u of units)
      if (
        descendants.has(u.properties?.parentUnitId) &&
        !descendants.has(u.id)
      ) {
        descendants.add(u.id);
        changed = true;
      }
  }
  const related = doc.relations.filter(
    (r) =>
      r.ageId === age &&
      (r.unitId === e.id || r.destination === e.id || r.source === e.id),
  );
  async function saveRecord() {
    const moves =
      office && record.properties?.unitId !== p.unitId
        ? related
            .filter((r) => r.kind === "appointed-to" && r.destination === e.id)
            .map((r) => ({
              kind: "relation",
              relation: { ...r, unitId: p.unitId || "" },
            }))
        : [];
    return command(
      moves.length
        ? {
            kind: "batch",
            ageId: age,
            commands: [{ kind: "entity", entity: e }, ...moves],
          }
        : { kind: "entity", ageId: age, entity: e },
    );
  }
  return (
    <Dialog
      title={<>{office ? "Office definition" : "Organizational unit"}</>}
      size="form"
      className="relation-dialog"
      onClose={onClose}
      busy={busy}
      onSubmit={async (ev) => {
        ev.preventDefault();
        setBusy(true);
        try {
          if (await saveRecord()) onClose();
          else setError("Unable to save. Check the page error.");
        } finally {
          setBusy(false);
        }
      }}
      actions={
        <>
          <button type="button" onClick={onClose} data-dialog-dismiss>
            Close
          </button>
          <button className="primary" disabled={busy}>
            Save
          </button>
          {doc.entities.some((x) => x.ageId === age && x.id === e.id) && (
            <button
              type="button"
              onClick={async () => {
                if (
                  (await confirmAction(
                    "Remove this record? Move dependent units, offices and memberships first.",
                  )) &&
                  (await command({
                    kind: "delete-entities",
                    ageId: age,
                    entityIds: [e.id],
                  }))
                )
                  onClose();
              }}
            >
              Remove
            </button>
          )}
        </>
      }
    >
      <label>
        Name
        <input
          type="text"
          required
          maxLength={200}
          value={e.name}
          onChange={(ev) => set((e) => ({ ...e, name: ev.target.value }))}
        />
      </label>
      <label>
        {office ? "Unit" : "Parent unit"}
        <select
          value={p[office ? "unitId" : "parentUnitId"] || ""}
          onChange={(ev) =>
            prop(office ? "unitId" : "parentUnitId", ev.target.value)
          }
        >
          <option value="">{organization.name} (root)</option>
          {units
            .filter((u) => !descendants.has(u.id))
            .map((u) => (
              <option key={u.id} value={u.id}>
                {u.name}
              </option>
            ))}
        </select>
      </label>
      <label>
        {office ? "Responsibilities" : "Unit type"}
        <input
          type="text"
          value={p[office ? "responsibilities" : "unitType"] || ""}
          onChange={(ev) =>
            prop(office ? "responsibilities" : "unitType", ev.target.value)
          }
          placeholder={
            office
              ? "Treasury, command, shared leadership…"
              : "Department, branch, chapter, cell…"
          }
        />
      </label>
      <label>
        Description
        <textarea
          value={p.description || ""}
          onChange={(ev) => prop("description", ev.target.value)}
        />
      </label>
      <div className="field-pair">
        {["startYear", "endYear"].map((k) => (
          <label key={k}>
            {k === "startYear" ? "Start year" : "End year"}
            <input
              type="text"
              inputMode="numeric"
              value={p[k] ?? ""}
              onChange={(ev) => prop(k, ev.target.value)}
            />
          </label>
        ))}
      </div>
      {office && (
        <label>
          <input
            type="checkbox"
            checked={!!p.shared}
            onChange={(ev) => prop("shared", ev.target.checked)}
          />{" "}
          Shared office
        </label>
      )}
      <h3>{office ? "Appointments" : "Members & locations"}</h3>
      {related.length ? (
        related.map((r) => {
          const other = entityIn(
            doc,
            age,
            r.source === e.id ? r.destination : r.source,
          );
          return (
            <p key={r.id}>
              <button type="button" onClick={() => onProfile(other)}>
                {other?.name}
              </button>{" "}
              · {label(r.kind)} {r.role} {r.status}
            </p>
          );
        })
      ) : (
        <p>
          {office
            ? "Vacant — appoint a character from Members & Offices."
            : "No members or locations recorded yet."}
        </p>
      )}
      {error && <p role="alert">{error}</p>}
    </Dialog>
  );
}

export function OrganizationProfile({
  doc,
  entity,
  command,
  onProfile,
  onBack,
  onAtlas,
  initialRecord,
}) {
  const age = doc.ages.find((a) => a.id === entity.ageId),
    ageID = age.id,
    all = activeEntities(doc, ageID),
    [tab, setTab] = useState(
      initialRecord?.kind === "organization-position"
        ? "Members & Offices"
        : initialRecord
          ? "Structure"
          : "Overview",
    ),
    [draft, setDraft] = useState(entity),
    [record, setRecord] = useState(initialRecord || null),
    [connection, setConnection] = useState(null),
    [search, setSearch] = useState(""),
    [filterUnit, setFilterUnit] = useState(""),
    [filterStatus, setFilterStatus] = useState(""),
    [collapsed, setCollapsed] = useState([]),
    [saved, setSaved] = useState(""),
    [event, setEvent] = useState(null),
    [error, setError] = useState("");
  useUnsavedChanges(JSON.stringify(draft) !== JSON.stringify(entity), () =>
    setDraft(entity),
  );
  const p = draft.properties || {},
    units = all.filter(
      (e) =>
        e.kind === "organization-unit" &&
        e.properties?.organizationId === entity.id,
    ),
    offices = all.filter(
      (e) =>
        e.kind === "organization-position" &&
        e.properties?.organizationId === entity.id,
    ),
    unitIDs = new Set(units.map((u) => u.id)),
    officeIDs = new Set(offices.map((o) => o.id));
  const relations = doc.relations.filter((r) => r.ageId === ageID),
    members = relations.filter(
      (r) => r.kind === "member-of" && r.destination === entity.id,
    ),
    appointments = relations.filter(
      (r) => r.kind === "appointed-to" && officeIDs.has(r.destination),
    ),
    presence = relations.filter(
      (r) =>
        presenceKinds.includes(r.kind) &&
        (r.source === entity.id || unitIDs.has(r.source)),
    ),
    connections = relations.filter(
      (r) =>
        (r.source === entity.id || r.destination === entity.id) &&
        externalKinds.includes(r.kind),
    ),
    events = relations.filter(
      (r) => r.source === entity.id && r.kind === "institution-event",
    );
  const prop = (k, v) =>
    setDraft((e) => ({ ...e, properties: { ...e.properties, [k]: v } }));
  function addRecord(kind, parent = "") {
    setRecord({
      id: crypto.randomUUID(),
      name: "",
      kind,
      properties: {
        organizationId: entity.id,
        ...(kind === "organization-position"
          ? { unitId: parent }
          : { parentUnitId: parent, order: units.length }),
      },
    });
  }
  function addConnection(kind, source = entity.id, destination = "") {
    setConnection({
      relation: {
        source,
        destination,
        kind,
        description: "",
        status: kind === "member-of" || kind === "appointed-to" ? "active" : "",
        unitId:
          kind === "appointed-to"
            ? entityIn(doc, ageID, destination)?.properties?.unitId || ""
            : "",
      },
      kinds:
        kind === "member-of"
          ? ["member-of"]
          : kind === "appointed-to"
            ? ["appointed-to"]
            : presenceKinds.includes(kind)
              ? presenceKinds
              : externalKinds,
    });
  }
  const openConnection = (r) =>
    setConnection({
      relation: r,
      kinds:
        r.kind === "member-of"
          ? ["member-of"]
          : r.kind === "appointed-to"
            ? ["appointed-to"]
            : presenceKinds.includes(r.kind)
              ? presenceKinds
              : r.kind === "institution-event"
                ? ["institution-event"]
                : externalKinds,
    });
  const filtered = (rows) =>
    rows.filter(
      (r) =>
        `${entityIn(doc, ageID, r.source)?.name} ${entityIn(doc, ageID, r.destination)?.name} ${r.kind} ${r.role || ""} ${r.rank || ""} ${r.description || ""}`
          .toLowerCase()
          .includes(search.toLowerCase()) &&
        (!filterUnit || r.unitId === filterUnit) &&
        (!filterStatus || (r.status || "active") === filterStatus),
    );
  const relationList = (rows) => (
    <ul className="institution-connections">
      {filtered(rows).map((r) => {
        const other = entityIn(
          doc,
          ageID,
          r.source === entity.id || unitIDs.has(r.source)
            ? r.destination
            : r.source,
        );
        return (
          <li key={r.id}>
            <button onClick={() => other && onProfile(other)}>
              {other?.name}
            </button>
            <span>
              {label(r.kind)}
              {r.source !== entity.id && !unitIDs.has(r.source)
                ? " → " + entity.name
                : ""}
            </span>
            <p>{r.description || "No additional description"}</p>
            <small>
              {r.validFrom || "Unspecified start"} — {r.validTo || "ongoing"}{" "}
              {r.status} {r.automatic ? "· derived" : ""}
            </small>
            <button onClick={() => openConnection(r)}>Details / Edit</button>
          </li>
        );
      })}
    </ul>
  );
  async function reorder(u, delta) {
    const siblings = units
        .filter(
          (v) =>
            (v.properties?.parentUnitId || "") ===
            (u.properties?.parentUnitId || ""),
        )
        .sort(
          (a, b) =>
            (a.properties?.order || 0) - (b.properties?.order || 0) ||
            a.id.localeCompare(b.id),
        ),
      i = siblings.findIndex((v) => v.id === u.id),
      j = i + delta;
    if (j < 0 || j >= siblings.length) return;
    [siblings[i], siblings[j]] = [siblings[j], siblings[i]];
    await command({
      kind: "batch",
      ageId: ageID,
      commands: siblings.map((e, order) => ({
        kind: "entity",
        entity: { ...e, properties: { ...e.properties, order } },
      })),
    });
  }
  function outline(parent = "", seen = new Set()) {
    const visible = new Set(
      units
        .filter(
          (u) =>
            !search ||
            `${u.name} ${u.properties?.description || ""}`
              .toLowerCase()
              .includes(search.toLowerCase()),
        )
        .map((u) => u.id),
    );
    for (const id of [...visible]) {
      let u = units.find((u) => u.id === id);
      const visited = new Set();
      while (u?.properties?.parentUnitId && !visited.has(u.id)) {
        visited.add(u.id);
        visible.add(u.properties.parentUnitId);
        u = units.find((v) => v.id === u.properties.parentUnitId);
      }
    }
    return units
      .filter(
        (u) =>
          (u.properties?.parentUnitId || "") === parent &&
          !seen.has(u.id) &&
          visible.has(u.id),
      )
      .sort(
        (a, b) =>
          (a.properties?.order || 0) - (b.properties?.order || 0) ||
          a.name.localeCompare(b.name),
      )
      .map((u) => (
        <li key={u.id}>
          <div className="unit-row">
            <button
              aria-label={`Toggle ${u.name}`}
              onClick={() =>
                setCollapsed((c) =>
                  c.includes(u.id)
                    ? c.filter((id) => id !== u.id)
                    : [...c, u.id],
                )
              }
            >
              {collapsed.includes(u.id) ? "▸" : "▾"}
            </button>
            <button className="unit-name" onClick={() => setRecord(u)}>
              {u.name}
              <small>{u.properties?.unitType || "Unit"}</small>
            </button>
            <span>
              {members.filter((r) => r.unitId === u.id).length} members
            </span>
            <button onClick={() => addRecord("organization-unit", u.id)}>
              Add unit
            </button>
            <button onClick={() => setRecord(u)}>Edit / Move</button>
            <button
              aria-label={`Move ${u.name} up`}
              onClick={() => reorder(u, -1)}
            >
              ↑
            </button>
            <button
              aria-label={`Move ${u.name} down`}
              onClick={() => reorder(u, 1)}
            >
              ↓
            </button>
          </div>
          {!collapsed.includes(u.id) && (
            <ul>{outline(u.id, new Set([...seen, u.id]))}</ul>
          )}
        </li>
      ));
  }
  async function emblem(file) {
    if (!file) return;
    if (
      !["image/png", "image/jpeg", "image/webp"].includes(file.type) ||
      file.size > 1500000
    ) {
      setError("Choose a PNG, JPEG or WebP image smaller than 1.5 MB.");
      return;
    }
    const reader = new FileReader();
    reader.onload = () => prop("emblem", reader.result);
    reader.readAsDataURL(file);
  }
  return (
    <main className="world-page institution-profile">
      <div className="profile-breadcrumb">
        <button onClick={onBack}>Back to Organizations</button>
        <span>
          {doc.world.name} / <strong>{age.name}</strong>
        </span>
      </div>
      <header className="institution-heading">
        <Emblem entity={draft} />
        <div>
          <span className="eyebrow">
            {p.organizationType || label(entity.kind)} · {p.status || "active"}
          </span>
          <h1>{entity.name}</h1>
          <p>{p.summary || "An organization with its own identity."}</p>
        </div>
      </header>
      <nav className="institution-tabs" aria-label="Organization sections">
        {[
          "Overview",
          "Structure",
          "Members & Offices",
          "Presence & Operations",
          "Connections",
          "History",
        ].map((t) => (
          <button
            key={t}
            aria-current={tab === t ? "page" : undefined}
            onClick={() => {
              setTab(t);
              setSearch("");
              setFilterUnit("");
              setFilterStatus("");
            }}
          >
            {t}
          </button>
        ))}
      </nav>
      {error && <p role="alert">{error}</p>}
      {tab !== "Overview" && (
        <div className="collection-toolbar">
          <input
            type="text"
            aria-label="Search organization records"
            placeholder="Search records, roles, ranks"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
      )}
      {tab === "Overview" && (
        <form
          className="institution-overview"
          onSubmit={async (e) => {
            e.preventDefault();
            const result = await command({
              kind: "entity",
              ageId: ageID,
              entity: draft,
            });
            if (result) {
              setDraft(
                result.entities.find(
                  (x) => x.id === entity.id && x.ageId === ageID,
                ),
              );
              setSaved("Saved in " + age.name);
            }
          }}
        >
          <label>
            Name
            <input
              type="text"
              required
              maxLength={200}
              value={draft.name}
              onChange={(e) =>
                setDraft((d) => ({ ...d, name: e.target.value }))
              }
            />
          </label>
          <div className="field-pair">
            <label>
              Organization type
              <input
                type="text"
                list="institution-types"
                value={p.organizationType || ""}
                onChange={(e) => prop("organizationType", e.target.value)}
              />
            </label>
            <label>
              Subtype
              <input
                type="text"
                value={p.subtype || ""}
                onChange={(e) => prop("subtype", e.target.value)}
              />
            </label>
          </div>
          <Suggestions id="institution-types" values={organizationTypes} />
          <label>
            Emblem
            <input
              type="file"
              accept="image/png,image/jpeg,image/webp"
              onChange={(e) => emblem(e.target.files[0])}
            />
          </label>
          {p.emblem && (
            <button type="button" onClick={() => prop("emblem", "")}>
              Remove emblem
            </button>
          )}
          {[
            "summary",
            "description",
            "goals",
            "activities",
            "foundingCircumstances",
            "principles",
            "customs",
            "governance",
          ].map((k) => (
            <label key={k}>
              {label(k).replace("Circumstances", " circumstances")}
              <textarea
                rows={k === "description" ? 5 : 2}
                value={p[k] || ""}
                onChange={(e) => prop(k, e.target.value)}
              />
            </label>
          ))}
          <div className="field-pair">
            <label>
              Operational status
              <input
                type="text"
                list="institution-status"
                value={p.status || "active"}
                onChange={(e) => prop("status", e.target.value)}
              />
            </label>
            <label>
              Scope
              <input
                type="text"
                list="institution-scope"
                value={p.scope || ""}
                onChange={(e) => prop("scope", e.target.value)}
              />
            </label>
          </div>
          <Suggestions id="institution-status" values={statuses} />
          <Suggestions id="institution-scope" values={scopes} />
          <div className="field-pair">
            <label>
              Founding year
              <input
                type="text"
                inputMode="numeric"
                value={p.startYear ?? ""}
                onChange={(e) => prop("startYear", e.target.value)}
              />
            </label>
            <label>
              Dissolution year
              <input
                type="text"
                inputMode="numeric"
                value={p.endYear ?? ""}
                onChange={(e) => prop("endYear", e.target.value)}
              />
            </label>
          </div>
          <label>
            Location policy
            <input
              type="text"
              placeholder="Unknown, secret, nomadic, no physical presence…"
              value={p.locationPolicy || ""}
              onChange={(e) => prop("locationPolicy", e.target.value)}
            />
          </label>
          <button className="primary">Save organization</button>
          <output role="status">{saved}</output>
        </form>
      )}
      {tab === "Structure" && (
        <section>
          <div className="section-heading">
            <h2>Internal units</h2>
            <button onClick={() => addRecord("organization-unit")}>
              Add unit
            </button>
          </div>
          <p>
            Use a flat or nested structure, or leave it empty. Edit a unit to
            move it to another parent.
          </p>
          <ul className="unit-outline">{outline()}</ul>
          {!units.length && (
            <p>
              No internal units. This organization can operate without a
              hierarchy.
            </p>
          )}
          <h2>Independent member organizations</h2>
          <button
            onClick={() =>
              addConnection(
                "member-of",
                all.find(
                  (e) =>
                    registry.types[e.kind]?.category === "organizations" &&
                    e.id !== entity.id,
                )?.id || "",
                entity.id,
              )
            }
          >
            Add member organization
          </button>
          {relationList(
            members.filter(
              (r) => entityIn(doc, ageID, r.source)?.kind !== "character",
            ),
          )}
          <p>
            An independent branch keeps its own organization profile. Link it
            through Connections using “branch of”.
          </p>
        </section>
      )}
      {tab === "Members & Offices" && (
        <section>
          <div className="section-heading">
            <h2>Membership roster</h2>
            <button
              onClick={() =>
                addConnection(
                  "member-of",
                  all.find((e) => e.kind === "character")?.id || "",
                  entity.id,
                )
              }
            >
              Add member
            </button>
          </div>
          <div className="collection-toolbar">
            <label>
              Unit
              <select
                value={filterUnit}
                onChange={(e) => setFilterUnit(e.target.value)}
              >
                <option value="">All units</option>
                {units.map((u) => (
                  <option key={u.id} value={u.id}>
                    {u.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Status
              <select
                value={filterStatus}
                onChange={(e) => setFilterStatus(e.target.value)}
              >
                <option value="">All statuses</option>
                {[
                  ...new Set(
                    members
                      .concat(appointments)
                      .map((r) => r.status || "active"),
                  ),
                ].map((s) => (
                  <option key={s}>{s}</option>
                ))}
              </select>
            </label>
          </div>
          <div className="institution-table-scroll">
            <table className="institution-table">
              <thead>
                <tr>
                  {[
                    "Member",
                    "Unit",
                    "Role / rank",
                    "Status",
                    "Validity",
                    "",
                  ].map((h, i) => (
                    <th key={i}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {filtered(members).map((r) => (
                  <tr key={r.id}>
                    <td>
                      <button
                        onClick={() =>
                          onProfile(entityIn(doc, ageID, r.source))
                        }
                      >
                        {entityIn(doc, ageID, r.source)?.name}
                      </button>
                    </td>
                    <td>
                      <button
                        disabled={!r.unitId}
                        onClick={() =>
                          setRecord(entityIn(doc, ageID, r.unitId))
                        }
                      >
                        {entityIn(doc, ageID, r.unitId)?.name ||
                          "Whole organization"}
                      </button>
                    </td>
                    <td>
                      {r.role || "Member"} / {r.rank || "—"}
                    </td>
                    <td>{r.status || "active"}</td>
                    <td>
                      {r.validFrom || "—"} / {r.validTo || "ongoing"}
                    </td>
                    <td>
                      <button onClick={() => openConnection(r)}>
                        Edit / Transfer
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="section-heading">
            <h2>Offices & appointments</h2>
            <button onClick={() => addRecord("organization-position")}>
              Define office
            </button>
          </div>
          {offices
            .filter((o) => o.name.toLowerCase().includes(search.toLowerCase()))
            .map((o) => (
              <article className="office-row" key={o.id}>
                <div>
                  <button onClick={() => setRecord(o)}>{o.name}</button>
                  <small>
                    {entityIn(doc, ageID, o.properties?.unitId)?.name ||
                      "Whole organization"}{" "}
                    · {o.properties?.shared ? "Shared office" : "Office"}
                  </small>
                </div>
                <button
                  onClick={() =>
                    addConnection(
                      "appointed-to",
                      all.find((e) => e.kind === "character")?.id || "",
                      o.id,
                    )
                  }
                >
                  Appoint character
                </button>
                {!appointments.some(
                  (r) =>
                    r.destination === o.id &&
                    (r.status || "active") === "active",
                ) && <span>Vacant</span>}
                {relationList(
                  appointments.filter((r) => r.destination === o.id),
                )}
              </article>
            ))}
        </section>
      )}
      {tab === "Presence & Operations" && (
        <section>
          <div className="section-heading">
            <h2>Places of operation</h2>
            <button onClick={() => addConnection("operates-in")}>
              Add location
            </button>
          </div>
          <p>
            {p.locationPolicy ||
              "No location is required. Record headquarters, sites and areas of operation independently."}
          </p>
          <button onClick={onAtlas}>Create a missing place in Atlas</button>
          {presenceKinds.map((k) => (
            <section key={k}>
              <h3>{label(k)}</h3>
              {relationList(presence.filter((r) => r.kind === k))}
            </section>
          ))}
        </section>
      )}
      {tab === "Connections" && (
        <section>
          <div className="section-heading">
            <h2>External relationships</h2>
            <button onClick={() => addConnection("affiliate-of")}>
              Add connection
            </button>
          </div>
          {[...new Set(connections.map((r) => r.kind))].map((k) => (
            <section key={k}>
              <h3>{label(k)}</h3>
              {relationList(connections.filter((r) => r.kind === k))}
            </section>
          ))}
          {!connections.length && <p>No external relationships recorded.</p>}
        </section>
      )}
      {tab === "History" && (
        <section>
          <div className="section-heading">
            <h2>Institutional history</h2>
            <button
              onClick={() => setEvent({ name: "", year: "", description: "" })}
            >
              Record event
            </button>
          </div>
          <p>
            Founded: {p.startYear ?? "not recorded"} · Dissolved:{" "}
            {p.endYear ?? "not recorded"} · Status: {p.status || "active"}
          </p>
          {relationList(events)}
          <h3>Earlier and other historical versions</h3>
          {doc.entities
            .filter(
              (e) => e.ageId !== ageID && identity(e) === identity(entity),
            )
            .map((e) => (
              <p key={e.ageId}>
                <button onClick={() => onProfile(e)}>
                  {doc.ages.find((a) => a.id === e.ageId)?.name}: {e.name}
                </button>
              </p>
            ))}
          <h3>Membership and office history</h3>
          {relationList(
            [...members, ...appointments].filter(
              (r) =>
                r.validFrom || r.validTo || (r.status && r.status !== "active"),
            ),
          )}
        </section>
      )}
      {record && (
        <RecordEditor
          key={record.id}
          record={record}
          doc={doc}
          age={ageID}
          organization={entity}
          command={command}
          onClose={() => setRecord(null)}
          onProfile={onProfile}
        />
      )}
      {connection && (
        <ConnectionEditor
          key={connection.relation.id || connection.relation.kind}
          doc={doc}
          age={ageID}
          relation={connection.relation}
          institutionId={entity.id}
          kinds={connection.kinds}
          command={command}
          onClose={() => setConnection(null)}
        />
      )}
      {event && (
        <Dialog
          title={<>Record institutional event</>}
          size="form"
          className=""
          onClose={() => setEvent(null)}
          onSubmit={async (ev) => {
            ev.preventDefault();
            const id = crypto.randomUUID();
            if (
              await command({
                kind: "batch",
                ageId: ageID,
                commands: [
                  {
                    kind: "entity",
                    entity: {
                      id,
                      name: event.name,
                      kind: "event",
                      properties: {
                        summary: event.description,
                        startYear: event.year,
                      },
                    },
                  },
                  {
                    kind: "relation",
                    relation: {
                      source: entity.id,
                      destination: id,
                      kind: "institution-event",
                      description: event.description,
                      validFrom: event.year,
                    },
                  },
                ],
              })
            )
              setEvent(null);
          }}
          actions={
            <>
              <button
                type="button"
                onClick={() => setEvent(null)}
                data-dialog-dismiss
              >
                Cancel
              </button>
              <button className="primary">Save event</button>
            </>
          }
        >
          <label>
            Event
            <input
              type="text"
              required
              maxLength={200}
              value={event.name}
              onChange={(e) =>
                setEvent((v) => ({ ...v, name: e.target.value }))
              }
            />
          </label>
          <label>
            Year (optional)
            <input
              type="text"
              inputMode="numeric"
              value={event.year}
              onChange={(e) =>
                setEvent((v) => ({ ...v, year: e.target.value }))
              }
            />
          </label>
          <label>
            Description
            <textarea
              value={event.description}
              onChange={(e) =>
                setEvent((v) => ({ ...v, description: e.target.value }))
              }
            />
          </label>
        </Dialog>
      )}
    </main>
  );
}
