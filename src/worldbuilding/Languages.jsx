import { useEffect, useRef, useState } from "react";
import Dialog from "./ui/Dialog.jsx";
import useUnsavedChanges from "./ui/useUnsavedChanges.js";
import { confirmAction, requestText } from "./ui/feedback.js";
import { historyRequest } from "../history.js";
import {
  ENGINE,
  ADAPTER,
  sections,
  blankDefinition,
  blankScript,
  emptyPhonology,
  emptyMorphology,
  emptySyntax,
  metadataFor,
  stateMetadata,
  definitionStats,
  languageRequest,
  diffDefinitions,
} from "./languages.js";
import {
  StructuredEditor,
  phonologySchema,
  lexiconSchema,
  lexicalSchema,
  morphologySchema,
  syntaxSchema,
  graphemesSchema,
  mappingsSchema,
  changesSchema,
} from "./LanguageEditors.jsx";
import "./languages.css";
import LanguageHistory from "./LanguageHistory.jsx";

function Diagnostics({ report }) {
  if (!report) return null;
  return (
    <section className="language-diagnostics" aria-label="Compile diagnostics">
      <h3>Operation readiness</h3>
      <div className="language-readiness">
        {[
          "generate",
          "validate",
          "inflect",
          "analyze",
          "analyze-written",
          "realize",
          "paradigm",
          "parse",
          "write",
          "regression",
        ].map((op) => (
          <span key={op} data-ready={!!report.ready?.[op]}>
            {op}: {report.ready?.[op] ? "ready" : "needs dependencies"}
          </span>
        ))}
      </div>
      <p>
        {report.forms} compiled forms · {report.states} relation states ·{" "}
        {report.engine} · {report.adapter}
      </p>
      <ul>
        {report.diagnostics?.map((d, i) => (
          <li key={i}>
            <strong>
              {d.severity}: {d.code}
            </strong>{" "}
            · {d.path}
            {d.ruleId ? ` · ${d.ruleId}` : ""}
            <p>{d.message}</p>
          </li>
        ))}
      </ul>
      {report.diagnosticsOmitted > 0 && (
        <p>{report.diagnosticsOmitted} additional diagnostics omitted.</p>
      )}
    </section>
  );
}
function CreateLanguage({ onClose, onCreate, records }) {
  const [step, setStep] = useState(0),
    [kind, setKind] = useState("language"),
    [name, setName] = useState(""),
    [status, setStatus] = useState(""),
    [parent, setParent] = useState(""),
    [modules, setModules] = useState({}),
    [error, setError] = useState("");
  const titles = [
    "Identity",
    "Lineage",
    "Modules",
    "Script and policy",
    "Review",
  ];
  async function finish() {
    setError("");
    try {
      const d =
        kind === "script"
          ? blankScript(name.trim())
          : blankDefinition(name.trim());
      if (kind === "language") {
        if (modules.phonology) d.phonology = emptyPhonology();
        if (modules.morphology) d.morphology = emptyMorphology();
        if (modules.syntax) d.syntax = emptySyntax();
      }
      await onCreate(kind, d, {
        kind: "",
        status,
        description: "",
        links: parent ? [{ kind: "parent", id: parent }] : [],
      });
    } catch (e) {
      setError(e.message);
      throw e;
    }
  }
  return (
    <Dialog
      title={`Create ${kind} · ${titles[step]}`}
      onClose={onClose}
      onSubmit={step === 4 ? finish : () => setStep(step + 1)}
      actions={
        <>
          <button type="button" onClick={onClose}>
            Cancel
          </button>
          {step > 0 && (
            <button type="button" onClick={() => setStep(step - 1)}>
              Back
            </button>
          )}
          <button type="submit" disabled={!name.trim()}>
            {step === 4 ? "Save draft" : "Continue"}
          </button>
        </>
      }
    >
      <ol className="language-steps">
        {titles.map((t, i) => (
          <li key={t} aria-current={i === step ? "step" : undefined}>
            {t}
          </li>
        ))}
      </ol>
      {step === 0 ? (
        <>
          <label>
            Record type
            <select
              value={kind}
              onChange={(e) => {
                setKind(e.target.value);
                setParent("");
              }}
            >
              <option value="language">Language</option>
              <option value="script">Independent script</option>
            </select>
          </label>
          <label>
            Name
            <input
              required
              maxLength={160}
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <label>
            Author-defined status
            <input
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              placeholder="Optional; leave undecided"
            />
          </label>
        </>
      ) : step === 1 ? (
        <>
          <p>
            A descendant has its own stable identity. A later Age of the same
            language inherits its existing identity through the Age bar.
          </p>
          {kind === "language" && (
            <label>
              Parent language
              <select
                value={parent}
                onChange={(e) => setParent(e.target.value)}
              >
                <option value="">Unrelated or undecided</option>
                {records
                  .filter((e) => e.kind === "language")
                  .map((e) => (
                    <option key={e.id} value={e.id}>
                      {e.name}
                    </option>
                  ))}
              </select>
            </label>
          )}
        </>
      ) : step === 2 ? (
        <>
          <p>
            Enable only the modules you intend to author. Empty modules remain
            incomplete; no example language is inserted.
          </p>
          {kind === "language" &&
            ["phonology", "morphology", "syntax"].map((m) => (
              <label key={m}>
                <input
                  type="checkbox"
                  checked={!!modules[m]}
                  onChange={(e) =>
                    setModules({ ...modules, [m]: e.target.checked })
                  }
                />
                {m}
              </label>
            ))}
        </>
      ) : step === 3 ? (
        <>
          <p>
            A script is optional and may be shared by languages. Pin an
            independently saved script in the Scripts section when ready.
          </p>
          <p>
            Every laboratory run records its selection policy. Seeded selection
            requires your seed and chooses only among fully enumerated valid
            candidates.
          </p>
        </>
      ) : (
        <>
          <h3>{name}</h3>
          <p>
            {kind} · {status || "Status undecided"}
          </p>
          <p>
            {Object.keys(modules)
              .filter((k) => modules[k])
              .join(", ") || "No modules enabled"}
          </p>
          <p>
            This saves an incomplete draft. It does not compile or activate it.
          </p>
        </>
      )}
      {error && <p role="alert">{error}</p>}
    </Dialog>
  );
}

function Laboratory({
  definition,
  report,
  revision,
  run,
  busy,
  result,
  onSaveTest,
  canEdit,
  onTestsChange,
}) {
  const [op, setOp] = useState("inflect"),
    [lexeme, setLexeme] = useState(""),
    [template, setTemplate] = useState(""),
    [segments, setSegments] = useState(""),
    [text, setText] = useState(""),
    [features, setFeatures] = useState({}),
    [policy, setPolicy] = useState("all"),
    [seed, setSeed] = useState(""),
    [limit, setLimit] = useState(32),
    [construction, setConstruction] = useState(""),
    [roles, setRoles] = useState({});
  const grammatical = definition.morphology?.features || [],
    c = definition.syntax?.constructions?.find((c) => c.id === construction);
  function execute() {
    const q = {
      revision,
      engine: ENGINE,
      adapter: ADAPTER,
      operation: op,
      policy: ["realize", "regression"].includes(op) ? "explicit" : policy,
      limit,
    };
    if (policy === "seeded" && !["realize", "regression"].includes(op))
      q.seed = seed;
    if (["inflect", "paradigm"].includes(op)) {
      q.lexeme = lexeme;
      q.features = features;
    }
    if (op === "generate") q.template = template;
    if (["validate", "analyze", "write"].includes(op))
      q.segments = segments.split(/\s+/).filter(Boolean);
    if (["analyze-written", "parse"].includes(op)) q.text = text;
    if (op === "realize") {
      q.construction = construction;
      q.roles = Object.fromEntries(
        (c?.roles || []).map((r) => [
          r.id,
          { lexeme: roles[r.id] || "", features: {} },
        ]),
      );
    }
    run(q);
  }
  return (
    <section className="language-laboratory">
      <h2>Laboratory</h2>
      <p>
        Results are candidates, never automatic additions to the lexicon.
        Execution uses the selected saved revision and its pinned dependencies.
      </p>
      <div className="language-grid">
        <label>
          Operation
          <select value={op} onChange={(e) => setOp(e.target.value)}>
            {[
              "generate",
              "validate",
              "inflect",
              "paradigm",
              "analyze",
              "analyze-written",
              "realize",
              "write",
              "parse",
              "regression",
            ].map((v) => (
              <option key={v} value={v}>
                {v}
                {report?.ready?.[v] ? "" : " — needs dependencies"}
              </option>
            ))}
          </select>
        </label>
        {["inflect", "paradigm"].includes(op) && (
          <label>
            Lexeme
            <select value={lexeme} onChange={(e) => setLexeme(e.target.value)}>
              <option value="">Choose…</option>
              {(definition.lexicon || []).map((l) => (
                <option key={l.id} value={l.id}>
                  {l.id} · {l.senses?.[0]?.meaning || "No sense"}
                </option>
              ))}
            </select>
          </label>
        )}
        {op === "generate" && (
          <label>
            Template
            <select
              value={template}
              onChange={(e) => setTemplate(e.target.value)}
            >
              <option value="">Choose…</option>
              {definition.phonology?.templates?.map((t) => (
                <option key={t.id}>{t.id}</option>
              ))}
            </select>
          </label>
        )}
        {["validate", "analyze", "write"].includes(op) && (
          <label>
            Segment IDs
            <input
              value={segments}
              onChange={(e) => setSegments(e.target.value)}
              placeholder="p a t"
            />
          </label>
        )}
        {["analyze-written", "parse"].includes(op) && (
          <label>
            Written form
            <input
              dir="auto"
              value={text}
              onChange={(e) => setText(e.target.value)}
            />
          </label>
        )}
        {op === "inflect" &&
          grammatical.map((f) => (
            <label key={f.id}>
              {f.id}
              <select
                value={features[f.id] || ""}
                onChange={(e) => {
                  const next = { ...features };
                  if (e.target.value) next[f.id] = e.target.value;
                  else delete next[f.id];
                  setFeatures(next);
                }}
              >
                <option value="">Unspecified</option>
                {f.values.map((v) => (
                  <option key={v}>{v}</option>
                ))}
              </select>
            </label>
          ))}
        {op === "realize" && (
          <>
            <label>
              Construction
              <select
                value={construction}
                onChange={(e) => {
                  setConstruction(e.target.value);
                  setRoles({});
                }}
              >
                <option value="">Choose…</option>
                {definition.syntax?.constructions?.map((c) => (
                  <option key={c.id}>{c.id}</option>
                ))}
              </select>
            </label>
            {c?.roles?.map((r) => (
              <label key={r.id}>
                Role: {r.id}
                <select
                  value={roles[r.id] || ""}
                  onChange={(e) =>
                    setRoles({ ...roles, [r.id]: e.target.value })
                  }
                >
                  <option value="">Choose…</option>
                  {(definition.lexicon || [])
                    .filter((l) => l.category === r.category)
                    .map((l) => (
                      <option key={l.id}>{l.id}</option>
                    ))}
                </select>
              </label>
            ))}
          </>
        )}
        {!["realize", "regression"].includes(op) && (
          <>
            <label>
              Selection policy
              <select
                value={policy}
                onChange={(e) => setPolicy(e.target.value)}
              >
                <option value="all">All valid alternatives</option>
                <option value="explicit">
                  Require one explicit alternative
                </option>
                <option value="seeded">Deterministic seed</option>
              </select>
            </label>
            {policy === "seeded" && (
              <label>
                Seed
                <input value={seed} onChange={(e) => setSeed(e.target.value)} />
              </label>
            )}
          </>
        )}
        <label>
          Candidate limit
          <input
            type="number"
            min="1"
            max="256"
            value={limit}
            onChange={(e) => setLimit(Number(e.target.value))}
          />
        </label>
      </div>
      <button disabled={busy || !report?.ready?.[op]} onClick={execute}>
        Run operation
      </button>
      <section>
        <h3>Regression corpus · {(definition.tests || []).length} cases</h3>
        <p>
          Capture a laboratory result to add a case. To change its input, run
          the revised request and capture a new case.
        </p>
        {(definition.tests || []).map((t, i) => (
          <details key={i}>
            <summary>{t.id}</summary>
            <label>
              Case ID
              <input
                disabled={!canEdit}
                value={t.id}
                onChange={(e) =>
                  onTestsChange(
                    definition.tests.map((v, j) =>
                      i === j ? { ...v, id: e.target.value } : v,
                    ),
                  )
                }
              />
            </label>
            <label>
              Expected status
              <input
                disabled={!canEdit}
                value={t.expectedStatus}
                onChange={(e) =>
                  onTestsChange(
                    definition.tests.map((v, j) =>
                      i === j ? { ...v, expectedStatus: e.target.value } : v,
                    ),
                  )
                }
              />
            </label>
            <label>
              Case notes
              <textarea
                disabled={!canEdit}
                value={t.notes}
                onChange={(e) =>
                  onTestsChange(
                    definition.tests.map((v, j) =>
                      i === j ? { ...v, notes: e.target.value } : v,
                    ),
                  )
                }
              />
            </label>
            <pre>
              {JSON.stringify(
                {
                  request: t.request,
                  expected: t.expected,
                  expectedSentences: t.expectedSentences,
                  expectedDiagnostics: t.expectedDiagnostics,
                },
                null,
                2,
              )}
            </pre>
            <button
              disabled={!canEdit}
              onClick={() =>
                onTestsChange(definition.tests.filter((_, j) => j !== i))
              }
            >
              Remove regression case
            </button>
          </details>
        ))}
      </section>
      {result && (
        <div className="language-results" role="status">
          <h3>{result.status}</h3>
          {canEdit && result.input?.operation !== "regression" && (
            <button onClick={onSaveTest}>Save result as regression case</button>
          )}
          <p>
            {result.knownValid} known valid · {result.examined} examined ·{" "}
            {result.rejected} rejected
            {result.truncated ? " · Display truncated" : ""}
          </p>
          {result.diagnostics?.map((d, i) => (
            <p key={i}>
              {d.code}: {d.message}
            </p>
          ))}
          {result.candidates?.map((v, i) => (
            <article key={i}>
              <strong dir="auto">{v.written || v.segments.join(" ")}</strong>
              <p>
                {v.lexeme || "Generated candidate"}
                {v.cell ? ` · ${v.cell}` : ""}
              </p>
              <details>
                <summary>Derivation trace</summary>
                <ol>
                  {v.trace.map((s, j) => (
                    <li key={j}>
                      {s.stage} · {s.ruleId}: {s.before.join(" ")} →{" "}
                      {s.after.join(" ")}
                    </li>
                  ))}
                </ol>
              </details>
            </article>
          ))}
          {result.sentences?.map((s, i) => (
            <article key={i}>
              <strong dir="auto">{s.written}</strong>
              <p>Construction: {s.construction}</p>
              {s.gloss?.length > 0 && (
                <table>
                  <caption>Authored interlinear annotations</caption>
                  <thead>
                    <tr>
                      <th>Form</th>
                      <th>Senses</th>
                      <th>Features</th>
                    </tr>
                  </thead>
                  <tbody>
                    {s.gloss.map((w, i) => (
                      <tr key={i}>
                        <td dir="auto">{w.surface}</td>
                        <td>{w.meanings.join("; ")}</td>
                        <td>
                          {Object.entries(w.features || {})
                            .map(([k, v]) => `${k}=${v}`)
                            .join(", ")}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
              <ul>
                {s.trace.map((t, j) => (
                  <li key={j}>
                    {t.stage} · {t.ruleId} · {t.after.join(", ")}
                  </li>
                ))}
              </ul>
            </article>
          ))}
          {result.tests?.map((t) => (
            <article key={t.id}>
              <strong>
                {t.id}: {t.passed ? "passed" : "failed"}
              </strong>
              <p>
                Expected {t.expectedStatus} · received {t.result.status}
              </p>
            </article>
          ))}
          <details>
            <summary>Exact request and provenance</summary>
            <pre>
              {JSON.stringify(
                {
                  input: result.input,
                  revision: result.revision,
                  engine: result.engine,
                  adapter: result.adapter,
                  kernel: result.kernel,
                  dependencies: result.dependencies,
                },
                null,
                2,
              )}
            </pre>
          </details>
        </div>
      )}
    </section>
  );
}

export default function Languages({ doc, age, onDocument, query, onNavigate }) {
  const records = doc.entities.filter(
      (e) =>
        e.ageId === age.id &&
        !e.deleted &&
        ["language", "script"].includes(e.kind),
    ),
    selected = query.get("language") || "",
    record = records.find((e) => e.id === selected),
    state = record?.language || record?.script;
  const [section, setSection] = useState("Overview"),
    [role, setRole] = useState(""),
    [definition, setDefinition] = useState(null),
    [saved, setSaved] = useState(""),
    [metadata, setMetadata] = useState({}),
    [savedMetadata, setSavedMetadata] = useState(""),
    [version, setVersion] = useState(""),
    [report, setReport] = useState(null),
    [result, setResult] = useState(null),
    [busy, setBusy] = useState(""),
    [error, setError] = useState(""),
    [message, setMessage] = useState(""),
    [search, setSearch] = useState(""),
    [create, setCreate] = useState(false),
    [importing, setImporting] = useState(false),
    [importText, setImportText] = useState(""),
    [importKind, setImportKind] = useState("language"),
    [importError, setImportError] = useState(""),
    [typeFilter, setTypeFilter] = useState(""),
    [note, setNote] = useState(""),
    [comparing, setComparing] = useState(false),
    [comparison, setComparison] = useState([]),
    [compareHash, setCompareHash] = useState(""),
    [conflict, setConflict] = useState(false);
  const sequence = useRef(0),
    controller = useRef(null),
    baseHead = useRef("");
  const canEdit = ["owner", "administrator", "editor"].includes(role);
  useEffect(() => {
    let current = true;
    historyRequest(`/${doc.world.id}/members`)
      .then((data) => {
        if (current) setRole(data.role);
      })
      .catch((e) => {
        if (current) setError(e.message);
      });
    const change = (e) => {
      if (e.detail.worldId === doc.world.id) setRole(e.detail.role);
    };
    window.addEventListener("world-role", change);
    return () => {
      current = false;
      window.removeEventListener("world-role", change);
    };
  }, [doc.world.id]);
  const dirty =
      !!definition &&
      (JSON.stringify(definition) !== saved ||
        JSON.stringify(metadata) !== savedMetadata),
    confirmLeave = useUnsavedChanges(dirty, () => {
      setDefinition(saved ? JSON.parse(saved) : null);
      setMetadata(savedMetadata ? JSON.parse(savedMetadata) : {});
    });
  function cancelWork() {
    sequence.current++;
    controller.current?.abort();
    controller.current = null;
    setBusy("");
  }
  useEffect(() => {
    return () => {
      sequence.current++;
      controller.current?.abort();
    };
  }, []);
  useEffect(() => {
    cancelWork();
    setDefinition(null);
    setSaved("");
    setReport(null);
    setResult(null);
    setError("");
    setMessage("");
    setVersion("");
    setSection("Overview");
    if (!record) return;
    void loadVersion(
      state?.draftRevision || "",
      metadataFor(record),
      state?.draftRevision || "",
    );
  }, [selected, age.id]);
  async function loadVersion(
    hash,
    meta = metadataFor(record),
    head = state?.draftRevision || "",
  ) {
    cancelWork();
    const generation = sequence.current,
      abort = new AbortController();
    controller.current = abort;
    setBusy("Loading revision");
    try {
      const d = await languageRequest(
        doc.world.id,
        `/${selected}?age=${age.id}&revision=${hash}`,
        undefined,
        abort.signal,
      );
      if (generation !== sequence.current) return;
      setDefinition(d);
      setSaved(JSON.stringify(d));
      setMetadata(meta);
      setSavedMetadata(JSON.stringify(meta));
      setVersion(hash);
      baseHead.current = head;
      setReport(
        state?.compiled?.find(
          (c) =>
            c.revision === hash && c.engine === ENGINE && c.adapter === ADAPTER,
        )?.report || null,
      );
      setResult(null);
      setConflict(false);
    } catch (e) {
      if (e.name !== "AbortError" && generation === sequence.current)
        setError(e.message);
    } finally {
      if (generation === sequence.current) setBusy("");
    }
  }
  async function action(label, fn) {
    cancelWork();
    setError("");
    setMessage("");
    setBusy(label);
    const generation = sequence.current,
      abort = new AbortController();
    controller.current = abort;
    try {
      await fn(abort.signal, generation);
    } catch (e) {
      if (e.name !== "AbortError" && generation === sequence.current) {
        setError(e.message);
        if (e.status === 409) setConflict(true);
      }
    } finally {
      if (generation === sequence.current) setBusy("");
    }
  }
  async function select(id) {
    if (await confirmLeave()) onNavigate(id);
  }
  async function createRecord(kind, d, meta) {
    const next = await languageRequest(doc.world.id, "", {
      ageId: age.id,
      revision: doc.world.revision,
      kind,
      definition: d,
      metadata: stateMetadata(meta),
      displayName: meta.name || d.name,
      note: "Created draft",
    });
    onDocument(next);
    setCreate(false);
    onNavigate(d.languageId || d.scriptId);
  }
  async function save() {
    await action("Saving draft", async (signal, generation) => {
      const next = await languageRequest(
        doc.world.id,
        "",
        {
          ageId: age.id,
          revision: doc.world.revision,
          kind: record.kind,
          definition,
          metadata: stateMetadata(metadata),
          displayName: metadata.name,
          note,
          expectedDraft: baseHead.current,
        },
        signal,
      );
      if (generation !== sequence.current) return;
      onDocument(next);
      const e = next.entities.find(
          (e) => e.ageId === age.id && e.id === selected,
        ),
        s = e.language || e.script;
      setSaved(JSON.stringify(definition));
      setSavedMetadata(JSON.stringify(metadata));
      setVersion(s.draftRevision);
      baseHead.current = s.draftRevision;
      setReport(
        s.compiled?.find(
          (c) =>
            c.revision === s.draftRevision &&
            c.engine === ENGINE &&
            c.adapter === ADAPTER,
        )?.report || null,
      );
      setNote("");
      setResult(null);
      setMessage("Draft saved. Compilation and activation are separate.");
      setConflict(false);
    });
  }
  async function compute(op, request) {
    await action(
      op === "run"
        ? "Running operation"
        : op === "compile"
          ? "Compiling revision"
          : "Validating draft",
      async (signal, generation) => {
        const body = {
          ageId: age.id,
          revision: doc.world.revision,
          definitionRevision: version,
          request: request || {},
        };
        if (op === "validate") body.definition = definition;
        const next = await languageRequest(
          doc.world.id,
          `/${selected}/${op}`,
          body,
          signal,
        );
        if (generation !== sequence.current) return;
        setReport(next.report);
        if (next.document) onDocument(next.document);
        if (next.result) setResult(next.result);
        setMessage(
          op === "compile"
            ? "Compile report saved for this revision."
            : op === "validate"
              ? "Draft validated without saving or compiling it."
              : "Operation finished; author data is unchanged.",
        );
      },
    );
  }
  async function activate() {
    if (
      !(await confirmAction(
        `Use this compiled revision of ${record.name} as the active definition in ${age.name}?`,
        { accept: "Activate revision" },
      ))
    )
      return;
    await action("Activating revision", async (signal, generation) => {
      const next = await languageRequest(
        doc.world.id,
        `/${selected}/activate`,
        {
          ageId: age.id,
          revision: doc.world.revision,
          definitionRevision: version,
        },
        signal,
      );
      if (generation === sequence.current) {
        onDocument(next);
        setMessage("Active definition updated for this Age.");
      }
    });
  }
  async function restoreDraft() {
    if (
      !canEdit ||
      !(await confirmAction(
        `Restore this saved definition of ${record.name} as the draft in ${age.name}? All retained revisions and the active definition are preserved.`,
        { accept: "Restore draft" },
      ))
    )
      return;
    await action("Restoring draft", async (signal, generation) => {
      const next = await languageRequest(
        doc.world.id,
        `/${selected}/restore`,
        {
          ageId: age.id,
          revision: doc.world.revision,
          definitionRevision: version,
          expectedDraft: state.draftRevision,
        },
        signal,
      );
      if (generation !== sequence.current) return;
      onDocument(next);
      baseHead.current = version;
      setMessage(
        "Saved definition restored as the draft; other revisions and activation are preserved.",
      );
    });
  }
  async function compare() {
    await action("Comparing revisions", async (signal, generation) => {
      const old = await languageRequest(
        doc.world.id,
        `/${selected}?age=${age.id}&revision=${compareHash}`,
        undefined,
        signal,
      );
      if (generation === sequence.current) {
        setComparison(diffDefinitions(old, definition));
        setComparing(true);
      }
    });
  }
  async function historical(operation, previewHash) {
    let preview;
    await action(
      operation === "history-apply"
        ? "Applying historical changes"
        : "Previewing historical changes",
      async (signal, generation) => {
        const next = await languageRequest(
          doc.world.id,
          `/${selected}/${operation}`,
          {
            ageId: age.id,
            revision: doc.world.revision,
            definitionRevision: version,
            request: {},
            ...(previewHash ? { previewHash, approved: true } : {}),
          },
          signal,
        );
        if (generation !== sequence.current) return;
        preview = next.preview;
        if (next.document) {
          onDocument(next.document);
          const e = next.document.entities.find(
            (e) => e.ageId === age.id && e.id === selected,
          );
          setDefinition(next.preview.target);
          setSaved(JSON.stringify(next.preview.target));
          setVersion(e.language.draftRevision);
          baseHead.current = e.language.draftRevision;
          setReport(null);
          setResult(null);
          setMessage(
            "Historical changes saved as a new target revision. The source is unchanged; compile the new target when ready.",
          );
        }
      },
    );
    return preview;
  }
  async function saveRegression() {
    if (!result || !canEdit) return;
    const id = await requestText("Regression case ID", "");
    if (!id?.trim()) return;
    if ((definition.tests || []).some((t) => t.id === id.trim())) {
      setError("Choose a unique regression case ID.");
      return;
    }
    const request = structuredClone(result.input);
    request.revision = "";
    edit("tests", [
      ...(definition.tests || []),
      {
        id: id.trim(),
        request,
        expectedStatus: result.status,
        expected: result.candidates,
        expectedSentences: result.sentences,
        expectedDiagnostics: result.diagnostics,
        notes: "",
      },
    ]);
    setMessage(
      "Regression case added to your draft. Review and save it, then compile before running the corpus.",
    );
  }
  async function reviewConflict() {
    await action("Refreshing World revision", async (signal, generation) => {
      const next = await historyRequest(`/${doc.world.id}`);
      if (generation !== sequence.current) return;
      onDocument(next);
      const e = next.entities.find(
          (e) => e.ageId === age.id && e.id === selected,
        ),
        s = e?.language || e?.script;
      if (!s)
        throw Error(
          "This record is no longer available. Export your draft before leaving.",
        );
      const latest = await languageRequest(
        doc.world.id,
        `/${selected}?age=${age.id}&revision=${s.draftRevision}`,
        undefined,
        signal,
      );
      if (generation !== sequence.current) return;
      setComparison(diffDefinitions(latest, definition));
      setComparing(true);
      baseHead.current = s.draftRevision;
      setConflict(false);
      setMessage(
        "Review the latest saved definition against your draft before saving a new revision. Your edits are retained.",
      );
    });
  }
  const scripts = records.filter((e) => e.kind === "script" && e.script),
    context = {
      segments: definition?.phonology?.segments || [],
      classes: definition?.phonology?.classes || [],
    },
    stats = definitionStats(definition),
    isScript = record?.kind === "script",
    compiled = state?.compiled?.some(
      (c) =>
        c.revision === version && c.engine === ENGINE && c.adapter === ADAPTER,
    ),
    fresh = !dirty && report?.revision === version && compiled;
  function edit(key, value) {
    setDefinition({ ...definition, [key]: value });
    setResult(null);
  }
  return (
    <main className="world-page languages-page">
      <header className="language-page-header">
        <div>
          <span className="eyebrow">{age.name}</span>
          <h1>Languages</h1>
          <p>Author sounds, words, grammar and writing systems.</p>
        </div>
        <div className="language-inline">
          <button
            onClick={async () => {
              if (await confirmLeave()) setImporting(true);
            }}
            disabled={!!busy || !canEdit}
          >
            Import definition
          </button>
          <button
            onClick={async () => {
              if (canEdit && (await confirmLeave())) setCreate(true);
            }}
            disabled={!canEdit}
          >
            Create language or script
          </button>
        </div>
      </header>
      <div className="language-workspace">
        <aside className="language-index">
          <label>
            Find language or script
            <input
              type="search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </label>
          <label>
            Record type filter
            <select
              value={typeFilter}
              onChange={(e) => setTypeFilter(e.target.value)}
            >
              <option value="">Languages and scripts</option>
              <option value="language">Languages</option>
              <option value="script">Scripts</option>
            </select>
          </label>
          {records
            .filter((e) => !typeFilter || e.kind === typeFilter)
            .filter((e) => e.name.toLowerCase().includes(search.toLowerCase()))
            .map((e) => (
              <button
                key={e.id}
                aria-current={e.id === selected ? "page" : undefined}
                onClick={() => select(e.id)}
              >
                <strong>{e.name}</strong>
                <small>
                  {e.kind} ·{" "}
                  {(e.language || e.script)?.status || "Status undecided"}
                </small>
              </button>
            ))}
          {!records.length && (
            <p>
              No languages or scripts in this Age. Create an incomplete draft or
              import an authored definition.
            </p>
          )}
        </aside>
        <div className="language-detail" aria-busy={!!busy}>
          {error && (
            <p role="alert" className="language-error">
              {error}
            </p>
          )}
          {conflict && (
            <button onClick={reviewConflict} disabled={!!busy}>
              Review latest saved revision; keep my edits
            </button>
          )}
          {busy && (
            <p role="status">
              {busy}…{" "}
              {![
                "Saving draft",
                "Activating revision",
                "Applying historical changes",
                "Restoring draft",
              ].includes(busy) && <button onClick={cancelWork}>Cancel</button>}
            </p>
          )}
          {message && <p role="status">{message}</p>}
          {record && definition ? (
            <>
              <header>
                <h2>{record.name}</h2>
                <p>
                  {dirty
                    ? "Unsaved edits"
                    : version === state?.draftRevision
                      ? "Saved draft"
                      : "Viewing an older saved revision"}{" "}
                  ·{" "}
                  {state?.activeRevision === version
                    ? "Active in this Age"
                    : "Not active"}
                </p>
              </header>
              <div className="language-toolbar">
                <label>
                  Definition revision
                  <select
                    value={version}
                    disabled={!!busy}
                    onChange={async (e) => {
                      const hash = e.target.value;
                      if (await confirmLeave()) await loadVersion(hash);
                    }}
                  >
                    {state?.revisions
                      ?.slice()
                      .reverse()
                      .map((r, i) => (
                        <option key={r.hash} value={r.hash}>
                          {r.hash.slice(0, 10)}
                          {i === 0 ? " · draft head" : ""}
                          {r.note ? ` · ${r.note}` : ""}
                        </option>
                      ))}
                  </select>
                </label>
                <button disabled={!!busy || !dirty || !canEdit} onClick={save}>
                  Save draft
                </button>
                {!isScript && (
                  <>
                    <button
                      disabled={!!busy}
                      onClick={() => compute("validate")}
                    >
                      Validate draft
                    </button>
                    <button
                      disabled={!!busy || dirty || !canEdit}
                      onClick={() => compute("compile")}
                    >
                      Compile saved revision
                    </button>
                    <button
                      disabled={
                        !!busy ||
                        !canEdit ||
                        !fresh ||
                        !Object.values(report?.ready || {}).some(Boolean)
                      }
                      onClick={activate}
                    >
                      Activate revision
                    </button>
                  </>
                )}
                <button
                  disabled={
                    !canEdit ||
                    !!busy ||
                    dirty ||
                    version === state?.draftRevision
                  }
                  onClick={restoreDraft}
                >
                  Restore saved revision as draft
                </button>
                <a
                  href={`/api/worlds/${doc.world.id}/languages/${selected}?age=${age.id}&revision=${version}&download=1`}
                  download
                >
                  Export saved JSON
                </a>
              </div>
              {dirty && (
                <label>
                  Revision note
                  <input
                    value={note}
                    maxLength={1000}
                    onChange={(e) => setNote(e.target.value)}
                  />
                </label>
              )}
              <nav className="institution-tabs" aria-label="Language sections">
                {(isScript ? ["Overview", "Scripts", "History"] : sections).map(
                  (s) => (
                    <button
                      key={s}
                      aria-current={section === s ? "page" : undefined}
                      onClick={() => setSection(s)}
                    >
                      {s}
                    </button>
                  ),
                )}
              </nav>
              {!canEdit && (
                <p role="status">
                  Read-only access. You can inspect definitions, validate
                  dependencies and run compiled operations.
                </p>
              )}
              <fieldset
                className="language-editing-surface"
                disabled={
                  !canEdit && !["Laboratory", "History"].includes(section)
                }
              >
                {section === "Overview" ? (
                  <>
                    <div className="language-grid">
                      <label>
                        Name
                        <input
                          value={metadata.name || ""}
                          maxLength={200}
                          onChange={(e) =>
                            setMetadata({ ...metadata, name: e.target.value })
                          }
                        />
                      </label>
                      <label>
                        Variety / kind
                        <input
                          value={metadata.kind || ""}
                          onChange={(e) =>
                            setMetadata({ ...metadata, kind: e.target.value })
                          }
                        />
                      </label>
                      <label>
                        Status
                        <input
                          value={metadata.status || ""}
                          onChange={(e) =>
                            setMetadata({ ...metadata, status: e.target.value })
                          }
                        />
                      </label>
                      <label>
                        Description
                        <textarea
                          value={metadata.description || ""}
                          onChange={(e) =>
                            setMetadata({
                              ...metadata,
                              description: e.target.value,
                            })
                          }
                        />
                      </label>
                      <label>
                        Author notes
                        <textarea
                          value={definition.notes}
                          onChange={(e) => edit("notes", e.target.value)}
                        />
                      </label>
                      <label>
                        Definition label
                        <input
                          value={definition.name}
                          maxLength={160}
                          onChange={(e) => edit("name", e.target.value)}
                        />
                      </label>
                    </div>
                    {!isScript && (
                      <dl className="language-stats">
                        {Object.entries(stats).map(([k, v]) => (
                          <div key={k}>
                            <dt>{k}</dt>
                            <dd>{v}</dd>
                          </div>
                        ))}
                      </dl>
                    )}
                    <h3>World relationships</h3>
                    {(metadata.links || []).map((l, i) => (
                      <div className="language-inline" key={i}>
                        <label>
                          Relationship
                          <select
                            value={l.kind}
                            onChange={(e) =>
                              setMetadata({
                                ...metadata,
                                links: metadata.links.map((v, j) =>
                                  i === j
                                    ? { ...v, kind: e.target.value, id: "" }
                                    : v,
                                ),
                              })
                            }
                          >
                            {[
                              "parent",
                              "variety",
                              "cognate",
                              "contact",
                              "speaker",
                              "region",
                              "script",
                            ].map((k) => (
                              <option key={k}>{k}</option>
                            ))}
                          </select>
                        </label>
                        <label>
                          Related record
                          <select
                            value={l.id}
                            onChange={(e) =>
                              setMetadata({
                                ...metadata,
                                links: metadata.links.map((v, j) =>
                                  i === j ? { ...v, id: e.target.value } : v,
                                ),
                              })
                            }
                          >
                            <option value="">Choose…</option>
                            {doc.entities
                              .filter(
                                (e) =>
                                  e.ageId === age.id &&
                                  !e.deleted &&
                                  e.id !== selected,
                              )
                              .map((e) => (
                                <option key={e.id} value={e.id}>
                                  {e.name} · {e.kind}
                                </option>
                              ))}
                          </select>
                        </label>
                        <button
                          onClick={() =>
                            setMetadata({
                              ...metadata,
                              links: metadata.links.filter((_, j) => j !== i),
                            })
                          }
                        >
                          Remove relationship
                        </button>
                      </div>
                    ))}
                    <button
                      onClick={() =>
                        setMetadata({
                          ...metadata,
                          links: [
                            ...(metadata.links || []),
                            { kind: "parent", id: "" },
                          ],
                        })
                      }
                    >
                      Add relationship
                    </button>
                    <Diagnostics report={report} />
                  </>
                ) : section === "Phonology" ? (
                  <>
                    <h3>Segments, natural classes and word rules</h3>
                    <p>
                      Work with stable segment IDs. Rules consume a snapshot
                      once in authored order. The word boundary is explicit;
                      output segments are never recursively rewritten by the
                      same rule.
                    </p>
                    {definition.phonology ? (
                      <StructuredEditor
                        label="Phonology"
                        schema={phonologySchema}
                        value={definition.phonology}
                        onChange={(v) => edit("phonology", v)}
                        context={context}
                      />
                    ) : (
                      <button
                        onClick={() => edit("phonology", emptyPhonology())}
                      >
                        Enable phonology
                      </button>
                    )}
                  </>
                ) : section === "Lexicon" ? (
                  <>
                    <h3>Lexemes and senses</h3>
                    <p>
                      Homophones retain separate identities. A suppletive cell
                      overrides its entire regular realization. Generated
                      candidates require explicit author entry here.
                    </p>
                    <StructuredEditor
                      label="Lexemes"
                      schema={lexiconSchema}
                      value={definition.lexicon}
                      onChange={(v) => edit("lexicon", v)}
                      context={context}
                    />
                    <h3>Morphemes, lexical relationships and provenance</h3>
                    <p>
                      Segmentation, derivational structures, cognacy/contact
                      sources and attestations are retained as authored
                      annotations. Inflection uses the explicit underlying
                      lexical form. Automatic derivation/compounding is not
                      executed by this adapter.
                    </p>
                    {definition.lexical ? (
                      <StructuredEditor
                        label="Lexical structure"
                        schema={lexicalSchema}
                        value={definition.lexical}
                        onChange={(v) => edit("lexical", v)}
                        context={context}
                      />
                    ) : (
                      <button
                        onClick={() =>
                          edit("lexical", {
                            morphemes: [],
                            entries: [],
                            relations: [],
                            derivations: [],
                            sources: [],
                            attestations: [],
                          })
                        }
                      >
                        Enable lexical structure
                      </button>
                    )}
                  </>
                ) : section === "Morphology" ? (
                  <>
                    <h3>Features and paradigms</h3>
                    <p>
                      No grammatical inventory is imposed. This adapter executes
                      prefix, suffix, zero and lexical suppletion. Other
                      authored mechanisms remain drafts and produce explicit
                      compile diagnostics.
                    </p>
                    {definition.morphology ? (
                      <StructuredEditor
                        label="Morphology"
                        schema={morphologySchema}
                        value={definition.morphology}
                        onChange={(v) => edit("morphology", v)}
                        context={context}
                      />
                    ) : (
                      <button
                        onClick={() => edit("morphology", emptyMorphology())}
                      >
                        Enable morphology
                      </button>
                    )}
                  </>
                ) : section === "Syntax" ? (
                  <>
                    <h3>Constructions and agreement</h3>
                    <p>
                      Realize explicit role/lexeme structures with flat feature
                      agreement. Nested constructions and recursive typed
                      structures are retained in imported definitions but are
                      not executable in this adapter. This is not a general
                      translator.
                    </p>
                    {definition.syntax ? (
                      <StructuredEditor
                        label="Syntax"
                        schema={syntaxSchema}
                        value={definition.syntax}
                        onChange={(v) => edit("syntax", v)}
                        context={context}
                      />
                    ) : (
                      <button onClick={() => edit("syntax", emptySyntax())}>
                        Enable syntax
                      </button>
                    )}
                  </>
                ) : section === "Scripts" ? (
                  isScript ? (
                    <>
                      <h3>Independent script</h3>
                      <label>
                        Writing direction
                        <select
                          value={definition.direction}
                          onChange={(e) => edit("direction", e.target.value)}
                        >
                          {["undecided", "ltr", "rtl", "ttb"].map((v) => (
                            <option key={v}>{v}</option>
                          ))}
                        </select>
                      </label>
                      <p>
                        Unicode strings are preserved as authored. Glyph asset
                        references are retained metadata; custom glyph rendering
                        is not implemented.
                      </p>
                      <StructuredEditor
                        label="Graphemes"
                        schema={graphemesSchema}
                        value={definition.graphemes}
                        onChange={(v) => edit("graphemes", v)}
                        context={context}
                      />
                    </>
                  ) : (
                    <>
                      <h3>Orthography and pinned script</h3>
                      <p>
                        Scriptless languages remain usable. Normalization is
                        explicit and applies to the complete written form.
                      </p>
                      <label>
                        Script revision
                        <select
                          value={
                            definition.orthography
                              ? `${definition.orthography.script.id}:${definition.orthography.script.revision}`
                              : ""
                          }
                          onChange={(e) => {
                            if (!e.target.value) {
                              edit("orthography", null);
                              return;
                            }
                            const [id, revision] = e.target.value.split(":");
                            edit("orthography", {
                              script: { id, revision },
                              normalization:
                                definition.orthography?.normalization || "none",
                              mappings: definition.orthography?.mappings || [],
                            });
                          }}
                        >
                          <option value="">No script / undecided</option>
                          {scripts.flatMap((s) =>
                            s.script.revisions.map((r) => (
                              <option
                                key={`${s.id}:${r.hash}`}
                                value={`${s.id}:${r.hash}`}
                              >
                                {s.name} · {r.hash.slice(0, 10)}
                              </option>
                            )),
                          )}
                        </select>
                      </label>
                      {definition.orthography && (
                        <>
                          <label>
                            Normalization
                            <select
                              value={definition.orthography.normalization}
                              onChange={(e) =>
                                edit("orthography", {
                                  ...definition.orthography,
                                  normalization: e.target.value,
                                })
                              }
                            >
                              {["none", "NFC", "NFD"].map((v) => (
                                <option key={v}>{v}</option>
                              ))}
                            </select>
                          </label>
                          <StructuredEditor
                            label="Mappings"
                            schema={mappingsSchema}
                            value={definition.orthography.mappings}
                            onChange={(v) =>
                              edit("orthography", {
                                ...definition.orthography,
                                mappings: v,
                              })
                            }
                          />
                        </>
                      )}
                    </>
                  )
                ) : section === "History" ? (
                  <>
                    <h3>Immutable definition history</h3>
                    <p>
                      Saved definitions retain their exact bodies. Changes in
                      this Age do not rewrite another Age or an ancestor.
                    </p>
                    <label>
                      Compare with revision
                      <select
                        value={compareHash}
                        onChange={(e) => setCompareHash(e.target.value)}
                      >
                        <option value="">Choose…</option>
                        {state?.revisions?.map((r) => (
                          <option key={r.hash} value={r.hash}>
                            {r.hash.slice(0, 10)} · {r.note}
                          </option>
                        ))}
                      </select>
                    </label>
                    <button disabled={!compareHash || !!busy} onClick={compare}>
                      Compare with current draft
                    </button>
                    <ol>
                      {state?.revisions
                        ?.slice()
                        .reverse()
                        .map((r) => (
                          <li key={r.hash}>
                            <code>{r.hash.slice(0, 16)}</code> ·{" "}
                            {r.note || "No revision note"} · {r.size} bytes
                            {r.parent
                              ? ` · parent ${r.parent.slice(0, 10)}`
                              : ""}
                          </li>
                        ))}
                    </ol>
                    {!isScript && (
                      <LanguageHistory
                        doc={doc}
                        age={age}
                        definition={definition}
                        onChange={(v) => edit("history", v)}
                        dirty={dirty}
                        readOnly={!canEdit}
                        busy={!!busy}
                        onPreview={() => historical("history-preview")}
                        onApply={(hash) => historical("history-apply", hash)}
                      />
                    )}
                  </>
                ) : section === "Laboratory" ? (
                  <>
                    <Diagnostics report={report} />
                    {!fresh && (
                      <p>
                        Save and compile this exact revision before running.
                        Validation alone does not create an executable revision.
                      </p>
                    )}
                    <Laboratory
                      definition={definition}
                      report={fresh ? report : null}
                      revision={version}
                      run={(q) => compute("run", q)}
                      busy={!!busy}
                      result={result}
                      canEdit={canEdit}
                      onSaveTest={saveRegression}
                      onTestsChange={(v) => edit("tests", v)}
                    />
                  </>
                ) : null}
              </fieldset>
            </>
          ) : (
            !busy && (
              <section className="language-empty">
                <h2>Build a language from explicit rules</h2>
                <p>
                  Select a language or script to author its definition, inspect
                  readiness and run deterministic experiments.
                </p>
              </section>
            )
          )}
        </div>
      </div>
      {create && (
        <CreateLanguage
          records={records}
          onClose={() => setCreate(false)}
          onCreate={createRecord}
        />
      )}
      {importing && (
        <Dialog
          title="Import authored definition"
          onClose={() => setImporting(false)}
          onSubmit={async () => {
            const d = JSON.parse(importText),
              existing = records.find(
                (e) => e.id === (d.languageId || d.scriptId),
              ),
              existingState = existing?.language || existing?.script;
            const next = await languageRequest(doc.world.id, "", {
              ageId: age.id,
              revision: doc.world.revision,
              kind: importKind,
              definitionText: importText,
              metadata: stateMetadata(metadataFor(existing)),
              displayName: existing?.name || d.name,
              expectedDraft: existingState?.draftRevision || "",
              note: "Imported authored definition",
            });
            onDocument(next);
            setImporting(false);
            setImportText("");
            const owner = d.languageId || d.scriptId;
            if (owner === selected) {
              const e = next.entities.find(
                  (e) => e.id === owner && e.ageId === age.id,
                ),
                s = e.language || e.script;
              await loadVersion(
                s.draftRevision,
                metadataFor(e),
                s.draftRevision,
              );
            } else onNavigate(owner);
          }}
          actions={
            <>
              <button type="button" onClick={() => setImporting(false)}>
                Cancel
              </button>
              <button type="submit" disabled={!importText.trim()}>
                Validate and save draft
              </button>
            </>
          }
        >
          <p>
            The server rejects duplicate/unknown keys, incompatible schemas and
            invalid Unicode. Import preserves the authored identity. Existing
            identities append a revision; they do not create a copy.
          </p>
          <label>
            Definition type
            <select
              value={importKind}
              onChange={(e) => setImportKind(e.target.value)}
            >
              <option value="language">Language</option>
              <option value="script">Script</option>
            </select>
          </label>
          <label>
            JSON file
            <input
              type="file"
              accept=".json,application/json"
              onChange={async (e) => {
                setImportError("");
                const file = e.target.files?.[0];
                if (file) {
                  if (file.size > 2 * 1024 * 1024) {
                    setImportError("Definition exceeds 2 MiB");
                    return;
                  }
                  try {
                    setImportText(
                      new TextDecoder("utf-8", {
                        fatal: true,
                        ignoreBOM: true,
                      }).decode(await file.arrayBuffer()),
                    );
                  } catch (e) {
                    setImportError(e.message);
                  }
                }
              }}
            />
          </label>
          <label>
            Definition JSON
            <textarea
              className="language-import"
              value={importText}
              onChange={(e) => setImportText(e.target.value)}
              spellCheck={false}
            />
          </label>
          {importError && <p role="alert">{importError}</p>}
        </Dialog>
      )}
      {comparing && (
        <Dialog
          title="Definition comparison"
          onClose={() => setComparing(false)}
          size="wide"
          actions={
            <button onClick={() => setComparing(false)}>
              Close comparison
            </button>
          }
          trackChanges={false}
        >
          {!comparison.length ? (
            <p>No definition changes.</p>
          ) : (
            comparison.map((v) => (
              <section key={v.path}>
                <h3>{v.path}</h3>
                <div className="language-comparison">
                  <pre>{JSON.stringify(v.before, null, 2) ?? "Absent"}</pre>
                  <pre>{JSON.stringify(v.after, null, 2) ?? "Absent"}</pre>
                </div>
              </section>
            ))
          )}
        </Dialog>
      )}
    </main>
  );
}
