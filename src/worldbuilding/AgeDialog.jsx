import { useMemo, useState } from "react";
import Dialog from "./ui/Dialog.jsx";
import { mergeConflicts } from "../history.js";

export default function AgeDialog({
  doc,
  sourceAge,
  sourceYear,
  onClose,
  onSubmit,
  busy,
}) {
  const [name, setName] = useState(""),
    [mode, setMode] = useState(sourceYear ? "branch" : "after"),
    [other, setOther] = useState(
      doc.ages.find((a) => a.id !== sourceAge)?.id || "",
    ),
    [policy, setPolicy] = useState(""),
    [excluded, setExcluded] = useState(() => new Set()),
    [entityNames, setEntityNames] = useState({}),
    [reviewOpen, setReviewOpen] = useState(false),
    [query, setQuery] = useState(""),
    [page, setPage] = useState(0),
    [conflictPage, setConflictPage] = useState(0);
  const inherited = useMemo(() => {
    const records = new Map();
    for (const e of doc.entities)
      if (e.ageId === sourceAge) records.set(e.id, e);
    if (mode === "merge")
      for (const e of doc.entities)
        if (e.ageId === other && (!records.has(e.id) || policy === "secondary"))
          records.set(e.id, e);
    return [...records.values()].filter((e) => !e.deleted);
  }, [doc.entities, sourceAge, mode, other, policy]);
  const inheritedIDs = useMemo(
    () => new Set(inherited.map((e) => e.id)),
    [inherited],
  );
  const primaryIDs = useMemo(
    () =>
      new Set(
        doc.entities
          .filter((e) => e.ageId === sourceAge && !e.deleted)
          .map((e) => e.id),
      ),
    [doc.entities, sourceAge],
  );
  const conflicts = useMemo(
    () => (mode === "merge" ? mergeConflicts(doc, sourceAge, other) : []),
    [doc.entities, doc.maps, doc.representations, mode, sourceAge, other],
  );
  const filtered = useMemo(() => {
    const search = query.trim().toLowerCase();
    return search
      ? inherited.filter((e) =>
          `${e.name} ${e.kind}`.toLowerCase().includes(search),
        )
      : inherited;
  }, [inherited, query]);
  const pageSize = 100,
    lastPage = Math.max(0, Math.ceil(filtered.length / pageSize) - 1),
    currentPage = Math.min(page, lastPage);
  const includedCount = useMemo(
    () => inherited.reduce((n, e) => n + Number(!excluded.has(e.id)), 0),
    [inherited, excluded],
  );
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
          excludeIds: [...excluded].filter((id) => inheritedIDs.has(id)),
          entityNames: Object.fromEntries(
            Object.entries(entityNames).filter(
              ([id]) => !excluded.has(id) && inheritedIDs.has(id),
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
          saved state. Earlier events are not used to rewind terrain or records.
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
          <summary>{conflicts.length} conflicting map or record states</summary>
          <ul>
            {conflicts
              .slice(conflictPage * pageSize, (conflictPage + 1) * pageSize)
              .map((c) => (
                <li key={c.kind + c.id}>
                  {c.name} - {c.kind}
                </li>
              ))}
          </ul>
          {conflicts.length > pageSize && (
            <div>
              <button
                type="button"
                disabled={conflictPage === 0}
                onClick={() => setConflictPage((p) => p - 1)}
              >
                Previous conflicts
              </button>
              <button
                type="button"
                disabled={(conflictPage + 1) * pageSize >= conflicts.length}
                onClick={() => setConflictPage((p) => p + 1)}
              >
                Next conflicts
              </button>
            </div>
          )}
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
              Keep current Age's conflicting maps and records
            </option>
            <option value="secondary">
              Keep the other Age's conflicting maps and records
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
            : "New Ages inherit the saved terrains, maps and records of their source Age."}
      </p>
      {mode !== "connect" && (
        <details onToggle={(event) => setReviewOpen(event.currentTarget.open)}>
          <summary>
            Review inherited records ({includedCount} of {inherited.length}{" "}
            included)
          </summary>
          {reviewOpen && (
            <>
              <p>
                Maps and terrain are inherited. Uncheck records to remove them
                from the new Age; the source is preserved. Open their new
                profiles after creation to make changes.
              </p>
              <label>
                Find inherited records
                <input
                  type="search"
                  value={query}
                  onChange={(event) => {
                    setQuery(event.target.value);
                    setPage(0);
                  }}
                />
              </label>
              <button type="button" onClick={() => setExcluded(new Set())}>
                Include all
              </button>
              <button
                type="button"
                onClick={() => setExcluded(new Set(inheritedIDs))}
              >
                Exclude all
              </button>
              <div className="inheritance-review">
                {filtered
                  .slice(currentPage * pageSize, (currentPage + 1) * pageSize)
                  .map((e) => (
                    <label key={e.id}>
                      <input
                        type="checkbox"
                        checked={!excluded.has(e.id)}
                        onChange={(ev) =>
                          setExcluded((ids) => {
                            const next = new Set(ids);
                            if (ev.target.checked) next.delete(e.id);
                            else next.add(e.id);
                            return next;
                          })
                        }
                      />
                      {e.name}{" "}
                      <small>
                        {e.kind} /{" "}
                        {excluded.has(e.id)
                          ? "removed"
                          : entityNames[e.id] && entityNames[e.id] !== e.name
                            ? "modified"
                            : primaryIDs.has(e.id)
                              ? "carried forward"
                              : "added from merge"}
                      </small>
                      <input
                        type="text"
                        aria-label={`Name ${e.name} in new Age`}
                        disabled={excluded.has(e.id)}
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
              <p role="status">
                {filtered.length ? currentPage * pageSize + 1 : 0}–
                {Math.min((currentPage + 1) * pageSize, filtered.length)} of{" "}
                {filtered.length} matching records
              </p>
              <button
                type="button"
                disabled={currentPage === 0}
                onClick={() => setPage(currentPage - 1)}
              >
                Previous records
              </button>
              <button
                type="button"
                disabled={currentPage === lastPage}
                onClick={() => setPage(currentPage + 1)}
              >
                Next records
              </button>
            </>
          )}
        </details>
      )}
    </Dialog>
  );
}
