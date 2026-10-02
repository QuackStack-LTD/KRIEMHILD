"use client";
import { useEffect, useState } from "react";
import { api, Entry, State } from "../lib/types";

type Page = { records: Entry[]; next: string; snapshot: string };
type Hit = {
  age: string;
  ageName: string;
  sourceAge?: string;
  snapshot: string;
  record: Entry;
};
type AcrossPage = {
  hits: Hit[];
  next: string;
  revision: string;
  agesScanned: number;
};

export default function SearchResults({
  state,
  query,
  open,
}: {
  state: State;
  query: string;
  open: (entry: Entry, age?: string) => void | Promise<void>;
}) {
  const [cursors, setCursors] = useState([""]);
  const [page, setPage] = useState<Page | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [across, setAcross] = useState(false);
  const [history, setHistory] = useState<AcrossPage | null>(null);
  const [opening, setOpening] = useState(false);
  const cursor = cursors[cursors.length - 1];
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    const timer = setTimeout(() => {
      api<Page | AcrossPage>(
        `/projects/${state.root.world.id}/${across ? "search-ages" : "search-page"}?age=${state.age.id}&q=${encodeURIComponent(query)}&cursor=${encodeURIComponent(cursor)}`,
      )
        .then((result) => {
          if (!cancelled) {
            if (across) setHistory(result as AcrossPage);
            else setPage(result as Page);
          }
        })
        .catch((e) => {
          if (!cancelled) setError(e.message);
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
    }, 200);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [state.root.world.id, state.age.id, query, cursor, across]);
  const next = across ? history?.next : page?.next;
  const groups = new Map<string, Hit[]>();
  for (const hit of history?.hits || [])
    groups.set(hit.record.id, [...(groups.get(hit.record.id) || []), hit]);
  return (
    <section className="search-results" aria-busy={loading}>
      <h2>{across ? "Search across Ages" : `Search in ${state.age.name}`}</h2>
      <label className="check-label">
        <input
          type="checkbox"
          checked={across}
          disabled={opening}
          onChange={(e) => {
            setAcross(e.target.checked);
            setCursors([""]);
            setPage(null);
            setHistory(null);
          }}
        />
        Search all Ages
      </label>
      <p role="status">
        {loading
          ? "Searching…"
          : error
            ? "Search unavailable."
            : `${across ? (history?.hits.length ?? 0) : (page?.records.length ?? 0)} results on page ${cursors.length}`}
      </p>
      {error && <p role="alert">{error}</p>}
      {!loading &&
        !error &&
        !across &&
        page?.records.map((r) => (
          <article className="card" key={r.id}>
            <span className="eyebrow">
              {r.kind} {r.type && `/ ${r.type}`}
            </span>
            <h3>{r.name}</h3>
            <p>{r.notes?.slice(0, 250)}</p>
            <button disabled={opening} onClick={() => void open(r)}>
              Open {r.kind === "relation" ? "entities" : r.kind}
            </button>
          </article>
        ))}
      {!loading && !error && across && (
        <>
          <p>
            Matches on this page are grouped by identity. Ages are searched in
            name order; each page checks at most three Ages. Historical dates
            within an Age are not searched here.
          </p>
          {Array.from(groups).map(([id, hits]) => (
            <article className="card" key={id}>
              <h3>{hits[0].record.name}</h3>
              {hits.map((hit) => (
                <div key={hit.age}>
                  <p>
                    <strong>{hit.ageName}</strong>
                    {hit.sourceAge &&
                      ` · derived from ${state.root.ages[hit.sourceAge]?.name || "an earlier Age"}`}
                  </p>
                  <p>
                    {hit.record.name} · {hit.record.type || hit.record.kind}
                  </p>
                  <button
                    disabled={opening}
                    onClick={async () => {
                      setOpening(true);
                      try {
                        await open(hit.record, hit.age);
                      } finally {
                        setOpening(false);
                      }
                    }}
                  >
                    Open {hit.record.name} in {hit.ageName}
                  </button>
                </div>
              ))}
            </article>
          ))}
          {!history?.hits.length && next && (
            <p>
              No matches in this part of the search. Continue to check the
              remaining Ages.
            </p>
          )}
        </>
      )}
      <nav aria-label="Search pages" className="toolbar">
        <button
          disabled={loading || opening || cursors.length === 1}
          onClick={() => setCursors((c) => c.slice(0, -1))}
        >
          Previous results
        </button>
        <button
          disabled={loading || opening || !!error || !next}
          onClick={() => {
            if (next) setCursors((c) => [...c, next]);
          }}
        >
          Next results
        </button>
      </nav>
    </section>
  );
}
