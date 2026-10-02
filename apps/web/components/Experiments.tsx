"use client";
import { useState } from "react";
import { api, Command, Entry, newID, State } from "../lib/types";
const operations: Record<
  string,
  { name: string; fields: [string, string, string][] }
> = {
  "sound-change": {
    name: "Ordered sound changes",
    fields: [
      [
        "rules",
        "Rules: token > replacement / left _ right (optional context)",
        "p > f\nt > θ",
      ],
      [
        "classes",
        "Sound classes: Name = tokens; # means word boundary",
        "V = a e i o u\nC = p t k b d g",
      ],
    ],
  },
  phonotactics: {
    name: "Phonotactic pattern checks",
    fields: [
      ["classes", "Sound classes", "V = a e i o u\nC = p t k b d g"],
      [
        "patterns",
        "Allowed token patterns, one per line",
        "C V\nC V C\nC V C V",
      ],
    ],
  },
  inflection: {
    name: "Inflection paradigm preview",
    fields: [
      [
        "affixes",
        "Forms: label | prefix | suffix",
        "singular | |\nplural | | i",
      ],
    ],
  },
  production: {
    name: "Production balance",
    fields: [
      ["supply", "Supply per period", "100"],
      ["demand", "Demand per period", "120"],
      ["duration", "Periods", "3"],
      ["unit", "Unit", "sacks"],
    ],
  },
  "production-network": {
    name: "Recipe network inventories",
    fields: [["periods", "Production periods (1–100)", "3"]],
  },
  inheritance: {
    name: "Custom inherited trait",
    fields: [
      ["parentA", "Parent A state", "Aa"],
      ["parentB", "Parent B state", "Aa"],
    ],
  },
  succession: {
    name: "Succession candidates",
    fields: [
      [
        "roles",
        "Allowed parent roles, comma separated",
        "biological parent of,adoptive parent of",
      ],
    ],
  },
  climate: {
    name: "Climate offset",
    fields: [
      ["delta", "Temperature change", "2"],
      ["unit", "Unit", "degrees"],
    ],
  },
  erosion: {
    name: "Terrain smoothing experiment",
    fields: [["steps", "Steps (1–30)", "5"]],
  },
  route: {
    name: "Least-cost transport path",
    fields: [
      ["origin", "Origin entity ID", ""],
      ["destination", "Destination entity ID", ""],
      ["unit", "Cost unit", "days"],
    ],
  },
};
type Result = {
  allOrNothing?: boolean;
  revision: string;
  snapshot: string;
  algorithm: string;
  rows: Record<string, string>[];
  proposals: Entry[];
  warnings: string[];
};
export default function Experiments({
  state,
  run,
  busy,
}: {
  state: State;
  run: (c: Command) => Promise<boolean>;
  busy: boolean;
}) {
  const [operation, setOperation] = useState("sound-change"),
    [values, setValues] = useState<Record<string, string>>({
      rules: "p > f\nt > θ",
    }),
    [ids, setIDs] = useState<string[]>([]),
    [result, setResult] = useState<Result | null>(null),
    [accepted, setAccepted] = useState<string[]>([]),
    [error, setError] = useState(""),
    [working, setWorking] = useState(false),
    [eventName, setEventName] = useState(""),
    [tick, setTick] = useState("");
  return (
    <div>
      <div className="eyebrow">Optional workbench</div>
      <h2>Explore an idea. Choose what becomes true.</h2>
      <p>
        Every calculation uses a pinned snapshot. Nothing changes until you
        accept selected proposals. These tools contain no AI or autonomous world
        simulation.
      </p>
      <label>
        Experiment
        <select
          aria-label="Experiment"
          value={operation}
          onChange={(e) => {
            const key = e.target.value;
            setOperation(key);
            setValues(
              Object.fromEntries(
                operations[key].fields.map(([k, , v]) => [k, v]),
              ),
            );
            setResult(null);
            setIDs([]);
          }}
        >
          {Object.entries(operations).map(([key, v]) => (
            <option key={key} value={key}>
              {v.name}
            </option>
          ))}
        </select>
      </label>
      <div className="two-col">
        {operations[operation].fields.map(([key, label]) => (
          <label key={key}>
            {label}
            {operation === "route" && key !== "unit" ? (
              <select
                aria-label={label}
                value={values[key] || ""}
                onChange={(e) =>
                  setValues({ ...values, [key]: e.target.value })
                }
              >
                <option value="">Choose</option>
                {Object.values(state.records)
                  .filter((r) => r.kind === "entity")
                  .map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.name}
                    </option>
                  ))}
              </select>
            ) : (
              <textarea
                aria-label={label}
                value={values[key] || ""}
                onChange={(e) =>
                  setValues({ ...values, [key]: e.target.value })
                }
              />
            )}
          </label>
        ))}
      </div>
      <details open>
        <summary>Input records</summary>
        <div className="export-selection">
          {Object.values(state.records)
            .filter((r) =>
              operation === "production-network"
                ? r.properties?._domain === "recipe" &&
                  !!r.properties?._production
                : r.kind === "entity" || r.kind === "map",
            )
            .map((r) => (
              <label key={r.id} className="check-label">
                <input
                  type="checkbox"
                  checked={ids.includes(r.id)}
                  onChange={(e) =>
                    setIDs(
                      e.target.checked
                        ? [...ids, r.id]
                        : ids.filter((id) => id !== r.id),
                    )
                  }
                />
                {r.name} · {r.type || r.kind}
              </label>
            ))}
        </div>
      </details>
      <button
        disabled={busy || working}
        onClick={async () => {
          setWorking(true);
          setError("");
          setResult(null);
          try {
            const out = await api<Result>(
              `/projects/${state.root.world.id}/experiment`,
              {
                method: "POST",
                body: JSON.stringify({
                  operation,
                  values,
                  ids,
                  age: state.age.id,
                  expected: state.revision,
                }),
              },
            );
            setResult(out);
            setAccepted(out.proposals.map((r) => r.id));
          } catch (e) {
            setError((e as Error).message);
          } finally {
            setWorking(false);
          }
        }}
      >
        Preview experiment
      </button>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {result && (
        <section className="card">
          <h3>Review results</h3>
          <small>
            {result.algorithm} · input snapshot {result.snapshot.slice(0, 10)}
          </small>
          {result.warnings.map((s) => (
            <p className="notice" key={s}>
              {s}
            </p>
          ))}
          {result.rows.map((row, i) => (
            <dl className="result-row" key={i}>
              {Object.entries(row).map(([k, v]) => (
                <div key={k}>
                  <dt>{k}</dt>
                  <dd>{v}</dd>
                </div>
              ))}
            </dl>
          ))}
          {result.proposals.length > 0 && (
            <>
              <h4>
                {result.allOrNothing
                  ? "Linked inventory changes"
                  : "Choose records to update"}
              </h4>
              {result.proposals.map((r) => (
                <label className="check-label" key={r.id}>
                  <input
                    type="checkbox"
                    checked={accepted.includes(r.id)}
                    disabled={result.allOrNothing}
                    onChange={(e) =>
                      setAccepted(
                        e.target.checked
                          ? [...accepted, r.id]
                          : accepted.filter((id) => id !== r.id),
                      )
                    }
                  />
                  {r.name}
                </label>
              ))}
              <div className="two-col">
                <label>
                  Optional event name
                  <input
                    value={eventName}
                    onChange={(e) => setEventName(e.target.value)}
                  />
                </label>
                <label>
                  Exact event tick
                  <input
                    value={tick}
                    onChange={(e) => setTick(e.target.value)}
                  />
                </label>
              </div>
              <button
                disabled={
                  busy ||
                  state.readOnly ||
                  !accepted.length ||
                  state.revision !== result.revision
                }
                onClick={async () => {
                  const records = result.proposals.filter((r) =>
                    accepted.includes(r.id),
                  );
                  if (
                    await run(
                      eventName
                        ? {
                            action: "record-event",
                            records,
                            event: {
                              id: newID(),
                              kind: "event",
                              name: eventName,
                              notes: `Accepted ${result.algorithm} from snapshot ${result.snapshot}.`,
                              event: {
                                age: state.age.id,
                                date: { precision: "exact", tick },
                                track: "Authored experiments",
                              },
                            },
                          }
                        : { action: "put-many", records },
                    )
                  )
                    setResult(null);
                }}
              >
                {result.allOrNothing
                  ? "Accept linked balances"
                  : "Accept selected proposals"}
              </button>
              {state.revision !== result.revision && (
                <p className="notice">
                  The world changed after this preview. Run the experiment
                  again.
                </p>
              )}
            </>
          )}
          <button onClick={() => setResult(null)}>Discard results</button>
        </section>
      )}
    </div>
  );
}
