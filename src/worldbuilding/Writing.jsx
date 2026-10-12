import { useEffect, useRef, useState } from "react";
import Dialog from "./ui/Dialog.jsx";
import { confirmAction, requestText } from "./ui/feedback.js";
import useWriter from "./writer/useWriter.js";
import ManuscriptEditor from "./writer/ManuscriptEditor.jsx";
import WriterContext from "./writer/WriterContext.jsx";
import { markdown } from "./writer/editor.js";
import { exportText, exportDocx } from "./writer/export.js";
import {
  nodeTypes,
  statuses,
  orderedNodes,
  manuscriptNodes,
  newNode,
  plainDocument,
  words,
  textContent,
  request,
  download,
} from "./writer/model.js";
import "./writer/writer.css";
export default function Writing({ onBack, doc }) {
  const w = useWriter(doc.world.id),
    { project, draft, nodes } = w;
  const [view, setView] = useState("write"),
    [tree, setTree] = useState(() => window.innerWidth >= 800),
    [context, setContext] = useState(false),
    [focus, setFocus] = useState(false),
    [find, setFind] = useState(false),
    [search, setSearch] = useState(""),
    [modal, setModal] = useState(""),
    [versions, setVersions] = useState([]),
    [selectedVersion, setSelectedVersion] = useState(null),
    [editor, setEditor] = useState(null),
    [busy, setBusy] = useState(false),
    [collapsed, setCollapsed] = useState(new Set()),
    [creationType, setCreationType] = useState("scene"),
    [exportScope, setExportScope] = useState("all"),
    [includeNotes, setIncludeNotes] = useState(false),
    [chronologyFilter, setChronologyFilter] = useState(""),
    [showStatus, setShowStatus] = useState(true),
    [session, setSession] = useState(0);
  const baselines = useRef(new Map()),
    latest = useRef({}),
    drag = useRef(null),
    importRef = useRef(null),
    backupRef = useRef(null);
  const ordered = orderedNodes(nodes),
    prefs = project?.settings || {},
    count = (n) => words(textContent(n?.content));
  const projectWords = manuscriptNodes(nodes).reduce(
    (sum, n) => sum + count(n),
    0,
  );
  latest.current = { w, draft, ordered, focus };
  async function run(fn) {
    if (busy) return;
    setBusy(true);
    w.setError("");
    try {
      await fn();
    } catch (e) {
      w.setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  async function createProject() {
    const name = await requestText("Writing project title");
    if (!name?.trim()) return;
    await run(async () => {
      if (!(await w.flush())) return;
      const p = await request("/projects", {
        id: crypto.randomUUID(),
        worldId: doc.world.id,
        name: name.trim(),
        projectType: "novel",
        revision: 0,
        settings: { defaultAge: doc.world.currentAge },
      });
      await w.refreshProjects();
      await w.load(p.id);
      const book = await request("/projects/" + p.id + "/nodes", {
        node: { ...newNode(p.id, "book"), title: "Manuscript" },
      });
      const chapter = await request("/projects/" + p.id + "/nodes", {
        node: { ...newNode(p.id, "chapter", book.id), title: "Chapter One" },
      });
      await request("/projects/" + p.id + "/nodes", {
        node: {
          ...newNode(p.id, "scene", chapter.id, 0, doc.world.currentAge),
          title: "Opening scene",
        },
      });
      await w.load(p.id);
    });
  }
  async function add(type = creationType) {
    await run(async () => {
      if (!(await w.flush())) return;
      const current = latest.current.w.draft;
      const parent =
        current && ["book", "part", "chapter"].includes(current.type)
          ? current.id
          : current?.parentId || "";
      const siblings = nodes.filter((n) => n.parentId === parent && !n.deleted);
      await w.saveNode(
        newNode(
          project.id,
          type,
          parent,
          Math.max(0, ...siblings.map((n) => n.position)) + 1,
          prefs.defaultAge || doc.world.currentAge,
        ),
        "Created " + type,
      );
      setView("write");
    });
  }
  async function select(n) {
    if (n.id === draft?.id) return;
    await run(async () => {
      if (await w.choose(n)) {
        if (window.innerWidth < 800) setTree(false);
      }
    });
  }
  async function move(id, targetId, before = true) {
    await run(async () => {
      if (!(await w.flush())) return;
      const n = latest.current.w.nodes.find((x) => x.id === id),
        target = latest.current.w.nodes.find((x) => x.id === targetId);
      if (!n || !target || n.id === target.id) return;
      const siblings = latest.current.w.nodes
        .filter(
          (x) => x.parentId === target.parentId && !x.deleted && x.id !== id,
        )
        .sort((a, b) => a.position - b.position);
      const index = siblings.findIndex((x) => x.id === target.id);
      const a = before ? siblings[index - 1]?.position : target.position,
        b = before ? target.position : siblings[index + 1]?.position;
      await w.saveNode(
        {
          ...n,
          parentId: target.parentId,
          position: a == null ? b - 1 : b == null ? a + 1 : (a + b) / 2,
        },
        "Reordered",
      );
    });
  }
  function change(patch) {
    if ("content" in patch && draft) {
      if (!baselines.current.has(draft.id))
        baselines.current.set(draft.id, count(draft));
      const next = nodes.map((n) =>
        n.id === draft.id ? { ...n, ...patch } : n,
      );
      setSession(
        next.reduce(
          (sum, n) =>
            sum +
            (baselines.current.has(n.id)
              ? count(n) - baselines.current.get(n.id)
              : 0),
          0,
        ),
      );
    }
    w.edit(patch);
  }
  useEffect(() => {
    const handler = (e) => {
      if (e.key === "Escape") {
        setContext(false);
        if (window.innerWidth < 800) setTree(false);
      }
      if (!(e.ctrlKey || e.metaKey)) return;
      const x = latest.current;
      if (e.key.toLowerCase() === "s") {
        e.preventDefault();
        void x.w.flush();
      } else if (!e.shiftKey && e.key.toLowerCase() === "f") {
        e.preventDefault();
        setView("write");
        setFind((v) => !v);
      } else if (e.shiftKey && e.key.toLowerCase() === "f") {
        e.preventDefault();
        setFocus((v) => !v);
      } else if (e.altKey && ["ArrowDown", "ArrowUp"].includes(e.key)) {
        e.preventDefault();
        const i = x.ordered.findIndex((n) => n.id === x.draft?.id),
          n = x.ordered[i + (e.key === "ArrowDown" ? 1 : -1)];
        if (n) void x.w.choose(n);
      } else if (e.shiftKey && e.key.toLowerCase() === "n") {
        e.preventDefault();
        if (x.w.project)
          void (async () => {
            if (await x.w.flush())
              await x.w.saveNode(
                newNode(
                  x.w.project.id,
                  "scene",
                  x.draft?.parentId || "",
                  Math.max(0, ...x.w.nodes.map((n) => n.position)) + 1,
                ),
                "New scene",
              );
          })();
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);
  async function history() {
    await run(async () => {
      if (!(await w.flush())) return;
      const v = await request(
        "/projects/" + project.id + "/versions/" + draft.id,
      );
      setVersions(v.reverse());
      setSelectedVersion(null);
      setModal("history");
    });
  }
  async function duplicate() {
    await run(async () => {
      if (!(await w.flush())) return;
      const n = latest.current.w.draft;
      await w.saveNode(
        {
          ...n,
          id: crypto.randomUUID(),
          revision: 0,
          title: n.title + " (copy)",
          position: n.position + 0.1,
          createdAt: 0,
        },
        "Duplicated node",
      );
    });
  }
  async function remove() {
    if (
      !(await confirmAction(
        `Delete “${draft.title}”? Its revisions remain recoverable in project history.`,
        { accept: "Delete node" },
      ))
    )
      return;
    await run(async () => {
      if (!(await w.flush())) return;
      await w.saveNode(
        { ...latest.current.w.draft, deleted: true },
        "Deleted node",
      );
    });
  }
  async function importText(file) {
    if (!file) return;
    await run(async () => {
      if (file.size > 2 * 1024 * 1024)
        throw Error("Text imports are limited to 2 MB.");
      if (!/\.(txt|md)$/i.test(file.name))
        throw Error("Choose a .txt or .md file.");
      const original = new TextDecoder("utf-8", {
        fatal: true,
        ignoreBOM: true,
      }).decode(await file.arrayBuffer());
      let content = /\.md$/i.test(file.name)
        ? markdown.parse(original)
        : plainDocument(original);
      const n = {
        ...newNode(
          project.id,
          "scene",
          draft?.parentId || "",
          Math.max(0, ...nodes.map((n) => n.position)) + 1,
          prefs.defaultAge,
        ),
        title: file.name.replace(/\.(txt|md)$/i, ""),
        content,
        originalName: file.name,
        originalText: original,
      };
      await w.saveNode(n, "Original import");
      setView("write");
    });
  }
  async function exportFile(format) {
    await run(async () => {
      if (!(await w.flush())) return;
      const chosen = manuscriptNodes(
        latest.current.w.nodes,
        exportScope === "selected" ? draft?.id : "",
        includeNotes,
      );
      if (!chosen.length)
        throw Error(
          "Nothing selected for export. Enable private material to export notes.",
        );
      download(
        project.name + "." + format,
        format === "docx" ? exportDocx(chosen) : exportText(chosen, format),
        format === "docx"
          ? "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
          : format === "md"
            ? "text/markdown;charset=utf-8"
            : "text/plain;charset=utf-8",
      );
    });
  }
  async function backup() {
    await run(async () => {
      if (!(await w.flush())) return;
      const b = await request("/projects/" + project.id + "?backup=1");
      download(
        project.name + ".writing.json",
        JSON.stringify(b, null, 2),
        "application/json",
      );
    });
  }
  async function restoreBackup(file) {
    if (!file) return;
    await run(async () => {
      if (!(await w.flush())) return;
      if (file.size > 64 * 1024 * 1024)
        throw Error("Backup is too large (64 MB limit).");
      const b = JSON.parse(await file.text());
      if (b.project?.worldId && b.project.worldId !== doc.world.id)
        throw Error(
          "This backup belongs to a different World. Open that World before restoring its manuscript.",
        );
      const p = await request(
        "/import?world=" + encodeURIComponent(b.project?.worldId || ""),
        b,
      );
      await w.refreshProjects();
      await w.load(p.id);
      setModal("");
    });
  }
  const hidden = new Set();
  for (const n of ordered)
    if (collapsed.has(n.parentId) || hidden.has(n.parentId)) hidden.add(n.id);
  const filtered = ordered.filter((n) =>
    (n.title + " " + textContent(n.content) + " " + n.synopsis)
      .toLocaleLowerCase()
      .includes(search.toLocaleLowerCase()),
  );
  const chapter = (() => {
    let n = draft;
    while (n && n.type !== "chapter")
      n = nodes.find((x) => x.id === n.parentId);
    return n;
  })();
  const book = (() => {
    let n = draft;
    while (n && n.type !== "book") n = nodes.find((x) => x.id === n.parentId);
    return n;
  })();
  return (
    <main className={`writer-workspace ${focus ? "writer-focus" : ""}`}>
      <header className="writer-top">
        <button onClick={onBack}>← Atlas</button>
        <div className="writer-project-selector">
          <small>Writing · {doc.world.name}</small>
          <select
            aria-label="Writing project"
            value={project?.id || ""}
            onChange={(e) => run(() => w.chooseProject(e.target.value))}
          >
            <option value="" disabled>
              Select a project
            </option>
            {w.projects.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
                {p.worldId ? "" : " · Unassigned"}
              </option>
            ))}
          </select>
        </div>
        <span
          role="status"
          className={`writer-save ${w.status.includes("failed") || w.status.includes("Conflict") ? "writer-warning" : ""}`}
        >
          {w.status}
        </span>
        <button onClick={() => setFocus((v) => !v)}>
          {focus ? "Exit focus" : "Focus"}
        </button>
        <button
          disabled={!draft}
          onClick={() => {
            setView("write");
            setFind((v) => !v);
          }}
        >
          Find
        </button>
        <button disabled={!project} onClick={() => setModal("export")}>
          Export
        </button>
        <button onClick={() => setModal("tools")}>More</button>
      </header>
      {w.error && (
        <div className="writer-notice" role="alert">
          {w.error}
          <button onClick={() => run(() => w.flush())}>Retry save</button>
          {w.status === "Conflict needs attention" && (
            <>
              <button onClick={() => setModal("conflict")}>
                Compare / recover
              </button>
              <button onClick={() => run(w.saveCopy)}>
                Save local as copy
              </button>
            </>
          )}
        </div>
      )}
      {w.recovery && (
        <div className="writer-notice" role="alert">
          A local recovery draft is available.
          <button
            onClick={() => {
              w.recover();
            }}
          >
            Recover local edits
          </button>
          <button
            onClick={() => {
              download(
                "recovered-draft.json",
                JSON.stringify(w.recovery, null, 2),
                "application/json",
              );
            }}
          >
            Download recovery
          </button>
          <button onClick={() => w.setRecovery(null)}>
            Keep server version
          </button>
        </div>
      )}
      {!project ? (
        <section className="writer-welcome">
          <span className="eyebrow">A place for your story</span>
          <h1>Your manuscript, your world.</h1>
          <p>Draft now. Shape chapters later. Keep your world within reach.</p>
          <button className="primary" disabled={busy} onClick={createProject}>
            Create writing project
          </button>
          <button onClick={() => backupRef.current.click()}>
            Restore project backup
          </button>
          {w.status === "Loading…" && <p role="status">Loading your drafts…</p>}
        </section>
      ) : (
        <>
          {!focus && (
            <div className="writer-viewbar">
              <button aria-expanded={tree} onClick={() => setTree((v) => !v)}>
                Manuscript
              </button>
              <div className="writer-tabs" aria-label="Writing views">
                {["write", "outline", "corkboard", "chronology"].map((v) => (
                  <button
                    key={v}
                    aria-pressed={view === v}
                    onClick={() => setView(v)}
                  >
                    {v}
                  </button>
                ))}
              </div>
              <button
                aria-expanded={context}
                onClick={() => setContext((v) => !v)}
              >
                Context & notes
              </button>
            </div>
          )}
          <div
            className={`writer-layout ${tree && !focus ? "with-tree" : ""} ${context && !focus ? "with-context" : ""}`}
          >
            {tree && !focus && (
              <nav
                className="writer-navigator"
                aria-label="Manuscript structure"
              >
                <div className="writer-panel-heading">
                  <strong>Manuscript</strong>
                  <button onClick={() => setTree(false)}>Close</button>
                </div>
                <div className="writer-add">
                  <select
                    aria-label="New node type"
                    value={creationType}
                    onChange={(e) => setCreationType(e.target.value)}
                  >
                    {nodeTypes.map((t) => (
                      <option key={t}>{t}</option>
                    ))}
                  </select>
                  <button disabled={busy} onClick={() => add()}>
                    Add
                  </button>
                </div>
                <label>
                  Project search
                  <input
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    placeholder="Title or manuscript text"
                  />
                </label>
                <ol className="writer-tree">
                  {(search
                    ? filtered
                    : ordered.filter((n) => !hidden.has(n.id))
                  ).map((n) => (
                    <li
                      key={n.id}
                      style={{ "--depth": Math.min(n.depth, 8) }}
                      draggable={!busy}
                      onDragStart={(e) => {
                        drag.current = n.id;
                        e.dataTransfer.setData("text/plain", n.id);
                      }}
                      onDragOver={(e) => e.preventDefault()}
                      onDrop={(e) => {
                        e.preventDefault();
                        void move(drag.current, n.id);
                      }}
                    >
                      <div>
                        <button
                          className="writer-disclosure"
                          aria-label={`${collapsed.has(n.id) ? "Expand" : "Collapse"} ${n.title}`}
                          disabled={
                            !nodes.some(
                              (c) => c.parentId === n.id && !c.deleted,
                            )
                          }
                          onClick={() =>
                            setCollapsed((prev) => {
                              const next = new Set(prev);
                              next.has(n.id)
                                ? next.delete(n.id)
                                : next.add(n.id);
                              return next;
                            })
                          }
                        >
                          {collapsed.has(n.id) ? "▸" : "▾"}
                        </button>
                        <button
                          className={n.id === draft?.id ? "selected" : ""}
                          onClick={() => select(n)}
                        >
                          <span>{n.title}</span>
                          <small>
                            {n.type} · {count(n)} words · {n.status}
                          </small>
                          {search && (
                            <small>
                              {textContent(n.content).slice(0, 100)}
                            </small>
                          )}
                        </button>
                      </div>
                    </li>
                  ))}
                </ol>
                {search && filtered.length === 0 && <p>No matching scenes.</p>}
                {ordered.length === 0 && (
                  <p>
                    Add a scene to begin writing, or a chapter to plan your
                    structure.
                  </p>
                )}
              </nav>
            )}
            <section className="writer-main">
              {draft ? (
                <>
                  <div className="writer-document-heading">
                    <input
                      aria-label="Scene title"
                      value={draft.title}
                      onChange={(e) => change({ title: e.target.value })}
                    />
                    <select
                      aria-label="Scene status"
                      value={draft.status}
                      onChange={(e) => change({ status: e.target.value })}
                    >
                      {statuses.map((s) => (
                        <option key={s}>{s}</option>
                      ))}
                    </select>
                    <button onClick={() => setModal("node")}>
                      Scene options
                    </button>
                  </div>
                  <label className="writer-synopsis">
                    Synopsis
                    <input
                      value={draft.synopsis}
                      onChange={(e) => change({ synopsis: e.target.value })}
                      placeholder="Optional planning summary — excluded from export"
                    />
                  </label>
                  {view === "write" && (
                    <ManuscriptEditor
                      key={draft.id + ":" + w.epoch}
                      node={draft}
                      onChange={change}
                      onBlur={() => void w.flush()}
                      onEditor={setEditor}
                      preferences={prefs}
                      findOpen={find}
                    />
                  )}
                  {view === "outline" && (
                    <div className="writer-table-wrap">
                      <table className="writer-outline">
                        <thead>
                          <tr>
                            <th>Order / title</th>
                            <th>Synopsis</th>
                            <th>Status</th>
                            <th>POV</th>
                            <th>Words</th>
                          </tr>
                        </thead>
                        <tbody>
                          {ordered.map((n, i) => (
                            <tr key={n.id}>
                              <td>
                                <button onClick={() => select(n)}>
                                  {i + 1}. {"—".repeat(n.depth)} {n.title}
                                </button>
                              </td>
                              <td>{n.synopsis || "—"}</td>
                              <td>{n.status}</td>
                              <td>
                                {doc.entities.find(
                                  (e) =>
                                    e.id === n.povEntityId &&
                                    e.ageId === n.storyAgeId,
                                )?.name || "—"}
                              </td>
                              <td>{count(n)}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                  {view === "corkboard" && (
                    <div className="writer-corkboard">
                      {ordered
                        .filter((n) => ["chapter", "scene"].includes(n.type))
                        .map((n) => (
                          <article
                            key={n.id}
                            draggable
                            onDragStart={() => (drag.current = n.id)}
                            onDragOver={(e) => e.preventDefault()}
                            onDrop={(e) => {
                              e.preventDefault();
                              void move(drag.current, n.id);
                            }}
                          >
                            <small>
                              {n.type} · {n.status}
                            </small>
                            <button onClick={() => select(n)}>{n.title}</button>
                            <p>
                              {n.synopsis ||
                                "Select this card to write its synopsis above."}
                            </p>
                            <small>
                              {n.tags} · {count(n)} words
                            </small>
                          </article>
                        ))}
                    </div>
                  )}
                  {view === "chronology" && (
                    <div className="writer-chronology">
                      <p>
                        Story time alongside narrative order. Changing dates
                        never rearranges the manuscript.
                      </p>
                      <label>
                        Filter character or plotline
                        <input
                          value={chronologyFilter}
                          onChange={(e) => setChronologyFilter(e.target.value)}
                        />
                      </label>
                      {doc.ages.map((a) => (
                        <section key={a.id}>
                          <h2>{a.name}</h2>
                          {ordered
                            .filter(
                              (n) =>
                                n.type === "scene" &&
                                n.storyAgeId === a.id &&
                                (
                                  n.tags +
                                  " " +
                                  (doc.entities.find(
                                    (e) =>
                                      e.id === n.povEntityId &&
                                      e.ageId === a.id,
                                  )?.name || "")
                                )
                                  .toLowerCase()
                                  .includes(chronologyFilter.toLowerCase()),
                            )
                            .sort(
                              (a, b) =>
                                (a.storyYear ?? Infinity) -
                                (b.storyYear ?? Infinity),
                            )
                            .map((n) => (
                              <button key={n.id} onClick={() => select(n)}>
                                <strong>{n.storyYear ?? "Undated"}</strong>{" "}
                                {n.title}
                                <small>
                                  Narrative position{" "}
                                  {ordered.findIndex((x) => x.id === n.id) + 1}{" "}
                                  · {n.tags}
                                </small>
                              </button>
                            ))}
                        </section>
                      ))}
                      <section>
                        <h2>No story Age</h2>
                        {ordered
                          .filter((n) => n.type === "scene" && !n.storyAgeId)
                          .map((n) => (
                            <button key={n.id} onClick={() => select(n)}>
                              {n.title}
                            </button>
                          ))}
                      </section>
                    </div>
                  )}
                </>
              ) : (
                <div className="writer-empty">
                  <h2>A blank page is a beginning.</h2>
                  <button onClick={() => add("scene")} disabled={busy}>
                    Create scene
                  </button>
                  <button onClick={() => setModal("tools")}>
                    Recover a deleted scene
                  </button>
                </div>
              )}
            </section>
            {context && !focus && draft && (
              <WriterContext
                key={draft.id}
                node={draft}
                doc={doc}
                project={project}
                onChange={change}
                editor={editor}
                onClose={() => setContext(false)}
              />
            )}
          </div>
          {showStatus && (
            <footer className="writer-status">
              <span>
                {draft ? count(draft) : 0} scene words ·{" "}
                {draft ? textContent(draft.content).trim().length : 0}{" "}
                characters
              </span>
              {chapter && (
                <span>
                  {manuscriptNodes(nodes, chapter.id).reduce(
                    (s, n) => s + count(n),
                    0,
                  )}{" "}
                  chapter words
                </span>
              )}
              {book && (
                <span>
                  {manuscriptNodes(nodes, book.id).reduce(
                    (s, n) => s + count(n),
                    0,
                  )}{" "}
                  book words
                </span>
              )}
              <span>
                {projectWords} project words
                {prefs.target ? ` / ${prefs.target} target` : ""}
              </span>
              <span>
                {session >= 0 ? "+" : ""}
                {session} session words
                {prefs.sessionTarget ? ` / ${prefs.sessionTarget} target` : ""}
              </span>
              {draft?.targetWords > 0 && (
                <span>
                  Scene target{" "}
                  {Math.round((count(draft) / draft.targetWords) * 100)}%
                </span>
              )}
            </footer>
          )}
        </>
      )}
      <input
        ref={importRef}
        type="file"
        hidden
        accept=".txt,.md"
        onChange={(e) => {
          void importText(e.target.files[0]);
          e.target.value = "";
        }}
      />
      <input
        ref={backupRef}
        type="file"
        hidden
        accept=".json"
        onChange={(e) => {
          void restoreBackup(e.target.files[0]);
          e.target.value = "";
        }}
      />
      {modal === "export" && (
        <Dialog
          trackChanges={false}
          title="Export manuscript"
          onClose={() => setModal("")}
        >
          <label>
            Scope
            <select
              value={exportScope}
              onChange={(e) => setExportScope(e.target.value)}
            >
              <option value="all">Complete manuscript</option>
              <option value="selected" disabled={!draft}>
                Selected node and descendants
              </option>
            </select>
          </label>
          <label>
            <input
              type="checkbox"
              checked={includeNotes}
              onChange={(e) => setIncludeNotes(e.target.checked)}
            />
            Include private notes / research nodes
          </label>
          <p>
            Scene notes, metadata and references are preserved in project
            backups, not added to your prose.
          </p>
          <div className="history-actions">
            {["txt", "md", "docx"].map((f) => (
              <button key={f} disabled={busy} onClick={() => exportFile(f)}>
                Export {f.toUpperCase()}
              </button>
            ))}
            <button disabled={busy} onClick={backup}>
              Project backup JSON
            </button>
          </div>
          <p>
            World ZIPs intentionally exclude author manuscripts. Keep this
            separate backup alongside your World ZIP.
          </p>
        </Dialog>
      )}
      {modal === "tools" && (
        <Dialog
          trackChanges={false}
          title="Writing tools"
          onClose={() => setModal("")}
        >
          <div className="writer-tool-list">
            <button onClick={createProject} disabled={busy}>
              Create project
            </button>
            <button onClick={() => backupRef.current.click()} disabled={busy}>
              Restore project backup as a new project
            </button>
            {project && (
              <>
                <button
                  onClick={() => importRef.current.click()}
                  disabled={busy}
                >
                  Import TXT or Markdown as a new scene
                </button>
                <button
                  onClick={async () => {
                    const name = await requestText(
                      "Project name",
                      project.name,
                    );
                    if (name)
                      await run(() => w.saveProject({ ...project, name }));
                  }}
                >
                  Rename project
                </button>
                {!project.worldId && (
                  <button
                    onClick={() =>
                      run(() =>
                        w.saveProject({ ...project, worldId: doc.world.id }),
                      )
                    }
                  >
                    Associate with {doc.world.name}
                  </button>
                )}
                <button onClick={() => setModal("preferences")}>
                  Editor preferences & goals
                </button>
                <button onClick={() => setShowStatus((v) => !v)}>
                  {showStatus ? "Hide" : "Show"} word-count strip
                </button>
                {draft && (
                  <>
                    <button onClick={history}>Version history / restore</button>
                    <button
                      onClick={async () => {
                        const label = await requestText(
                          "Snapshot name",
                          "Manual snapshot",
                        );
                        if (label) await run(() => w.flush(label));
                      }}
                    >
                      Create snapshot
                    </button>
                  </>
                )}
                <h3>Deleted nodes</h3>
                {nodes
                  .filter((n) => n.deleted)
                  .map((n) => (
                    <button
                      key={n.id}
                      onClick={() =>
                        run(() =>
                          w.saveNode(
                            { ...n, deleted: false },
                            "Restored deleted node",
                          ),
                        )
                      }
                    >
                      Restore {n.title}
                    </button>
                  ))}
              </>
            )}
            <p>
              Shortcuts: Ctrl/⌘ S save, Ctrl/⌘ F find, Ctrl/⌘ Shift F focus,
              Ctrl/⌘ Shift N new scene, Ctrl/⌘ Alt ↑/↓ previous/next node.
            </p>
          </div>
        </Dialog>
      )}
      {modal === "preferences" && (
        <Dialog
          title="Editor preferences & goals"
          onClose={() => setModal("")}
          onSubmit={async (e) => {
            const f = new FormData(e.currentTarget);
            await w.saveProject({
              ...project,
              projectType: f.get("projectType"),
              settings: {
                ...prefs,
                font: f.get("font"),
                theme: f.get("theme"),
                fontSize: Number(f.get("fontSize")),
                lineHeight: Number(f.get("lineHeight")),
                target: Number(f.get("target")),
                sessionTarget: Number(f.get("sessionTarget")),
              },
            });
            setModal("");
          }}
          actions={
            <button type="submit" className="primary">
              Save preferences
            </button>
          }
        >
          <label>
            Project type
            <select
              name="projectType"
              defaultValue={project.projectType || "novel"}
            >
              {[
                "novel",
                "anthology",
                "campaign narrative",
                "short story",
                "series volume",
                "personal",
              ].map((t) => (
                <option key={t}>{t}</option>
              ))}
            </select>
          </label>
          <label>
            Font
            <select name="font" defaultValue={prefs.font || "serif"}>
              <option value="serif">Serif</option>
              <option value="sans">Sans serif</option>
              <option value="mono">Monospace</option>
            </select>
          </label>
          <label>
            Font size
            <input
              name="fontSize"
              type="number"
              min="14"
              max="32"
              defaultValue={prefs.fontSize || 19}
            />
          </label>
          <label>
            Line height
            <input
              name="lineHeight"
              type="number"
              min="1.2"
              max="2.5"
              step=".1"
              defaultValue={prefs.lineHeight || 1.8}
            />
          </label>
          <label>
            Writing surface
            <select name="theme" defaultValue={prefs.theme || "paper"}>
              <option value="paper">White paper</option>
              <option value="warm">Warm paper</option>
              <option value="dark">Dark</option>
            </select>
          </label>
          <label>
            Project target (0 = off)
            <input
              type="number"
              name="target"
              min="0"
              defaultValue={prefs.target || 0}
            />
          </label>
          <label>
            Session target (0 = off)
            <input
              type="number"
              name="sessionTarget"
              min="0"
              defaultValue={prefs.sessionTarget || 0}
            />
          </label>
        </Dialog>
      )}
      {modal === "node" && draft && (
        <Dialog
          trackChanges={false}
          title="Scene & structure options"
          onClose={() => setModal("")}
        >
          <label>
            Move into
            <select
              value={draft.parentId}
              onChange={(e) =>
                run(() =>
                  w.saveNode(
                    {
                      ...draft,
                      parentId: e.target.value,
                      position:
                        Math.max(0, ...nodes.map((n) => n.position)) + 1,
                    },
                    "Moved node",
                  ),
                )
              }
            >
              <option value="">Project root</option>
              {ordered
                .filter(
                  (n) =>
                    n.id !== draft.id &&
                    ["book", "part", "chapter"].includes(n.type),
                )
                .map((n) => (
                  <option key={n.id} value={n.id}>
                    {"—".repeat(n.depth)} {n.title}
                  </option>
                ))}
            </select>
          </label>
          <label>
            Node type
            <select
              value={draft.type}
              onChange={(e) => change({ type: e.target.value })}
            >
              {nodeTypes.map((t) => (
                <option key={t}>{t}</option>
              ))}
            </select>
          </label>
          <label>
            Word target (0 = off)
            <input
              type="number"
              min="0"
              value={draft.targetWords || 0}
              onChange={(e) => change({ targetWords: Number(e.target.value) })}
            />
          </label>
          <div className="history-actions">
            {[-1, 1].map((dir) => {
              const peers = ordered.filter(
                  (n) => n.parentId === draft.parentId,
                ),
                i = peers.findIndex((n) => n.id === draft.id),
                target = peers[i + dir];
              return (
                <button
                  key={dir}
                  disabled={!target || busy}
                  onClick={() => move(draft.id, target.id, dir < 0)}
                >
                  Move {dir < 0 ? "up" : "down"}
                </button>
              );
            })}
            <button onClick={duplicate} disabled={busy}>
              Duplicate node
            </button>
            <button onClick={remove} disabled={busy} className="danger">
              Delete node
            </button>
            <button onClick={history}>History</button>
            {draft.originalName && (
              <button
                onClick={() => download(draft.originalName, draft.originalText)}
              >
                Download original import
              </button>
            )}
          </div>
        </Dialog>
      )}
      {modal === "history" && (
        <Dialog
          trackChanges={false}
          title="Revision history"
          size="editor"
          onClose={() => setModal("")}
        >
          <p>
            Restoring creates a new revision and retains the current version.
          </p>
          <select
            aria-label="Saved revision"
            value={selectedVersion?.node.revision || ""}
            onChange={(e) =>
              setSelectedVersion(
                versions.find(
                  (v) => v.node.revision === Number(e.target.value),
                ),
              )
            }
          >
            <option value="">Select a revision</option>
            {versions.map((v) => (
              <option key={v.node.revision} value={v.node.revision}>
                #{v.node.revision} · {v.label} ·{" "}
                {new Date(v.node.updatedAt).toLocaleString()}
              </option>
            ))}
          </select>
          {selectedVersion && (
            <>
              <h3>{selectedVersion.node.title}</h3>
              <pre className="writer-revision-preview">
                {textContent(selectedVersion.node.content)}
              </pre>
              <button
                disabled={busy}
                onClick={() =>
                  run(async () => {
                    if (!(await w.flush())) return;
                    await w.saveNode(
                      {
                        ...selectedVersion.node,
                        revision: latest.current.w.draft.revision,
                        deleted: false,
                      },
                      "Restored revision " + selectedVersion.node.revision,
                    );
                    setModal("");
                  })
                }
              >
                Restore this revision
              </button>
            </>
          )}
        </Dialog>
      )}
      {modal === "conflict" && (
        <Dialog
          trackChanges={false}
          title="Recover conflicting edits"
          size="editor"
          onClose={() => setModal("")}
        >
          <p>
            Your local edits have not overwritten the server. Save a copy to
            preserve both, or reload and review the local recovery draft.
          </p>
          <h3>Local draft</h3>
          <pre className="writer-revision-preview">
            {textContent(draft.content)}
          </pre>
          <button
            onClick={() =>
              run(async () => {
                const b = await request("/projects/" + project.id);
                setSelectedVersion({
                  node: b.nodes.find((n) => n.id === draft.id),
                });
              })
            }
          >
            Show server version
          </button>
          {selectedVersion?.node && (
            <>
              <h3>Server draft</h3>
              <pre className="writer-revision-preview">
                {textContent(selectedVersion.node.content)}
              </pre>
            </>
          )}
          <div className="history-actions">
            <button
              onClick={() =>
                run(async () => {
                  await w.saveCopy();
                  setModal("");
                })
              }
            >
              Save local as copy
            </button>
            <button
              onClick={() =>
                run(async () => {
                  await w.reload();
                  setModal("");
                })
              }
            >
              Reload server draft
            </button>
            <button
              onClick={() =>
                download(
                  "local-recovery.json",
                  JSON.stringify(draft, null, 2),
                  "application/json",
                )
              }
            >
              Download local recovery
            </button>
          </div>
        </Dialog>
      )}
    </main>
  );
}
