"use client";
import { useCallback, useEffect, useState } from "react";
import { api, Command, Entry, newID, State } from "../lib/types";
import { useRecordDraft } from "../lib/useRecordDraft";
import DraftRecovery from "./DraftRecovery";
type Props = {
  state: State;
  run: (c: Command) => Promise<boolean>;
  busy: boolean;
  onPending?: (pending: boolean) => void;
};
export default function Storyboards({ state, run, busy, onPending }: Props) {
  const [drafts, setDrafts] = useState<Record<string, boolean>>({});
  const report = useCallback(
    (id: string, pending: boolean) =>
      setDrafts((previous) =>
        previous[id] === pending ? previous : { ...previous, [id]: pending },
      ),
    [],
  );
  const pending = Object.values(drafts).some(Boolean);
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
  const [work, setWork] = useState(""),
    [name, setName] = useState("");
  const panels = Object.values(state.records)
    .filter(
      (r) =>
        r.properties?._domain === "storyboard" &&
        (!work || r.properties.work === work),
    )
    .sort(
      (a, b) => (a.order || 0) - (b.order || 0) || a.id.localeCompare(b.id),
    );
  return (
    <div>
      <div className="eyebrow">Writing / {state.age.name}</div>
      <h2>Storyboards</h2>
      <p>
        Arrange your own drawings, shot descriptions and dialogue. Panels share
        the world's normal Age history and can be connected to works.
      </p>
      <label>
        Storyboard work
        <select
          aria-label="Storyboard work"
          value={work}
          disabled={pending}
          onChange={(e) => setWork(e.target.value)}
        >
          <option value="">All works</option>
          {Object.values(state.records)
            .filter((r) => r.properties?._domain === "work")
            .map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
        </select>
      </label>
      <form
        className="map-tools"
        onSubmit={async (e) => {
          e.preventDefault();
          if (
            await run({
              action: "put",
              record: {
                id: newID(),
                kind: "entity",
                type: "Storyboard panel",
                name,
                order: panels.length + 1,
                properties: { _domain: "storyboard", work },
              },
            })
          )
            setName("");
        }}
      >
        <label>
          New panel title
          <input
            required
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </label>
        <button disabled={busy || state.readOnly}>
          Create storyboard panel
        </button>
      </form>
      <div className="corkboard">
        {panels.map((panel, i) => (
          <Panel
            key={panel.id + JSON.stringify(panel)}
            {...{ state, run, busy, panel }}
            report={report}
            pending={pending}
            earlier={i > 0}
            later={i < panels.length - 1}
            move={async (direction) => {
              const ordered = [...panels];
              [ordered[i], ordered[i + direction]] = [
                ordered[i + direction],
                ordered[i],
              ];
              await run({
                action: "put-many",
                records: ordered.map((p, n) => ({ ...p, order: n + 1 })),
              });
            }}
          />
        ))}
      </div>
    </div>
  );
}
function Panel({
  state,
  run,
  busy,
  panel,
  earlier,
  later,
  move,
  report,
  pending,
}: Props & {
  panel: Entry;
  earlier: boolean;
  later: boolean;
  move: (direction: number) => Promise<void>;
  report: (id: string, pending: boolean) => void;
  pending: boolean;
}) {
  const draft = useRecordDraft(
    `kriemhild-panel:${state.root.world.id}:${state.age.id}:${panel.id}`,
    panel,
  );
  const { value, setValue } = draft;
  const [working, setWorking] = useState(false),
    [error, setError] = useState("");
  useEffect(() => {
    report(panel.id, draft.pending);
    return () => report(panel.id, false);
  }, [panel.id, draft.pending, report]);
  return (
    <article className="card">
      <h3>{panel.name}</h3>
      <DraftRecovery
        draft={draft}
        kind="panel"
        records={state.records}
        readOnly={state.readOnly}
      />
      {panel.asset && (
        <img
          src={`/api/v1/projects/${state.root.world.id}/assets/${panel.asset}`}
          alt={String(panel.properties?.caption || panel.name)}
          style={{ width: "100%", maxHeight: 250, objectFit: "contain" }}
        />
      )}
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          if (await run({ action: "put", record: value })) draft.saved();
        }}
      >
        <fieldset
          disabled={
            busy ||
            working ||
            state.readOnly ||
            !draft.ready ||
            !!draft.recovery
          }
        >
          <legend>Panel {panel.order || ""}</legend>
          <label>
            Panel title
            <input
              aria-label="Panel title"
              required
              value={value.name}
              onChange={(e) => setValue({ ...value, name: e.target.value })}
            />
          </label>
          {["shot", "framing", "action", "dialogue", "caption"].map((key) => (
            <label key={key}>
              {key}
              <textarea
                aria-label={`Panel ${key}`}
                value={String(value.properties?.[key] || "")}
                onChange={(e) =>
                  setValue({
                    ...value,
                    properties: { ...value.properties, [key]: e.target.value },
                  })
                }
              />
            </label>
          ))}
          <label>
            Public description
            <textarea
              aria-label="Panel public description"
              value={value.notes || ""}
              onChange={(e) => setValue({ ...value, notes: e.target.value })}
            />
          </label>
          <label>
            Panel image
            <input
              type="file"
              accept="image/png,image/jpeg,image/gif"
              onChange={async (e) => {
                const file = e.target.files?.[0];
                if (!file) return;
                setWorking(true);
                setError("");
                try {
                  const image = await api<{
                    asset: string;
                    width: number;
                    height: number;
                  }>(`/projects/${state.root.world.id}/assets`, {
                    method: "POST",
                    body: file,
                  });
                  setValue((v) => ({ ...v, ...image }));
                } catch (e) {
                  setError((e as Error).message);
                } finally {
                  setWorking(false);
                }
              }}
            />
          </label>
          {value.asset !== panel.asset && <p>New image ready to save.</p>}
          <button>Save storyboard panel</button>
          <button type="button" disabled={!draft.dirty} onClick={draft.discard}>
            Discard panel changes
          </button>
        </fieldset>
      </form>
      {error && <p role="alert">{error}</p>}
      <button
        disabled={busy || pending || !earlier || state.readOnly}
        onClick={() => void move(-1)}
      >
        Move panel earlier
      </button>
      <button
        disabled={busy || pending || !later || state.readOnly}
        onClick={() => void move(1)}
      >
        Move panel later
      </button>
    </article>
  );
}
