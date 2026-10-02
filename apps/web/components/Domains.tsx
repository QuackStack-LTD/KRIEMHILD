"use client";
import { useEffect, useState } from "react";
import {
  api,
  Command,
  Entry,
  newID,
  State,
  HistoricalState,
} from "../lib/types";
import { useRecordDraft } from "../lib/useRecordDraft";
import DraftRecovery from "./DraftRecovery";
export type DomainType = {
  id: string;
  group: string;
  name: string;
  fields: { key: string; label: string; kind: string }[];
};
type Quantity = {
  mode: string;
  value?: string;
  min?: string;
  max?: string;
  unit?: string;
  text?: string;
};
type Props = {
  state: State;
  run: (c: Command) => Promise<boolean>;
  busy: boolean;
  onPending?: (pending: boolean) => void;
};
export default function Domains({ state, run, busy, onPending }: Props) {
  const [pending, setPending] = useState(false);
  useEffect(() => {
    onPending?.(pending);
    return () => onPending?.(false);
  }, [pending, onPending]);
  useEffect(() => {
    if (!pending) return;
    const warn = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [pending]);
  const [catalog, setCatalog] = useState<DomainType[]>([]),
    [domain, setDomain] = useState("settlement"),
    [selected, setSelected] = useState(""),
    [create, setCreate] = useState(false),
    [query, setQuery] = useState(""),
    [error, setError] = useState("");
  const [at, setAt] = useState(""),
    [dated, setDated] = useState<HistoricalState | null>(null);
  useEffect(() => {
    api<DomainType[]>("/domains")
      .then(setCatalog)
      .catch((e) => setError(e.message));
  }, []);
  useEffect(() => setDated(null), [state.revision]);
  const values = Object.values(dated?.records || state.records),
    entries = values.filter((r) => r.kind === "entity"),
    entry = entries.find((r) => r.id === selected),
    definition = catalog.find(
      (d) => d.id === (entry?.properties?._domain || domain),
    );
  const select = (id: string) => {
    if (pending) return;
    setSelected(id);
    setCreate(false);
  };
  return (
    <div className="domain-page">
      <div className="eyebrow">Societies & systems / {state.age.name}</div>
      <h2>One world, many connections.</h2>
      <p>
        Use optional templates, link shared identities, and keep numerical or
        qualitative accounts in the same Age.
      </p>
      <div className="map-tools">
        <label>
          Domain template
          <select
            disabled={pending}
            aria-label="Domain template"
            value={domain}
            onChange={(e) => {
              setDomain(e.target.value);
              setSelected("");
              setCreate(false);
            }}
          >
            {Array.from(new Set(catalog.map((d) => d.group))).map((group) => (
              <optgroup key={group} label={group}>
                {catalog
                  .filter((d) => d.group === group)
                  .map((d) => (
                    <option key={d.id} value={d.id}>
                      {d.name}
                    </option>
                  ))}
              </optgroup>
            ))}
          </select>
        </label>
        <button
          disabled={busy || !!dated || pending || state.readOnly}
          onClick={() => {
            setCreate(true);
            setSelected("");
          }}
        >
          New domain entity
        </button>
        <label>
          Filter domain entities
          <input value={query} onChange={(e) => setQuery(e.target.value)} />
        </label>
      </div>
      <form
        className="map-tools"
        onSubmit={async (e) => {
          e.preventDefault();
          if (pending) return;
          try {
            setDated(
              await api<HistoricalState>(
                `/projects/${state.root.world.id}/historical?snapshot=${state.age.snapshot}&tick=${encodeURIComponent(at)}`,
              ),
            );
            setCreate(false);
            setError("");
          } catch (e) {
            setError((e as Error).message);
          }
        }}
      >
        <label>
          Historical tick
          <input
            disabled={pending}
            value={at}
            onChange={(e) => setAt(e.target.value)}
          />
        </label>
        <button disabled={!at || pending}>View dated domains</button>
        <button disabled={pending} type="button" onClick={() => setDated(null)}>
          Return to overview
        </button>
      </form>
      {dated && (
        <div className="notice">
          Read-only historical view: {dated.label}
          {dated.unresolved.map((s, i) => (
            <p key={i}>{s}</p>
          ))}
        </div>
      )}
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      <div className="split">
        <section className="list-pane">
          {entries
            .filter(
              (r) =>
                (r.properties?._domain === domain || r.id === selected) &&
                r.name.toLowerCase().includes(query.toLowerCase()),
            )
            .map((r) => (
              <button
                key={r.id}
                disabled={pending}
                className={`record ${r.id === selected ? "active" : ""}`}
                onClick={() => select(r.id)}
              >
                <strong>{r.name}</strong>
                <small>{r.type}</small>
              </button>
            ))}
        </section>
        <section className="detail-pane">
          {definition && (create || entry) ? (
            <>
              <DomainForm
                key={
                  (dated?.tick || "present") +
                  (entry?.id || domain) +
                  JSON.stringify(entry)
                }
                definition={definition}
                entry={entry}
                entities={entries}
                world={state.root.world.id}
                age={dated ? `${state.age.id}:at:${dated.tick}` : state.age.id}
                onPending={setPending}
                disabled={busy || !!dated || !!state.readOnly}
                save={async (record) => {
                  if (await run({ action: "put", record })) {
                    setSelected(record.id);
                    setCreate(false);
                    return true;
                  }
                  return false;
                }}
              />
              {entry && (
                <Connections
                  entry={entry}
                  records={values}
                  state={state}
                  run={run}
                  disabled={busy || !!dated || pending || !!state.readOnly}
                  select={select}
                />
              )}
            </>
          ) : (
            <p className="muted">
              Choose or create an entity. Settlements, people, cultures,
              beliefs, rules and stories share the same identities and links.
            </p>
          )}
        </section>
      </div>
    </div>
  );
}
function DomainForm({
  definition,
  entry,
  entities,
  disabled,
  save,
  world,
  age,
  onPending,
}: {
  definition: DomainType;
  entry?: Entry;
  entities: Entry[];
  disabled: boolean;
  save: (r: Entry) => Promise<boolean>;
  world: string;
  age: string;
  onPending: (pending: boolean) => void;
}) {
  const [saved] = useState<Entry>(
    entry || {
      id: newID(),
      kind: "entity",
      type: definition.name,
      name: "",
      notes: "",
      properties: { _domain: definition.id },
    },
  );
  const draft = useRecordDraft(
    `kriemhild-domain:${world}:${age}:${entry?.id || "new-" + definition.id}`,
    saved,
    !entry,
  );
  const { name, notes } = draft.value,
    props = draft.value.properties || {};
  useEffect(() => {
    onPending(draft.pending);
    return () => onPending(false);
  }, [draft.pending, onPending]);
  const change = (key: string, v: unknown) =>
    draft.setValue({ ...draft.value, properties: { ...props, [key]: v } });
  return (
    <form
      onSubmit={async (e) => {
        e.preventDefault();
        if (await save(draft.value)) draft.saved();
      }}
    >
      <DraftRecovery
        draft={draft}
        kind="dossier"
        records={Object.fromEntries(entities.map((r) => [r.id, r]))}
        readOnly={disabled}
      />
      {!entry && draft.recovery && (
        <p>The recovered dossier will be saved as a new entity.</p>
      )}
      <fieldset disabled={disabled || !draft.ready || !!draft.recovery}>
        <legend>{definition.name} dossier</legend>
        <label>
          Domain entity name
          <input
            required
            value={name}
            onChange={(e) =>
              draft.setValue({ ...draft.value, name: e.target.value })
            }
          />
        </label>
        <label>
          Summary & sources
          <textarea
            aria-label="Summary & sources"
            value={notes || ""}
            onChange={(e) =>
              draft.setValue({ ...draft.value, notes: e.target.value })
            }
          />
        </label>
        <div className="two-col">
          {definition.fields.map((f) => (
            <div key={f.key}>
              {f.kind === "quantity" ? (
                <QuantityField
                  label={f.label}
                  value={(props[f.key] as Quantity) || { mode: "unknown" }}
                  change={(v) => change(f.key, v)}
                />
              ) : (
                <label>
                  {f.label}
                  {f.kind === "entity" ? (
                    <select
                      aria-label={f.label}
                      value={String(props[f.key] || "")}
                      onChange={(e) => change(f.key, e.target.value)}
                    >
                      <option value="">Unspecified</option>
                      {entities.map((r) => (
                        <option key={r.id} value={r.id}>
                          {r.name} · {r.type}
                        </option>
                      ))}
                    </select>
                  ) : (
                    <textarea
                      aria-label={f.label}
                      value={String(props[f.key] || "")}
                      onChange={(e) => change(f.key, e.target.value)}
                    />
                  )}
                </label>
              )}
            </div>
          ))}
        </div>
        <button className="primary">Save domain entity</button>
        <button type="button" disabled={!draft.dirty} onClick={draft.discard}>
          Discard dossier changes
        </button>
      </fieldset>
    </form>
  );
}
export function QuantityField({
  label,
  value,
  change,
}: {
  label: string;
  value: Quantity;
  change: (q: Quantity) => void;
}) {
  return (
    <fieldset className="quantity">
      <legend>{label}</legend>
      <label>
        {label} mode
        <select
          value={value.mode}
          onChange={(e) => change({ ...value, mode: e.target.value })}
        >
          {["unknown", "qualitative", "exact", "range"].map((v) => (
            <option key={v}>{v}</option>
          ))}
        </select>
      </label>
      {value.mode === "qualitative" ? (
        <label>
          {label} description
          <input
            value={value.text || ""}
            onChange={(e) => change({ ...value, text: e.target.value })}
          />
        </label>
      ) : (
        value.mode !== "unknown" && (
          <>
            {(value.mode === "range" ? ["min", "max"] : ["value"]).map(
              (key) => (
                <label key={key}>
                  {label} {key}
                  <input
                    inputMode="decimal"
                    value={String(value[key as keyof Quantity] || "")}
                    onChange={(e) =>
                      change({ ...value, [key]: e.target.value })
                    }
                  />
                </label>
              ),
            )}
            <label>
              {label} unit
              <input
                value={value.unit || ""}
                onChange={(e) => change({ ...value, unit: e.target.value })}
              />
            </label>
          </>
        )
      )}
    </fieldset>
  );
}
function Connections({
  entry,
  records,
  state,
  run,
  disabled,
  select,
}: {
  entry: Entry;
  records: Entry[];
  state: State;
  run: Props["run"];
  disabled: boolean;
  select: (id: string) => void;
}) {
  const [to, setTo] = useState(""),
    [role, setRole] = useState("associated with"),
    [disputed, setDisputed] = useState(false);
  const links = records.filter(
      (r) =>
        r.kind === "relation" && (r.from === entry.id || r.to === entry.id),
    ),
    nodes = [
      entry,
      ...records.filter(
        (r) =>
          r.kind === "entity" &&
          r.id !== entry.id &&
          links.some((l) => l.from === r.id || l.to === r.id),
      ),
    ];
  const works = records.filter(
    (r) => r.kind === "scene" && (r.references || []).includes(entry.id),
  );
  return (
    <section className="connections">
      <h3>Connected world</h3>
      <p className="muted">
        Links belong to {state.age.name}. Parent roles may be biological,
        adopted, guardian, ritual, disputed or custom; divine cycles are
        allowed.
      </p>
      <svg
        className="relation-graph"
        viewBox="0 0 600 350"
        role="img"
        aria-label={`Connections of ${entry.name}`}
      >
        <circle cx="300" cy="170" r="40" fill="#d9c9a0" />
        <text x="300" y="175" textAnchor="middle">
          {entry.name.slice(0, 20)}
        </text>
        {nodes.slice(1, 13).map((r, i) => {
          const angle = (2 * Math.PI * i) / Math.min(nodes.length - 1, 12),
            x = 300 + 230 * Math.cos(angle),
            y = 170 + 130 * Math.sin(angle);
          return (
            <g key={r.id}>
              <line x1="300" y1="170" x2={x} y2={y} stroke="#779181" />
              <circle cx={x} cy={y} r="26" fill="#d3e1d4" />
              <text x={x} y={y + 5} textAnchor="middle" fontSize="12">
                {r.name.slice(0, 16)}
              </text>
            </g>
          );
        })}
      </svg>
      <ul>
        {links.map((l) => (
          <li key={l.id}>
            <button
              className="text-button"
              onClick={() => select(l.from === entry.id ? l.to! : l.from!)}
            >
              {records.find((r) => r.id === l.from)?.name} → {l.name} →{" "}
              {records.find((r) => r.id === l.to)?.name}
            </button>
            {l.properties?.disputed ? " (disputed)" : ""}
          </li>
        ))}
      </ul>
      <form
        className="map-tools"
        onSubmit={(e) => {
          e.preventDefault();
          void run({
            action: "put",
            record: {
              id: newID(),
              kind: "relation",
              name: role,
              from: entry.id,
              to,
              properties: { disputed },
            },
          });
        }}
      >
        <label>
          Relationship role
          <input
            value={role}
            onChange={(e) => setRole(e.target.value)}
            list="relation-roles"
          />
        </label>
        <datalist id="relation-roles">
          {[
            "biological parent of",
            "adoptive parent of",
            "guardian of",
            "partner of",
            "member of",
            "governed by",
            "speaks",
            "practices",
            "supplies",
            "depends on",
            "inhabits",
            "claims",
            "worships",
          ].map((r) => (
            <option key={r}>{r}</option>
          ))}
        </datalist>
        <label>
          Connection target
          <select required value={to} onChange={(e) => setTo(e.target.value)}>
            <option value="">Choose entity</option>
            {records
              .filter((r) => r.kind === "entity")
              .map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name} · {r.type}
                </option>
              ))}
          </select>
        </label>
        <label className="check-label">
          <input
            type="checkbox"
            checked={disputed}
            onChange={(e) => setDisputed(e.target.checked)}
          />
          Disputed account
        </label>
        <button disabled={disabled || !to}>Add connection</button>
      </form>
      <h4>Manuscript appearances</h4>
      {works.length ? (
        works.map((r) => (
          <p key={r.id}>
            {r.name} · setting {state.root.ages[r.settingAge!]?.name}{" "}
            {r.settingDate ? `/ tick ${r.settingDate}` : "/ overview"}
          </p>
        ))
      ) : (
        <p className="muted">No declared scene references in this Age.</p>
      )}
    </section>
  );
}
