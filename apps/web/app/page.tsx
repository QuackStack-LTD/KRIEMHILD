"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { api, Command, State, World } from "../lib/types";
import { Entities, Notebook, Types } from "../components/Records";
import Atlas from "../components/Atlas";
import SearchResults from "../components/SearchResults";
import LocalMaps from "../components/LocalMaps";
import FamilyGraph from "../components/FamilyGraph";
import Storyboards from "../components/Storyboards";
import Writing from "../components/Writing";
import History from "../components/History";
import Timeline from "../components/Timeline";
import Domains from "../components/Domains";
import Outline from "../components/Outline";
import ProjectTools, {
  Consistency,
  ImportPanel,
} from "../components/ProjectTools";
import Experiments from "../components/Experiments";
import Economy from "../components/Economy";
import Access, { Login } from "../components/Access";
import AccountSecurity from "../components/AccountSecurity";
import dynamic from "next/dynamic";
const LiveWriting = dynamic(() => import("../components/LiveWriting"), {
  ssr: false,
});

type View =
  | "overview"
  | "entities"
  | "atlas"
  | "localmaps"
  | "families"
  | "storyboards"
  | "writing"
  | "history"
  | "timeline"
  | "domains"
  | "outline"
  | "project"
  | "consistency"
  | "experiments"
  | "economy"
  | "access"
  | "live"
  | "types"
  | "notes";
const navigation: [View, string, string][] = [
  ["overview", "◈", "Overview"],
  ["entities", "♙", "Entities"],
  ["atlas", "⌖", "Atlas"],
  ["localmaps", "⌗", "Local maps & tactics"],
  ["families", "♧", "Families & connections"],
  ["writing", "✎", "Writing"],
  ["history", "◷", "History"],
  ["timeline", "◴", "Timeline"],
  ["domains", "♧", "Societies & systems"],
  ["outline", "▦", "Story architecture"],
  ["storyboards", "▤", "Storyboards"],
  ["consistency", "✓", "Consistency"],
  ["project", "↗", "Project & export"],
  ["experiments", "⌁", "Experiments"],
  ["economy", "⇄", "Economy & production"],
  ["access", "♙", "Workspace access"],
  ["live", "⇄", "Shared writing"],
  ["notes", "▤", "Research & notes"],
  ["types", "◇", "Entity types"],
];
export default function Workspace() {
  const [needsLogin, setNeedsLogin] = useState(false);
  const [hosted, setHosted] = useState(false);
  const [state, setState] = useState<State | null>(null),
    [projects, setProjects] = useState<World[]>([]),
    [library, setLibrary] = useState(""),
    [loading, setLoading] = useState(true),
    [busy, setBusy] = useState(false),
    [draftPending, setDraftPending] = useState(false),
    [error, setError] = useState(""),
    [view, setView] = useState<View>("overview"),
    [selected, setSelected] = useState(""),
    [query, setQuery] = useState(""),
    [newAge, setNewAge] = useState(""),
    [copying, setCopying] = useState(false);
  const latest = useRef<State | null>(null),
    mutating = useRef(false);
  latest.current = state;
  const install = useCallback((next: State) => {
    if (latest.current?.age.id !== next.age.id) window.scrollTo(0, 0);
    latest.current = next;
    setState(next);
    const url = new URL(window.location.href);
    url.searchParams.set("world", next.root.world.id);
    url.searchParams.set("age", next.age.id);
    window.history.replaceState(null, "", url);
  }, []);
  const load = useCallback(
    async (world: string, age = "") => {
      setLoading(true);
      setError("");
      try {
        const next = await api<State>(`/projects/${world}/state?age=${age}`);
        install(next);
        return true;
      } catch (e) {
        setError((e as Error).message);
        return false;
      } finally {
        setLoading(false);
      }
    },
    [install],
  );
  useEffect(() => {
    let cancelled = false;
    async function init() {
      try {
        const session = await api<{
          library: string;
          hosted?: boolean;
          authenticated?: boolean;
        }>("/session");
        setHosted(!!session.hosted);
        if (session.hosted && !session.authenticated) {
          setNeedsLogin(true);
          return;
        }
        const list = await api<World[]>("/projects");
        if (cancelled) return;
        setLibrary(session.library);
        setProjects(list);
        const p = new URLSearchParams(location.search);
        if (p.get("world")) await load(p.get("world")!, p.get("age") || "");
        const requested = p.get("view") as View;
        if (navigation.some(([id]) => id === requested)) setView(requested);
      } catch (e) {
        setError((e as Error).message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void init();
    return () => {
      cancelled = true;
    };
  }, [load]);
  const run = useCallback(
    async (command: Command) => {
      const current = latest.current;
      if (current?.readOnly) {
        setError("This account has read-only access to the world.");
        return false;
      }
      if (!current || mutating.current) return false;
      mutating.current = true;
      setBusy(true);
      setError("");
      try {
        const next = await api<State>(
          `/projects/${current.root.world.id}/commands`,
          {
            method: "POST",
            body: JSON.stringify({
              ...command,
              expected: current.revision,
              age: current.age.id,
            }),
          },
        );
        install(next);
        return true;
      } catch (e) {
        setError((e as Error).message);
        return false;
      } finally {
        mutating.current = false;
        setBusy(false);
      }
    },
    [install],
  );
  const navigate = (v: View) => {
    setView(v);
    window.scrollTo(0, 0);
    const url = new URL(location.href);
    url.searchParams.set("view", v);
    window.history.replaceState(null, "", url);
  };
  const inspect = (id: string) => {
    setSelected(id);
    navigate("entities");
  };
  const exit = async () => {
    setState(null);
    latest.current = null;
    setSelected("");
    setQuery("");
    history.replaceState(null, "", "/");
    try {
      setProjects(await api<World[]>("/projects"));
    } catch (e) {
      setError((e as Error).message);
    }
  };
  if (needsLogin) return <Login />;
  if (!state)
    return (
      <div className="library">
        <header className="library-header">
          <Brand />
          <span className="badge">
            Development · {hosted ? "Shared workspace" : "Local workspace"}
          </span>
        </header>
        <main className="library-main">
          {hosted && <AccountSecurity />}
          <div className="eyebrow">Worlds worth remembering</div>
          <h1>
            Build a world.
            <br />
            <em>Keep its history.</em>
          </h1>
          <p className="lead">
            Maps, people and words, connected across independent Ages.
            <br />
            Your imagination. Your files. Entirely yours.
          </p>
          {error && (
            <p className="error" role="alert">
              {error}
            </p>
          )}
          <div className="library-grid">
            <section className="card">
              <h2>Create a world</h2>
              <CreateWorld
                disabled={loading || busy}
                create={async (name, age) => {
                  setBusy(true);
                  setError("");
                  try {
                    install(
                      await api<State>("/projects", {
                        method: "POST",
                        body: JSON.stringify({ name, age }),
                      }),
                    );
                    setView("overview");
                  } catch (e) {
                    setError((e as Error).message);
                  } finally {
                    setBusy(false);
                  }
                }}
              />
            </section>
            <section className="card">
              <h2>
                Your library <span>{projects.length}</span>
              </h2>
              {loading ? (
                <p>Opening library…</p>
              ) : projects.length ? (
                projects.map((p) => (
                  <button
                    className="world-card"
                    key={p.id}
                    onClick={() => void load(p.id)}
                  >
                    <span>◈</span>
                    <strong>{p.name}</strong>
                    <span>Open →</span>
                  </button>
                ))
              ) : (
                <p className="muted">
                  Your first world will appear here. You can return to it at any
                  time.
                </p>
              )}
              <details>
                <summary>Where are my files?</summary>
                <p className="mono">{library}</p>
                <p>
                  Each world is a folder in this library. Import a native backup
                  below to validate and migrate it into a separate copy. Choose
                  another library with the server’s <code>-data</code> option.
                </p>
              </details>
            </section>
          </div>
          <p className="library-footer">
            {hosted
              ? "Your shared workspace. "
              : "No account or cloud connection required. "}
            No AI. Your tools, your world.
          </p>
          <ImportPanel />
        </main>
      </div>
    );
  const ages = Object.values(state.root.ages).sort((a, b) =>
    a.name.localeCompare(b.name),
  );
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <Brand />
        <button
          className="back-library"
          disabled={busy || draftPending}
          onClick={() => void exit()}
        >
          ← World library
        </button>
        <div className="world-label">
          <small>WORLD</small>
          <strong>{state.root.world.name}</strong>
        </div>
        <label className="age-picker">
          CURRENT AGE
          <select
            aria-label="Current Age"
            disabled={busy || loading || draftPending}
            value={state.age.id}
            onChange={(e) => void load(state.root.world.id, e.target.value)}
          >
            {ages.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </select>
        </label>
        <button
          className="copy-age"
          disabled={busy || draftPending}
          onClick={() => setCopying(!copying)}
        >
          + Begin a new Age
        </button>
        {copying && (
          <form
            className="copy-form"
            onSubmit={async (e) => {
              e.preventDefault();
              if (await run({ action: "copy-age", name: newAge })) {
                setCopying(false);
                setNewAge("");
              }
            }}
          >
            <label>
              New Age name
              <input
                autoFocus
                required
                value={newAge}
                onChange={(e) => setNewAge(e.target.value)}
              />
            </label>
            <small>Copies the complete saved state of {state.age.name}.</small>
            <button disabled={busy || draftPending}>
              Create independent copy
            </button>
          </form>
        )}
        <nav aria-label="Workspaces">
          {navigation.map(([id, icon, label]) => (
            <button
              key={id}
              aria-current={view === id ? "page" : undefined}
              onClick={() => navigate(id)}
              aria-label={label}
              disabled={busy || draftPending}
            >
              <span>{icon}</span>
              {label}
            </button>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <span className="online-dot" />{" "}
          {hosted ? "Shared workspace" : "Local & private"}
          <small>History stays in your hands.</small>
        </div>
      </aside>
      <main className="workspace">
        <header className="topbar">
          <div className="breadcrumb">
            {state.root.world.name}
            <span>/</span>
            <strong>{state.age.name}</strong>
            <span>/</span>
            {navigation.find(([id]) => id === view)?.[2]}
          </div>
          <div className="top-actions">
            <span role="status" className="muted">
              {busy
                ? "Saving…"
                : loading
                  ? "Opening…"
                  : draftPending
                    ? "Unsaved draft pending"
                    : hosted
                      ? "Saved to workspace"
                      : "Saved locally"}
            </span>
            <button
              title="Undo last saved change in this Age"
              disabled={busy || draftPending || !state.age.undo?.length}
              onClick={() => void run({ action: "undo" })}
            >
              Undo
            </button>
            <button
              title="Redo saved change"
              disabled={busy || draftPending || !state.age.redo?.length}
              onClick={() => void run({ action: "redo" })}
            >
              Redo
            </button>
            <button
              disabled={busy || draftPending}
              onClick={() => void load(state.root.world.id, state.age.id)}
            >
              Reload
            </button>
          </div>
        </header>
        <div className="searchbar">
          <span>⌕</span>
          <input
            aria-label="Search this Age"
            disabled={draftPending}
            placeholder="Search names, notes, properties and manuscripts in this Age…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          {query && <button onClick={() => setQuery("")}>Clear</button>}
        </div>
        {error && (
          <div className="error-banner" role="alert">
            <strong>Change not saved.</strong> {error}{" "}
            <button
              onClick={() => void load(state.root.world.id, state.age.id)}
            >
              Reload saved state
            </button>
          </div>
        )}
        {state.recovered && (
          <div className="notice">
            The project recovered its last complete save after an interrupted
            write. Your previous state is available.
          </div>
        )}
        {query ? (
          <SearchResults
            key={`${state.root.world.id}:${state.age.id}:${state.revision}:${query.trim()}`}
            state={state}
            query={query.trim()}
            open={async (r, age) => {
              if (
                age &&
                age !== state.age.id &&
                !(await load(state.root.world.id, age))
              )
                return;
              setQuery("");
              setSelected(r.kind === "relation" ? r.from || "" : r.id);
              navigate(
                r.kind === "scene"
                  ? "writing"
                  : r.kind === "map"
                    ? "atlas"
                    : r.kind === "schema"
                      ? "types"
                      : r.kind === "note"
                        ? "notes"
                        : r.kind === "event"
                          ? "timeline"
                          : "entities",
              );
            }}
          />
        ) : (
          <div className="workspace-content" inert={loading}>
            {view === "live" && (
              <LiveWriting
                key={state.age.id}
                state={state}
                run={run}
                install={install}
              />
            )}
            {state.readOnly && (
              <p className="notice">
                Read-only world access. Your account can inspect and export this
                world.
              </p>
            )}
            {view === "access" && <Access state={state} />}
            {view === "storyboards" && (
              <Storyboards
                key={state.age.id}
                onPending={setDraftPending}
                state={state}
                run={run}
                busy={busy}
              />
            )}
            {view === "families" && (
              <FamilyGraph key={state.age.id} state={state} />
            )}
            {view === "localmaps" && (
              <LocalMaps
                key={state.age.id}
                onPending={setDraftPending}
                state={state}
                run={run}
                busy={busy}
              />
            )}
            {view === "experiments" && (
              <Experiments
                key={state.age.id}
                state={state}
                run={run}
                busy={busy}
              />
            )}
            {view === "economy" && (
              <Economy
                key={state.age.id}
                state={state}
                run={run}
                busy={busy}
                onPending={setDraftPending}
              />
            )}
            {view === "project" && (
              <ProjectTools
                key={state.age.id}
                state={state}
                run={run}
                busy={busy}
              />
            )}
            {view === "consistency" && (
              <Consistency
                key={state.age.id}
                state={state}
                run={run}
                busy={busy}
              />
            )}
            {view === "domains" && (
              <Domains
                key={state.age.id}
                state={state}
                run={run}
                busy={busy}
                onPending={setDraftPending}
              />
            )}
            {view === "outline" && (
              <Outline
                key={state.age.id}
                state={state}
                run={run}
                busy={busy}
                onPending={setDraftPending}
              />
            )}
            {view === "timeline" && (
              <Timeline
                key={state.age.id}
                state={state}
                run={run}
                busy={busy}
              />
            )}
            {view === "overview" && (
              <Overview
                state={state}
                navigate={navigate}
                run={run}
                busy={busy}
              />
            )}{" "}
            {view === "entities" && (
              <Entities
                onPending={setDraftPending}
                key={state.age.id}
                state={state}
                selected={selected}
                select={setSelected}
                run={run}
                busy={busy}
              />
            )}{" "}
            {view === "atlas" && (
              <Atlas
                selected={selected}
                key={state.age.id}
                state={state}
                run={run}
                busy={busy}
                inspect={inspect}
              />
            )}{" "}
            {view === "writing" && (
              <Writing
                selected={selected}
                pending={draftPending}
                onPending={setDraftPending}
                key={state.age.id}
                state={state}
                run={run}
                busy={busy}
                inspect={inspect}
              />
            )}{" "}
            {view === "history" && (
              <History key={state.age.id} state={state} run={run} busy={busy} />
            )}{" "}
            {view === "types" && (
              <Types
                initialSelection={selected}
                key={state.age.id}
                state={state}
                run={run}
                busy={busy}
                onPending={setDraftPending}
              />
            )}{" "}
            {view === "notes" && (
              <Notebook
                onPending={setDraftPending}
                initialSelection={selected}
                key={state.age.id}
                state={state}
                run={run}
                busy={busy}
              />
            )}
          </div>
        )}
      </main>
    </div>
  );
}
function Brand() {
  return (
    <div className="brand">
      <span className="brand-icon">K</span>
      <div>
        KRIEMHILD<small>WORLD & WORD</small>
      </div>
    </div>
  );
}
function CreateWorld({
  create,
  disabled,
}: {
  create: (name: string, age: string) => Promise<void>;
  disabled: boolean;
}) {
  const [name, setName] = useState(""),
    [age, setAge] = useState("First Age");
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        void create(name, age);
      }}
    >
      <label>
        World name
        <input
          required
          maxLength={300}
          placeholder="A name for your world"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </label>
      <label>
        First Age
        <input
          required
          maxLength={300}
          value={age}
          onChange={(e) => setAge(e.target.value)}
        />
      </label>
      <button className="primary" disabled={disabled}>
        Create world →
      </button>
    </form>
  );
}
function Overview({
  state,
  navigate,
  run,
  busy,
}: {
  state: State;
  navigate: (v: View) => void;
  run: (c: Command) => Promise<boolean>;
  busy: boolean;
}) {
  const records = Object.values(state.records),
    [name, setName] = useState(state.age.name);
  useEffect(() => setName(state.age.name), [state.age.id, state.age.name]);
  return (
    <div className="overview">
      <div className="eyebrow">An independent chapter of your world</div>
      <h1>{state.age.name}</h1>
      <p className="lead">
        {state.age.sourceAge
          ? `Inherited from ${state.root.ages[state.age.sourceAge]?.name}. Everything you change here belongs to this Age alone.`
          : "A blank beginning. Give it places, people and stories at your own pace."}
      </p>
      <div className="stat-grid">
        {(
          [
            ["entity", "Entities", "entities"],
            ["map", "Maps", "atlas"],
            ["scene", "Scenes", "writing"],
            ["relation", "Relationships", "entities"],
          ] as const
        ).map(([kind, label, target]) => (
          <button className="stat" key={kind} onClick={() => navigate(target)}>
            <strong>{records.filter((r) => r.kind === kind).length}</strong>
            <span>{label} ↗</span>
          </button>
        ))}
      </div>
      <div className="journey-grid">
        <button className="journey" onClick={() => navigate("entities")}>
          <span>01 / PEOPLE & PLACES</span>
          <h2>Give an idea a home.</h2>
          <p>
            Create a village, a person or a custom entity. Connect it to the
            rest of your world.
          </p>
        </button>
        <button className="journey" onClick={() => navigate("atlas")}>
          <span>02 / THE ATLAS</span>
          <h2>Put it on the map.</h2>
          <p>
            Import an image or begin with a blank map. Place and move
            entity-linked pins.
          </p>
        </button>
        <button className="journey" onClick={() => navigate("writing")}>
          <span>03 / YOUR WORDS</span>
          <h2>Tell its story.</h2>
          <p>
            Write scenes with a preserved historical setting and your lore close
            at hand.
          </p>
        </button>
      </div>
      <div className="card">
        <h2>Age details</h2>
        <form
          className="inline"
          onSubmit={(e) => {
            e.preventDefault();
            void run({ action: "rename-age", name });
          }}
        >
          <label>
            Age name
            <input
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <button disabled={busy}>Rename Age</button>
        </form>
        <p className="muted">
          Saved {new Date(state.root.savedAt).toLocaleString()} ·{" "}
          {state.age.undo?.length || 0} undo steps
        </p>
        <details>
          <summary>Saved snapshot & provenance</summary>
          <p className="mono">
            Snapshot: {state.age.snapshot}
            <br />
            Revision: {state.revision}
            <br />
            Source: {state.age.sourceSnapshot || "Original Age"}
          </p>
          <p>
            Scenes keep their original setting when copied. Use the scene’s
            retarget action to deliberately adopt this Age.
          </p>
        </details>
      </div>
    </div>
  );
}
