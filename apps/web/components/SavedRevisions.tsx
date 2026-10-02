"use client";
import { useEffect, useState } from "react";
import { api, State } from "../lib/types";
type Revision = {
  revision: string;
  message: string;
  savedAt: string;
  ages: number;
};
type Page = { items: Revision[]; next: string };
type Review = {
  changes: {
    id: string;
    before: string;
    after: string;
    added: number;
    removed: number;
    changed: number;
  }[];
  applied: boolean;
};
export default function SavedRevisions({ state }: { state: State }) {
  const [page, setPage] = useState<Page | null>(null),
    [cursors, setCursors] = useState([""]),
    [revision, setRevision] = useState(""),
    [review, setReview] = useState<Review | null>(null),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const base = `/projects/${state.root.world.id}`,
    cursor = cursors[cursors.length - 1];
  useEffect(() => {
    let cancelled = false;
    setBusy(true);
    api<Page>(`${base}/revisions?cursor=${encodeURIComponent(cursor)}`)
      .then((p) => {
        if (!cancelled) setPage(p);
      })
      .catch((e) => {
        if (!cancelled) setError(e.message);
      })
      .finally(() => {
        if (!cancelled) setBusy(false);
      });
    return () => {
      cancelled = true;
    };
  }, [base, cursor, state.revision]);
  async function restore(apply: boolean) {
    setBusy(true);
    setError("");
    try {
      const result = await api<Review>(base + "/restore", {
        method: "POST",
        body: JSON.stringify({ expected: state.revision, revision, apply }),
      });
      if (result.applied)
        location.href = `/?world=${state.root.world.id}&view=project`;
      else setReview(result);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="card">
      <h3>Saved revisions</h3>
      <p>
        Review and restore a complete saved world. Restoration creates a new
        revision; intervening work remains in history. All Ages return to their
        selected saved contents. Shared drafts must be reconciled separately.
      </p>
      <label>
        Saved world revision
        <select
          aria-label="Saved world revision"
          value={revision}
          disabled={busy}
          onChange={(e) => {
            setRevision(e.target.value);
            setReview(null);
          }}
        >
          <option value="">Choose a saved revision</option>
          {page?.items.map((r) => (
            <option
              key={r.revision}
              value={r.revision}
              disabled={r.revision === state.revision}
            >
              {r.savedAt} · {r.message} · {r.ages} Ages
              {r.revision === state.revision ? " (current)" : ""}
            </option>
          ))}
        </select>
      </label>
      <button
        disabled={busy || cursors.length === 1}
        onClick={() => {
          setCursors((c) => c.slice(0, -1));
          setRevision("");
          setReview(null);
        }}
      >
        Newer saves
      </button>
      <button
        disabled={busy || !page?.next}
        onClick={() => {
          if (page?.next) setCursors((c) => [...c, page.next]);
          setRevision("");
          setReview(null);
        }}
      >
        Older saves
      </button>
      <button disabled={busy || !revision} onClick={() => void restore(false)}>
        Review restoration
      </button>
      {review && (
        <div className="notice">
          <table>
            <thead>
              <tr>
                <th>Age now</th>
                <th>Age after restoration</th>
                <th>Records added</th>
                <th>Removed</th>
                <th>Changed</th>
              </tr>
            </thead>
            <tbody>
              {review.changes.map((c) => (
                <tr key={c.id}>
                  <td>{c.before || "Not present"}</td>
                  <td>{c.after || "Removed from active world"}</td>
                  <td>{c.added}</td>
                  <td>{c.removed}</td>
                  <td>{c.changed}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {!review.changes.length && (
            <p>The saved Age contents already match.</p>
          )}
          <button disabled={busy} onClick={() => void restore(true)}>
            Restore reviewed world as a new revision
          </button>
          <button disabled={busy} onClick={() => setReview(null)}>
            Cancel restoration
          </button>
        </div>
      )}
      {error && <p role="alert">{error}</p>}
    </section>
  );
}
