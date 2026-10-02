"use client";
import { draftStorageKey } from "../lib/draftStorage";
import dynamic from "next/dynamic";
import { useEffect, useRef, useState } from "react";
import {
  api,
  Command,
  Doc,
  emptyDoc,
  Entry,
  HistoricalState,
  newID,
  State,
  textOf,
} from "../lib/types";
import { Empty } from "./Records";
import { MapPicture } from "./Terrain";
const Editor = dynamic(() => import("./Editor"), { ssr: false });
type Props = {
  state: State;
  run: (c: Command) => Promise<boolean>;
  busy: boolean;
  inspect: (id: string) => void;
  selected?: string;
  pending?: boolean;
  onPending?: (pending: boolean) => void;
};
export default function Writing({
  state,
  run,
  busy,
  inspect,
  selected,
  pending,
  onPending,
}: Props) {
  const scenes = Object.values(state.records)
    .filter((r) => r.kind === "scene")
    .sort(
      (a, b) => (a.order || 0) - (b.order || 0) || a.name.localeCompare(b.name),
    );
  const [id, setID] = useState(selected || "");
  const [name, setName] = useState("");
  const scene = scenes.find((s) => s.id === id) || scenes[0];
  return (
    <div className="writing-layout">
      <aside className="scene-list">
        <div className="eyebrow">Manuscript</div>
        <h2>Scenes</h2>
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            const r: Entry = {
              id: newID(),
              kind: "scene",
              name,
              document: emptyDoc,
              story: "Untitled work",
              order: scenes.length + 1,
              settingAge: state.age.id,
              settingSnapshot: state.age.snapshot,
              references: [],
            };
            if (await run({ action: "put", record: r })) {
              setID(r.id);
              setName("");
            }
          }}
        >
          <label>
            New scene title
            <input
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <button disabled={busy || pending}>+ Create scene</button>
        </form>
        {scenes.map((s) => (
          <button
            className={s.id === scene?.id ? "record active" : "record"}
            key={s.id}
            disabled={busy || pending}
            onClick={() => setID(s.id)}
          >
            <span>
              <small>
                {s.story} · {s.order}
              </small>
              <strong>{s.name}</strong>
              <small>
                {textOf(s.document).trim().split(/\s+/).filter(Boolean).length}{" "}
                words
              </small>
            </span>
          </button>
        ))}
      </aside>
      <section className="writing-main">
        {scene ? (
          <Scene
            key={state.age.id + scene.id}
            {...{ state, run, busy, inspect, onPending }}
            scene={scene}
          />
        ) : (
          <Empty
            title="Write from inside your world."
            text="Create a scene. Its setting will preserve this Age as it exists now, even as your world develops."
          />
        )}
      </section>
    </div>
  );
}
type Draft = { record: Entry; base: Entry };
function Scene({
  scene,
  state,
  run,
  busy,
  inspect,
  onPending,
}: Props & { scene: Entry }) {
  const baseKey = `kriemhild:draft:${state.root.world.id}:${state.age.id}:${scene.id}`;
  const key = useRef("");
  const [settingTick, setSettingTick] = useState(scene.settingDate || ""),
    [unresolved, setUnresolved] = useState<string[]>([]);
  const [value, setValue] = useState(scene),
    [status, setStatus] = useState("Saved on this device"),
    [recovery, setRecovery] = useState<Draft | null>(null),
    [context, setContext] = useState<Record<string, Entry>>({});
  const [dirty, setDirty] = useState(false),
    [blocked, setBlocked] = useState(false),
    [draftError, setDraftError] = useState("");
  const current = useRef(scene),
    base = useRef(scene),
    version = useRef(0),
    inFlight = useRef(false),
    mounted = useRef(true),
    runRef = useRef(run);
  runRef.current = run;
  useEffect(() => {
    onPending?.(dirty || !!recovery);
    return () => onPending?.(false);
  }, [dirty, recovery, onPending]);
  useEffect(() => {
    mounted.current = true;
    try {
      key.current = draftStorageKey(baseKey);
      const d = localStorage.getItem(key.current);
      if (d) {
        const parsed = JSON.parse(d) as Draft;
        if (JSON.stringify(parsed.record) !== JSON.stringify(scene)) {
          setRecovery(parsed);
          setStatus("Recovery draft available");
        } else localStorage.removeItem(key.current);
      }
    } catch {
      setDraftError(
        "Browser recovery storage is unavailable. Keep this scene open until the server confirms saving.",
      );
    }
    return () => {
      mounted.current = false;
    };
  }, [baseKey]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!dirty && !inFlight.current) {
      current.current = scene;
      base.current = scene;
      setValue(scene);
    }
  }, [scene, dirty]);
  useEffect(() => {
    let cancelled = false;
    const request = value.settingDate
      ? api<HistoricalState>(
          `/projects/${state.root.world.id}/historical?snapshot=${value.settingSnapshot}&tick=${encodeURIComponent(value.settingDate)}`,
        ).then((r) => ({ records: r.records, unresolved: r.unresolved }))
      : api<Record<string, Entry>>(
          `/projects/${state.root.world.id}/snapshots/${value.settingSnapshot}`,
        ).then((records) => ({ records, unresolved: [] as string[] }));
    setContext({});
    request
      .then((r) => {
        if (!cancelled) {
          setContext(r.records);
          setUnresolved(r.unresolved);
        }
      })
      .catch((e) => {
        if (!cancelled) {
          setContext({});
          setUnresolved([e.message]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [state.root.world.id, value.settingSnapshot, value.settingDate]);
  const persist = (r: Entry) => {
    try {
      if (!key.current) throw new Error("Recovery storage is unavailable");
      localStorage.setItem(
        key.current,
        JSON.stringify({ record: r, base: base.current }),
      );
      setDraftError("");
    } catch {
      setDraftError(
        "Browser recovery storage is full or unavailable. Do not close this scene until it is saved.",
      );
    }
  };
  const change = (r: Entry) => {
    current.current = r;
    version.current++;
    setValue(r);
    setDirty(true);
    setStatus("Unsaved draft · saving shortly");
    persist(r);
  };
  const save = async () => {
    if (inFlight.current) return;
    inFlight.current = true;
    const stamp = version.current,
      r = current.current;
    setStatus("Saving…");
    const ok = await runRef.current({ action: "put", record: r });
    inFlight.current = false;
    if (!mounted.current) return;
    if (ok) {
      base.current = r;
      if (stamp === version.current) {
        setDirty(false);
        setStatus("Saved on this device");
        try {
          if (key.current) localStorage.removeItem(key.current);
        } catch {}
      } else {
        persist(current.current);
        setStatus("Saving newer changes…");
      }
    } else {
      setStatus("Not saved · recovery draft retained");
      setBlocked(true);
    }
  };
  useEffect(() => {
    if (!dirty || blocked || recovery || busy) return;
    const timer = setTimeout(() => void save(), 800);
    return () => clearTimeout(timer);
  }, [value, dirty, blocked, recovery, busy]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    const before = (e: BeforeUnloadEvent) => {
      if (dirty) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", before);
    return () => window.removeEventListener("beforeunload", before);
  }, [dirty]);
  const entities = Object.values(context).filter((r) => r.kind === "entity");
  return (
    <>
      <div className="scene-top">
        <span className="eyebrow">{value.story || "Untitled work"}</span>
        <span
          role="status"
          className={dirty ? "save-status pending" : "save-status"}
        >
          {status}
        </span>
      </div>
      {recovery && (
        <div className="notice">
          <strong>Unsaved scene recovered from this browser.</strong>
          <p>
            {JSON.stringify(recovery.base) !== JSON.stringify(scene)
              ? "The saved scene has changed since this draft began. Restoring replaces it only when you explicitly save. Review the draft carefully."
              : "Restore your draft or keep the server version."}
          </p>
          <button
            onClick={() => {
              change(recovery.record);
              setRecovery(null);
              setBlocked(true);
              setStatus("Recovered draft · review, then save");
            }}
          >
            Restore draft for review
          </button>
          <button
            onClick={() => {
              try {
                if (key.current) localStorage.removeItem(key.current);
              } catch {
                /* Keep the saved scene. */
              }
              setRecovery(null);
              setStatus("Saved on this device");
            }}
          >
            Discard recovery draft
          </button>
        </div>
      )}
      {draftError && (
        <p role="alert" className="error">
          {draftError}
        </p>
      )}
      <label className="scene-title-label">
        Scene title
        <input
          className="scene-title"
          value={value.name}
          onChange={(e) => change({ ...current.current, name: e.target.value })}
        />
      </label>
      <div className="two-col">
        <label>
          Work / story
          <input
            value={value.story || ""}
            onChange={(e) =>
              change({ ...current.current, story: e.target.value })
            }
          />
        </label>
        <label>
          Scene order
          <input
            type="number"
            value={value.order || 0}
            onChange={(e) =>
              change({ ...current.current, order: Number(e.target.value) })
            }
          />
        </label>
      </div>
      <Editor
        value={value.document || emptyDoc}
        onChange={(document: Doc) => change({ ...current.current, document })}
      />
      <div className="form-actions">
        <span className="muted">
          {textOf(value.document).trim().split(/\s+/).filter(Boolean).length}{" "}
          words · automatic save after a pause
        </span>
        <button
          className="primary"
          disabled={busy || !dirty || !!recovery}
          onClick={() => {
            setBlocked(false);
            void save();
          }}
        >
          Save scene now
        </button>
      </div>
      <details className="scene-context" open>
        <summary>Historical setting & referenced entities</summary>
        <p>
          Setting:{" "}
          <strong>
            {state.root.ages[value.settingAge!]?.name || value.settingAge}
          </strong>{" "}
          · preserved snapshot{" "}
          <code>{value.settingSnapshot?.slice(0, 10)}</code>
        </p>
        <p className="muted">
          Copying an Age keeps this setting. Updating it is an explicit change;
          prose is never renamed automatically.
        </p>
        <button
          disabled={busy}
          onClick={() => {
            if (
              window.confirm(
                "Update this scene setting to the current saved Age? References missing here will be removed; prose will remain unchanged.",
              )
            )
              change({
                ...current.current,
                settingAge: state.age.id,
                settingSnapshot: state.age.snapshot,
                references: (current.current.references || []).filter(
                  (id) => state.records[id]?.kind === "entity",
                ),
              });
          }}
        >
          Retarget setting to current Age
        </button>
        <div className="reference-grid">
          <form
            onSubmit={(e) => {
              e.preventDefault();
              change({
                ...current.current,
                settingDate: settingTick,
                references: [],
              });
            }}
          >
            <label>
              Scene setting tick (empty uses overview)
              <input
                value={settingTick}
                onChange={(e) => setSettingTick(e.target.value)}
              />
            </label>
            <button disabled={busy}>Apply scene date</button>
            <p className="muted">
              Changing the date clears selected references so you can choose
              them from the dated setting.
            </p>
          </form>
          {unresolved.map((s, i) => (
            <p className="notice" key={i}>
              {s}
            </p>
          ))}
          {entities.map((r) => (
            <label className="check-label" key={r.id}>
              <input
                type="checkbox"
                checked={(value.references || []).includes(r.id)}
                onChange={(e) =>
                  change({
                    ...current.current,
                    references: e.target.checked
                      ? [...(current.current.references || []), r.id]
                      : (current.current.references || []).filter(
                          (id) => id !== r.id,
                        ),
                  })
                }
              />
              {r.name} <small>{r.type}</small>
            </label>
          ))}
        </div>
        {value.settingDate &&
          Object.values(context)
            .filter((r) => r.kind === "map")
            .map((r) => (
              <MapPicture
                key={r.id}
                map={r}
                records={context}
                world={state.root.world.id}
                label={`${r.name} scene setting at tick ${value.settingDate}`}
              />
            ))}
        {(value.references || []).map((id) => (
          <div className="context-card" key={id}>
            <strong>{context[id]?.name}</strong>
            <p>{context[id]?.notes || "No notes in this setting."}</p>
            {state.records[id] && (
              <button onClick={() => inspect(id)}>
                Open current-Age dossier
              </button>
            )}
          </div>
        ))}
      </details>
    </>
  );
}
