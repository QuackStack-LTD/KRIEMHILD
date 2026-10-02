"use client";
import { useEffect, useState } from "react";
import {
  api,
  Command,
  Entry,
  HistoricalState,
  State,
  textOf,
} from "../lib/types";
import { useRecordDraft } from "../lib/useRecordDraft";
import DraftRecovery from "./DraftRecovery";
type Props = {
  state: State;
  run: (c: Command) => Promise<boolean>;
  busy: boolean;
  onPending?: (pending: boolean) => void;
};
export default function Outline({ state, run, busy, onPending }: Props) {
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
  const [work, setWork] = useState(""),
    [selected, setSelected] = useState("");
  const entities = Object.values(state.records).filter(
      (r) => r.kind === "entity",
    ),
    scenes = Object.values(state.records)
      .filter(
        (r) => r.kind === "scene" && (!work || r.properties?.workID === work),
      )
      .sort(
        (a, b) => (a.order || 0) - (b.order || 0) || a.id.localeCompare(b.id),
      );
  const scene = scenes.find((s) => s.id === selected) || scenes[0];
  return (
    <div>
      <div className="eyebrow">Writing / {state.age.name}</div>
      <h2>Story architecture</h2>
      <p>
        Works, arcs and chapters are shared entities. Narrative order is
        independent of fictional chronology; planning never rewrites your prose.
      </p>
      <label>
        Filter work
        <select
          disabled={pending}
          value={work}
          onChange={(e) => setWork(e.target.value)}
        >
          <option value="">All works</option>
          {entities
            .filter((r) => r.properties?._domain === "work")
            .map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
        </select>
      </label>
      <div className="corkboard">
        {scenes.map((r, i) => (
          <article key={r.id} className="card">
            <small>
              {r.order} · {String(r.properties?.draftStatus || "draft")}
            </small>
            <h3>
              <button
                disabled={pending}
                className="text-button"
                onClick={() => setSelected(r.id)}
              >
                {r.name}
              </button>
            </h3>
            <p>
              {String(r.properties?.goal || textOf(r.document).slice(0, 120))}
            </p>
            <p className="muted">
              {state.root.ages[r.settingAge!]?.name} ·{" "}
              {r.settingDate || "overview"}
            </p>
            {[-1, 1].map((direction) => (
              <button
                key={direction}
                disabled={
                  busy ||
                  pending ||
                  state.readOnly ||
                  i + direction < 0 ||
                  i + direction >= scenes.length
                }
                onClick={() => {
                  const reordered = [...scenes];
                  [reordered[i], reordered[i + direction]] = [
                    reordered[i + direction],
                    reordered[i],
                  ];
                  void run({
                    action: "put-many",
                    records: reordered.map((s, n) => ({ ...s, order: n + 1 })),
                  });
                }}
              >
                {direction < 0 ? "Move earlier" : "Move later"}
              </button>
            ))}
          </article>
        ))}
      </div>
      {scene ? (
        <PlanForm
          key={JSON.stringify(scene)}
          age={state.age.id}
          onPending={setPending}
          scene={scene}
          entities={entities}
          world={state.root.world.id}
          busy={busy || !!state.readOnly}
          run={run}
        />
      ) : (
        <p>
          Create scenes in Writing; create works, arcs and chapters in
          Societies.
        </p>
      )}
    </div>
  );
}
function PlanForm({
  scene,
  entities,
  world,
  busy,
  run,
  age,
  onPending,
}: {
  scene: Entry;
  entities: Entry[];
  world: string;
  busy: boolean;
  run: Props["run"];
  age: string;
  onPending: (pending: boolean) => void;
}) {
  const [saved] = useState<Entry>({
    id: scene.id,
    kind: scene.kind,
    name: scene.name,
    properties: Object.fromEntries(
      Object.entries(scene.properties || {}).filter(([key]) =>
        [
          "workID",
          "chapterID",
          "arcID",
          "povID",
          "locationID",
          "draftStatus",
          "goal",
          "conflict",
          "outcome",
          "foreshadowing",
          "payoff",
          "knowledge",
          "requirements",
          "research",
        ].includes(key),
      ),
    ),
  });
  const draft = useRecordDraft(
    `kriemhild-plan:${world}:${age}:${scene.id}`,
    saved,
  );
  const properties = draft.value.properties || {};
  useEffect(() => {
    onPending(draft.pending);
    return () => onPending(false);
  }, [draft.pending, onPending]);
  const [setting, setSetting] = useState<Entry[]>([]);
  const [settingError, setSettingError] = useState("");
  useEffect(() => {
    let active = true;
    const request = scene.settingDate
      ? api<HistoricalState>(
          `/projects/${world}/historical?snapshot=${scene.settingSnapshot}&tick=${encodeURIComponent(scene.settingDate)}`,
        ).then((r) => r.records)
      : api<Record<string, Entry>>(
          `/projects/${world}/snapshots/${scene.settingSnapshot}`,
        );
    request
      .then((records) => {
        if (active)
          setSetting(Object.values(records).filter((r) => r.kind === "entity"));
      })
      .catch((e) => {
        if (active) setSettingError((e as Error).message);
      });
    return () => {
      active = false;
    };
  }, [world, scene.settingSnapshot, scene.settingDate]);
  const update = (key: string, v: unknown) =>
    draft.setValue({ ...draft.value, properties: { ...properties, [key]: v } });
  return (
    <form
      className="card"
      onSubmit={async (e) => {
        e.preventDefault();
        if (
          await run({
            action: "put",
            record: {
              ...scene,
              properties: { ...scene.properties, ...properties },
            },
          })
        )
          draft.saved();
      }}
    >
      <h3>Plan: {scene.name}</h3>
      <p className="muted">
        Location and point of view refer to the scene's pinned historical
        setting.
      </p>
      {settingError && <p role="alert">{settingError}</p>}
      <DraftRecovery
        draft={draft}
        kind="scene plan"
        records={Object.fromEntries(
          [...entities, ...setting].map((r) => [r.id, r]),
        )}
        readOnly={busy}
      />
      <fieldset disabled={busy || !draft.ready || !!draft.recovery}>
        <legend>Scene planning fields</legend>
        <div className="two-col">
          {[
            ["workID", "Work", "work"],
            ["chapterID", "Chapter / act", "chapter"],
            ["arcID", "Plot thread / arc", "arc"],
            ["povID", "Point of view", ""],
            ["locationID", "Location", ""],
          ].map(([key, label, domain]) => (
            <label key={key}>
              {label}
              <select
                aria-label={label}
                value={String(properties[key] || "")}
                onChange={(e) => update(key, e.target.value)}
              >
                <option value="">Unspecified</option>
                {(domain ? entities : setting)
                  .filter((r) => !domain || r.properties?._domain === domain)
                  .map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.name}
                    </option>
                  ))}
              </select>
            </label>
          ))}
          <label>
            Draft status
            <select
              aria-label="Draft status"
              value={String(properties.draftStatus || "draft")}
              onChange={(e) => update("draftStatus", e.target.value)}
            >
              {["planned", "draft", "revision", "complete"].map((s) => (
                <option key={s}>{s}</option>
              ))}
            </select>
          </label>
          {[
            "goal",
            "conflict",
            "outcome",
            "foreshadowing",
            "payoff",
            "knowledge",
            "requirements",
            "research",
          ].map((key) => (
            <label key={key}>
              {key}
              <textarea
                aria-label={key}
                value={String(properties[key] || "")}
                onChange={(e) => update(key, e.target.value)}
              />
            </label>
          ))}
        </div>
        <button disabled={busy}>Save scene plan</button>
        <button type="button" disabled={!draft.dirty} onClick={draft.discard}>
          Discard scene plan changes
        </button>
      </fieldset>
    </form>
  );
}
