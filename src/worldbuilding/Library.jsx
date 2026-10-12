import { confirmAction } from "./ui/feedback.js";
import useUnsavedChanges from "./ui/useUnsavedChanges.js";
import Dialog from "./ui/Dialog.jsx";
import { useEffect, useRef, useState } from "react";
import { activeEntities, registry, label } from "./model.js";
import { newLibraryFile } from "./library-files.js";
import LibraryEditor from "./LibraryEditor.jsx";
import "./library.css";
import {
  ArchiveOwnerField,
  ArchiveSettings,
  GatherDocumentsDialog,
  ArchiveCustodyDialog,
  DocumentProvenance,
} from "./ArchiveTools.jsx";
import {
  archiveCategories,
  archiveDefaults,
  matchesArchiveFilters,
} from "./archives.js";

const id = () => crypto.randomUUID();
const documentTypes = Object.entries(registry.types).filter(
  ([k, v]) => v.category === "records" && k !== "archive",
);
export default function Library({
  doc,
  age,
  command,
  onDocument,
  onProfile,
  onAtlas,
  onOrganizations,
  query,
}) {
  const all = activeEntities(doc, age.id),
    archives = all.filter((e) => e.kind === "archive"),
    files = (doc.libraryFiles || []).filter((f) => f.ageId === age.id),
    initial = query?.get("record") || "";
  const [showArchives, setShowArchives] = useState(false),
    [folder, setFolder] = useState(
      archives.some((a) => a.id === initial)
        ? initial
        : files.find((f) => f.id === initial)?.archiveId || "",
    ),
    [selected, setSelected] = useState(
      files.some((f) => f.id === initial) ? initial : "",
    ),
    [search, setSearch] = useState(""),
    [dialog, setDialog] = useState(null),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [dirty, setDirty] = useState(false),
    [content, setContent] = useState(null),
    [version, setVersion] = useState("");
  const [archiveDialog, setArchiveDialog] = useState("");
  const [filters, setFilters] = useState({
    age: "",
    type: "",
    entity: "",
    year: "",
    recorded: "",
  });
  const upload = useRef(null),
    replace = useRef(null),
    current = all.find((e) => e.id === selected),
    archive = archives.find((a) => a.id === folder),
    file = files.find((f) => f.id === selected),
    latest = file?.versions.at(-1),
    shown = file?.versions.find((v) => v.hash === version) || latest;
  const mayLeave = useUnsavedChanges(
    dirty,
    () => setDirty(false),
    "Discard unsaved file edits?",
  );
  const url = (entity, hash = "") =>
    `/api/worlds/${doc.world.id}/library/${entity}?age=${age.id}${hash ? "&version=" + hash : ""}`;
  async function openFolder(value) {
    if (!(await mayLeave())) return;
    setDirty(false);
    setFolder(value);
    setFilters({ age: "", type: "", entity: "", year: "", recorded: "" });
    setShowArchives(false);
    setSelected("");
    setVersion("");
    setError("");
  }
  async function openFile(value) {
    if (!(await mayLeave())) return;
    setDirty(false);
    setSelected(value);
    setVersion("");
    setError("");
  }
  useEffect(() => {
    if (!selected || !shown) return;
    const controller = new AbortController();
    setContent(null);
    setError("");
    fetch(url(selected, shown.hash), { signal: controller.signal })
      .then(async (r) => {
        if (!r.ok) throw Error("Unable to load document");
        return new Uint8Array(await r.arrayBuffer());
      })
      .then(setContent)
      .catch((e) => {
        if (e.name !== "AbortError") setError(e.message);
      });
    return () => controller.abort();
  }, [selected, shown?.hash]);
  async function uploadFile(blob, name, entity, archiveID, base = doc) {
    const form = new FormData();
    form.append(
      "metadata",
      JSON.stringify({
        revision: base.world.revision,
        entity,
        archiveId: archiveID,
      }),
    );
    form.append("file", new Blob([blob]), name);
    const r = await fetch(`/api/worlds/${doc.world.id}/library`, {
      method: "POST",
      body: form,
    });
    const d = await r.json();
    if (!r.ok) throw Error(d.error || "Unable to save file");
    onDocument(d);
    return d;
  }
  async function saveBytes(bytes, name) {
    setBusy(true);
    setError("");
    try {
      await uploadFile(bytes, name, current, folder);
      setVersion("");
      setDirty(false);
    } catch (e) {
      setError(e.message);
      throw e;
    } finally {
      setBusy(false);
    }
  }
  async function ingest(list, isReplace = false) {
    if (!list?.length || !(await mayLeave())) return;
    setBusy(true);
    setError("");
    try {
      let base = doc;
      for (const f of list) {
        if (f.size > 64 * 1024 * 1024) throw Error(`${f.name} exceeds 64 MiB`);
        const e = isReplace
          ? current
          : {
              id: id(),
              ageId: age.id,
              name: f.name.replace(/\.[^.]+$/, ""),
              kind: "document",
              properties: {},
            };
        base = await uploadFile(await f.arrayBuffer(), f.name, e, folder, base);
        setSelected(e.id);
      }
      setVersion("");
      setDirty(false);
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
      if (upload.current) upload.current.value = "";
      if (replace.current) replace.current.value = "";
    }
  }
  async function remove(entity) {
    if (
      !(await confirmAction(
        `Delete ${entity.name} from this Age? Other Ages retain their records.${dirty ? " Unsaved file edits will be discarded." : ""}`,
      ))
    )
      return;
    const d = await command({
      kind: "delete-entities",
      ageId: age.id,
      entityIds: [entity.id],
    });
    if (d) {
      setDirty(false);
      setSelected("");
      if (entity.kind === "archive")
        setFolder(entity.properties?.parentArchive || "");
    }
  }
  const locations = (a) =>
    doc.relations
      .filter(
        (r) =>
          r.ageId === age.id &&
          r.source === a?.id &&
          r.kind === "archive-located-at",
      )
      .map((r) => all.find((e) => e.id === r.destination))
      .filter(Boolean);
  function archiveRows(parent = "", depth = 0) {
    return archives
      .filter((a) => (a.properties?.parentArchive || "") === parent)
      .sort((a, b) => a.name.localeCompare(b.name))
      .map((a) => (
        <div key={a.id}>
          <button
            className={folder === a.id ? "selected" : ""}
            style={{ paddingLeft: 12 + depth * 14 }}
            onClick={() => openFolder(a.id)}
          >
            <span aria-hidden="true">▱</span> {a.name}
          </button>
          {archiveRows(a.id, depth + 1)}
        </div>
      ));
  }
  return (
    <main className="world-page library-workspace">
      <header className="workspace-heading">
        <div>
          <span className="eyebrow">
            {doc.world.name} / {age.name}
          </span>
          <h1>Library</h1>
          <p>Archives and documents that exist inside your world.</p>
        </div>
        <button
          onClick={async () => {
            if (await mayLeave()) {
              setDirty(false);
              setDialog({ type: "archive", parent: folder });
            }
          }}
        >
          Create archive folder
        </button>
      </header>
      {error && (
        <p role="alert" className="library-error">
          {error}
        </p>
      )}
      <button
        className="library-folder-toggle"
        aria-expanded={showArchives}
        aria-controls="library-archives"
        onClick={() => setShowArchives(!showArchives)}
      >
        {showArchives ? "Hide archives" : "Browse archives"}
      </button>
      <div className="library-layout">
        <aside
          id="library-archives"
          className={`archive-tree ${showArchives ? "is-expanded" : ""}`}
        >
          <h2>Archives</h2>
          <button onClick={() => openFolder("")}>All archives</button>
          {archiveRows()}
          {!archives.length && (
            <p>Create an archive and link it to a place or organization.</p>
          )}
        </aside>
        <section className="library-main">
          {current && file ? (
            <>
              <div className="library-toolbar">
                <button onClick={() => openFolder(folder)}>
                  ← {archive?.name || "Archive"}
                </button>
                <h2>{current.name}</h2>
                {dirty && <span>Unsaved edits</span>}
              </div>
              <div className="library-toolbar">
                <a href={url(selected, shown?.hash)}>Download file</a>
                <button
                  disabled={busy || ["lost", "destroyed"].includes(file.status)}
                  onClick={() => replace.current.click()}
                >
                  Replace file
                </button>
                <button
                  onClick={() =>
                    setDialog({
                      type: "metadata",
                      entity: current,
                      archive: folder,
                    })
                  }
                >
                  Document properties
                </button>
                <button onClick={() => remove(current)}>Delete document</button>
                <label>
                  File version
                  <select
                    value={version}
                    onChange={async (e) => {
                      const nextVersion = e.target.value;
                      if (await mayLeave()) {
                        setVersion(nextVersion);
                        setDirty(false);
                      }
                    }}
                  >
                    <option value="">Latest — {latest.name}</option>
                    {file.versions.slice(0, -1).map((v, i) => (
                      <option key={i} value={v.hash}>
                        {i + 1}: {v.name} ·{" "}
                        {new Date(v.createdAt).toLocaleString()}
                      </option>
                    ))}
                  </select>
                </label>
                {version && (
                  <button
                    disabled={busy || !content}
                    onClick={() =>
                      saveBytes(content, shown.name).catch(() => {})
                    }
                  >
                    Restore as new version
                  </button>
                )}
              </div>
              <div className="library-toolbar">
                <button onClick={() => setArchiveDialog("custody")}>
                  Preservation & custody
                </button>
                {file.record && (
                  <button
                    disabled={
                      !archive?.archive ||
                      file.status === "destroyed" ||
                      file.status === "lost"
                    }
                    onClick={() => setArchiveDialog("revise")}
                  >
                    Revise generated document
                  </button>
                )}
              </div>
              <DocumentProvenance
                file={{ ...file, record: shown?.provenance || file.record }}
                doc={doc}
                onProfile={onProfile}
              />
              {["lost", "destroyed"].includes(file.status) && (
                <p>
                  This is a preserved historical view of a {file.status}{" "}
                  physical document. Its contents cannot be saved or replaced.
                </p>
              )}
              <input
                hidden
                type="file"
                ref={replace}
                onChange={(e) => ingest(e.target.files, true)}
              />
              <p className="subtle">
                {label(current.kind)} · {shown?.name} ·{" "}
                {Math.ceil((shown?.size || 0) / 1024)} KiB · {age.name}
              </p>
              {content ? (
                <LibraryEditor
                  key={selected + shown.hash + version}
                  bytes={content}
                  name={shown.name}
                  onSave={saveBytes}
                  onDirty={setDirty}
                  busy={busy || ["lost", "destroyed"].includes(file.status)}
                />
              ) : (
                <p>Loading file…</p>
              )}
            </>
          ) : (
            <>
              <div className="library-toolbar">
                <h2>{archive?.name || "Archives"}</h2>
                {archive && (
                  <>
                    <button
                      onClick={() =>
                        setDialog({
                          type: "archive",
                          entity: archive,
                          parent: archive.properties?.parentArchive || "",
                        })
                      }
                    >
                      Archive properties & locations
                    </button>
                    <button onClick={() => remove(archive)}>
                      Delete archive
                    </button>
                  </>
                )}
              </div>
              {archive && (
                <>
                  <section className="archive-context">
                    <div className="library-toolbar">
                      <button onClick={() => setArchiveDialog("settings")}>
                        Archive Settings
                      </button>
                      <button
                        className="primary"
                        disabled={!archive.archive || busy}
                        onClick={() => setArchiveDialog("gather")}
                      >
                        Gather Documents
                      </button>
                    </div>
                    {archive.archive ? (
                      <p>
                        Owner:{" "}
                        {all.find((e) => e.id === archive.archive.ownerId)
                          ?.name || "Historical owner"}{" "}
                        · {archive.archive.purpose} archive ·{" "}
                        {archive.archive.intensity} documentation ·{" "}
                        {archive.archive.knowledgeScope} knowledge.{" "}
                        {(archive.archive.coverage || []).length} covered
                        records.{" "}
                        {archive.archive.retention === "review" &&
                          "Review inherited custody and preservation in this Age."}
                      </p>
                    ) : (
                      <p>
                        Select an owner in Archive Settings to enable
                        knowledge-limited document gathering. Existing files
                        remain available.
                      </p>
                    )}
                    {files.some(
                      (f) => f.archiveId === archive.id && f.record,
                    ) && (
                      <p>
                        Historical coverage:{" "}
                        {[
                          ...new Set(
                            files
                              .filter(
                                (f) => f.archiveId === archive.id && f.record,
                              )
                              .map(
                                (f) =>
                                  `${doc.ages.find((a) => a.id === f.record.sourceAge)?.name || "Historical Age"}, year ${f.record.fromYear}`,
                              ),
                          ),
                        ]
                          .slice(0, 12)
                          .join("; ")}
                      </p>
                    )}
                  </section>
                  <div className="archive-locations">
                    <span>Locations & custodians: </span>
                    {locations(archive).map((e) => (
                      <button key={e.id} onClick={() => onProfile(e)}>
                        {e.name} <small>{label(e.kind)}</small>
                      </button>
                    ))}
                    {!locations(archive).length && (
                      <span>
                        Link a place or organization in Archive properties.
                      </span>
                    )}
                  </div>
                  <div className="archive-form-grid archive-filters">
                    <label>
                      Source Age
                      <select
                        value={filters.age}
                        onChange={(e) =>
                          setFilters({ ...filters, age: e.target.value })
                        }
                      >
                        <option value="">All Ages</option>
                        {doc.ages.map((a) => (
                          <option key={a.id} value={a.id}>
                            {a.name}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label>
                      Record category
                      <select
                        value={filters.type}
                        onChange={(e) =>
                          setFilters({ ...filters, type: e.target.value })
                        }
                      >
                        <option value="">All categories</option>
                        {archiveCategories.map((v) => (
                          <option key={v}>{v}</option>
                        ))}
                      </select>
                    </label>
                    <label>
                      Related entity
                      <select
                        value={filters.entity}
                        onChange={(e) =>
                          setFilters({ ...filters, entity: e.target.value })
                        }
                      >
                        <option value="">All entities</option>
                        {all.map((e) => (
                          <option key={e.id} value={e.id}>
                            {e.name}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label>
                      Covered year
                      <input
                        type="number"
                        step="1"
                        value={filters.year}
                        onChange={(e) =>
                          setFilters({ ...filters, year: e.target.value })
                        }
                      />
                    </label>
                    <label>
                      Recording year
                      <input
                        type="number"
                        step="1"
                        value={filters.recorded}
                        onChange={(e) =>
                          setFilters({ ...filters, recorded: e.target.value })
                        }
                      />
                    </label>
                  </div>
                  <div className="library-toolbar">
                    <button
                      disabled={busy}
                      onClick={() => setDialog({ type: "document" })}
                    >
                      Create document
                    </button>
                    <button
                      disabled={busy}
                      onClick={() => upload.current.click()}
                    >
                      Upload files
                    </button>
                    <input
                      hidden
                      multiple
                      type="file"
                      ref={upload}
                      onChange={(e) => ingest(e.target.files)}
                    />
                    <label>
                      Search documents
                      <input
                        type="text"
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                      />
                    </label>
                  </div>
                </>
              )}
              <div className="archive-list">
                {archives
                  .filter((a) => (a.properties?.parentArchive || "") === folder)
                  .map((a) => (
                    <button key={a.id} onClick={() => openFolder(a.id)}>
                      ▱ {a.name}
                      <small>
                        {locations(a)
                          .map((e) => e.name)
                          .join(", ")}
                      </small>
                    </button>
                  ))}
              </div>
              {archive && (
                <div className="library-file-list">
                  {files
                    .filter(
                      (f) =>
                        f.archiveId === folder &&
                        matchesArchiveFilters(f, filters),
                    )
                    .map((f) => ({ f, e: all.find((e) => e.id === f.id) }))
                    .filter(
                      ({ e }) =>
                        e &&
                        `${e.name} ${e.properties?.summary || ""}`
                          .toLowerCase()
                          .includes(search.toLowerCase()),
                    )
                    .map(({ f, e }) => (
                      <button key={e.id} onClick={() => openFile(e.id)}>
                        <span>▤ {e.name}</span>
                        <small>
                          {label(e.kind)} · {f.versions.at(-1).name} ·{" "}
                          {f.versions.length} version(s) ·{" "}
                          {f.status || "present"}
                          {f.record &&
                            ` · ${f.record.category} · recorded ${f.record.recordingYear}`}
                        </small>
                      </button>
                    ))}
                  {!files.some((f) => f.archiveId === folder) && (
                    <p>
                      No documents yet. Create a document or upload a file, up
                      to 64 MiB each.
                    </p>
                  )}
                </div>
              )}
            </>
          )}
        </section>
      </div>
      {archiveDialog === "settings" && archive && (
        <ArchiveSettings
          archive={archive}
          doc={doc}
          age={age}
          all={all}
          command={command}
          close={() => setArchiveDialog("")}
        />
      )}
      {["gather", "revise"].includes(archiveDialog) && archive && (
        <GatherDocumentsDialog
          doc={doc}
          age={age}
          archive={archive}
          onDocument={onDocument}
          reviseId={archiveDialog === "revise" ? selected : ""}
          close={() => setArchiveDialog("")}
        />
      )}
      {archiveDialog === "custody" && file && (
        <ArchiveCustodyDialog
          doc={doc}
          age={age}
          file={file}
          archives={archives}
          onDocument={(d) => {
            onDocument(d);
            setSelected("");
          }}
          close={() => setArchiveDialog("")}
        />
      )}
      {dialog && (
        <LibraryDialog
          key={dialog.type + (dialog.entity?.id || "")}
          dialog={dialog}
          error={error}
          all={all}
          archives={archives}
          locations={locations}
          age={age}
          busy={busy}
          onAtlas={onAtlas}
          onOrganizations={onOrganizations}
          close={() => setDialog(null)}
          save={async (values) => {
            setBusy(true);
            setError("");
            try {
              if (dialog.type === "archive") {
                const e = {
                    ...dialog.entity,
                    id: dialog.entity?.id || id(),
                    ageId: age.id,
                    kind: "archive",
                    name: values.name,
                    archive: values.archiveConfig,
                    properties: {
                      ...dialog.entity?.properties,
                      summary: values.summary,
                      parentArchive: values.parent,
                      startYear: values.startYear,
                      endYear: values.endYear,
                    },
                  },
                  old = doc.relations.filter(
                    (r) =>
                      r.ageId === age.id &&
                      r.source === e.id &&
                      r.kind === "archive-located-at",
                  );
                const result = await command({
                  kind: "batch",
                  ageId: age.id,
                  commands: [
                    { kind: "entity", entity: e },
                    ...old.map((r) => ({
                      kind: "delete-relation",
                      edgeId: r.id,
                    })),
                    ...values.locations.map((destination) => ({
                      kind: "relation",
                      relation: {
                        id: id(),
                        ageId: age.id,
                        source: e.id,
                        destination,
                        kind: "archive-located-at",
                      },
                    })),
                  ],
                });
                if (!result) {
                  setError(
                    "Unable to save archive. Check its parent and location links.",
                  );
                  return;
                }
                setFolder(e.id);
                setSelected("");
              } else if (dialog.type === "metadata") {
                const e = {
                  ...dialog.entity,
                  name: values.name,
                  kind: values.kind,
                  properties: {
                    ...dialog.entity.properties,
                    summary: values.summary,
                    startYear: values.startYear,
                    endYear: values.endYear,
                  },
                };
                const r = await fetch(url(e.id, latest.hash));
                if (!r.ok) throw Error("Cannot load current file");
                await uploadFile(
                  await r.arrayBuffer(),
                  latest.name,
                  e,
                  values.archive,
                );
                setFolder(values.archive);
              } else {
                const e = {
                    id: id(),
                    ageId: age.id,
                    name: values.name,
                    kind: values.kind,
                    properties: {
                      summary: values.summary,
                      startYear: values.startYear,
                      endYear: values.endYear,
                    },
                  },
                  bytes = await newLibraryFile(values.format);
                await uploadFile(
                  bytes,
                  values.name.replace(/[\\/:*?"<>|]/g, "_") +
                    "." +
                    values.format,
                  e,
                  folder,
                );
                setSelected(e.id);
              }
              setDialog(null);
            } catch (e) {
              setError(e.message);
            } finally {
              setBusy(false);
            }
          }}
        />
      )}
    </main>
  );
}
export function LibraryDialog({
  dialog,
  error,
  all,
  archives,
  locations,
  age,
  busy,
  onAtlas,
  onOrganizations,
  close,
  save,
}) {
  const entity = dialog.entity,
    isArchive = dialog.type === "archive";
  const [values, setValues] = useState({
    archiveConfig: entity?.archive || null,
    name: entity?.name || "",
    kind: entity?.kind || "document",
    summary: entity?.properties?.summary || "",
    startYear: entity?.properties?.startYear || "",
    endYear: entity?.properties?.endYear || "",
    locations: entity ? locations(entity).map((e) => e.id) : [],
    parent: dialog.parent || "",
    archive: dialog.archive || "",
    format: "md",
  });
  const field = (key, v) => setValues({ ...values, [key]: v });
  const places = all.filter((e) =>
    ["geography", "settlements", "politics", "organizations"].includes(
      registry.types[e.kind]?.category,
    ),
  );
  return (
    <Dialog
      title={
        <>
          {entity ? "Edit" : "Create"}{" "}
          {isArchive ? "archive folder" : "document"}
        </>
      }
      size="form"
      className="relation-dialog library-dialog"
      onClose={close}
      busy={busy}
      onSubmit={(e) => {
        e.preventDefault();
        return save(values);
      }}
      actions={
        <>
          <button
            type="button"
            disabled={busy}
            onClick={close}
            data-dialog-dismiss
          >
            Cancel
          </button>
          <button
            className="primary"
            disabled={busy || (isArchive && !values.archiveConfig?.ownerId)}
          >
            Save
          </button>
        </>
      }
    >
      {error && <p role="alert">{error}</p>}
      <label>
        Name
        <input
          type="text"
          autoFocus
          required
          maxLength="160"
          value={values.name}
          onChange={(e) => field("name", e.target.value)}
        />
      </label>
      <label>
        Summary
        <textarea
          value={values.summary}
          onChange={(e) => field("summary", e.target.value)}
        />
      </label>
      {isArchive ? (
        <>
          <ArchiveOwnerField
            all={all}
            config={values.archiveConfig}
            onChange={(next) =>
              field("archiveConfig", {
                ...next,
                coverage: values.archiveConfig?.coverage || [],
                sources: values.archiveConfig?.sources || [],
              })
            }
          />
          <label>
            Parent archive
            <select
              value={values.parent}
              onChange={(e) => field("parent", e.target.value)}
            >
              <option value="">Top level</option>
              {archives
                .filter((a) => a.id !== entity?.id)
                .map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
            </select>
          </label>
          <label>
            Linked places & organizations (optional)
            <small>
              Hold Ctrl (Command on Mac) to select more than one location; Shift
              selects a range.
            </small>
            <select
              multiple
              size="6"
              value={values.locations}
              onChange={(e) =>
                field(
                  "locations",
                  Array.from(e.target.selectedOptions, (o) => o.value),
                )
              }
            >
              {places.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name} — {label(p.kind)}
                </option>
              ))}
            </select>
          </label>
          {!places.length && (
            <p>
              Create a place or organization first, then return to link this
              archive.{" "}
              <button type="button" onClick={onAtlas}>
                Open Atlas
              </button>{" "}
              <button type="button" onClick={onOrganizations}>
                Open Organizations
              </button>
            </p>
          )}
        </>
      ) : (
        <>
          <label>
            Document type
            <select
              value={values.kind}
              onChange={(e) => field("kind", e.target.value)}
            >
              {documentTypes.map(([k, v]) => (
                <option key={k} value={k}>
                  {v.label}
                </option>
              ))}
            </select>
          </label>
          {dialog.type === "metadata" ? (
            <label>
              Archive
              <select
                value={values.archive}
                onChange={(e) => field("archive", e.target.value)}
              >
                {archives.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </label>
          ) : (
            <label>
              File format
              <select
                value={values.format}
                onChange={(e) => field("format", e.target.value)}
              >
                {["md", "txt", "docx", "xlsx", "xls", "ods", "csv", "pdf"].map(
                  (f) => (
                    <option key={f}>{f}</option>
                  ),
                )}
              </select>
            </label>
          )}
        </>
      )}
      <div className="library-toolbar">
        <label>
          Start year
          <input
            type="number"
            step="1"
            value={values.startYear}
            onChange={(e) => field("startYear", e.target.value)}
          />
        </label>
        <label>
          End year
          <input
            type="number"
            step="1"
            value={values.endYear}
            onChange={(e) => field("endYear", e.target.value)}
          />
        </label>
      </div>
      <small>
        Historical dates belong to {age.name} and appear in its year timeline.
      </small>
    </Dialog>
  );
}
