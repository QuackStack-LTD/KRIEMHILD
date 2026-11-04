import { useEffect, useId, useState } from "react";
import { newID } from "./languages.js";

const text = { type: "text" },
  tokens = { type: "tokens" },
  features = { type: "map" };
const array = (item, blank) => ({ type: "array", item, blank });
const object = (fields) => ({ type: "object", fields });
const feature = object({ id: text, values: tokens });
const selector = { type: "selector" };
const selectors = array(selector, () => ({ segment: "" }));
const pattern = object({ id: text, slots: selectors });
const override = object({ cell: text, ruleId: text, segments: tokens });
const rule = object({
  id: text,
  domain: { type: "choice", options: ["word"] },
  input: selectors,
  output: tokens,
  left: selectors,
  right: selectors,
});
const boundaryRule = object({
  ...rule.fields,
  domain: { type: "choice", options: ["morpheme"] },
});
export const productiveSchema = object({
  version: { type: "choice", options: ["productive/1"] },
  features: array(feature, () => ({ id: newID(), values: [] })),
  paradigms: array(
    object({
      id: text,
      cells: array(
        object({
          id: text,
          features,
          operations: array(
            object({
              id: text,
              kind: {
                type: "choice",
                options: ["zero", "prefix", "suffix", "circumfix"],
              },
              policy: { type: "choice", options: ["first", "all"] },
              allomorphs: array(
                object({
                  id: text,
                  when: object({
                    features,
                    category: text,
                    previous: text,
                    left: selectors,
                    right: selectors,
                  }),
                  prefix: tokens,
                  suffix: tokens,
                }),
                () => ({
                  id: newID(),
                  when: {
                    features: {},
                    category: "",
                    previous: "",
                    left: [],
                    right: [],
                  },
                  prefix: [],
                  suffix: [],
                }),
              ),
            }),
            () => ({
              id: newID(),
              kind: "zero",
              policy: "first",
              allomorphs: [],
            }),
          ),
        }),
        () => ({ id: newID(), features: {}, operations: [] }),
      ),
    }),
    () => ({ id: newID(), cells: [] }),
  ),
  stratum: object({
    id: text,
    boundaryPolicy: { type: "choice", options: ["retain-until-surface"] },
    rules: array(boundaryRule, () => ({
      id: newID(),
      domain: "morpheme",
      input: [],
      output: [],
      left: [],
      right: [],
    })),
  }),
});
export const phonologySchema = object({
  features: array(feature, () => ({ id: newID(), values: [] })),
  segments: array(
    object({ id: text, label: text, phonemic: { type: "boolean" }, features }),
    () => ({ id: newID(), label: "", phonemic: true, features: {} }),
  ),
  classes: array(object({ id: text, features }), () => ({
    id: newID(),
    features: {},
  })),
  templates: array(pattern, () => ({ id: newID(), slots: [] })),
  allowed: array(pattern, () => ({ id: newID(), slots: [] })),
  forbidden: array(pattern, () => ({ id: newID(), slots: [] })),
  rules: array(rule, () => ({
    id: newID(),
    domain: "word",
    input: [],
    output: [],
    left: [],
    right: [],
  })),
});
export const lexiconSchema = array(
  object({
    id: text,
    category: text,
    senses: array(object({ id: text, meaning: text }), () => ({
      id: newID(),
      meaning: "",
    })),
    underlying: tokens,
    features,
    paradigm: text,
    overrides: array(override, () => ({
      cell: "",
      ruleId: newID(),
      segments: [],
    })),
  }),
  () => ({
    id: newID(),
    category: "",
    senses: [],
    underlying: [],
    features: {},
    paradigm: "",
    overrides: [],
  }),
);
export const morphologySchema = object({
  features: array(feature, () => ({ id: newID(), values: [] })),
  paradigms: array(
    object({
      id: text,
      cells: array(
        object({
          id: text,
          features,
          operations: array(
            object({
              id: text,
              kind: {
                type: "choice",
                options: [
                  "prefix",
                  "suffix",
                  "zero",
                  "infix",
                  "circumfix",
                  "reduplication",
                  "interdigitation",
                ],
              },
              segments: tokens,
            }),
            () => ({ id: newID(), kind: "zero", segments: [] }),
          ),
        }),
        () => ({ id: newID(), features: {}, operations: [] }),
      ),
    }),
    () => ({ id: newID(), cells: [] }),
  ),
});
export const syntaxSchema = object({
  constructions: array(
    object({
      id: text,
      meaning: text,
      roles: array(object({ id: text, category: text, features }), () => ({
        id: newID(),
        category: "",
        features: {},
      })),
      order: array(
        object({ role: text, literal: text, construction: text }),
        () => ({ role: "" }),
      ),
      agreements: array(
        object({ id: text, leftRole: text, rightRole: text, features: tokens }),
        () => ({ id: newID(), leftRole: "", rightRole: "", features: [] }),
      ),
      separator: text,
    }),
    () => ({
      id: newID(),
      meaning: "",
      roles: [],
      order: [],
      agreements: [],
      separator: "",
    }),
  ),
});
export const lexicalSchema = object({
  morphemes: array(
    object({
      id: text,
      kind: text,
      underlying: tokens,
      meanings: array(text, () => ""),
      features,
      notes: text,
    }),
    () => ({
      id: newID(),
      kind: "",
      underlying: [],
      meanings: [],
      features: {},
      notes: "",
    }),
  ),
  entries: array(
    object({ lexeme: text, morphemes: tokens, notes: text }),
    () => ({ lexeme: "", morphemes: [], notes: "" }),
  ),
  relations: array(
    object({ id: text, kind: text, source: text, target: text, notes: text }),
    () => ({ id: newID(), kind: "", source: "", target: "", notes: "" }),
  ),
  derivations: array(
    object({ id: text, kind: text, inputs: tokens, target: text, notes: text }),
    () => ({ id: newID(), kind: "", inputs: [], target: "", notes: "" }),
  ),
  attestations: array(
    object({
      id: text,
      lexeme: text,
      form: text,
      source: text,
      date: text,
      notes: text,
    }),
    () => ({
      id: newID(),
      lexeme: "",
      form: "",
      source: "",
      date: "",
      notes: "",
    }),
  ),
  sources: array(
    object({
      id: text,
      kind: text,
      ageId: text,
      languageId: text,
      revision: text,
      lexeme: text,
      target: text,
      confidence: text,
      notes: text,
    }),
    () => ({
      id: newID(),
      kind: "",
      ageId: "",
      languageId: "",
      revision: "",
      lexeme: "",
      target: "",
      confidence: "",
      notes: "",
    }),
  ),
});
export const graphemesSchema = array(
  object({ id: text, text, label: text, glyphAsset: text }),
  () => ({ id: newID(), text: "", label: "" }),
);
export const mappingsSchema = array(
  object({ id: text, segment: text, grapheme: text }),
  () => ({ id: newID(), segment: "", grapheme: "" }),
);
export const changesSchema = array(rule, () => ({
  id: newID(),
  domain: "word",
  input: [],
  output: [],
  left: [],
  right: [],
}));

const title = (s) =>
  s.replace(/([a-z])([A-Z])/g, "$1 $2").replace(/^./, (s) => s.toUpperCase());
function TokenInput({ value, onChange, id }) {
  const encoded = JSON.stringify(value || []),
    [input, setInput] = useState((value || []).join(" "));
  useEffect(() => setInput(JSON.parse(encoded).join(" ")), [encoded]);
  return (
    <input
      id={id}
      value={input}
      onChange={(e) => {
        setInput(e.target.value);
        onChange(e.target.value.split(/\s+/).filter(Boolean));
      }}
      placeholder="Space-separated stable IDs"
    />
  );
}
function MapInput({ value = {}, onChange, label }) {
  const [draft, setDraft] = useState("");
  return (
    <fieldset className="language-feature-map">
      <legend>{label}</legend>
      {Object.entries(value || {}).map(([key, val]) => (
        <label key={key}>
          {key}
          <span>
            <input
              aria-label={`${key} value`}
              value={val}
              onChange={(e) => onChange({ ...value, [key]: e.target.value })}
            />
            <button
              type="button"
              aria-label={`Remove feature ${key}`}
              onClick={() => {
                const next = { ...value };
                delete next[key];
                onChange(next);
              }}
            >
              Remove
            </button>
          </span>
        </label>
      ))}
      <div className="language-inline">
        <input
          aria-label={`New ${label.toLowerCase()} key`}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
        />
        <button
          type="button"
          disabled={!draft.trim() || Object.hasOwn(value || {}, draft.trim())}
          onClick={() => {
            onChange({ ...value, [draft.trim()]: "" });
            setDraft("");
          }}
        >
          Add feature
        </button>
      </div>
    </fieldset>
  );
}
function ArrayEditor({ schema, value = [], onChange, label, context }) {
  const [search, setSearch] = useState(""),
    [page, setPage] = useState(0),
    rows = (value || [])
      .map((item, index) => ({ item, index }))
      .filter(({ item }) =>
        JSON.stringify(item).toLowerCase().includes(search.toLowerCase()),
      ),
    pages = Math.max(1, Math.ceil(rows.length / 20)),
    current = Math.min(page, pages - 1);
  return (
    <fieldset className="language-array">
      <legend>
        {label} <small>{(value || []).length}</small>
      </legend>
      {(value || []).length > 20 && (
        <label>
          Find {label.toLowerCase()}
          <input
            type="search"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(0);
            }}
          />
        </label>
      )}
      {rows.slice(current * 20, current * 20 + 20).map(({ item, index }) => (
        <details key={index} open={(value || []).length < 4}>
          <summary>
            {item?.id || item?.ruleId || `${label} ${index + 1}`}
          </summary>
          <StructuredEditor
            schema={schema.item}
            value={item}
            onChange={(next) =>
              onChange(value.map((v, i) => (i === index ? next : v)))
            }
            label={`${label} ${index + 1}`}
            context={context}
          />
          <button
            type="button"
            className="language-remove"
            onClick={() => onChange(value.filter((_, i) => i !== index))}
          >
            Remove {label.toLowerCase()} {index + 1}
          </button>
        </details>
      ))}
      {pages > 1 && (
        <div className="language-inline">
          <button
            type="button"
            disabled={current === 0}
            onClick={() => setPage(current - 1)}
          >
            Previous {label.toLowerCase()}
          </button>
          <span>
            Page {current + 1} of {pages} · {rows.length} matching
          </span>
          <button
            type="button"
            disabled={current === pages - 1}
            onClick={() => setPage(current + 1)}
          >
            Next {label.toLowerCase()}
          </button>
        </div>
      )}
      <button
        type="button"
        disabled={(value || []).length >= 1000}
        onClick={() => {
          onChange([...(value || []), schema.blank()]);
          setSearch("");
          setPage(Math.floor((value || []).length / 20));
        }}
      >
        Add {label.toLowerCase()}
      </button>
    </fieldset>
  );
}
export function StructuredEditor({
  schema,
  value,
  onChange,
  label,
  context = {},
}) {
  const id = useId();
  if (context.readOnly)
    return (
      <fieldset disabled>
        <StructuredEditor
          schema={schema}
          value={value}
          onChange={onChange}
          label={label}
          context={{ ...context, readOnly: false }}
        />
      </fieldset>
    );
  if (schema.type === "array")
    return (
      <ArrayEditor
        schema={schema}
        value={value}
        onChange={onChange}
        label={label}
        context={context}
      />
    );
  if (schema.type === "object")
    return (
      <div className="language-object" role="group" aria-label={label}>
        {Object.entries(schema.fields).map(([key, child]) => (
          <StructuredEditor
            key={key}
            label={title(key)}
            schema={child}
            value={value?.[key]}
            onChange={(next) => onChange({ ...value, [key]: next })}
            context={context}
          />
        ))}
      </div>
    );
  if (schema.type === "map")
    return <MapInput label={label} value={value} onChange={onChange} />;
  if (schema.type === "selector") {
    const kind =
        Object.keys(value || {}).find((k) => value[k] !== undefined) ||
        "segment",
      options =
        kind === "segment"
          ? context.segments
          : kind === "class"
            ? context.classes
            : context.boundaries || [{ id: "word" }];
    return (
      <div className="language-inline">
        <label>
          Selector type
          <select
            value={kind}
            onChange={(e) => onChange({ [e.target.value]: "" })}
          >
            {["segment", "class", "boundary"].map((s) => (
              <option key={s}>{s}</option>
            ))}
          </select>
        </label>
        <label>
          Reference
          <select
            value={value?.[kind] || ""}
            onChange={(e) => onChange({ [kind]: e.target.value })}
          >
            <option value="">Choose…</option>
            {(options || []).map((v) => (
              <option key={v.id} value={v.id}>
                {v.label || v.id}
              </option>
            ))}
          </select>
        </label>
      </div>
    );
  }
  return (
    <label htmlFor={id}>
      {label}
      {schema.type === "tokens" ? (
        <TokenInput id={id} value={value} onChange={onChange} />
      ) : schema.type === "boolean" ? (
        <input
          id={id}
          type="checkbox"
          checked={!!value}
          onChange={(e) => onChange(e.target.checked)}
        />
      ) : schema.type === "choice" ? (
        <select
          id={id}
          value={value || ""}
          onChange={(e) => onChange(e.target.value)}
        >
          <option value="">Undecided</option>
          {schema.options.map((s) => (
            <option key={s}>{s}</option>
          ))}
        </select>
      ) : (
        <input
          id={id}
          value={value || ""}
          maxLength={8192}
          onChange={(e) => onChange(e.target.value)}
        />
      )}
    </label>
  );
}
