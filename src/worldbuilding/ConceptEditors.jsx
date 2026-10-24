import { activeEntities, registry, label } from "./model.js";
import { matches } from "./connections.js";
import { culturalKinds, componentKinds } from "./concepts.js";
export const fieldTypes = [
  "text",
  "long-text",
  "number",
  "boolean",
  "date",
  "age",
  "select",
  "multi-select",
  "entity",
  "section",
];
export const truthStates = [
  "",
  "true",
  "false",
  "disputed",
  "unknown",
  "undecided",
  "not-applicable",
];
const stances = [
  "affirmed",
  "rejected",
  "disputed",
  "metaphorical",
  "uncertain",
];
export function ClaimEditor({ value, onChange }) {
  const v = value || { statement: "", worldTruth: "" };
  return (
    <div className="claim-editor">
      <label>
        Claim / statement
        <textarea
          required
          value={v.statement}
          onChange={(e) => onChange({ ...v, statement: e.target.value })}
        />
      </label>
      <details>
        <summary>Structured claim & interpretation (optional)</summary>
        {["subject", "predicate", "object", "interpretation"].map((k) => (
          <label key={k}>
            {label(k)}
            <input
              type="text"
              value={v[k] || ""}
              onChange={(e) => onChange({ ...v, [k]: e.target.value })}
            />
          </label>
        ))}
      </details>
      <label>
        Actual truth in the fictional world
        <select
          value={v.worldTruth || ""}
          onChange={(e) => onChange({ ...v, worldTruth: e.target.value })}
        >
          {truthStates.map((t) => (
            <option key={t} value={t}>
              {t ? label(t) : "Not established by the author"}
            </option>
          ))}
        </select>
      </label>
      <p className="muted">
        This does not say who accepts the claim. Record acceptance, rejection
        and authority on the relevant relationships or contextual variations.
      </p>
    </div>
  );
}
export function ClaimView({ claim }) {
  return (
    claim && (
      <div className="concept-claim">
        <blockquote>{claim.statement}</blockquote>
        {(claim.subject || claim.predicate || claim.object) && (
          <p>
            {[claim.subject, claim.predicate, claim.object]
              .filter(Boolean)
              .join(" · ")}
          </p>
        )}
        {claim.interpretation && <p>{claim.interpretation}</p>}
        <small>
          World truth:{" "}
          {claim.worldTruth ? label(claim.worldTruth) : "not established"}
        </small>
      </div>
    )
  );
}
function DefinitionEditor({ value, onChange, depth = 0 }) {
  return (
    <div className="field-definition">
      <label>
        Field name
        <input
          type="text"
          required
          maxLength={200}
          value={value.name}
          onChange={(e) => onChange({ ...value, name: e.target.value })}
        />
      </label>
      <label>
        Type
        <select
          value={value.type}
          onChange={(e) =>
            onChange({
              ...value,
              type: e.target.value,
              options: [],
              fields: [],
            })
          }
        >
          {fieldTypes
            .filter((t) => depth < 3 || t !== "section")
            .map((t) => (
              <option key={t} value={t}>
                {label(t)}
              </option>
            ))}
        </select>
      </label>
      {["select", "multi-select"].includes(value.type) && (
        <label>
          Options, one per line
          <textarea
            required
            value={(value.options || []).join("\n")}
            onChange={(e) =>
              onChange({ ...value, options: e.target.value.split("\n") })
            }
          />
        </label>
      )}
      {value.type === "section" && (
        <div>
          {(value.fields || []).map((f, i) => (
            <div className="repeat-field" key={f.id}>
              <DefinitionEditor
                depth={depth + 1}
                value={f}
                onChange={(next) =>
                  onChange({
                    ...value,
                    fields: value.fields.map((x, j) => (j === i ? next : x)),
                  })
                }
              />
              <button
                type="button"
                onClick={() =>
                  onChange({
                    ...value,
                    fields: value.fields.filter((_, j) => i !== j),
                  })
                }
              >
                Remove column
              </button>
            </div>
          ))}
          <button
            type="button"
            onClick={() =>
              onChange({
                ...value,
                fields: [
                  ...(value.fields || []),
                  { id: crypto.randomUUID(), name: "", type: "text" },
                ],
              })
            }
          >
            Add typed column
          </button>
        </div>
      )}
    </div>
  );
}
export function FieldValue({ definition: d, value, onChange, doc, age }) {
  if (d.type === "section")
    return (
      <div className="repeat-values">
        {(Array.isArray(value) ? value : []).map((row, i) => (
          <div className="repeat-row" key={i}>
            {(d.fields || []).map((f) => (
              <label key={f.id}>
                {f.name}
                <FieldValue
                  definition={f}
                  value={row[f.id]}
                  onChange={(v) =>
                    onChange(
                      value.map((r, j) => (i === j ? { ...r, [f.id]: v } : r)),
                    )
                  }
                  doc={doc}
                  age={age}
                />
              </label>
            ))}
            <button
              type="button"
              onClick={() => onChange(value.filter((_, j) => i !== j))}
            >
              Remove row
            </button>
          </div>
        ))}
        <button
          type="button"
          onClick={() => onChange([...(Array.isArray(value) ? value : []), {}])}
        >
          Add row
        </button>
      </div>
    );
  if (d.type === "boolean")
    return (
      <select
        value={value == null ? "" : String(value)}
        onChange={(e) =>
          onChange(e.target.value === "" ? null : e.target.value === "true")
        }
      >
        <option value="">Unset</option>
        <option value="true">Yes</option>
        <option value="false">No</option>
      </select>
    );
  if (d.type === "multi-select")
    return (
      <div>
        {(d.options || []).map((o, i) => (
          <label className="inline-check" key={i}>
            <input
              type="checkbox"
              checked={Array.isArray(value) && value.includes(o)}
              onChange={(e) =>
                onChange(
                  e.target.checked
                    ? [...(value || []), o]
                    : (value || []).filter((v) => v !== o),
                )
              }
            />
            {o}
          </label>
        ))}
      </div>
    );
  if (["select", "entity", "age"].includes(d.type)) {
    const options =
      d.type === "select"
        ? (d.options || []).map((v) => ({ id: v, name: v }))
        : d.type === "age"
          ? doc.ages
          : activeEntities(doc, age);
    return (
      <select
        value={value || ""}
        onChange={(e) => onChange(e.target.value || null)}
      >
        <option value="">Unset</option>
        {options.map((o, i) => (
          <option key={o.id || i} value={o.id}>
            {o.name}
          </option>
        ))}
      </select>
    );
  }
  if (d.type === "long-text")
    return (
      <textarea
        value={value || ""}
        onChange={(e) => onChange(e.target.value)}
      />
    );
  return (
    <input
      type={["number", "date"].includes(d.type) ? "number" : "text"}
      step={d.type === "date" ? "1" : "any"}
      placeholder={d.type === "date" ? "Whole year" : ""}
      value={value ?? ""}
      onChange={(e) =>
        onChange(
          e.target.value === ""
            ? null
            : ["number", "date"].includes(d.type)
              ? Number(e.target.value)
              : e.target.value,
        )
      }
    />
  );
}
export function FieldsEditor({ fields = [], onChange, doc, age }) {
  return (
    <div className="typed-fields">
      {fields.map((f, i) => (
        <section key={f.definition.id}>
          <details open>
            <summary>
              {f.definition.name || "Define custom field"} ·{" "}
              {label(f.definition.type)}
            </summary>
            <DefinitionEditor
              value={f.definition}
              onChange={(definition) =>
                onChange(
                  fields.map((x, j) =>
                    i === j
                      ? {
                          definition,
                          value:
                            definition.type !== x.definition.type
                              ? null
                              : x.value,
                        }
                      : x,
                  ),
                )
              }
            />
          </details>
          <label>
            {f.definition.name || "Value"}
            <FieldValue
              definition={f.definition}
              value={f.value}
              onChange={(value) =>
                onChange(fields.map((x, j) => (i === j ? { ...x, value } : x)))
              }
              doc={doc}
              age={age}
            />
          </label>
          <button
            type="button"
            onClick={() => onChange(fields.filter((_, j) => i !== j))}
          >
            Remove field
          </button>
        </section>
      ))}
      <button
        type="button"
        onClick={() =>
          onChange([
            ...fields,
            {
              definition: { id: crypto.randomUUID(), name: "", type: "text" },
              value: null,
            },
          ])
        }
      >
        Add typed field
      </button>
      <label>
        Reuse definitions from another concept
        <select
          value=""
          onChange={(e) => {
            const source = doc.entities.find(
              (x) => x.ageId === age && x.id === e.target.value,
            );
            if (source)
              onChange([
                ...fields,
                ...source.content.fields.map((f) => ({
                  definition: {
                    ...structuredClone(f.definition),
                    id: crypto.randomUUID(),
                  },
                  value: null,
                })),
              ]);
          }}
        >
          <option value="">Choose a field template</option>
          {activeEntities(doc, age)
            .filter((e) => e.content?.fields?.length)
            .map((e) => (
              <option key={e.id} value={e.id}>
                {e.name}
              </option>
            ))}
        </select>
      </label>
    </div>
  );
}
export function FieldsView({ fields = [], doc, age, onProfile }) {
  function display(d, v) {
    if (v == null) return <span className="muted">Not specified</span>;
    if (d.type === "entity") {
      const e = doc.entities.find((e) => e.id === v && e.ageId === age);
      return (
        <button type="button" onClick={() => e && onProfile(e)}>
          {e?.name || v}
        </button>
      );
    }
    if (d.type === "age") return doc.ages.find((a) => a.id === v)?.name || v;
    if (d.type === "section")
      return (
        <ul>
          {v.map((row, i) => (
            <li key={i}>
              {d.fields.map((f) => (
                <div key={f.id}>
                  <strong>{f.name}: </strong>
                  {display(f, row[f.id])}
                </div>
              ))}
            </li>
          ))}
        </ul>
      );
    return typeof v === "boolean"
      ? v
        ? "Yes"
        : "No"
      : Array.isArray(v)
        ? v.join(", ")
        : String(v);
  }
  return (
    <dl className="concept-fields">
      {fields.map((f) => (
        <div key={f.definition.id}>
          <dt>{f.definition.name}</dt>
          <dd>{display(f.definition, f.value)}</dd>
        </div>
      ))}
    </dl>
  );
}
export function VariationsEditor({ value = [], onChange, doc, age }) {
  const all = activeEntities(doc, age);
  return (
    <div>
      {value.map((v, i) => (
        <section className="context-variation" key={v.id}>
          <label>
            Context record
            <select
              required
              value={v.contextId}
              onChange={(e) =>
                onChange(
                  value.map((x, j) =>
                    i === j ? { ...x, contextId: e.target.value } : x,
                  ),
                )
              }
            >
              <option value="">Choose region, group, branch or person</option>
              {all.map((e) => (
                <option key={e.id} value={e.id}>
                  {e.name} · {label(e.kind)}
                </option>
              ))}
            </select>
          </label>
          {["description", "stance", "prevalence", "interpretation"].map(
            (k) => (
              <label key={k}>
                {label(k)}
                <input
                  type="text"
                  list={k === "stance" ? "claim-stances" : undefined}
                  value={v[k] || ""}
                  onChange={(e) =>
                    onChange(
                      value.map((x, j) =>
                        i === j ? { ...x, [k]: e.target.value } : x,
                      ),
                    )
                  }
                />
              </label>
            ),
          )}
          <button
            type="button"
            onClick={() => onChange(value.filter((_, j) => i !== j))}
          >
            Remove variation
          </button>
        </section>
      ))}
      <datalist id="claim-stances">
        {stances.map((s) => (
          <option key={s}>{s}</option>
        ))}
      </datalist>
      <button
        type="button"
        onClick={() =>
          onChange([
            ...value,
            { id: crypto.randomUUID(), contextId: "", description: "" },
          ])
        }
      >
        Add contextual variation
      </button>
    </div>
  );
}
export function VariationsView({ value = [], doc, age, onProfile }) {
  return value.map((v) => {
    const e = doc.entities.find((e) => e.ageId === age && e.id === v.contextId);
    return (
      <aside className="concept-variation" key={v.id}>
        <button onClick={() => e && onProfile(e)}>
          {e?.name || "Unknown context"}
        </button>
        <span>
          {v.stance} {v.prevalence && " · " + v.prevalence}
        </span>
        <p>{v.description}</p>
        {v.interpretation && <p>{v.interpretation}</p>}
      </aside>
    );
  });
}
export function ComponentLinksEditor({ value = [], onChange, type, doc, age }) {
  const all = activeEntities(doc, age),
    kinds = culturalKinds.filter((k) =>
      matches({ kind: componentKinds[type] }, registry.relations[k].sources),
    ),
    edit = (i, k, v) =>
      onChange(value.map((x, j) => (i === j ? { ...x, [k]: v } : x)));
  return (
    <div>
      {value.map((v, i) => (
        <section className="context-variation" key={v.id}>
          <label>
            Relationship
            <select
              value={v.kind}
              onChange={(e) =>
                onChange(
                  value.map((x, j) =>
                    i === j ? { ...x, kind: e.target.value, targetId: "" } : x,
                  ),
                )
              }
            >
              {kinds.map((k) => (
                <option key={k} value={k}>
                  {label(k)}
                </option>
              ))}
            </select>
          </label>
          <label>
            Related record
            <select
              required
              value={v.targetId}
              onChange={(e) => edit(i, "targetId", e.target.value)}
            >
              <option value="">Choose an existing record</option>
              {all
                .filter((e) =>
                  matches(e, registry.relations[v.kind]?.destinations),
                )
                .map((e) => (
                  <option key={e.id} value={e.id}>
                    {e.name}
                  </option>
                ))}
            </select>
          </label>
          {[
            "description",
            "stance",
            "authority",
            "prevalence",
            "interpretation",
            "provenance",
            "validFrom",
            "validTo",
          ].map((k) => (
            <label key={k}>
              {label(k)}
              <input
                type="text"
                value={v[k] || ""}
                onChange={(e) => edit(i, k, e.target.value)}
              />
            </label>
          ))}
          <label>
            Context
            <select
              value={v.contextId || ""}
              onChange={(e) => edit(i, "contextId", e.target.value)}
            >
              <option value="">None</option>
              {all.map((e) => (
                <option key={e.id} value={e.id}>
                  {e.name}
                </option>
              ))}
            </select>
          </label>
          <button
            type="button"
            onClick={() => onChange(value.filter((_, j) => i !== j))}
          >
            Remove link
          </button>
        </section>
      ))}
      <button
        type="button"
        onClick={() =>
          onChange([
            ...value,
            {
              id: crypto.randomUUID(),
              kind: kinds.includes("associated-with")
                ? "associated-with"
                : kinds[0],
              targetId: "",
            },
          ])
        }
      >
        Link related content
      </button>
    </div>
  );
}
