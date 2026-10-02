"use client";
import { useEffect, useState } from "react";
import { api, Command, State, newID, World } from "../lib/types";
import TextImport from "./TextImport";
import SavedRevisions from "./SavedRevisions";
type Props = {
  state: State;
  run: (c: Command) => Promise<boolean>;
  busy: boolean;
};
async function download(path: string, filename: string, body?: unknown) {
  const r = await fetch(
    "/api/v1" + path,
    body
      ? {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        }
      : {},
  );
  if (!r.ok) {
    const e = await r.json();
    throw new Error(e.error);
  }
  const blob = await r.blob(),
    url = URL.createObjectURL(blob),
    a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 10000);
}
export function ImportPanel() {
  const [remote, setRemote] = useState("");
  const [canClone, setCanClone] = useState(false);
  useEffect(() => {
    api<{ hosted?: boolean; user?: string }>("/session")
      .then((s) => setCanClone(!s.hosted || s.user === "admin"))
      .catch(() => {});
  }, []);
  const [preview, setPreview] = useState<{
      token: string;
      world: World;
      ages: number;
      records: number;
      sourceFormat: number;
      warnings: string[];
      repository?: boolean;
    } | null>(null),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  return (
    <section className="card">
      <h3>Import a native backup</h3>
      <p>
        Preview and validate the archive, including P1 migration. Acceptance
        creates a separate world; existing worlds are preserved.
      </p>
      <input
        aria-label="Native archive"
        type="file"
        accept=".zip"
        disabled={busy || !!preview}
        onChange={async (e) => {
          const file = e.target.files?.[0];
          if (!file) return;
          e.target.value = "";
          setBusy(true);
          setError("");
          setPreview(null);
          try {
            const r = await fetch("/api/v1/import-preview", {
              method: "POST",
              body: file,
            });
            const data = await r.json();
            if (!r.ok) throw new Error(data.error);
            setPreview(data);
          } catch (e) {
            setError((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      />
      {canClone && (
        <details>
          <summary>Clone an existing world from Git</summary>
          <p>
            Clone the main branch from an HTTPS repository. The preview retains
            the world's identity and Git history; this library must not already
            contain that world.
          </p>
          <form
            onSubmit={async (e) => {
              e.preventDefault();
              setBusy(true);
              setError("");
              try {
                setPreview(
                  await api("/clone-preview", {
                    method: "POST",
                    body: JSON.stringify({ remote }),
                  }),
                );
              } catch (e) {
                setError((e as Error).message);
              } finally {
                setBusy(false);
              }
            }}
          >
            <label>
              Repository HTTPS URL
              <input
                required
                type="url"
                value={remote}
                onChange={(e) => setRemote(e.target.value)}
                disabled={busy || !!preview}
              />
            </label>
            <button disabled={busy || !!preview}>
              Preview repository clone
            </button>
          </form>
        </details>
      )}
      {busy && <p role="status">Preparing and validating project…</p>}
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {preview && (
        <div className="notice">
          <strong>{preview.world.name}</strong>
          <p>
            {preview.ages} Ages · {preview.records} historical record versions ·
            source format {preview.sourceFormat}
          </p>
          <ul>
            {preview.warnings.map((s) => (
              <li key={s}>{s}</li>
            ))}
          </ul>
          <button
            disabled={busy}
            onClick={async () => {
              setBusy(true);
              try {
                const world = await api<World>("/import-accept", {
                  method: "POST",
                  body: JSON.stringify({ token: preview.token }),
                });
                location.href = `/?world=${world.id}`;
              } catch (e) {
                setError((e as Error).message);
              } finally {
                setBusy(false);
              }
            }}
          >
            {preview.repository
              ? "Add cloned world to library"
              : "Import as a new world"}
          </button>
          <button
            disabled={busy}
            onClick={async () => {
              setBusy(true);
              try {
                await api("/import-discard", {
                  method: "POST",
                  body: JSON.stringify({ token: preview.token }),
                });
                setPreview(null);
              } catch (e) {
                setError((e as Error).message);
              } finally {
                setBusy(false);
              }
            }}
          >
            Discard preview
          </button>
        </div>
      )}
    </section>
  );
}
type Finding = {
  id: string;
  rule: string;
  severity: string;
  record: string;
  message: string;
  evidence: string;
  fingerprint: string;
  excepted: boolean;
};
export function Consistency({ state, run, busy }: Props) {
  const [findings, setFindings] = useState<Finding[]>([]),
    [error, setError] = useState(""),
    [reason, setReason] = useState(""),
    [showAll, setShowAll] = useState(false);
  useEffect(() => {
    api<Finding[]>(
      `/projects/${state.root.world.id}/checks?age=${state.age.id}`,
    )
      .then(setFindings)
      .catch((e) => setError(e.message));
  }, [state.root.world.id, state.revision, state.age.id]);
  return (
    <div>
      <div className="eyebrow">Consistency / {state.age.name}</div>
      <h2>Check the declared facts.</h2>
      <p>
        These checks use recorded dates, references, names and geography. They
        do not infer facts from prose. Intentional exceptions are welcome.
      </p>
      <label>
        Exception reason
        <input
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          placeholder="For example: this is an underwater settlement"
        />
      </label>
      <label className="check-label">
        <input
          type="checkbox"
          checked={showAll}
          onChange={(e) => setShowAll(e.target.checked)}
        />
        Show acknowledged exceptions
      </label>
      {error && <p role="alert">{error}</p>}
      {findings
        .filter((f) => showAll || !f.excepted)
        .map((f) => (
          <article className="card" key={f.fingerprint}>
            <small>
              {f.severity} · {f.rule}
            </small>
            <h3>{state.records[f.record]?.name}</h3>
            <p>{f.message}</p>
            <blockquote>{f.evidence}</blockquote>
            {f.excepted ? (
              <p>Exception recorded for these exact inputs.</p>
            ) : (
              <button
                disabled={busy || !reason.trim()}
                onClick={() =>
                  void run({
                    action: "put",
                    record: {
                      id: newID(),
                      kind: "note",
                      name: "Consistency exception",
                      notes: reason,
                      properties: {
                        exceptionFingerprint: f.fingerprint,
                        rule: f.rule,
                        record: f.record,
                      },
                    },
                  })
                }
              >
                Record intentional exception
              </button>
            )}
          </article>
        ))}
      {!findings.some((f) => showAll || !f.excepted) && (
        <p>No unacknowledged findings from the implemented checks.</p>
      )}
    </div>
  );
}
type GitInfo = {
  available: boolean;
  initialized: boolean;
  changes: string;
  branch: string;
  remote: string;
};
type Merge = {
  alreadyIncluded: boolean;
  revision: string;
  remote: string;
  base: string;
  conflicts: { path: string; base: unknown; local: unknown; remote: unknown }[];
  applied: boolean;
};
export default function ProjectTools({ state, run }: Props) {
  const [selected, setSelected] = useState<string[]>([]),
    [format, setFormat] = useState("html"),
    [title, setTitle] = useState(state.root.world.name),
    [language, setLanguage] = useState("und"),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [status, setStatus] = useState(""),
    [git, setGit] = useState<GitInfo | null>(null),
    [message, setMessage] = useState("Save world revision"),
    [remote, setRemote] = useState(""),
    [merge, setMerge] = useState<Merge | null>(null),
    [choices, setChoices] = useState<Record<string, string>>({});
  const base = `/projects/${state.root.world.id}`;
  const refresh = () =>
    api<GitInfo>(base + "/repository")
      .then((v) => {
        setGit(v);
        setRemote(v.remote || "");
      })
      .catch((e) => setError(e.message));
  useEffect(() => {
    if (!state.role || state.role === "owner") void refresh();
  }, [state.root.world.id, state.role]);
  async function task(work: () => Promise<unknown>) {
    setBusy(true);
    setError("");
    setStatus("");
    try {
      await work();
      setStatus("Finished.");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  const action = (action: string) =>
    task(async () => {
      await api(base + "/repository", {
        method: "POST",
        body: JSON.stringify({ action, message, remote }),
      });
      await refresh();
    });
  const extension: Record<string, string> = {
    markdown: "md",
    html: "html",
    publication: "html",
    epub: "epub",
    docx: "docx",
    fountain: "fountain",
    maps: "json",
    svg: "zip",
  };
  return (
    <div>
      <div className="eyebrow">Project ownership</div>
      <h2>Keep it. Exchange it. Publish deliberately.</h2>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {status && <p role="status">{status}</p>}
      <section className="card">
        <h3>Lossless backup</h3>
        <p>
          A native archive includes the whole world, every Age, private records,
          media and saved history. Do not use it as a public edition.
        </p>
        <button
          disabled={busy}
          onClick={() =>
            void task(() => download(base + "/archive", "world.kriemhild.zip"))
          }
        >
          Download native backup
        </button>
      </section>
      <ImportPanel />
      <TextImport state={state} run={run} busy={busy} />
      {(!state.role || state.role === "owner") && (
        <SavedRevisions key={state.root.world.id} state={state} />
      )}
      <section className="card">
        <h3>Codex, story bible & selected export</h3>
        <p>
          Only checked records are exported. Linked dossiers, hidden properties,
          source Ages and old revisions are not pulled into a public edition.
          Review your selected text before sharing. Map images include their
          whole background; select linked entities to include their pins and
          labels. Review images for private annotations.
        </p>
        <div className="two-col">
          <label>
            Edition title
            <input value={title} onChange={(e) => setTitle(e.target.value)} />
          </label>
          <label>
            Export format
            <select value={format} onChange={(e) => setFormat(e.target.value)}>
              {[
                ["html", "HTML codex / story bible"],
                ["markdown", "Markdown text"],
                ["publication", "Offline edition with search and maps"],
                ["epub", "EPUB"],
                ["docx", "DOCX manuscript"],
                ["fountain", "Fountain text"],
                ["maps", "Native planar map JSON"],
                ["svg", "Selected maps as SVG images"],
              ].map(([v, n]) => (
                <option key={v} value={v}>
                  {n}
                </option>
              ))}
            </select>
          </label>
        </div>
        {["html","publication","epub","docx"].includes(format) && <label>Edition language tag<input aria-label="Edition language tag" value={language} maxLength={64} onChange={e=>setLanguage(e.target.value)} /><small>Use en, uk, ar, or another language tag. Leave und for unspecified; a fictional language can use a private tag such as x-river.</small></label>}
        <p className="muted">
          Native backups are lossless. Text exchanges omit planning metadata,
          comments and unsupported formatting. DOCX preserves headings, emphasis,
          lists, quotations and line breaks; Fountain exports scene text. Map JSON retains fictional planar coordinates and
          does not claim WGS84.
        </p>
        <div className="export-selection">
          {Object.values(state.records)
            .filter(
              (r) =>
                ["entity", "note", "scene", "map"].includes(r.kind) &&
                r.properties?.exceptionFingerprint === undefined,
            )
            .map((r) => (
              <label className="check-label" key={r.id}>
                <input
                  type="checkbox"
                  checked={selected.includes(r.id)}
                  onChange={(e) =>
                    setSelected(
                      e.target.checked
                        ? [...selected, r.id]
                        : selected.filter((id) => id !== r.id),
                    )
                  }
                />
                {r.name} <small>{r.kind}</small>
              </label>
            ))}
        </div>
        <button
          disabled={busy || !selected.length}
          onClick={() =>
            void task(() =>
              download(base + "/export", `kriemhild.${extension[format]}`, {
                snapshot: state.age.snapshot,
                ids: selected,
                format,
                title,
                language,
              }),
            )
          }
        >
          Export selected records
        </button>
      </section>
      <section className="card">
        <h3>Local repository & GitHub</h3>
        {state.role && state.role !== "owner" && (
          <p>Repository synchronization is managed by a world owner.</p>
        )}
        {git && !git.available ? (
          <p>
            Install Git to enable repository operations. Local projects work
            without it.
          </p>
        ) : git && !git.initialized ? (
          <button disabled={busy} onClick={() => void action("init")}>
            Initialize project repository
          </button>
        ) : (
          git && (
            <>
              <p>Branch: {git.branch}</p>
              <details>
                <summary>Changed project files</summary>
                <pre>{git.changes || "Working tree is clean."}</pre>
              </details>
              <label>
                Commit message
                <input
                  value={message}
                  onChange={(e) => setMessage(e.target.value)}
                />
              </label>
              <button disabled={busy} onClick={() => void action("commit")}>
                Commit project changes
              </button>
              <label>
                HTTPS remote URL
                <input
                  value={remote}
                  onChange={(e) => setRemote(e.target.value)}
                  placeholder="https://github.com/you/your-world.git"
                />
              </label>
              <p className="muted">
                Credentials are handled by your installed Git credential helper.
                Never enter a token in this URL.
              </p>
              <div className="map-tools">
                <button
                  disabled={busy || !remote}
                  onClick={() => void action("remote")}
                >
                  Save remote
                </button>
                <button
                  disabled={busy || !git.remote}
                  onClick={() => void action("fetch")}
                >
                  Fetch main
                </button>
                <button
                  disabled={busy || !git.remote}
                  onClick={() => void action("push")}
                >
                  Push current revision to main
                </button>
              </div>
              <button
                disabled={busy || !!git.changes}
                onClick={() =>
                  void task(async () => {
                    setChoices({});
                    setMerge(
                      await api<Merge>(base + "/merge", {
                        method: "POST",
                        body: JSON.stringify({
                          expected: state.revision,
                          choices: {},
                          apply: false,
                        }),
                      }),
                    );
                  })
                }
              >
                Review fetched changes
              </button>
              {merge && (
                <div className="notice">
                  {merge.alreadyIncluded && (
                    <p>
                      The fetched revision is already included in this world's
                      history.
                    </p>
                  )}
                  <p>
                    {merge.conflicts.length} unresolved field conflicts.
                    Non-overlapping fields are combined automatically. No active
                    files change until you apply.
                  </p>
                  {merge.conflicts.map((c) => (
                    <div key={c.path}>
                      <h4>
                        {Object.values(state.records).find((r) =>
                          c.path.includes(r.id),
                        )?.name || "Age metadata"}
                      </h4>
                      <code>{c.path}</code>
                      <div className="diff-columns">
                        {["base", "local", "remote"].map((side) => (
                          <div key={side}>
                            <strong>{side}</strong>
                            <pre>
                              {JSON.stringify(
                                c[side as "base" | "local" | "remote"],
                                null,
                                2,
                              )}
                            </pre>
                          </div>
                        ))}
                      </div>
                      <select
                        aria-label={`Resolution ${c.path}`}
                        value={choices[c.path] || ""}
                        onChange={(e) =>
                          setChoices({ ...choices, [c.path]: e.target.value })
                        }
                      >
                        <option value="">Choose a version</option>
                        <option value="local">Keep mine</option>
                        <option value="remote">Use fetched</option>
                      </select>
                    </div>
                  ))}
                  <button
                    disabled={
                      busy ||
                      merge.alreadyIncluded ||
                      merge.conflicts.some((c) => !choices[c.path])
                    }
                    onClick={() =>
                      void task(async () => {
                        const result = await api<Merge>(base + "/merge", {
                          method: "POST",
                          body: JSON.stringify({
                            expected: state.revision,
                            remote: merge.remote,
                            choices,
                            apply: true,
                          }),
                        });
                        if (result.applied) location.reload();
                        else setMerge(result);
                      })
                    }
                  >
                    Apply reviewed merge
                  </button>
                </div>
              )}
            </>
          )
        )}
      </section>
    </div>
  );
}
