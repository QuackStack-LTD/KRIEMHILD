import {
  confirmAction,
  requestText,
  allowNavigation,
} from "./worldbuilding/ui/feedback.js";
import Dialog from "./worldbuilding/ui/Dialog.jsx";
import AccountControls from "./auth/AccountControls.jsx";
import { lazy, Suspense } from "react";
const LibraryPage = lazy(() => import("./worldbuilding/Library.jsx"));
function Library(props) {
  return (
    <Suspense fallback={<main className="world-page">Loading Library...</main>}>
      <LibraryPage {...props} />
    </Suspense>
  );
}
const SystemDesignerPage = lazy(
  () => import("./worldbuilding/SystemDesigner.jsx"),
);
function SystemDesigner(props) {
  return (
    <Suspense
      fallback={<main className="world-page">Loading System Designer...</main>}
    >
      <SystemDesignerPage {...props} />
    </Suspense>
  );
}
const BiospherePage = lazy(() => import("./worldbuilding/Biosphere.jsx"));
const LifeformPage = lazy(() => import("./worldbuilding/LifeformProfile.jsx"));
function Biosphere(props) {
  return (
    <Suspense
      fallback={<main className="world-page">Loading Biosphere...</main>}
    >
      <BiospherePage {...props} />
    </Suspense>
  );
}
function LifeformProfile(props) {
  return (
    <Suspense
      fallback={<main className="world-page">Loading lifeform...</main>}
    >
      <LifeformPage {...props} />
    </Suspense>
  );
}

import Culture, { ConceptProfile } from "./worldbuilding/Culture.jsx";
import { registry } from "./worldbuilding/model.js";
import NavigationIcon from "./worldbuilding/NavigationIcon.jsx";
import Organizations, {
  OrganizationProfile,
} from "./worldbuilding/Organizations.jsx";
import { useEffect, useState } from "react";
import TerrainApp from "./TerrainApp.jsx";
import {
  historyRequest,
  graphLayout,
  edgePath,
  timelineWindow,
  mergeConflicts,
  ageActionSource,
} from "./history.js";
import "./application.css";
import "./history.css";
import { Atlas, Characters } from "./worldbuilding/Workspace.jsx";
import Profile from "./worldbuilding/Profile.jsx";
import Networks from "./worldbuilding/Networks.jsx";
const Writing = lazy(() => import("./worldbuilding/Writing.jsx"));
import YearTimeline from "./worldbuilding/YearTimeline.jsx";
import "./worldbuilding/years.css";

function TimelineBar({ doc, onSelect, onNew, onGraph, onYears }) {
  const age = doc.ages.find((a) => a.id === doc.world.currentAge),
    timeline = doc.timelines.find((t) => t.id === age?.timelineId);
  if (!timeline) return null;
  const { index, ages, before, after } = timelineWindow(timeline, age.id),
    byID = new Map(doc.ages.map((a) => [a.id, a]));
  return (
    <footer className="age-bar">
      <button onClick={onYears}>Years</button>
      <button onClick={onGraph}>
        Time Graph <small>{timeline.name}</small>
      </button>
      <button
        aria-label="Previous Age"
        disabled={index === 0}
        onClick={() => onSelect(timeline.ages[index - 1])}
      >
        &larr;
      </button>
      <div
        className="age-window"
        onWheel={(e) => {
          if (
            Math.abs(e.deltaX) > Math.abs(e.deltaY) &&
            Math.abs(e.deltaX) > 20
          ) {
            const next = timeline.ages[index + Math.sign(e.deltaX)];
            if (next) onSelect(next);
          }
        }}
      >
        <div
          className="age-track"
          style={{
            "--line-start": before ? "0%" : `${50 - Math.min(2, index) * 19}%`,
            "--line-end":
              timeline.ages.length - index <= 2
                ? `${50 + (timeline.ages.length - index) * 19}%`
                : "100%",
          }}
        >
          {before && (
            <span
              className="age-overflow left"
              aria-label="Earlier Ages continue"
            >
              &hellip;
            </span>
          )}
          {ages.map((id) => {
            const offset = timeline.ages.indexOf(id) - index;
            return (
              <button
                key={id}
                style={{ left: `${50 + offset * 19}%` }}
                className={`age-node ${id === age.id ? "current" : ""}`}
                aria-current={id === age.id ? "step" : undefined}
                onClick={() => onSelect(id)}
              >
                <span className="age-circle" />
                <span>{byID.get(id)?.name}</span>
                {doc.edges.some(
                  (e) => e.source === id && e.kind !== "linear",
                ) && <i className="branch-stub" />}
                {doc.edges.some(
                  (e) => e.destination === id && e.kind === "merge",
                ) && <i className="merge-stub" />}
              </button>
            );
          })}
          {timeline.ages.length - index <= 2 && (
            <button
              className="age-new"
              style={{ left: `${50 + (timeline.ages.length - index) * 19}%` }}
              onClick={onNew}
            >
              New Age +
            </button>
          )}
          {after && <span className="age-overflow right">&hellip;</span>}
        </div>
      </div>
      <button
        aria-label="Next Age"
        disabled={index === timeline.ages.length - 1}
        onClick={() => onSelect(timeline.ages[index + 1])}
      >
        &rarr;
      </button>
      {timeline.ages.length - index > 2 && (
        <button onClick={onNew}>New Age</button>
      )}
    </footer>
  );
}
function AgeDialog({ doc, sourceAge, sourceYear, onClose, onSubmit, busy }) {
  const [name, setName] = useState(""),
    [mode, setMode] = useState(sourceYear ? "branch" : "after"),
    [other, setOther] = useState(
      doc.ages.find((a) => a.id !== sourceAge)?.id || "",
    ),
    [policy, setPolicy] = useState(""),
    [excluded, setExcluded] = useState([]),
    [entityNames, setEntityNames] = useState({});
  const inherited = [
    ...new Map(
      doc.entities
        .filter(
          (e) =>
            (e.ageId === sourceAge ||
              (mode === "merge" && e.ageId === other)) &&
            !e.deleted,
        )
        .map((e) => [e.id, e]),
    ).values(),
  ];
  const conflicts =
    mode === "merge" ? mergeConflicts(doc, sourceAge, other) : [];
  return (
    <Dialog
      title={<>Continue this history</>}
      size="editor"
      className="age-dialog"
      onClose={onClose}
      busy={busy}
      onSubmit={(e) => {
        e.preventDefault();
        return onSubmit({
          kind: mode === "connect" ? "connect" : "age",
          ageId: sourceAge,
          sourceYearId: sourceYear?.id || "",
          name,
          mode,
          otherAge: other,
          policy,
          excludeIds: excluded.filter((id) =>
            inherited.some((e) => e.id === id),
          ),
          entityNames: Object.fromEntries(
            Object.entries(entityNames).filter(
              ([id]) =>
                !excluded.includes(id) && inherited.some((e) => e.id === id),
            ),
          ),
        });
      }}
      actions={
        <>
          <button type="button" onClick={onClose} data-dialog-dismiss>
            Cancel
          </button>
          <button className="primary" disabled={busy}>
            Apply
          </button>
        </>
      }
    >
      <p>
        From <strong>{doc.ages.find((a) => a.id === sourceAge)?.name}</strong>
        {sourceYear && <> / Year {sourceYear.number}</>}
      </p>
      {sourceYear && (
        <p>
          The new Age records this year as its origin and inherits the current
          saved state. Earlier events are not used to rewind terrain or
          entities.
        </p>
      )}
      <label>
        Operation
        <select value={mode} onChange={(e) => setMode(e.target.value)}>
          <option value="after">Continue after this Age</option>
          {!sourceYear && (
            <option value="before">Insert before this Age</option>
          )}
          <option value="branch">Branch into another timeline</option>
          <option value="parallel">
            Start an independent parallel timeline
          </option>
          {!sourceYear && (
            <>
              <option value="merge">Merge histories into a new Age</option>
              <option value="connect">Connect to an existing Age</option>
            </>
          )}
        </select>
      </label>
      {mode !== "connect" && (
        <label>
          Age name
          <input
            type="text"
            autoFocus
            required
            maxLength={160}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </label>
      )}
      {["merge", "connect"].includes(mode) && (
        <label>
          {mode === "merge" ? "Other source Age" : "Destination Age"}
          <select
            required
            value={other}
            onChange={(e) => setOther(e.target.value)}
          >
            <option value="">Choose an Age</option>
            {doc.ages
              .filter((a) => mode === "connect" || a.id !== sourceAge)
              .map((a) => (
                <option value={a.id} key={a.id}>
                  {a.name}
                </option>
              ))}
          </select>
        </label>
      )}
      {mode === "merge" && (
        <details>
          <summary>{conflicts.length} conflicting map or entity states</summary>
          <ul>
            {conflicts.map((c) => (
              <li key={c.kind + c.id}>
                {c.name} - {c.kind}
              </li>
            ))}
          </ul>
        </details>
      )}
      {mode === "merge" && (
        <label>
          Conflicting state
          <select
            required
            value={policy}
            onChange={(e) => setPolicy(e.target.value)}
          >
            <option value="">Choose how to resolve conflicts</option>
            <option value="primary">
              Keep current Age's conflicting maps and entities
            </option>
            <option value="secondary">
              Keep the other Age's conflicting maps and entities
            </option>
          </select>
          <small>
            Unique content from both Ages is retained. For matching identities,
            this choice selects the map snapshot, geometry and properties.
            Earlier Ages stay unchanged.
          </small>
        </label>
      )}
      <p>
        {mode === "parallel"
          ? "Copies the current Age into an independent timeline without a chronological connection."
          : mode === "connect"
            ? "Adds a chronological merge link without changing either Age’s saved content. Histories may loop back to earlier Ages."
            : "New Ages inherit the saved terrains, maps and entities of their source Age."}
      </p>
      {mode !== "connect" && (
        <details>
          <summary>
            Review inherited entities (
            {inherited.filter((e) => !excluded.includes(e.id)).length} of{" "}
            {inherited.length} included)
          </summary>
          <p>
            Maps and terrain are inherited. Uncheck entities to remove them from
            the new Age; the source is preserved. Open their new profiles after
            creation to make changes.
          </p>
          <button type="button" onClick={() => setExcluded([])}>
            Include all
          </button>
          <button
            type="button"
            onClick={() => setExcluded(inherited.map((e) => e.id))}
          >
            Exclude all
          </button>
          <div className="inheritance-review">
            {inherited.map((e) => (
              <label key={e.id}>
                <input
                  type="checkbox"
                  checked={!excluded.includes(e.id)}
                  onChange={(ev) =>
                    setExcluded((ids) =>
                      ev.target.checked
                        ? ids.filter((id) => id !== e.id)
                        : [...ids, e.id],
                    )
                  }
                />
                {e.name}{" "}
                <small>
                  {e.kind} /{" "}
                  {excluded.includes(e.id)
                    ? "removed"
                    : entityNames[e.id] && entityNames[e.id] !== e.name
                      ? "modified"
                      : doc.entities.some(
                            (x) =>
                              x.ageId === sourceAge &&
                              x.id === e.id &&
                              !x.deleted,
                          )
                        ? "carried forward"
                        : "added from merge"}
                </small>
                <input
                  type="text"
                  aria-label={`Name ${e.name} in new Age`}
                  disabled={excluded.includes(e.id)}
                  value={entityNames[e.id] ?? e.name}
                  maxLength={200}
                  onChange={(ev) =>
                    setEntityNames((names) => ({
                      ...names,
                      [e.id]: ev.target.value,
                    }))
                  }
                />
              </label>
            ))}
          </div>
        </details>
      )}
    </Dialog>
  );
}
export default function App() {
  const [path, setPath] = useState(location.pathname + location.search),
    [doc, setDoc] = useState(null),
    [worlds, setWorlds] = useState([]),
    [legacy, setLegacy] = useState([]),
    [name, setName] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [dialog, setDialog] = useState(false),
    [sourceYear, setSourceYear] = useState(null),
    [zoom, setZoom] = useState(1),
    [selected, setSelected] = useState(null),
    [navOpen, setNavOpen] = useState(false);
  const route = path
      .split("?")[0]
      .match(
        /^\/worlds\/([a-f0-9]{32})(?:\/(graph|terrains|atlas|characters|profile|networks|years|organizations|culture|flora|biosphere|magic|library|writing))?\/?$/,
      ),
    id = route?.[1],
    page = route?.[2] || "world",
    query = new URLSearchParams(path.split("?")[1] || "");
  async function navigate(url) {
    if (!(await allowNavigation())) return;
    setNavOpen(false);
    history.pushState({}, "", url);
    setPath(location.pathname + location.search);
    setError("");
  }
  useEffect(() => {
    const pop = async () => {
      const next = location.pathname + location.search;
      if (await allowNavigation()) {
        setNavOpen(false);
        setPath(next);
      } else history.pushState({}, "", path);
    };
    window.addEventListener("popstate", pop);
    return () => window.removeEventListener("popstate", pop);
  }, [path]);
  async function run(fn) {
    setBusy(true);
    setError("");
    try {
      return await fn();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  useEffect(() => {
    let cancelled = false;
    run(async () => {
      if (path.startsWith("/world/")) return;
      if (id) {
        const d = await historyRequest(`/${id}`);
        if (!cancelled) {
          setDoc(d);
          setSelected(d.world.currentAge);
        }
      } else {
        const [w, l] = await Promise.all([
          historyRequest(),
          fetch("/api/projects").then((r) => r.json()),
        ]);
        if (!cancelled) {
          setWorlds(w);
          setLegacy(l.projects || []);
        }
      }
    });
    return () => {
      cancelled = true;
    };
  }, [path]);
  if (path.startsWith("/world/"))
    return (
      <div className="terrain-app">
        <TerrainApp
          onExit={(worldID, entityID, ageID) =>
            navigate(
              worldID
                ? `/worlds/${worldID}${entityID ? `/profile?entity=${entityID}&age=${ageID}` : ""}`
                : "/",
            )
          }
        />
      </div>
    );
  async function command(c) {
    if (c.kind === "select") {
      if (!(await allowNavigation())) return;
      setDoc((d) => ({ ...d, world: { ...d.world, currentAge: c.ageId } }));
      setSelected(c.ageId);
      return { ...doc, world: { ...doc.world, currentAge: c.ageId } };
    }
    if (["select", "age"].includes(c.kind) && !(await allowNavigation()))
      return;
    return run(async () => {
      const d = await historyRequest(`/${id}`, {
        ...c,
        revision: doc.world.revision,
      });
      setDoc(d);
      setSelected(d.world.currentAge);
      return d;
    });
  }
  async function applyAge(c) {
    const d = await command(c);
    if (d) {
      setDialog(false);
      setSourceYear(null);
      if (
        c.sourceYearId ||
        ["branch", "parallel", "merge", "connect"].includes(c.mode)
      )
        navigate(`/worlds/${id}/graph`);
    }
  }
  async function select(age) {
    if (busy) return;
    setSelected(age);
    const d = await command({ kind: "select", ageId: age });
    if (!d) setSelected(doc.world.currentAge);
    else if (page === "networks") {
      query.set("age", age);
      navigate(`/worlds/${id}/networks?${query}`);
    }
  }
  const editorURL = (project, age, view = "build") =>
    `/world/${project}/${view}?world=${id}&age=${age}`;
  const openMap = async (m) => {
    if (m.ageId !== doc.world.currentAge) {
      const d = await command({ kind: "select", ageId: m.ageId });
      if (!d) return;
    }
    navigate(editorURL(m.projectId, m.ageId));
  };
  const openSettlement = (entity) =>
    run(async () => {
      let d = await historyRequest(`/${id}`);
      let m = d.maps.find(
        (m) =>
          m.ageId === entity.ageId && m.entityId === entity.id && m.parentId,
      );
      if (!m) {
        const reps = d.representations.filter(
            (r) => r.ageId === entity.ageId && r.entityId === entity.id,
          ),
          r = reps.find((r) => r.mapId === entity.homeMap) || reps[0];
        if (!r)
          throw Error("Draw this place on a terrain in Map Builder first.");
        const parent = d.maps.find(
          (m) => m.ageId === entity.ageId && m.id === r.mapId,
        );
        const result = await historyRequest(`/${id}/child`, {
          ageId: entity.ageId,
          mapId: parent.id,
          entityId: entity.id,
          name: `${entity.name} - settlement map`,
          bounds: parent.bounds,
          revision: d.world.revision,
        });
        d = result.document;
        m = result.map;
        setDoc(d);
      }
      navigate(editorURL(m.projectId, m.ageId) + `&returnEntity=${entity.id}`);
    });
  const openYears = async (ageID = doc.world.currentAge, yearID = "") => {
    if (
      ageID !== doc.world.currentAge &&
      !(await command({ kind: "select", ageId: ageID }))
    )
      return;
    navigate(
      `/worlds/${id}/years${yearID ? `?year=${encodeURIComponent(yearID)}` : ""}`,
    );
  };
  const openProfile = async (e) => {
    if (!e) return;
    if (
      e.kind === "archive" ||
      doc.libraryFiles?.some((f) => f.id === e.id && f.ageId === e.ageId)
    ) {
      if (
        e.ageId !== current.id &&
        !(await command({ kind: "select", ageId: e.ageId }))
      )
        return;
      navigate(`/worlds/${id}/library?record=${e.id}`);
      return;
    }
    if (registry.types[e.kind]?.category === "systems") {
      if (
        e.ageId !== current.id &&
        !(await command({ kind: "select", ageId: e.ageId }))
      )
        return;
      navigate(
        `/worlds/${id}/magic?system=${e.kind === "system" ? e.id : e.system?.systemId}&record=${e.id}`,
      );
      return;
    }
    const owner =
      registry.types[e.kind]?.category === "institutional"
        ? doc.entities.find(
            (o) => o.ageId === e.ageId && o.id === e.properties?.organizationId,
          )
        : null;
    navigate(
      `/worlds/${id}/profile?entity=${owner?.id || e.id}&age=${e.ageId}${owner ? "&record=" + e.id : ""}`,
    );
  };
  const openGraph = async (n) => {
    if (n.ageId && n.ageId !== doc.world.currentAge) {
      if (!(await command({ kind: "select", ageId: n.ageId }))) return;
    }
    navigate(
      `/worlds/${id}/networks?${new URLSearchParams({ view: n.id || "", age: n.ageId || doc.world.currentAge, center: n.centerId || "", family: n.familyId || "", genealogy: n.genealogy ? "1" : "0", mode: n.mode || (n.genealogy ? "family" : "social"), type: n.relationTypes?.[0] || "" })}`,
    );
  };
  const generate = (age) => navigate(`/world/new?world=${id}&age=${age}`);
  const exportWorld = () =>
    run(async () => {
      const response = await fetch(`/api/worlds/${id}/export`);
      if (!response.ok) throw Error((await response.json()).error);
      const url = URL.createObjectURL(await response.blob()),
        a = document.createElement("a");
      a.href = url;
      a.download = `${doc.world.name}.world.zip`;
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    });
  const importWorld = (file) =>
    run(async () => {
      if (!file) return;
      const r = await fetch("/api/worlds/import", {
          method: "POST",
          headers: { "Content-Type": "application/zip" },
          body: file,
        }),
        d = await r.json();
      if (!r.ok) throw Error(d.error);
      navigate(`/worlds/${d.world.id}`);
    });
  const removeMap = async (m) => {
    if (
      await confirmAction(
        `Delete ${m.name} and its child maps from this Age? Other Ages are preserved.`,
      )
    )
      void command({ kind: "delete-map", ageId: m.ageId, mapId: m.id });
  };
  const current = doc?.ages.find((a) => a.id === doc.world.currentAge),
    preview = doc?.ages.find(
      (a) => a.id === (selected || doc.world.currentAge),
    ),
    layout = doc ? graphLayout(doc) : null;
  return (
    <div
      className={`history-app ${id ? "has-world" : ""} ${page === "graph" ? "showing-graph" : ""} ${page === "profile" ? "profile-page" : ""} ${page === "writing" ? "writing-page" : ""}`}
    >
      <header className="application-nav">
        {id && page !== "writing" && (
          <button
            className="mobile-navigation"
            aria-expanded={navOpen}
            aria-controls="world-navigation"
            onClick={() => setNavOpen(!navOpen)}
          >
            {navOpen ? "Close menu" : "Menu"}
          </button>
        )}
        <button className="brand" onClick={() => navigate("/")}>
          KRIEMHILD <span>/ Worlds</span>
        </button>
        {id && doc?.world.id === id && (
          <>
            <span className="active-world-name">{doc.world.name}</span>
          </>
        )}
        {busy && <span role="status">Working...</span>}
        <AccountControls
          worldId={id}
          revision={doc?.world.revision || 0}
          onRefresh={async () => {
            if (!(await allowNavigation())) return;
            const next = await historyRequest(`/${id}`);
            setDoc(next);
          }}
        />
      </header>
      {id && page !== "writing" && doc?.world.id === id && (
        <nav
          id="world-navigation"
          className={`world-sidebar ${navOpen ? "is-open" : ""}`}
          aria-label="World navigation"
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              setNavOpen(false);
              document.querySelector(".mobile-navigation")?.focus();
            }
          }}
        >
          {[
            ["world", "Atlas"],
            ["biosphere", "Biosphere"],
            ["magic", "Magic & Technology"],
            ["culture", "Culture & Religion"],
            ["characters", "Characters"],
            ["organizations", "Organizations"],
            ["library", "Library"],
          ].map(([key, title]) => (
            <button
              className="sidebar-section"
              key={key}
              aria-current={
                page === key ||
                (key === "world" && page === "atlas") ||
                (key === "biosphere" && page === "flora")
                  ? "page"
                  : undefined
              }
              onClick={() =>
                navigate(`/worlds/${id}${key === "world" ? "" : "/" + key}`)
              }
            >
              <NavigationIcon name={key} />
              {title}
            </button>
          ))}
          <div className="sidebar-bottom">
            <button
              className="sidebar-writing"
              onClick={() => navigate(`/worlds/${id}/writing`)}
            >
              <span>
                <NavigationIcon name="writing" />
                Writing
              </span>
              <small>Your drafts & notes</small>
            </button>
            <button onClick={exportWorld}>Export World ZIP</button>
          </div>
        </nav>
      )}
      {error && (
        <div className="shell-error" role="alert">
          {error}
          <button onClick={() => setError("")}>Dismiss</button>
        </div>
      )}
      {!id ? (
        <main className="project-hub">
          <div className="hub-intro">
            <span className="eyebrow">
              A place for worlds and their histories
            </span>
            <h1>KRIEMHILD</h1>
            <p>
              Create a fictional reality. Shape its terrain, build its
              civilizations, and explore the histories that could have been.
            </p>
          </div>
          <form
            className="hub-create"
            onSubmit={(e) => {
              e.preventDefault();
              run(async () => {
                const d = await historyRequest("", { name });
                navigate(`/worlds/${d.world.id}`);
              });
            }}
          >
            <input
              type="text"
              aria-label="New world name"
              required
              maxLength={160}
              placeholder="Name your world"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
            <button className="primary" disabled={busy}>
              Create New World
            </button>
            <label className="button">
              Import World ZIP
              <input
                hidden
                type="file"
                accept=".zip"
                onChange={(e) => importWorld(e.target.files[0])}
              />
            </label>
            <button
              type="button"
              onClick={() => navigate("/world/new?legacyImport=1")}
            >
              Import legacy terrain
            </button>
          </form>
          <h2>Continue World</h2>
          <div className="world-grid">
            {worlds.map((w) => (
              <article key={w.id}>
                <span className="world-glyph">&#9678;</span>
                <h3>{w.name}</h3>
                <p>{w.description || "A world of unfolding histories"}</p>
                {w.role && (
                  <small>
                    {w.role === "owner"
                      ? "Owned by you"
                      : `Shared with you · ${w.role}`}
                  </small>
                )}
                <div>
                  <button onClick={() => navigate(`/worlds/${w.id}`)}>
                    Continue World
                  </button>
                  <button
                    className="danger"
                    disabled={w.role && w.role !== "owner"}
                    onClick={async () => {
                      if (
                        await confirmAction(
                          `Delete ${w.name}, all its Ages and maps?`,
                        )
                      )
                        run(async () => {
                          await historyRequest(`/${w.id}`, undefined, "DELETE");
                          setWorlds(worlds.filter((x) => x.id !== w.id));
                        });
                    }}
                  >
                    Delete
                  </button>
                </div>
              </article>
            ))}
          </div>
          {worlds.length === 0 && (
            <p className="empty-worlds">Create your first World above.</p>
          )}
          {legacy.length > 0 && (
            <section>
              <h2>Existing terrain projects</h2>
              <p>
                Open an earlier map, then choose Move into a World. Its terrain
                and explored detail are preserved.
              </p>
              <div className="world-grid">
                {legacy.map((p) => (
                  <article key={p.id}>
                    <h3>{p.name}</h3>
                    <button onClick={() => navigate(`/world/${p.id}/build`)}>
                      Open legacy map
                    </button>
                  </article>
                ))}
              </div>
            </section>
          )}
        </main>
      ) : (
        doc?.world.id === id &&
        current && (
          <>
            {page === "graph" ? (
              <main className="time-graph-page">
                <section className="graph-panel">
                  <div className="section-heading">
                    <div>
                      <span className="eyebrow">
                        Branches, choices and converging histories
                      </span>
                      <h1>Time Graph</h1>
                    </div>
                    <button disabled={busy} onClick={() => setDialog(true)}>
                      New Age / Connect
                    </button>
                    <label>
                      Zoom{" "}
                      <input
                        aria-label="Time graph zoom"
                        type="range"
                        min=".35"
                        max="1.5"
                        step=".05"
                        value={zoom}
                        onChange={(e) => setZoom(Number(e.target.value))}
                      />
                    </label>
                  </div>
                  <div
                    className="time-graph-scroll"
                    tabIndex={0}
                    aria-label="Scrollable timeline graph"
                  >
                    <svg
                      width={layout.width * zoom}
                      height={layout.height * zoom}
                      viewBox={`0 0 ${layout.width} ${layout.height}`}
                    >
                      <defs>
                        <marker
                          id="age-arrow"
                          viewBox="0 0 10 10"
                          refX="20"
                          refY="5"
                          markerWidth="6"
                          markerHeight="6"
                          orient="auto-start-reverse"
                        >
                          <path d="M 0 0 L 10 5 L 0 10 z" fill="#b9b690" />
                        </marker>
                      </defs>
                      {doc.timelines.map((t, i) => (
                        <g key={t.id}>
                          <text x="20" y={48 + i * 140} className="lane-name">
                            {t.name}
                          </text>
                          <path
                            d={`M 80 ${90 + i * 140} H ${layout.width - 40}`}
                            className="graph-lane"
                          />
                        </g>
                      ))}
                      {doc.edges.map((e) => (
                        <path
                          key={e.id}
                          d={edgePath(
                            layout.positions.get(e.source),
                            layout.positions.get(e.destination),
                          )}
                          className={`graph-edge ${e.kind}`}
                          markerEnd="url(#age-arrow)"
                        >
                          <title>{e.kind}</title>
                        </path>
                      ))}
                      {doc.ages.map((a) => {
                        const p = layout.positions.get(a.id);
                        return (
                          <g
                            key={a.id}
                            className={`graph-age ${a.id === selected ? "selected" : ""}`}
                            role="button"
                            tabIndex={0}
                            aria-label={`Select Age ${a.name}`}
                            aria-pressed={a.id === selected}
                            onClick={() => select(a.id)}
                            onKeyDown={(e) => {
                              if (e.key === "Enter" || e.key === " ") {
                                e.preventDefault();
                                select(a.id);
                              }
                            }}
                          >
                            <circle cx={p.x} cy={p.y} r="15" />
                            <text x={p.x} y={p.y + 39} textAnchor="middle">
                              {a.name.length > 25
                                ? a.name.slice(0, 23) + "..."
                                : a.name}
                            </text>
                          </g>
                        );
                      })}
                    </svg>
                  </div>
                </section>
                <aside className="graph-inspector">
                  {preview && (
                    <>
                      <span className="eyebrow">Selected Age</span>
                      <h2>{preview.name}</h2>
                      {preview.startedFromYear && (
                        <p>
                          Started from Year {preview.startedFromYear.number} in{" "}
                          {doc.ages.find(
                            (a) => a.id === preview.startedFromYear.ageId,
                          )?.name || "an earlier Age"}
                          .
                        </p>
                      )}
                      <div className="history-actions">
                        <button
                          className="primary"
                          onClick={async () => {
                            const d = await command({
                              kind: "select",
                              ageId: preview.id,
                            });
                            if (d) navigate(`/worlds/${id}`);
                          }}
                        >
                          Explore Age
                        </button>
                        <button
                          onClick={async () => {
                            const d = await command({
                              kind: "select",
                              ageId: preview.id,
                            });
                            if (d) setDialog(true);
                          }}
                        >
                          Create from / Connect
                        </button>
                        <button
                          onClick={async () => {
                            const name = await requestText(
                              "Age name",
                              preview.name,
                            );
                            if (name)
                              command({
                                kind: "rename-age",
                                ageId: preview.id,
                                name,
                              });
                          }}
                        >
                          Rename
                        </button>
                        <button
                          className="danger"
                          disabled={busy || doc.ages.length === 1}
                          title={
                            doc.ages.length === 1
                              ? "A World must keep at least one Age"
                              : undefined
                          }
                          onClick={async () => {
                            if (
                              await confirmAction(
                                `Delete ${preview.name}, its maps and entities, and its timeline connections? Other Ages keep their existing content.`,
                              )
                            )
                              command({
                                kind: "delete-age",
                                ageId: preview.id,
                              });
                          }}
                        >
                          Delete Age
                        </button>
                      </div>
                      <h3>New in this Age</h3>
                      <ul>
                        {doc.entities
                          .filter(
                            (e) =>
                              e.ageId === preview.id &&
                              !e.deleted &&
                              ([
                                "country",
                                "state",
                                "kingdom",
                                "empire",
                                "character",
                              ].includes(e.kind) ||
                                registry.types[e.kind]?.category ===
                                  "organizations") &&
                              !doc.entities.some(
                                (old) =>
                                  old.ageId === preview.inheritedFrom &&
                                  !old.deleted &&
                                  (old.continuityId || old.id) ===
                                    (e.continuityId || e.id),
                              ),
                          )
                          .map((e) => (
                            <li key={e.id}>
                              <button onClick={() => openProfile(e)}>
                                {e.name}
                              </button>{" "}
                              <small>{e.kind}</small>
                            </li>
                          ))}
                      </ul>
                      <h3>Connections</h3>
                      {doc.edges
                        .filter(
                          (e) =>
                            e.source === preview.id ||
                            e.destination === preview.id,
                        )
                        .map((e) => (
                          <p key={e.id}>
                            {doc.ages.find((a) => a.id === e.source)?.name}{" "}
                            &rarr;{" "}
                            {doc.ages.find((a) => a.id === e.destination)?.name}{" "}
                            <small>{e.kind}</small>
                            {e.kind !== "linear" && (
                              <button
                                onClick={() =>
                                  command({ kind: "disconnect", edgeId: e.id })
                                }
                              >
                                Disconnect
                              </button>
                            )}
                          </p>
                        ))}
                    </>
                  )}
                </aside>
              </main>
            ) : page === "profile" ? (
              (() => {
                const entity = doc.entities.find(
                  (e) =>
                    e.id === query.get("entity") &&
                    e.ageId === (query.get("age") || current.id),
                );
                return entity ? (
                  registry.types[entity.kind]?.category === "organizations" ? (
                    <OrganizationProfile
                      key={entity.ageId + entity.id + query.get("record")}
                      initialRecord={doc.entities.find(
                        (e) =>
                          e.id === query.get("record") &&
                          e.ageId === entity.ageId &&
                          e.properties?.organizationId === entity.id,
                      )}
                      doc={doc}
                      entity={entity}
                      command={command}
                      onProfile={openProfile}
                      onAtlas={() => navigate(`/worlds/${id}/atlas`)}
                      onBack={() => navigate(`/worlds/${id}/organizations`)}
                    />
                  ) : entity.kind === "lifeform" ? (
                    <LifeformProfile
                      key={entity.ageId + entity.id}
                      doc={doc}
                      entity={entity}
                      command={command}
                      onProfile={openProfile}
                      onBack={async () => {
                        if (
                          entity.ageId !== current.id &&
                          !(await command({
                            kind: "select",
                            ageId: entity.ageId,
                          }))
                        )
                          return;
                        navigate(`/worlds/${id}/biosphere`);
                      }}
                      onBiosphere={async (terrain, habitat) => {
                        if (
                          entity.ageId !== current.id &&
                          !(await command({
                            kind: "select",
                            ageId: entity.ageId,
                          }))
                        )
                          return;
                        navigate(
                          `/worlds/${id}/biosphere?terrain=${terrain}&habitat=${habitat}`,
                        );
                      }}
                    />
                  ) : registry.types[entity.kind]?.category === "culture" ? (
                    <ConceptProfile
                      key={entity.ageId + entity.id}
                      doc={doc}
                      entity={entity}
                      command={command}
                      onProfile={openProfile}
                      onBack={() => navigate(`/worlds/${id}/culture`)}
                    />
                  ) : (
                    <Profile
                      key={entity.ageId + entity.id}
                      doc={doc}
                      entity={entity}
                      command={command}
                      onProfile={openProfile}
                      onGraph={openGraph}
                      onMap={openMap}
                      onSettlement={openSettlement}
                      onYears={openYears}
                      onBack={() =>
                        navigate(
                          `/worlds/${id}/${["character", "family", "house-family", "dynasty"].includes(entity.kind) ? "characters" : "atlas"}`,
                        )
                      }
                    />
                  )
                ) : (
                  <main className="world-page">
                    <h1>Entity not found in this Age</h1>
                  </main>
                );
              })()
            ) : page === "years" ? (
              <YearTimeline
                key={current.id + query.get("year")}
                doc={doc}
                age={current}
                initialYear={query.get("year")}
                onProfile={openProfile}
                busy={busy}
                onNewAge={(year) => {
                  setSourceYear(year);
                  setDialog(true);
                }}
                onGraph={async (ageID) => {
                  if (
                    ageID &&
                    !(await command({ kind: "select", ageId: ageID }))
                  )
                    return;
                  navigate(`/worlds/${id}/graph`);
                }}
              />
            ) : page === "writing" ? (
              <Writing
                key={id}
                doc={doc}
                onBack={() => navigate(`/worlds/${id}`)}
              />
            ) : page === "characters" ? (
              <Characters
                key={current.id}
                doc={doc}
                age={current}
                command={command}
                onProfile={openProfile}
                onGraph={openGraph}
              />
            ) : page === "networks" ? (
              <Networks
                key={path + current.id}
                doc={doc}
                age={doc.ages.find((a) => a.id === query.get("age")) || current}
                query={query}
                command={command}
                onProfile={openProfile}
                onBack={() => navigate(`/worlds/${id}/characters`)}
              />
            ) : page === "organizations" ? (
              <Organizations
                key={current.id}
                doc={doc}
                age={current}
                command={command}
                onProfile={openProfile}
              />
            ) : page === "culture" ? (
              <Culture
                key={current.id}
                doc={doc}
                age={current}
                command={command}
                onProfile={openProfile}
              />
            ) : ["biosphere", "flora"].includes(page) ? (
              <Biosphere
                key={current.id}
                doc={doc}
                age={current}
                command={command}
                onDocument={setDoc}
                onProfile={openProfile}
                query={query}
                onAtlas={(terrain = "", habitat = "") =>
                  navigate(
                    `/worlds/${id}/atlas?terrain=${terrain}&habitat=${habitat}`,
                  )
                }
              />
            ) : page === "magic" ? (
              <SystemDesigner
                key={current.id + query.get("record")}
                doc={doc}
                age={current}
                command={command}
                onProfile={openProfile}
                onAtlas={(terrain, resource = "") =>
                  navigate(
                    `/worlds/${id}/atlas?terrain=${terrain}&resource=${resource}`,
                  )
                }
                query={query}
              />
            ) : page === "library" ? (
              <Library
                key={current.id + query.get("record")}
                doc={doc}
                age={current}
                command={command}
                onDocument={setDoc}
                onProfile={openProfile}
                onAtlas={() => navigate(`/worlds/${id}/atlas`)}
                onOrganizations={() => navigate(`/worlds/${id}/organizations`)}
                query={query}
              />
            ) : (
              <Atlas
                resource={query.get("resource")}
                habitat={query.get("habitat")}
                habitatTerrain={query.get("terrain")}
                key={current.id}
                onRename={async () => {
                  const name = await requestText("World name", doc.world.name);
                  if (name)
                    command({
                      kind: "rename-world",
                      name,
                      description: doc.world.description,
                    });
                }}
                doc={doc}
                age={current}
                onGenerate={() => generate(current.id)}
                onMap={openMap}
                onDeleteMap={removeMap}
                onProfile={openProfile}
                onCreate={(m, kind, capitalOf, logicalId) =>
                  navigate(
                    editorURL(m.projectId, m.ageId) +
                      `&createKind=${kind}&capitalOf=${capitalOf}&logicalId=${logicalId || ""}&entityName=${encodeURIComponent(doc.entities.find((e) => e.id === logicalId && e.ageId === m.ageId)?.name || "")}`,
                  )
                }
              />
            )}

            {!["graph", "profile", "writing"].includes(page) && (
              <TimelineBar
                onYears={() => openYears()}
                doc={doc}
                onSelect={select}
                onNew={() => setDialog(true)}
                onGraph={() => navigate(`/worlds/${id}/graph`)}
              />
            )}
            {dialog && (
              <AgeDialog
                key={"age-dialog-" + ageActionSource(doc, page, selected)}
                sourceAge={
                  sourceYear?.ageId || ageActionSource(doc, page, selected)
                }
                sourceYear={sourceYear}
                doc={doc}
                busy={busy}
                onClose={() => {
                  setDialog(false);
                  setSourceYear(null);
                }}
                onSubmit={applyAge}
              />
            )}
          </>
        )
      )}
    </div>
  );
}
