"use client";
import { useEffect, useState } from "react";
import { api, Command, Difference, Entry, State, textOf } from "../lib/types";
import { MapPicture } from "./Terrain";
export default function History({
  state,
  run,
  busy,
}: {
  state: State;
  run: (c: Command) => Promise<boolean>;
  busy: boolean;
}) {
  const ages = Object.values(state.root.ages);
  const [leftRecords, setLeftRecords] = useState<Record<string, Entry>>({}),
    [rightRecords, setRightRecords] = useState<Record<string, Entry>>({});
  const [from, setFrom] = useState(
      state.age.sourceAge ||
        ages.find((a) => a.id !== state.age.id)?.id ||
        state.age.id,
    ),
    [to, setTo] = useState(state.age.id),
    [changes, setChanges] = useState<Difference[]>([]),
    [error, setError] = useState(""),
    [mode, setMode] = useState<"compare" | "corrections">("compare"),
    [incoming, setIncoming] = useState(""),
    [selected, setSelected] = useState<string[]>([]),
    [loading, setLoading] = useState(false);
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    setSelected([]);
    const base = `/projects/${state.root.world.id}`;
    const request =
      mode === "corrections"
        ? api<{ changes: Difference[]; incoming: string }>(
            `${base}/corrections?age=${state.age.id}`,
          )
        : api<Difference[]>(
            `${base}/compare?from=${state.root.ages[from]?.snapshot || state.age.snapshot}&to=${state.root.ages[to]?.snapshot || state.age.snapshot}`,
          );
    const leftSnapshot =
      mode === "corrections"
        ? state.age.sourceSnapshot
        : state.root.ages[from]?.snapshot;
    const rightSnapshot =
      mode === "corrections"
        ? state.root.ages[state.age.sourceAge || ""]?.snapshot
        : state.root.ages[to]?.snapshot;
    Promise.all([
      request,
      api<Record<string, Entry>>(
        `${base}/snapshots/${leftSnapshot || state.age.snapshot}`,
      ),
      api<Record<string, Entry>>(
        `${base}/snapshots/${rightSnapshot || state.age.snapshot}`,
      ),
    ])
      .then(([result, left, right]) => {
        if (cancelled) return;
        setLeftRecords(left);
        setRightRecords(right);
        if (Array.isArray(result)) {
          setChanges(result);
          setIncoming("");
        } else {
          setChanges(
            result.changes.filter(
              (d) =>
                d.after?.kind !== "chronology" &&
                d.before?.kind !== "chronology",
            ),
          );
          setIncoming(result.incoming);
        }
      })
      .catch((e) => {
        if (!cancelled) {
          setError(e.message);
          setChanges([]);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [
    state.revision,
    state.root.world.id,
    state.age.id,
    state.age.snapshot,
    from,
    to,
    mode,
  ]); // eslint-disable-line react-hooks/exhaustive-deps
  return (
    <div className="history-page">
      <div className="eyebrow">Historical perspective</div>
      <h2>What changed?</h2>
      <p className="muted">
        Compare complete saved states. Looking at history never changes it.
      </p>
      <div className="tabs">
        <button
          aria-pressed={mode === "compare"}
          onClick={() => setMode("compare")}
        >
          Compare Ages
        </button>
        <button
          aria-pressed={mode === "corrections"}
          disabled={!state.age.sourceAge}
          onClick={() => setMode("corrections")}
        >
          Review source corrections
        </button>
      </div>
      {mode === "compare" ? (
        <div className="two-col">
          <label>
            Earlier / left Age
            <select value={from} onChange={(e) => setFrom(e.target.value)}>
              {ages.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Later / right Age
            <select value={to} onChange={(e) => setTo(e.target.value)}>
              {ages.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </select>
          </label>
        </div>
      ) : (
        <div className="notice">
          <strong>Changes made to the source after this Age was copied.</strong>
          <p>
            Select only the records you want to replace in {state.age.name}.
            Conflicting records are marked and never selected automatically. P1
            reviews whole records; detailed field merging comes later.
          </p>
        </div>
      )}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {loading ? (
        <p role="status">Comparing snapshots…</p>
      ) : (
        <p>
          {changes.length} changed record{changes.length !== 1 ? "s" : ""}
        </p>
      )}
      {!loading &&
        changes.map((d) => (
          <article className="diff-card" key={d.id}>
            <div className="section-heading">
              <h3>
                {d.after?.name || d.before?.name}{" "}
                <span>{d.after?.kind || d.before?.kind}</span>
              </h3>
              <span className="badge">
                {d.change}
                {d.conflict ? " · conflict" : ""}
              </span>
              {mode === "corrections" && (
                <label className="check-label">
                  <input
                    type="checkbox"
                    checked={selected.includes(d.id)}
                    onChange={(e) =>
                      setSelected(
                        e.target.checked
                          ? [...selected, d.id]
                          : selected.filter((id) => id !== d.id),
                      )
                    }
                  />
                  Apply this record
                </label>
              )}
            </div>
            {d.conflict && (
              <p className="error">
                Both Ages changed this record. Applying the source replaces this
                Age’s version.
              </p>
            )}
            <div className="diff-columns">
              {d.before?.kind === "map" && (
                <div>
                  <h4>Earlier map</h4>
                  <MapPicture
                    map={d.before}
                    records={leftRecords}
                    world={state.root.world.id}
                    label="Earlier map comparison"
                  />
                </div>
              )}
              {d.after?.kind === "map" && (
                <div>
                  <h4>Later map</h4>
                  <MapPicture
                    map={d.after}
                    records={rightRecords}
                    world={state.root.world.id}
                    label="Later map comparison"
                  />
                </div>
              )}
            </div>
            <div className="diff-columns">
              <DiffRecord
                label={mode === "corrections" ? "Original source" : "Before"}
                record={d.before}
              />
              {mode === "corrections" && (
                <DiffRecord label="Current Age" record={d.current || null} />
              )}
              <DiffRecord
                label={mode === "corrections" ? "Updated source" : "After"}
                record={d.after}
              />
            </div>
          </article>
        ))}
      {mode === "corrections" && (
        <button
          className="primary"
          disabled={!selected.length || busy || loading}
          onClick={async () => {
            if (
              window.confirm(
                `Apply ${selected.length} selected source records to ${state.age.name}? This can be undone.`,
              )
            ) {
              if (
                await run({
                  action: "apply-corrections",
                  ids: selected,
                  incoming,
                })
              )
                setSelected([]);
            }
          }}
        >
          Apply {selected.length} selected corrections
        </button>
      )}
    </div>
  );
}
function DiffRecord({
  label,
  record,
}: {
  label: string;
  record: Entry | null;
}) {
  return (
    <div>
      <h4>{label}</h4>
      {record ? (
        <>
          <strong>{record.name}</strong>
          {record.type && (
            <p>
              {record.type} · {record.status}
            </p>
          )}
          {record.notes && <p className="preserve">{record.notes}</p>}
          {record.kind === "scene" && (
            <p className="preserve">{textOf(record.document)}</p>
          )}
          <details>
            <summary>All saved fields</summary>
            <pre>{JSON.stringify(record, null, 2)}</pre>
          </details>
        </>
      ) : (
        <p className="muted">Not present</p>
      )}
    </div>
  );
}
