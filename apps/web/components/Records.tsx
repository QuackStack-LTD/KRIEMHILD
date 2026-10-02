"use client";
import { useState } from "react";
import { Command, Entry, Field, newID, State } from "../lib/types";
import { useRecordDraft } from "../lib/useRecordDraft";
import { useDraftGuard } from "../lib/useDraftGuard";
import DraftRecovery from "./DraftRecovery";
type Props = {
  state: State;
  selected: string;
  select: (id: string) => void;
  run: (c: Command) => Promise<boolean>;
  busy: boolean;
  onPending?: (pending: boolean) => void;
};

export function Entities({
  state,
  selected,
  select,
  run,
  busy,
  onPending,
}: Props) {
  const [entityPending, setEntityPending] = useState(false);
  const [relationshipPending, setRelationshipPending] = useState(false);
  const pending = entityPending || relationshipPending;
  useDraftGuard(pending, onPending);
  const [filter, setFilter] = useState("");
  const [creating, setCreating] = useState(false);
  const records = Object.values(state.records),
    entities = records.filter((r) => r.kind === "entity"),
    schemas = records.filter((r) => r.kind === "schema");
  const selectedEntry = state.records[selected];
  const entry = selectedEntry?.kind === "entity" ? selectedEntry : undefined;
  return (
    <div className="split">
      <section className="list-pane">
        <div className="section-heading">
          <h2>
            Entities <span>{entities.length}</span>
          </h2>
          <button
            disabled={busy || pending || state.readOnly}
            onClick={() => {
              setCreating(true);
              select("");
            }}
          >
            + New entity
          </button>
        </div>
        <input
          aria-label="Filter entities"
          placeholder="Filter by name or type…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        <div className="record-list">
          {entities
            .filter((r) =>
              (r.name + " " + r.type)
                .toLowerCase()
                .includes(filter.toLowerCase()),
            )
            .sort((a, b) => a.name.localeCompare(b.name))
            .map((r) => (
              <button
                className={selected === r.id ? "record active" : "record"}
                key={r.id}
                disabled={pending}
                onClick={() => {
                  setCreating(false);
                  select(r.id);
                }}
              >
                <span className="entity-mark">{r.name.charAt(0)}</span>
                <span>
                  <strong>{r.name}</strong>
                  <small>
                    {r.type} · {r.status || "active"}
                  </small>
                </span>
              </button>
            ))}
        </div>
        {!entities.length && (
          <p className="muted">
            Begin with a person, village or any idea worth keeping.
          </p>
        )}
      </section>
      <section className="detail-pane">
        {creating ? (
          <EntityForm
            key="new"
            world={state.root.world.id}
            age={state.age.id}
            onPending={setEntityPending}
            schemas={schemas}
            entities={entities}
            busy={busy || !!state.readOnly}
            onSave={async (r) => {
              if (await run({ action: "put", record: r })) {
                setCreating(false);
                select(r.id);
                return true;
              }
              return false;
            }}
          />
        ) : entry ? (
          <>
            <EntityForm
              key={JSON.stringify(entry)}
              initial={entry}
              world={state.root.world.id}
              age={state.age.id}
              onPending={setEntityPending}
              schemas={schemas}
              entities={entities}
              busy={busy || !!state.readOnly || relationshipPending}
              onSave={(r) => run({ action: "put", record: r })}
            />
            <Relationships
              key={entry.id + state.age.snapshot}
              state={state}
              entity={entry}
              run={run}
              busy={busy || !!state.readOnly || entityPending}
              onPending={setRelationshipPending}
            />
            <details>
              <summary>Record identity and removal</summary>
              <p className="mono">{entry.id}</p>
              <p>
                Removing a record is different from marking it destroyed. Linked
                records must be unlinked first.
              </p>
              <button
                className="danger"
                disabled={busy || pending || state.readOnly}
                onClick={() => {
                  if (
                    window.confirm(
                      `Remove ${entry.name} from this Age? You can undo this.`,
                    )
                  )
                    void run({ action: "delete", id: entry.id });
                }}
              >
                Remove from this Age
              </button>
            </details>
          </>
        ) : (
          <Empty
            title="Every world starts somewhere."
            text="Create an entity, then give it a place, relationships and a history."
          />
        )}
      </section>
    </div>
  );
}
function EntityForm({
  initial,
  schemas,
  entities,
  busy,
  onSave,
  world,
  age,
  onPending,
}: {
  initial?: Entry;
  schemas: Entry[];
  entities: Entry[];
  busy: boolean;
  onSave: (r: Entry) => Promise<boolean>;
  world: string;
  age: string;
  onPending: (pending: boolean) => void;
}) {
  const [saved] = useState<Entry>(
    initial || {
      id: newID(),
      kind: "entity",
      name: "",
      type: "Settlement",
      status: "active",
      properties: {},
    },
  );
  const draft = useRecordDraft(
    `kriemhild-entity:${world}:${age}:${initial?.id || "new"}`,
    saved,
    !initial,
  );
  const { value: r, setValue: setR } = draft;
  const [emptyProperty] = useState<Entry>({
    id: newID(),
    kind: "note",
    name: "",
    notes: "",
  });
  const propertyDraft = useRecordDraft(
    `kriemhild-property:${world}:${age}:${initial?.id || "new"}`,
    emptyProperty,
    true,
  );
  const customKey = propertyDraft.value.name,
    customValue = propertyDraft.value.notes || "";
  const setCustomKey = (name: string) =>
    propertyDraft.setValue({ ...propertyDraft.value, name });
  const setCustomValue = (notes: string) =>
    propertyDraft.setValue({ ...propertyDraft.value, notes });
  useDraftGuard(draft.pending || propertyDraft.pending, onPending);
  const fields = schemas.find((s) => s.name === r.type)?.fields || [];
  const property = (key: string, value: unknown) =>
    setR({ ...r, properties: { ...r.properties, [key]: value } });
  return (
    <form
      onSubmit={async (e) => {
        e.preventDefault();
        if (await onSave(r)) draft.saved();
      }}
    >
      <DraftRecovery
        draft={draft}
        kind="entity"
        records={Object.fromEntries(entities.map((r) => [r.id, r]))}
        readOnly={busy}
      />
      <DraftRecovery draft={propertyDraft} kind="property" readOnly={busy} />
      <fieldset
        disabled={
          busy ||
          !draft.ready ||
          !!draft.recovery ||
          !propertyDraft.ready ||
          !!propertyDraft.recovery
        }
      >
        <legend>{initial ? "Edit entity" : "New entity"}</legend>
        <div className="eyebrow">
          {initial ? "Entity dossier" : "Create an entity"}
        </div>
        <label>
          Name
          <input
            required
            maxLength={300}
            value={r.name}
            onChange={(e) => setR({ ...r, name: e.target.value })}
          />
        </label>
        <div className="two-col">
          <label>
            Entity type
            <input
              required
              list="entity-types"
              aria-label="Entity type"
              value={r.type}
              onChange={(e) => setR({ ...r, type: e.target.value })}
            />
            <datalist id="entity-types">
              {Array.from(
                new Set([
                  "Settlement",
                  "Person",
                  "Culture",
                  "Organization",
                  "Artifact",
                  "Species",
                  ...schemas.map((s) => s.name),
                  ...entities.map((e) => e.type || ""),
                ]),
              ).map((t) => (
                <option key={t}>{t}</option>
              ))}
            </datalist>
          </label>
          <label>
            Lifecycle
            <select
              aria-label="Lifecycle"
              value={r.status}
              onChange={(e) => setR({ ...r, status: e.target.value })}
            >
              <option value="active">Active</option>
              <option value="destroyed">Destroyed / no longer exists</option>
              <option value="unknown">Unknown</option>
            </select>
          </label>
        </div>
        <label>
          Description and notes
          <textarea
            aria-label="Description and notes"
            rows={6}
            value={r.notes || ""}
            onChange={(e) => setR({ ...r, notes: e.target.value })}
          />
        </label>
        <h3>Properties</h3>
        {fields.map((f) => (
          <label key={f.key}>
            {f.key}
            {f.type === "boolean" ? (
              <select
                value={
                  r.properties?.[f.key] === undefined
                    ? ""
                    : String(r.properties[f.key])
                }
                onChange={(e) =>
                  property(
                    f.key,
                    e.target.value === "" ? null : e.target.value === "true",
                  )
                }
              >
                <option value="">Unknown</option>
                <option value="true">Yes</option>
                <option value="false">No</option>
              </select>
            ) : f.type === "entity" ? (
              <select
                value={String(r.properties?.[f.key] || "")}
                onChange={(e) => property(f.key, e.target.value)}
              >
                <option value="">Not specified</option>
                {entities.map((en) => (
                  <option key={en.id} value={en.id}>
                    {en.name}
                  </option>
                ))}
              </select>
            ) : (
              <input
                type={f.type === "number" ? "number" : "text"}
                step="any"
                value={String(r.properties?.[f.key] ?? "")}
                onChange={(e) =>
                  property(
                    f.key,
                    f.type === "number" && e.target.value !== ""
                      ? Number(e.target.value)
                      : e.target.value,
                  )
                }
              />
            )}
          </label>
        ))}
        {Object.entries(r.properties || {})
          .filter(([key]) => !fields.some((f) => f.key === key))
          .filter(([key]) => !key.startsWith("_"))
          .map(([key, value]) => (
            <label key={key}>
              {key}
              <div className="inline">
                <input
                  readOnly={typeof value === "object" && value !== null}
                  value={
                    typeof value === "object"
                      ? JSON.stringify(value)
                      : String(value ?? "")
                  }
                  onChange={(e) => property(key, e.target.value)}
                />
                <button
                  type="button"
                  aria-label={`Remove property ${key}`}
                  onClick={() => {
                    const properties = { ...r.properties };
                    delete properties[key];
                    setR({ ...r, properties });
                  }}
                >
                  ×
                </button>
              </div>
            </label>
          ))}
        <div className="inline">
          <input
            aria-label="New property name"
            placeholder="Property name"
            value={customKey}
            onChange={(e) => setCustomKey(e.target.value)}
          />
          <input
            aria-label="New property value"
            placeholder="Value"
            value={customValue}
            onChange={(e) => setCustomValue(e.target.value)}
          />
          <button
            type="button"
            disabled={!customKey.trim() || customKey.trim().startsWith("_")}
            onClick={() => {
              property(customKey.trim(), customValue);
              propertyDraft.discard();
            }}
          >
            Add property
          </button>
        </div>
        <div className="form-actions">
          <button
            className="primary"
            disabled={busy || !!customKey || !!customValue}
            type="submit"
          >
            {initial ? "Save entity" : "Create entity"}
          </button>
          <small>Saved only in the selected Age.</small>
        </div>
        {(customKey || customValue) && (
          <p className="notice">
            Add the property before saving, or clear its name and value.
          </p>
        )}
      </fieldset>
      {(draft.dirty || customKey || customValue) && (
        <button
          type="button"
          disabled={busy}
          onClick={() => {
            draft.discard();
            propertyDraft.discard();
          }}
        >
          Discard entity changes
        </button>
      )}
    </form>
  );
}
function Relationships({
  state,
  entity,
  run,
  busy,
  onPending,
}: {
  state: State;
  entity: Entry;
  run: Props["run"];
  busy: boolean;
  onPending: (pending: boolean) => void;
}) {
  const [saved] = useState<Entry>({
    id: newID(),
    kind: "relation",
    name: "",
    from: entity.id,
    to: "",
  });
  const draft = useRecordDraft(
    `kriemhild-relationship:${state.root.world.id}:${state.age.id}:${entity.id}`,
    saved,
    true,
  );
  const { name, to } = draft.value;
  useDraftGuard(draft.pending, onPending);
  const records = Object.values(state.records),
    relations = records.filter(
      (r) =>
        r.kind === "relation" && (r.from === entity.id || r.to === entity.id),
    );
  return (
    <section className="subsection">
      <h3>Relationships</h3>
      {relations.map((r) => (
        <div className="relationship" key={r.id}>
          <span>
            {state.records[r.from!]?.name} <b>{r.name}</b>{" "}
            {state.records[r.to!]?.name}
          </span>
          <button
            disabled={busy || draft.pending}
            aria-label={`Remove relationship ${r.name}`}
            onClick={() => void run({ action: "delete", id: r.id })}
          >
            ×
          </button>
        </div>
      ))}
      <form
        className="inline"
        onSubmit={async (e) => {
          e.preventDefault();
          if (
            await run({
              action: "put",
              record: {
                ...draft.value,
              },
            })
          ) {
            draft.saved();
          }
        }}
      >
        <DraftRecovery
          draft={draft}
          kind="relationship"
          records={state.records}
          readOnly={busy}
        />
        <fieldset disabled={busy || !draft.ready || !!draft.recovery}>
          <legend>New relationship</legend>
          <input
            required
            aria-label="Relationship name"
            placeholder="e.g. founded, rules, trades with"
            value={name}
            onChange={(e) =>
              draft.setValue({ ...draft.value, name: e.target.value })
            }
          />
          <select
            required
            aria-label="Related entity"
            value={to}
            onChange={(e) =>
              draft.setValue({ ...draft.value, to: e.target.value })
            }
          >
            <option value="">Choose an entity</option>
            {records
              .filter((r) => r.kind === "entity")
              .map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))}
          </select>
          <button disabled={busy}>Link</button>
        </fieldset>
        {draft.dirty && (
          <button type="button" onClick={draft.discard}>
            Discard relationship changes
          </button>
        )}
      </form>
    </section>
  );
}
export function Notebook({
  state,
  run,
  busy,
  initialSelection,
  onPending,
}: Pick<Props, "state" | "run" | "busy" | "onPending"> & {
  initialSelection?: string;
}) {
  const [pending, setPending] = useState(false);
  useDraftGuard(pending, onPending);
  const [selected, setSelected] = useState(
    state.records[initialSelection || ""]?.kind === "note"
      ? initialSelection!
      : "",
  );
  const notes = Object.values(state.records).filter((r) => r.kind === "note");
  return (
    <div className="split">
      <section className="list-pane">
        <h2>Research & notes</h2>
        <button
          disabled={busy || pending || state.readOnly}
          onClick={() => setSelected("new")}
        >
          + New note
        </button>
        {notes.map((r) => (
          <button
            className="record"
            key={r.id}
            disabled={pending}
            onClick={() => setSelected(r.id)}
          >
            {r.name}
          </button>
        ))}
      </section>
      <section className="detail-pane">
        {selected ? (
          <NoteForm
            key={selected + JSON.stringify(state.records[selected])}
            initial={state.records[selected]}
            world={state.root.world.id}
            age={state.age.id}
            onPending={setPending}
            busy={busy || !!state.readOnly}
            save={async (r) => {
              if (await run({ action: "put", record: r })) {
                setSelected(r.id);
                return true;
              }
              return false;
            }}
          />
        ) : (
          <Empty
            title="A place for unfinished ideas."
            text="Keep research, questions and fragments alongside your world."
          />
        )}
      </section>
    </div>
  );
}
function NoteForm({
  initial,
  busy,
  save,
  world,
  age,
  onPending,
}: {
  initial?: Entry;
  busy: boolean;
  save: (r: Entry) => Promise<boolean>;
  world: string;
  age: string;
  onPending: (pending: boolean) => void;
}) {
  const [saved] = useState<Entry>(
    initial || { id: newID(), kind: "note", name: "", notes: "" },
  );
  const draft = useRecordDraft(
    `kriemhild-note:${world}:${age}:${initial?.id || "new"}`,
    saved,
    !initial,
  );
  const { value: r, setValue: setR } = draft;
  useDraftGuard(draft.pending, onPending);
  return (
    <form
      onSubmit={async (e) => {
        e.preventDefault();
        if (await save(r)) draft.saved();
      }}
    >
      <DraftRecovery draft={draft} kind="note" readOnly={busy} />
      <fieldset disabled={busy || !draft.ready || !!draft.recovery}>
        <legend>{initial ? "Edit note" : "New note"}</legend>
        <label>
          Note title
          <input
            required
            value={r.name}
            onChange={(e) => setR({ ...r, name: e.target.value })}
          />
        </label>
        <label>
          Notes
          <textarea
            aria-label="Notes"
            rows={16}
            value={r.notes}
            onChange={(e) => setR({ ...r, notes: e.target.value })}
          />
        </label>
        <button className="primary" disabled={busy}>
          Save note
        </button>
      </fieldset>
      {draft.dirty && (
        <button type="button" disabled={busy} onClick={draft.discard}>
          Discard note changes
        </button>
      )}
    </form>
  );
}
export function Types({
  state,
  run,
  busy,
  onPending,
  initialSelection,
}: Pick<Props, "state" | "run" | "busy" | "onPending"> & {
  initialSelection?: string;
}) {
  const [editing, setEditing] = useState(
    state.records[initialSelection || ""]?.kind === "schema"
      ? initialSelection!
      : "",
  );
  const [pending, setPending] = useState(false);
  useDraftGuard(pending, onPending);
  const schemas = Object.values(state.records).filter(
    (r) => r.kind === "schema",
  );
  return (
    <div className="split">
      <section className="list-pane">
        <h2>Custom entity types</h2>
        <p className="muted">
          Define reusable fields for the things in your world. Definitions are
          copied with each Age.
        </p>
        <button
          disabled={busy || pending || state.readOnly}
          onClick={() => setEditing("")}
        >
          + New type
        </button>
        {schemas.map((r) => (
          <button
            className="record"
            key={r.id}
            disabled={pending}
            onClick={() => setEditing(r.id)}
          >
            {r.name} <small>{r.fields?.length || 0} fields</small>
          </button>
        ))}
      </section>
      <section className="detail-pane">
        <TypeForm
          key={editing + JSON.stringify(state.records[editing])}
          initial={state.records[editing]}
          affected={
            Object.values(state.records).filter(
              (r) =>
                r.kind === "entity" && r.type === state.records[editing]?.name,
            ).length
          }
          world={state.root.world.id}
          age={state.age.id}
          busy={busy || !!state.readOnly}
          onPending={setPending}
          save={async (r) => {
            if (await run({ action: "update-schema", record: r })) {
              setEditing(r.id);
              return true;
            }
            return false;
          }}
        />
      </section>
    </div>
  );
}
function TypeForm({
  initial,
  world,
  age,
  busy,
  onPending,
  save,
  affected,
}: {
  initial?: Entry;
  world: string;
  age: string;
  busy: boolean;
  onPending: (pending: boolean) => void;
  save: (r: Entry) => Promise<boolean>;
  affected: number;
}) {
  const [saved] = useState<Entry>(
    initial || { id: newID(), kind: "schema", name: "", fields: [] },
  );
  const draft = useRecordDraft(
    `kriemhild-type:${world}:${age}:${initial?.id || "new"}`,
    saved,
    !initial,
  );
  useDraftGuard(draft.pending, onPending);
  const r = draft.value,
    fields = r.fields || [];
  const setFields = (fields: Field[]) => draft.setValue({ ...r, fields });
  return (
    <form
      onSubmit={async (e) => {
        e.preventDefault();
        if (await save(r)) draft.saved();
      }}
    >
      <h2>{initial ? "Edit type" : "Define a type"}</h2>
      {initial && initial.name !== r.name && (
        <p className="notice">
          Saving this rename updates the type of {affected} existing entities in
          this Age together with the definition. Earlier Ages keep their own
          definitions and names.
        </p>
      )}
      <DraftRecovery draft={draft} kind="type" readOnly={busy} />
      <fieldset disabled={busy || !draft.ready || !!draft.recovery}>
        <legend>Type definition</legend>
        <label>
          Type name
          <input
            required
            value={r.name}
            onChange={(e) => draft.setValue({ ...r, name: e.target.value })}
          />
        </label>
        {fields.map((f, i) => (
          <div className="inline field-row" key={i}>
            <input
              required
              aria-label={`Field ${i + 1} name`}
              placeholder="Field name"
              value={f.key}
              onChange={(e) =>
                setFields(
                  fields.map((v, j) =>
                    i === j ? { ...v, key: e.target.value } : v,
                  ),
                )
              }
            />
            <select
              aria-label={`Field ${i + 1} type`}
              value={f.type}
              onChange={(e) =>
                setFields(
                  fields.map((v, j) =>
                    i === j
                      ? { ...v, type: e.target.value as Field["type"] }
                      : v,
                  ),
                )
              }
            >
              <option value="text">Text</option>
              <option value="number">Number</option>
              <option value="boolean">Yes / no</option>
              <option value="entity">Entity reference</option>
            </select>
            <button
              type="button"
              onClick={() => setFields(fields.filter((_, j) => j !== i))}
            >
              Remove
            </button>
          </div>
        ))}
        <div className="form-actions">
          <button
            type="button"
            onClick={() => setFields([...fields, { key: "", type: "text" }])}
          >
            + Add field
          </button>
          <button className="primary">Save type</button>
        </div>
        <p className="muted">
          Changing a field type validates existing entities before saving.
          Existing custom values are preserved.
        </p>
      </fieldset>
      {draft.dirty && (
        <button type="button" disabled={busy} onClick={draft.discard}>
          Discard type changes
        </button>
      )}
    </form>
  );
}
export function Empty({ title, text }: { title: string; text: string }) {
  return (
    <div className="empty">
      <span className="empty-symbol">◇</span>
      <h2>{title}</h2>
      <p>{text}</p>
    </div>
  );
}
