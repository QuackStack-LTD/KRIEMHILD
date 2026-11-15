import { useEffect, useRef, useState } from "react";
import Dialog from "../ui/Dialog.jsx";
import { requestText } from "../ui/feedback.js";
import useWriter from "./useWriter.js";
import ArchiveEditor from "./ArchiveEditor.jsx";
import ResourceArchiveReview from "../ResourceArchiveReview.jsx";
import ArchiveFilePreview, {
  LinkedArchiveFile,
} from "./ArchiveFilePreview.jsx";
import Icon from "./ArchiveIcon.jsx";
import {
  newNode,
  request,
  download,
  plainDocument,
  textContent,
} from "./model.js";
import { materializeManuscripts } from "./collaboration.js";
import { importDocument, exportDocument, mimeFor } from "./archive-files.js";
import {
  ROOT,
  INWORLD,
  isFolder,
  baseName,
  archiveItems,
  inworldItems,
  descendants,
  crumbs,
  sizeLabel,
  kindLabel,
  archiveRequest,
  uploadArchiveFile,
  fileURL,
  imageURL,
  libraryURL,
} from "./archive-model.js";
import "./archive.css";
const formats = [
  ["docx", "Word document (.docx)"],
  ["pdf", "PDF document (.pdf)"],
  ["txt", "Plain text (.txt)"],
  ["md", "Markdown (.md)"],
];
export default function WriterArchive({ doc, onBack, onLibrary, onDocument }) {
  const world = doc.world.id,
    rootProject = "archive-" + world,
    w = useWriter(world, { autoLoad: false });
  const [data, setData] = useState({ projects: [], nodes: [] }),
    [inworld, setInworld] = useState(null),
    [folder, setFolder] = useState(ROOT),
    [section, setSection] = useState("files"),
    [search, setSearch] = useState(""),
    [filter, setFilter] = useState("all"),
    [layout, setLayout] = useState("list"),
    [sort, setSort] = useState("name"),
    [selected, setSelected] = useState(""),
    [opened, setOpened] = useState(null),
    [busy, setBusy] = useState(false),
    [loading, setLoading] = useState(true),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [modal, setModal] = useState(null),
    [history, setHistory] = useState([]),
    [info, setInfo] = useState(false),
    [dragging, setDragging] = useState(false),
    [recent, setRecent] = useState([]);
  const input = useRef(null),
    replace = useRef(null),
    lock = useRef(false),
    observedWorldRevision = useRef(doc.world.revision);
  const all = [...archiveItems(data, world), ...inworldItems(inworld)],
    byKey = new Map(all.map((n) => [n.key, n])),
    current = byKey.get(folder),
    item = byKey.get(selected),
    linked = folder === INWORLD || current?.inworld;
  async function refresh() {
    const [a, b] = await Promise.all([
      archiveRequest(world),
      archiveRequest(world, "/inworld"),
    ]);
    setData(a);
    setInworld(b);
  }
  useEffect(() => {
    let live = true;
    Promise.all([archiveRequest(world), archiveRequest(world, "/inworld")])
      .then(([a, b]) => {
        if (live) {
          setData(a);
          setInworld(b);
        }
      })
      .catch((e) => live && setError(e.message))
      .finally(() => live && setLoading(false));
    try {
      setRecent(
        JSON.parse(
          localStorage.getItem("kriemhild.archive.recent." + world) || "[]",
        ),
      );
    } catch {}
    return () => {
      live = false;
    };
  }, [world]);
  useEffect(() => {
    if (observedWorldRevision.current === doc.world.revision) return;
    observedWorldRevision.current = doc.world.revision;
    let live = true;
    archiveRequest(world, "/inworld")
      .then((next) => {
        if (live) setInworld(next);
      })
      .catch((e) => live && setError(e.message));
    return () => {
      live = false;
    };
  }, [world, doc.world.revision]);
  async function run(fn) {
    if (lock.current) return;
    lock.current = true;
    setBusy(true);
    setError("");
    try {
      return await fn();
    } catch (e) {
      setError(e.message);
      return null;
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  function accept(n) {
    setData((d) => ({
      ...d,
      nodes: [...d.nodes.filter((x) => x.id !== n.id), n],
    }));
  }
  function remember(key) {
    setRecent((r) => {
      const next = [key, ...r.filter((k) => k !== key)].slice(0, 50);
      try {
        localStorage.setItem(
          "kriemhild.archive.recent." + world,
          JSON.stringify(next),
        );
      } catch {}
      return next;
    });
  }
  async function browse(key = ROOT, scope = "files") {
    if (!(await w.flush())) return;
    if (w.current()) accept(w.current());
    setOpened(null);
    setFolder(key);
    setSection(scope);
    setSelected("");
    setSearch("");
    setNotice("");
  }
  async function open(n, notes = false) {
    if (n.deleted || n.projectDeleted) return;
    if (isFolder(n) && !notes) return browse(n.key);
    await run(async () => {
      if (!(await w.flush())) return;
      if (w.current()) accept(w.current());
      if (!n.inworld) await w.load(n.projectId, n.id);
      setOpened(n);
      setSelected(n.key);
      remember(n.key);
    });
  }
  async function ensureProject() {
    let p = data.projects.find((p) => p.id === rootProject);
    if (!p) {
      p = await archiveRequest(world, "", {});
      setData((d) => ({
        ...d,
        projects: [...d.projects.filter((x) => x.id !== p.id), p],
      }));
    }
    return p;
  }
  async function destination() {
    if (current && !current.inworld)
      return {
        projectId: current.projectId,
        parentId: current.project ? "" : current.id,
      };
    return { projectId: (await ensureProject()).id, parentId: "" };
  }
  async function create(type, name) {
    await run(async () => {
      const d = await destination(),
        format = type === "folder" ? "" : type;
      const n = {
        ...newNode(
          d.projectId,
          format ? "document" : "folder",
          d.parentId,
          Date.now(),
        ),
        title:
          name.trim() +
          (format && !name.toLowerCase().endsWith("." + format)
            ? "." + format
            : ""),
        ...(format ? { format } : {}),
      };
      const saved = await request(`/projects/${d.projectId}/nodes`, {
        node: n,
        label: "Created in Writer's Archive",
      });
      accept(saved);
      setModal(null);
      if (format) {
        await w.load(saved.projectId, saved.id);
        const view = { ...saved, key: "node/" + saved.id };
        setOpened(view);
        remember(view.key);
      } else setNotice(`Created ${saved.title}`);
    });
  }
  async function ingest(files, replacing = false) {
    const list = [...(files || [])];
    if (!list.length) return;
    await run(async () => {
      if (!(await w.flush())) return;
      const d = await destination();
      for (const file of list) {
        if (file.size > 64 * 1024 * 1024)
          throw Error(`${file.name} exceeds 64 MiB`);
        const bytes = new Uint8Array(await file.arrayBuffer());
        let parsed = null;
        try {
          parsed = await importDocument(bytes, file.name);
        } catch (e) {
          setNotice(
            `${file.name} was stored unchanged. Text conversion was unavailable: ${e.message}`,
          );
        }
        if (
          parsed &&
          new TextEncoder().encode(JSON.stringify(parsed.content)).length >
            2 * 1024 * 1024
        ) {
          parsed = null;
          setNotice(
            `${file.name} was stored unchanged because its text is too large for the document editor.`,
          );
        }
        const source = replacing
          ? w.draft
          : newNode(
              d.projectId,
              parsed ? "document" : "asset",
              d.parentId,
              Date.now(),
            );
        if (replacing && (!source || source.type !== "asset"))
          throw Error(
            "Import the replacement as a new document to retain both editing histories.",
          );
        const n = await uploadArchiveFile(
          {
            ...source,
            type: parsed ? "document" : "asset",
            title: replacing ? source.title : file.name,
            format: parsed?.format || "",
            content: parsed?.content || plainDocument(),
          },
          bytes,
          file.name,
        );
        accept(n);
        setSelected("node/" + n.id);
        if (parsed?.notice) setNotice(parsed.notice);
        if (replacing) {
          await w.load(n.projectId, n.id);
          setOpened({ ...n, key: "node/" + n.id });
        }
      }
      setNotice(
        (n) =>
          n ||
          `Imported ${list.length} ${list.length === 1 ? "file" : "files"}`,
      );
    });
    if (input.current) input.current.value = "";
    if (replace.current) replace.current.value = "";
  }
  async function rename(n) {
    const title = await requestText(
      isFolder(n) ? "Folder name" : "Document name",
      n.title,
    );
    if (!title?.trim() || title.trim() === n.title) return;
    await run(async () => {
      if (!(await w.flush())) return;
      if (n.project)
        await request("/projects", {
          ...data.projects.find((p) => p.id === n.projectId),
          name: title.trim(),
        });
      else {
        const b = await request("/projects/" + n.projectId),
          saved = await request(`/projects/${n.projectId}/nodes`, {
            node: {
              ...b.nodes.find((x) => x.id === n.id),
              title: title.trim(),
            },
            label: "Renamed",
          });
        if (w.draft?.id === saved.id) w.accept(saved);
      }
      await refresh();
    });
  }
  async function action(n, actionName, target) {
    return run(async () => {
      if (!(await w.flush())) return;
      const latest = w.current()?.id === n.id ? w.current() : n;
      await archiveRequest(world, "/actions", {
        action: actionName,
        projectId: n.projectId,
        nodeId: n.project ? "" : n.id,
        revision: latest.revision,
        ...(target || {}),
      });
      const changed = descendants(all, n.key);
      if (changed.has(opened?.key)) setOpened(null);
      if (changed.has("node/" + w.current()?.id)) w.clear();
      setSelected("");
      setModal(null);
      await refresh();
      setNotice(
        actionName === "trash"
          ? `Moved “${n.title}” to trash. You can restore it at any time.`
          : actionName === "restore"
            ? `Restored “${n.title}”`
            : `Moved “${n.title}”`,
      );
    });
  }
  async function exportFile(format) {
    await run(async () => {
      await w.flush();
      const n = w.current();
      download(
        baseName(n.title) + "." + format,
        await exportDocument(n, format),
        mimeFor(format),
      );
    });
  }
  async function duplicate(n) {
    await run(async () => {
      if (!(await w.flush())) return;
      const b = await request(`/projects/${n.projectId}?backup=1`),
        original = materializeManuscripts(b).find((x) => x.id === n.id);
      const saved = await request(`/projects/${n.projectId}/nodes`, {
        node: {
          ...original,
          id: crypto.randomUUID(),
          revision: 0,
          createdAt: 0,
          title:
            baseName(original.title) +
            " (copy)" +
            (original.format ? "." + original.format : ""),
          position: Date.now(),
        },
        label: "Created a copy",
      });
      accept(saved);
      setNotice(`Created ${saved.title}`);
    });
  }
  async function versions() {
    await run(async () => {
      if (!(await w.flush())) return;
      setHistory(
        (
          await request(`/projects/${w.draft.projectId}/versions/${w.draft.id}`)
        ).reverse(),
      );
      setModal({ kind: "history" });
    });
  }
  async function insertImage(file) {
    return run(async () => {
      if (!(await w.flush())) return null;
      if (file.size > 64 * 1024 * 1024) throw Error("Image exceeds 64 MiB");
      const d = w.draft,
        n = await uploadArchiveFile(
          {
            ...newNode(d.projectId, "asset", d.parentId, Date.now()),
            title: file.name,
          },
          new Uint8Array(await file.arrayBuffer()),
          file.name,
        );
      accept(n);
      return imageURL(world, n.fileHash);
    });
  }
  const visible = all
    .filter((n) => {
      if (section === "trash")
        return (
          !n.inworld &&
          !n.projectDeleted &&
          n.deleted &&
          (n.project || !byKey.get(n.parent)?.deleted)
        );
      if (
        n.deleted ||
        n.projectDeleted ||
        crumbs(all, n.parent).some((p) => p.deleted)
      )
        return false;
      if (section === "recent") {
        if (!recent.includes(n.key) || isFolder(n)) return false;
      } else if (search) {
        if (!n.title.toLocaleLowerCase().includes(search.toLocaleLowerCase()))
          return false;
      } else if (n.parent !== folder) return false;
      return (
        filter === "all" ||
        (filter === "folders" && isFolder(n)) ||
        (filter === "images" && kindLabel(n) === "Image") ||
        (filter === "documents" && !isFolder(n) && n.type !== "asset")
      );
    })
    .sort((a, b) =>
      section === "recent"
        ? recent.indexOf(a.key) - recent.indexOf(b.key)
        : Number(isFolder(b)) - Number(isFolder(a)) ||
          (sort === "modified"
            ? (b.updatedAt || 0) - (a.updatedAt || 0)
            : a.title.localeCompare(b.title)),
    );
  const title =
      section === "trash"
        ? "Trash"
        : section === "recent"
          ? "Recent"
          : current?.title || "My archive",
    selectedSet = item ? descendants(all, item.key) : new Set();
  const destinations = [
    { key: ROOT, title: "My archive", projectId: rootProject, id: "" },
    ...all.filter(
      (n) =>
        isFolder(n) &&
        !n.inworld &&
        !n.deleted &&
        !n.projectDeleted &&
        !selectedSet.has(n.key),
    ),
  ];
  const active = opened && !opened.inworld ? w.draft : null,
    errorMessage = error || w.error;
  const iconFor = (n) =>
    isFolder(n)
      ? n.key === INWORLD
        ? "archive"
        : "folder"
      : kindLabel(n) === "Image"
        ? "image"
        : "document";
  function actions(n) {
    return (
      <details
        className="archive-item-menu"
        onClick={(e) => e.stopPropagation()}
      >
        <summary aria-label={`Actions for ${n.title}`}>
          <Icon name="more" />
        </summary>
        <div>
          {n.deleted ? (
            <button onClick={() => action(n, "restore")}>Restore</button>
          ) : n.inworld ? (
            <>
              <button onClick={() => open(n)}>Open</button>
              <button
                onClick={() =>
                  onLibrary(n.ageId || doc.world.currentAge, n.id || "")
                }
              >
                Manage in Library
              </button>
            </>
          ) : (
            <>
              <button onClick={() => open(n)}>Open</button>
              <button onClick={() => rename(n)}>Rename</button>
              {!n.project && (
                <button
                  onClick={() => {
                    setSelected(n.key);
                    setModal({ kind: "move", item: n });
                  }}
                >
                  Move to…
                </button>
              )}
              {!isFolder(n) && (
                <button onClick={() => duplicate(n)}>Make a copy</button>
              )}
              <button className="danger" onClick={() => action(n, "trash")}>
                Move to trash
              </button>
            </>
          )}
        </div>
      </details>
    );
  }
  return (
    <main
      className="writer-archive"
      aria-label="Writer's Archive"
      aria-busy={busy}
    >
      {errorMessage && (
        <div className="archive-alert" role="alert">
          {errorMessage}
          <button
            onClick={() => {
              setError("");
              w.setError("");
            }}
          >
            Dismiss
          </button>
          {w.error && (
            <button onClick={() => void w.flush()}>Retry save</button>
          )}
        </div>
      )}
      {opened?.resourceReview ? (
        <ResourceArchiveReview
          world={world}
          ageId={opened.ageId}
          archiveId={opened.resourceArchiveId}
          initialArea={opened.initialArea || ""}
          onBack={() => browse(folder)}
          onDocuments={() =>
            browse(opened.key.replace(/\/review$/, "/documents"))
          }
        />
      ) : opened?.inworld ? (
        <LinkedArchiveFile
          key={opened.key}
          item={opened}
          world={world}
          onBack={() => browse(folder)}
          onLibrary={() => onLibrary(opened.ageId, opened.id)}
          onResources={() => {
            let owner = opened.file?.archiveId,
              resources = "";
            const visited = new Set();
            while (owner && !visited.has(owner)) {
              visited.add(owner);
              const e = inworld?.entities.find(
                (e) => e.ageId === opened.ageId && e.id === owner,
              );
              if (!e?.properties?.resourceFolder) break;
              if (e.properties.resourceFolder === "resources") resources = e.id;
              owner = e.properties.parentArchive;
            }
            setOpened({
              ...opened,
              key: `inworld/${opened.ageId}/${resources || `resources/${owner}`}/review`,
              resourceReview: true,
              resourceArchiveId: owner,
              initialArea: opened.id,
            });
          }}
          onDocument={onDocument}
          onRefresh={refresh}
        />
      ) : opened && active ? (
        <section className="archive-document">
          <header className="archive-document-header">
            <button
              className="archive-icon-button"
              aria-label="Back to archive"
              onClick={() => browse(folder)}
            >
              <Icon name="back" />
            </button>
            <span className="archive-document-symbol">
              <Icon name="document" size={30} />
            </span>
            <div className="archive-document-heading">
              <input
                aria-label="Document name"
                value={active.title}
                onChange={(e) => w.edit({ title: e.target.value })}
                onBlur={() => {
                  if (active.title.trim()) void w.flush();
                }}
              />
              <small>
                {doc.world.name} / Writer's Archive{" "}
                <span role="status">{w.status}</span>
              </small>
            </div>
            <button
              className="archive-icon-button"
              aria-label="Version history"
              onClick={versions}
            >
              <Icon name="clock" />
            </button>
            <details className="archive-item-menu archive-file-menu">
              <summary>
                File <Icon name="chevron" size={14} />
              </summary>
              <div>
                <button onClick={() => rename({ ...opened, ...active })}>
                  Rename
                </button>
                <button onClick={() => duplicate(active)}>Make a copy</button>
                <button onClick={versions}>Version history</button>
                {active.fileHash && (
                  <button
                    onClick={() =>
                      run(async () => {
                        const r = await fetch(fileURL(active));
                        if (!r.ok) throw Error("Unable to download original");
                        download(
                          active.fileName,
                          await r.arrayBuffer(),
                          active.mime,
                        );
                      })
                    }
                  >
                    Download original import
                  </button>
                )}
                <button
                  className="danger"
                  onClick={() => action({ ...opened, ...active }, "trash")}
                >
                  Move to trash
                </button>
              </div>
            </details>
            {active.type !== "asset" && (
              <details className="archive-item-menu archive-export-menu">
                <summary>
                  <Icon name="download" size={17} />
                  Download
                </summary>
                <div>
                  {formats.map(([f, label]) => (
                    <button key={f} onClick={() => exportFile(f)}>
                      {label}
                    </button>
                  ))}
                </div>
              </details>
            )}
          </header>
          {w.recovery && (
            <div className="archive-notice">
              A local draft is available.
              <button onClick={w.recover}>Restore local draft</button>
              <button onClick={() => w.setRecovery(null)}>
                Keep server version
              </button>
            </div>
          )}
          {notice && (
            <div className="archive-notice">
              {notice}
              <button onClick={() => setNotice("")} aria-label="Dismiss notice">
                <Icon name="close" size={16} />
              </button>
            </div>
          )}
          {active.type === "asset" ? (
            <ArchiveFilePreview
              key={active.id + active.fileHash}
              node={active}
              url={fileURL(active)}
              onReplace={() => replace.current?.click()}
            />
          ) : (
            <ArchiveEditor
              key={active.id + ":" + w.epoch}
              worldId={world}
              sourceWorldId={w.project?.importedWorldId}
              node={active}
              onRoom={(r) => {
                w.room.current = r;
              }}
              onChange={w.edit}
              onSave={w.flush}
              onImage={insertImage}
            />
          )}
        </section>
      ) : (
        <div className="archive-browser">
          <aside className="archive-sidebar">
            <div className="archive-identity">
              <Icon name="archive" size={29} />
              <div>
                <h1>Writer's Archive</h1>
                <span>{doc.world.name}</span>
              </div>
            </div>
            <details className="archive-new-menu">
              <summary>
                <Icon name="plus" size={23} />
                New
              </summary>
              <div>
                <button
                  disabled={busy || linked || section === "trash"}
                  onClick={() => {
                    setModal({ kind: "create", type: "folder" });
                    document
                      .querySelector(".archive-new-menu")
                      ?.removeAttribute("open");
                  }}
                >
                  <Icon name="folder" />
                  New folder
                </button>
                {formats.map(([type, label]) => (
                  <button
                    disabled={busy || linked || section === "trash"}
                    key={type}
                    onClick={() => {
                      setModal({ kind: "create", type });
                      document
                        .querySelector(".archive-new-menu")
                        ?.removeAttribute("open");
                    }}
                  >
                    <Icon name="document" />
                    {label}
                  </button>
                ))}
                <hr />
                <button
                  disabled={busy || linked || section === "trash"}
                  onClick={() => input.current?.click()}
                >
                  <Icon name="upload" />
                  Import files
                </button>
              </div>
            </details>
            <nav aria-label="Archive collections">
              {[
                ["files", ROOT, "folder", "My archive"],
                ["files", INWORLD, "archive", "Inworld Archive"],
                ["recent", ROOT, "clock", "Recent"],
                ["trash", ROOT, "trash", "Trash"],
              ].map(([scope, key, icon, label]) => (
                <button
                  key={label}
                  aria-current={
                    section === scope &&
                    (scope !== "files" || (key === INWORLD ? linked : !linked))
                      ? "page"
                      : undefined
                  }
                  onClick={() => browse(key, scope)}
                >
                  <Icon name={icon} />
                  {label}
                </button>
              ))}
            </nav>
            <div className="archive-sidebar-bottom">
              <span>PART OF YOUR WORLD</span>
              <p>
                Notes, images, research, and finished stories, all in one place.
              </p>
              <button onClick={onBack}>
                <Icon name="back" size={16} />
                Return to Ages
              </button>
            </div>
          </aside>
          <section
            className="archive-main"
            onDragOver={(e) => {
              e.preventDefault();
              if (
                !linked &&
                section === "files" &&
                e.dataTransfer.types.includes("Files")
              )
                setDragging(true);
            }}
            onDragLeave={(e) => {
              if (!e.currentTarget.contains(e.relatedTarget))
                setDragging(false);
            }}
            onDrop={(e) => {
              e.preventDefault();
              setDragging(false);
              if (!linked && section === "files" && e.dataTransfer.files.length)
                void ingest(e.dataTransfer.files);
            }}
          >
            <div className="archive-search-row">
              <label className="archive-search">
                <Icon name="search" />
                <input
                  aria-label="Search archive by name"
                  placeholder="Search your archive"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
                {search && (
                  <button
                    aria-label="Clear search"
                    onClick={() => setSearch("")}
                  >
                    <Icon name="close" size={16} />
                  </button>
                )}
              </label>
              <button
                className="archive-icon-button"
                aria-label="Refresh archive"
                disabled={busy}
                onClick={() => run(refresh)}
              >
                <Icon name="restore" />
              </button>
              <button
                className="archive-icon-button"
                aria-label="Item details"
                aria-pressed={info}
                onClick={() => setInfo((v) => !v)}
              >
                <Icon name="info" />
              </button>
            </div>
            <div className="archive-location-row">
              <nav aria-label="Archive breadcrumbs">
                <button onClick={() => browse(ROOT)}>
                  {section === "files" ? "My archive" : title}
                </button>
                {section === "files" &&
                  crumbs(all, folder).map((c) => (
                    <span key={c.key}>
                      <Icon name="chevron" size={15} />
                      <button onClick={() => browse(c.key)}>{c.title}</button>
                    </span>
                  ))}
              </nav>
              <div className="archive-view-toggle" aria-label="Archive view">
                <button
                  aria-label="List view"
                  aria-pressed={layout === "list"}
                  onClick={() => setLayout("list")}
                >
                  <Icon name="list" size={18} />
                </button>
                <button
                  aria-label="Grid view"
                  aria-pressed={layout === "grid"}
                  onClick={() => setLayout("grid")}
                >
                  <Icon name="grid" size={17} />
                </button>
              </div>
            </div>
            <div className="archive-filter-row">
              <select
                aria-label="File type"
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
              >
                <option value="all">All types</option>
                <option value="folders">Folders</option>
                <option value="documents">Documents</option>
                <option value="images">Images</option>
              </select>
              <select
                aria-label="Sort files"
                value={sort}
                onChange={(e) => setSort(e.target.value)}
              >
                <option value="name">Name A–Z</option>
                <option value="modified">Last modified</option>
              </select>
              <span>{visible.length} items</span>
              {linked ? (
                <button
                  onClick={() =>
                    onLibrary(
                      current?.ageId || doc.world.currentAge,
                      current?.id || "",
                    )
                  }
                >
                  <Icon name="link" size={16} />
                  Open in Library
                </button>
              ) : (
                section !== "trash" && (
                  <button
                    disabled={busy}
                    onClick={() => input.current?.click()}
                  >
                    <Icon name="upload" size={16} />
                    Import files
                  </button>
                )
              )}
              {current && !current.inworld && !current.project && (
                <button onClick={() => open(current, true)}>
                  Edit folder notes
                </button>
              )}
            </div>
            {notice && (
              <div className="archive-notice" role="status">
                {notice}
                <button
                  aria-label="Dismiss notice"
                  onClick={() => setNotice("")}
                >
                  <Icon name="close" size={16} />
                </button>
              </div>
            )}
            {linked && (
              <p className="archive-linked-notice">
                <Icon name="link" size={15} />
                Linked to the inworld Library. Each Age keeps its own archives
                and document versions.
              </p>
            )}
            {section === "trash" && (
              <p className="archive-linked-notice">
                Deleted folders and documents stay here until you restore them.
              </p>
            )}
            {item && (
              <div className="archive-selection-bar">
                <button
                  className="archive-icon-button"
                  aria-label="Clear selection"
                  onClick={() => setSelected("")}
                >
                  <Icon name="close" size={17} />
                </button>
                <span>{item.title}</span>
                {item.deleted ? (
                  <button
                    disabled={busy}
                    onClick={() => action(item, "restore")}
                  >
                    <Icon name="restore" size={16} />
                    Restore
                  </button>
                ) : item.inworld ? (
                  <button
                    onClick={() =>
                      onLibrary(
                        item.ageId || doc.world.currentAge,
                        item.id || "",
                      )
                    }
                  >
                    Manage in Library
                  </button>
                ) : (
                  <>
                    <button disabled={busy} onClick={() => rename(item)}>
                      Rename
                    </button>
                    {!item.project && (
                      <button
                        disabled={busy}
                        onClick={() => setModal({ kind: "move", item })}
                      >
                        <Icon name="move" size={16} />
                        Move
                      </button>
                    )}
                    <button
                      disabled={busy}
                      onClick={() => action(item, "trash")}
                    >
                      <Icon name="trash" size={16} />
                      Delete
                    </button>
                  </>
                )}
              </div>
            )}
            <div className={`archive-content ${info ? "with-details" : ""}`}>
              <div className="archive-files-scroll">
                {loading ? (
                  <div className="archive-empty" role="status">
                    Loading your archive…
                  </div>
                ) : !visible.length ? (
                  <div className="archive-empty">
                    <Icon
                      name={
                        section === "trash"
                          ? "trash"
                          : search
                            ? "search"
                            : "folder"
                      }
                      size={54}
                    />
                    <h2>
                      {search
                        ? "No matching files"
                        : section === "trash"
                          ? "Your trash is empty"
                          : "A place for your next idea"}
                    </h2>
                    <p>
                      {search
                        ? "Try another name or file type."
                        : linked
                          ? "Create archives and documents in the Library. They will appear here automatically."
                          : section === "trash"
                            ? "Files you delete will appear here."
                            : "Create a folder or document, or drop files here to import them."}
                    </p>
                    {!linked && section !== "trash" && !search && (
                      <button
                        onClick={() =>
                          setModal({ kind: "create", type: "docx" })
                        }
                      >
                        Create a document
                      </button>
                    )}
                  </div>
                ) : layout === "list" ? (
                  <table className="archive-file-table">
                    <thead>
                      <tr>
                        <th>Name</th>
                        <th>Type</th>
                        <th>Last modified</th>
                        <th>Size</th>
                        <th>
                          <span className="archive-visually-hidden">
                            Actions
                          </span>
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {visible.map((n) => (
                        <tr
                          key={n.key}
                          className={selected === n.key ? "selected" : ""}
                          onClick={() => setSelected(n.key)}
                          onDoubleClick={() => open(n)}
                          tabIndex={0}
                          onKeyDown={(e) => {
                            if (e.target !== e.currentTarget) return;
                            if (e.key === "Enter") void open(n);
                            if (
                              e.key === "Delete" &&
                              !n.inworld &&
                              !n.deleted
                            ) {
                              e.preventDefault();
                              void action(n, "trash");
                            }
                          }}
                          draggable={!n.inworld && !n.project && !n.deleted}
                          onDragStart={(e) =>
                            e.dataTransfer.setData(
                              "application/x-kriemhild-archive",
                              n.key,
                            )
                          }
                          onDragOver={(e) => {
                            if (isFolder(n) && !n.inworld) e.preventDefault();
                          }}
                          onDrop={(e) => {
                            const source = byKey.get(
                              e.dataTransfer.getData(
                                "application/x-kriemhild-archive",
                              ),
                            );
                            if (!source || !isFolder(n) || n.inworld) return;
                            e.preventDefault();
                            e.stopPropagation();
                            void action(source, "move", {
                              targetProject: n.projectId,
                              parentId: n.project ? "" : n.id,
                            });
                          }}
                        >
                          <td>
                            <button
                              className="archive-name"
                              onClick={(e) => {
                                e.stopPropagation();
                                if (n.deleted) setSelected(n.key);
                                else void open(n);
                              }}
                            >
                              <span
                                className={`archive-file-icon ${iconFor(n)}`}
                              >
                                <Icon name={iconFor(n)} size={22} />
                              </span>
                              <span>
                                {n.title}
                                {n.key === INWORLD && (
                                  <small>Linked to your world</small>
                                )}
                              </span>
                            </button>
                          </td>
                          <td>{kindLabel(n)}</td>
                          <td>
                            {n.updatedAt
                              ? new Date(n.updatedAt).toLocaleDateString(
                                  undefined,
                                  {
                                    month: "short",
                                    day: "numeric",
                                    year: "numeric",
                                  },
                                )
                              : "—"}
                          </td>
                          <td>
                            {isFolder(n)
                              ? "—"
                              : sizeLabel(
                                  n.fileSize ?? n.file?.versions.at(-1)?.size,
                                )}
                          </td>
                          <td>{actions(n)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                ) : (
                  <div className="archive-file-grid">
                    {visible.map((n) => (
                      <article
                        key={n.key}
                        className={`archive-file-card ${selected === n.key ? "selected" : ""}`}
                        onClick={() => setSelected(n.key)}
                      >
                        <header>
                          <Icon name={iconFor(n)} size={18} />
                          <button
                            onClick={() =>
                              n.deleted ? setSelected(n.key) : open(n)
                            }
                          >
                            {n.title}
                          </button>
                          {actions(n)}
                        </header>
                        <button
                          className={`archive-card-preview ${iconFor(n)}`}
                          onClick={() =>
                            n.deleted ? setSelected(n.key) : open(n)
                          }
                          aria-label={`Open ${n.title}`}
                        >
                          {kindLabel(n) === "Image" ? (
                            <img
                              src={
                                n.inworld
                                  ? libraryURL(world, n.file)
                                  : fileURL(n)
                              }
                              alt=""
                              loading="lazy"
                            />
                          ) : (
                            <Icon name={iconFor(n)} size={64} />
                          )}
                        </button>
                        <footer>
                          {kindLabel(n)}
                          {n.key === INWORLD ? " · Linked" : ""}
                        </footer>
                      </article>
                    ))}
                  </div>
                )}
              </div>
              {info && (
                <aside className="archive-details">
                  <h3>{item?.title || title}</h3>
                  <Icon name={item ? iconFor(item) : "folder"} size={48} />
                  <dl>
                    <dt>World</dt>
                    <dd>{doc.world.name}</dd>
                    <dt>Location</dt>
                    <dd>
                      {item
                        ? crumbs(all, item.parent)
                            .map((n) => n.title)
                            .join(" / ") || "My archive"
                        : title}
                    </dd>
                    {item && (
                      <>
                        <dt>Type</dt>
                        <dd>{kindLabel(item)}</dd>
                      </>
                    )}
                  </dl>
                  <p>
                    {item?.inworld
                      ? "Linked to its Library record. Edits to document contents are shared."
                      : "Everything in this archive belongs to this World."}
                  </p>
                </aside>
              )}
            </div>
            {dragging && (
              <div className="archive-drop-overlay">
                <Icon name="upload" size={48} />
                <strong>Drop files to import</strong>
                <span>
                  Documents, images, and other files · up to 64 MiB each
                </span>
              </div>
            )}
          </section>
        </div>
      )}
      <input
        ref={input}
        type="file"
        multiple
        hidden
        onChange={(e) => void ingest(e.target.files)}
      />
      <input
        ref={replace}
        type="file"
        hidden
        onChange={(e) => void ingest(e.target.files, true)}
      />
      {modal?.kind === "create" && (
        <Dialog
          title={modal.type === "folder" ? "New folder" : "New document"}
          onClose={() => setModal(null)}
          onSubmit={async (e) => {
            const f = new FormData(e.currentTarget);
            await create(f.get("format") || "folder", f.get("name"));
          }}
          busy={busy}
          actions={
            <button type="submit" className="primary">
              Create
            </button>
          }
        >
          <label>
            Name
            <input
              name="name"
              required
              maxLength={240}
              defaultValue={
                modal.type === "folder"
                  ? "Untitled folder"
                  : "Untitled document"
              }
              autoFocus
              onFocus={(e) => e.target.select()}
            />
          </label>
          {modal.type !== "folder" && (
            <label>
              Document format
              <select name="format" defaultValue={modal.type}>
                {formats.map(([f, label]) => (
                  <option key={f} value={f}>
                    {label}
                  </option>
                ))}
              </select>
            </label>
          )}
        </Dialog>
      )}
      {modal?.kind === "move" && (
        <Dialog
          title={`Move “${modal.item.title}”`}
          onClose={() => setModal(null)}
          onSubmit={async (e) => {
            const key = new FormData(e.currentTarget).get("destination"),
              d = destinations.find((x) => x.key === key);
            if (d.projectId === rootProject) await ensureProject();
            await action(modal.item, "move", {
              targetProject: d.projectId,
              parentId: d.project ? "" : d.id || "",
            });
          }}
          busy={busy}
          actions={
            <button type="submit" className="primary">
              Move here
            </button>
          }
        >
          <label>
            Destination folder
            <select name="destination" defaultValue={ROOT}>
              {destinations.map((d) => (
                <option key={d.key} value={d.key}>
                  {d.key === ROOT
                    ? d.title
                    : crumbs(all, d.key)
                        .map((x) => x.title)
                        .join(" / ")}
                </option>
              ))}
            </select>
          </label>
        </Dialog>
      )}
      {modal?.kind === "history" && (
        <Dialog
          title="Version history"
          size="editor"
          trackChanges={false}
          onClose={() => setModal(null)}
        >
          <p>
            Every save keeps an earlier version. Restoring adds a new version.
          </p>
          <div className="archive-version-list">
            {history.map((v) => (
              <article key={v.node.revision}>
                <div>
                  <strong>Version {v.node.revision}</strong>
                  <span>
                    {new Date(v.node.updatedAt).toLocaleString()} · {v.label}
                  </span>
                </div>
                <pre>
                  {textContent(v.node.content).slice(0, 600) ||
                    v.node.fileName ||
                    "Empty document"}
                </pre>
                <button
                  disabled={busy || v.node.revision === w.draft?.revision}
                  onClick={() =>
                    run(async () => {
                      if (!(await w.flush())) return;
                      w.accept(
                        await request(
                          `/projects/${w.draft.projectId}/recovery/${w.draft.id}`,
                          {
                            version: v.node.revision,
                            revision: w.draft.revision,
                          },
                        ),
                      );
                      setModal(null);
                    })
                  }
                >
                  Restore this version
                </button>
                {v.node.fileHash && (
                  <a
                    href={fileURL(v.node, v.node.revision)}
                    download={v.node.fileName}
                  >
                    Download file version
                  </a>
                )}
              </article>
            ))}
          </div>
        </Dialog>
      )}
    </main>
  );
}
