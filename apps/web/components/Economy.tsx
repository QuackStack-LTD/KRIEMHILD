"use client";
import { useEffect, useState } from "react";
import { Command, Entry, State } from "../lib/types";
import { useRecordDraft } from "../lib/useRecordDraft";
import DraftRecovery from "./DraftRecovery";

type Flow = { resource: string; amount: string; unit: string };
type Rule = {
  inputs: Flow[];
  outputs: Flow[];
  batches: number;
  priority: number;
};
const emptyRule: Rule = { inputs: [], outputs: [], batches: 1, priority: 0 };
type Props = {
  state: State;
  run: (command: Command) => Promise<boolean>;
  busy: boolean;
  onPending: (pending: boolean) => void;
};

export default function Economy({ state, run, busy, onPending }: Props) {
  const [selected, setSelected] = useState("");
  const [pending, setPending] = useState(false);
  const recipes = Object.values(state.records).filter(
    (r) => r.properties?._domain === "recipe",
  );
  const resources = Object.values(state.records).filter(
    (r) => r.properties?._domain === "resource",
  );
  const recipe = recipes.find((r) => r.id === selected);
  useEffect(() => {
    onPending(pending);
    return () => onPending(false);
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
  return (
    <div>
      <div className="eyebrow">Economy / {state.age.name}</div>
      <h2>Resources and production</h2>
      <p>
        Create resource and recipe dossiers in Societies & systems, then
        describe their quantities here. Inventories remain authored facts until
        you accept a calculation in Experiments.
      </p>
      <p>
        Each period runs recipes in ascending priority, then by name. Outputs
        can supply later recipes in that period. Returning through a cycle waits
        until the next period.
      </p>
      <label>
        Production recipe
        <select
          aria-label="Production recipe"
          value={selected}
          disabled={pending}
          onChange={(e) => setSelected(e.target.value)}
        >
          <option value="">Choose a recipe</option>
          {recipes.map((r) => (
            <option key={r.id} value={r.id}>
              {r.name}
            </option>
          ))}
        </select>
      </label>
      {!recipes.length && (
        <p className="notice">Create a Production recipe dossier to begin.</p>
      )}
      {recipe && (
        <RecipeForm
          key={recipe.id + state.age.snapshot}
          recipe={recipe}
          resources={resources}
          state={state}
          run={run}
          busy={busy}
          onPending={setPending}
        />
      )}
      <h3>Starting inventories</h3>
      <p>
        Set exact stocks and units in each resource dossier. Unknown or ranged
        stocks can remain in your world; calculations that need them will ask
        for explicit values.
      </p>
      <table>
        <thead>
          <tr>
            <th>Resource</th>
            <th>Stock</th>
            <th>Unit</th>
          </tr>
        </thead>
        <tbody>
          {resources.map((r) => {
            const q = r.properties?.quantity as
              | {
                  mode?: string;
                  value?: string;
                  min?: string;
                  max?: string;
                  text?: string;
                  unit?: string;
                }
              | undefined;
            return (
              <tr key={r.id}>
                <th>{r.name}</th>
                <td>
                  {q?.mode === "exact"
                    ? q.value
                    : q?.mode === "range"
                      ? `${q.min}–${q.max}`
                      : q?.mode === "qualitative"
                        ? q.text
                        : "Unknown"}
                </td>
                <td>{q?.unit || "Unspecified"}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function RecipeForm({
  recipe,
  resources,
  state,
  run,
  busy,
  onPending,
}: Props & { recipe: Entry; resources: Entry[] }) {
  // Recover only production fields so a concurrent dossier edit cannot be overwritten.
  const [saved] = useState<Entry>({
    id: recipe.id,
    kind: recipe.kind,
    name: recipe.name,
    properties: { production: recipe.properties?._production || emptyRule },
  });
  const draft = useRecordDraft(
    `kriemhild-production:${state.root.world.id}:${state.age.id}:${recipe.id}`,
    saved,
  );
  const rule = draft.value.properties!.production as Rule;
  const change = (value: Rule) =>
    draft.setValue({ ...draft.value, properties: { production: value } });
  useEffect(() => {
    onPending(draft.pending);
    return () => onPending(false);
  }, [draft.pending, onPending]);
  return (
    <form
      onSubmit={async (e) => {
        e.preventDefault();
        if (
          await run({
            action: "put",
            record: {
              ...recipe,
              properties: { ...recipe.properties, _production: rule },
            },
          })
        )
          draft.saved();
      }}
    >
      <DraftRecovery
        draft={draft}
        kind="recipe"
        records={state.records}
        readOnly={state.readOnly}
      />
      <fieldset
        disabled={busy || state.readOnly || !draft.ready || !!draft.recovery}
      >
        <legend>{recipe.name}: quantities per whole batch</legend>
        <div className="two-col">
          <label>
            Planned batches per period
            <input
              required
              type="number"
              min="0"
              max="1000000"
              step="1"
              value={rule.batches}
              onChange={(e) =>
                change({ ...rule, batches: Number(e.target.value) })
              }
            />
          </label>
          <label>
            Production priority
            <input
              required
              type="number"
              min="0"
              max="10000"
              step="1"
              value={rule.priority}
              onChange={(e) =>
                change({ ...rule, priority: Number(e.target.value) })
              }
            />
          </label>
        </div>
        {(["inputs", "outputs"] as const).map((side) => (
          <fieldset key={side}>
            <legend>
              {side === "inputs" ? "Consumed per batch" : "Produced per batch"}
            </legend>
            {rule[side].map((flow, i) => {
              const update = (value: Flow) =>
                change({
                  ...rule,
                  [side]: rule[side].map((f, j) => (i === j ? value : f)),
                });
              const label = `${side === "inputs" ? "Input" : "Output"} ${i + 1}`;
              return (
                <div className="map-tools" key={i}>
                  <label>
                    {label} resource
                    <select
                      required
                      aria-label={`${label} resource`}
                      value={flow.resource}
                      onChange={(e) => {
                        const resource = resources.find(
                          (r) => r.id === e.target.value,
                        );
                        const unit =
                          (
                            resource?.properties?.quantity as
                              | { unit?: string }
                              | undefined
                          )?.unit || "";
                        update({ ...flow, resource: e.target.value, unit });
                      }}
                    >
                      <option value="">Choose resource</option>
                      {resources.map((r) => (
                        <option key={r.id} value={r.id}>
                          {r.name}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    {label} amount
                    <input
                      required
                      inputMode="decimal"
                      value={flow.amount}
                      onChange={(e) =>
                        update({ ...flow, amount: e.target.value })
                      }
                    />
                  </label>
                  <label>
                    {label} unit
                    <input
                      required
                      value={flow.unit}
                      onChange={(e) =>
                        update({ ...flow, unit: e.target.value })
                      }
                    />
                  </label>
                  <button
                    type="button"
                    aria-label={`Remove ${label.toLowerCase()}`}
                    onClick={() =>
                      change({
                        ...rule,
                        [side]: rule[side].filter((_, j) => j !== i),
                      })
                    }
                  >
                    Remove
                  </button>
                </div>
              );
            })}
            <button
              type="button"
              disabled={rule[side].length >= 20}
              onClick={() =>
                change({
                  ...rule,
                  [side]: [
                    ...rule[side],
                    { resource: "", amount: "1", unit: "" },
                  ],
                })
              }
            >
              Add {side === "inputs" ? "input" : "output"}
            </button>
          </fieldset>
        ))}
        {!rule.inputs.length && (
          <p className="notice">
            No inputs: this recipe assumes an externally available source.
          </p>
        )}
        <p>
          No unit conversions, labour, storage or transport limits are inferred.
          Describe those separately or include explicit resource inputs.
        </p>
        <button disabled={!draft.dirty || !rule.outputs.length}>
          Save production recipe
        </button>
      </fieldset>
      {draft.dirty && (
        <button type="button" disabled={busy} onClick={draft.discard}>
          Discard recipe changes
        </button>
      )}
    </form>
  );
}
