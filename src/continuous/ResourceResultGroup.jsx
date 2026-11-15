import { useMemo, useState } from "react";

// Keep large archived inventories out of the DOM until requested. Copy/export
// still use the complete report, independently of disclosure and pagination.
export default function ResourceResultGroup({ label, items, children }) {
  const [open, setOpen] = useState(false),
    [search, setSearch] = useState(""),
    [page, setPage] = useState(0);
  const matches = useMemo(() => {
    const q = search.trim().toLowerCase();
    return q
      ? items.filter((r) =>
          `${r.type} ${r.variant} ${r.availability || ""}`
            .toLowerCase()
            .includes(q),
        )
      : items;
  }, [items, search]);
  const size = 100,
    current = Math.min(page, Math.max(0, Math.ceil(matches.length / size) - 1));
  return (
    <details
      open={open}
      onToggle={(e) => {
        if (e.target === e.currentTarget) setOpen(e.currentTarget.open);
      }}
    >
      <summary>
        {label} <small>· {items.length}</small>
      </summary>
      {open && (
        <>
          {items.length > size && (
            <label>
              Find resources
              <input
                type="search"
                value={search}
                onChange={(e) => {
                  setSearch(e.target.value);
                  setPage(0);
                }}
              />
            </label>
          )}
          {matches.slice(current * size, (current + 1) * size).map(children)}
          {items.length > size && (
            <div className="resource-panel-controls">
              <span>
                {matches.length ? current * size + 1 : 0}–
                {Math.min((current + 1) * size, matches.length)} of{" "}
                {matches.length}
              </span>
              <button
                type="button"
                disabled={current === 0}
                onClick={() => setPage(current - 1)}
              >
                Previous resources
              </button>
              <button
                type="button"
                disabled={(current + 1) * size >= matches.length}
                onClick={() => setPage(current + 1)}
              >
                Next resources
              </button>
            </div>
          )}
        </>
      )}
    </details>
  );
}
