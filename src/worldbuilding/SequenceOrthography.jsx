import { useEffect, useRef, useState } from "react";
import { StructuredEditor } from "./LanguageEditors.jsx";
import { languageRequest, newID } from "./languages.js";

const empty = () => ({
  version: "sequence-orthography/1",
  inputLayer: "surface",
  script: { id: "", revision: "" },
  renderer: "vector-script/1",
  rendererAdapter: "sign-layout/1",
  rendererCompiler: "vector-script-compiler/1",
  segmentation: "uniseg/0.4.7-unicode/15.0.0",
  normalizer: "x-text/0.29.0-unicode/15.0.0",
  inverse: "all",
  stages: [],
});
const copy = (x) => structuredClone(x);
const label = (u) => (u.kind === "sign" ? u.sign?.sign : u.value);
function UnitList({
  name,
  value,
  layer,
  onChange,
  signs,
  contextOnly = false,
}) {
  const kinds = [
    ...(layer.startsWith("surface")
      ? ["segment", ...(layer === "surface-boundaries" ? ["boundary"] : [])]
      : [layer]),
    ...(contextOnly ? ["edge"] : []),
  ];
  const update = (i, next) =>
    onChange(value.map((u, j) => (i === j ? next : u)));
  return (
    <fieldset>
      <legend>{name}</legend>
      {value.map((u, i) => {
        const sign = signs.find((s) => s.id === u.sign?.sign);
        return (
          <div
            className="language-object"
            key={i}
            role="group"
            aria-label={`${name} unit ${i + 1}`}
          >
            <label>
              Unit kind
              <select
                value={u.kind}
                onChange={(e) =>
                  update(
                    i,
                    e.target.value === "sign"
                      ? {
                          kind: "sign",
                          sign: {
                            kind: "sign",
                            sign: "",
                            reading: "",
                            roles: [],
                          },
                        }
                      : {
                          kind: e.target.value,
                          value:
                            e.target.value === "boundary"
                              ? "morpheme"
                              : e.target.value === "edge"
                                ? "word"
                                : "",
                        },
                  )
                }
              >
                {kinds.map((k) => (
                  <option key={k}>{k}</option>
                ))}
              </select>
            </label>
            {u.kind !== "sign" ? (
              <label>
                Exact unit value
                <input
                  value={u.value || ""}
                  onChange={(e) => update(i, { ...u, value: e.target.value })}
                  maxLength={4096}
                />
              </label>
            ) : (
              <>
                <label>
                  Sign ID
                  <select
                    value={u.sign?.sign || ""}
                    onChange={(e) =>
                      update(i, {
                        kind: "sign",
                        sign: {
                          kind: "sign",
                          sign: e.target.value,
                          reading: "",
                          roles: [],
                        },
                      })
                    }
                  >
                    <option value="">Choose a sign</option>
                    {signs.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.label} · {s.id}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Reading ID
                  <select
                    value={u.sign?.reading || ""}
                    onChange={(e) =>
                      update(i, {
                        ...u,
                        sign: { ...u.sign, reading: e.target.value },
                      })
                    }
                  >
                    <option value="">Explicit choice / silent sign</option>
                    {sign?.readings.map((r) => (
                      <option key={r.id} value={r.id}>
                        {r.text} · {r.id}
                      </option>
                    ))}
                  </select>
                </label>
                <div className="glyph-role-choices">
                  {sign?.roles.map((role) => (
                    <label key={role}>
                      <input
                        type="checkbox"
                        checked={u.sign?.roles?.includes(role) || false}
                        onChange={(e) =>
                          update(i, {
                            ...u,
                            sign: {
                              ...u.sign,
                              roles: e.target.checked
                                ? [...(u.sign.roles || []), role]
                                : u.sign.roles.filter((r) => r !== role),
                            },
                          })
                        }
                      />
                      {role}
                    </label>
                  ))}
                </div>
                <StructuredEditor
                  label="Sign context"
                  schema={{ type: "map" }}
                  value={u.sign?.context || {}}
                  onChange={(v) =>
                    update(i, { ...u, sign: { ...u.sign, context: v } })
                  }
                />
                {sign && (
                  <p>
                    Glyph: {sign.glyph}. Unicode association:{" "}
                    {sign.grapheme || "none"}. Artwork remains in the pinned
                    Script.
                  </p>
                )}
              </>
            )}
            <button
              type="button"
              onClick={() => onChange(value.filter((_, j) => i !== j))}
            >
              Remove {name.toLowerCase()} unit {i + 1}
            </button>
          </div>
        );
      })}
      <button
        type="button"
        disabled={value.length >= (contextOnly ? 4 : 16)}
        onClick={() => onChange([...value, { kind: kinds[0], value: "" }])}
      >
        Add {name.toLowerCase()} unit
      </button>
    </fieldset>
  );
}
function RenderedSpelling({ run }) {
  const [url, setURL] = useState("");
  useEffect(() => {
    if (!run.render?.svg) {
      setURL("");
      return;
    }
    const u = URL.createObjectURL(
      new Blob([run.render.svg], { type: "image/svg+xml" }),
    );
    setURL(u);
    return () => URL.revokeObjectURL(u);
  }, [run]);
  return (
    <article className="glyph-output">
      <p>Spelling {run.id}</p>
      <ClusterText text={run.text} clusters={run.clusters} />
      {url && <img src={url} alt="Spelling rendered by pinned Script" />}
      <p>
        {run.output
          .map((v) => `${v.unit.kind}: ${label(v.unit)}`)
          .join(" · ") || "Empty output"}
      </p>
      <details>
        <summary>Stages, rule spans and source provenance</summary>
        <pre>
          {JSON.stringify(
            {
              input: run.input,
              form: run.form,
              derivation: run.derivation,
              stages: run.stages,
              output: run.output,
              signs: run.render?.signs,
            },
            null,
            2,
          )}
        </pre>
      </details>
    </article>
  );
}
// Browser selection APIs use UTF-16; accepted selection is snapped to the exact
// server-supplied Unicode-15 cluster boundaries, without Intl.Segmenter guesses.
export function ClusterText({ text, clusters }) {
  const ref = useRef(null),
    [selection, setSelection] = useState({ start: 0, end: 0 });
  useEffect(() => setSelection({ start: 0, end: 0 }), [text]);
  const boundaries = [0, ...clusters.map((g) => g.utf16End)];
  function choose(start, end) {
    setSelection({ start, end });
    ref.current?.focus();
    ref.current?.setSelectionRange(boundaries[start], boundaries[end]);
  }
  return (
    <section aria-label="Grapheme cluster selection">
      <label>
        Exact written output
        <textarea
          ref={ref}
          readOnly
          value={text}
          onSelect={(e) => {
            const a = e.target.selectionStart,
              b = e.target.selectionEnd;
            const start = Math.max(
                0,
                boundaries.findLastIndex((x) => x <= a),
              ),
              end =
                a === b
                  ? start
                  : Math.max(
                      start,
                      boundaries.findIndex((x) => x >= b),
                    );
            if (a !== boundaries[start] || b !== boundaries[end])
              e.target.setSelectionRange(boundaries[start], boundaries[end]);
            setSelection({ start, end });
          }}
        />
      </label>
      <p>
        Selection: clusters {selection.start}–{selection.end}. UTF-16 and UTF-8
        offsets are shown separately.
      </p>
      <div className="language-inline">
        {clusters.map((g) => (
          <button
            key={g.index}
            type="button"
            onClick={() => choose(g.index, g.index + 1)}
            aria-label={`Select grapheme cluster ${g.index + 1}`}
          >
            {g.text} · {g.codePoints.join(" ")}
          </button>
        ))}
      </div>
      <details>
        <summary>Cluster offsets and code points</summary>
        <pre>{JSON.stringify(clusters, null, 2)}</pre>
      </details>
    </section>
  );
}
export default function SequenceOrthography({
  definition,
  onChange,
  scripts,
  world,
  age,
  id,
  version,
  report,
  fresh,
  canEdit,
}) {
  const w = definition.sequenceWriting || empty();
  const [stageIndex, setStageIndex] = useState(0),
    [page, setPage] = useState(0),
    [script, setScript] = useState(null),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [result, setResult] = useState(null),
    [op, setOp] = useState("spell"),
    [mode, setMode] = useState("composed"),
    [lexeme, setLexeme] = useState(""),
    [features, setFeatures] = useState({}),
    [units, setUnits] = useState(""),
    [text, setText] = useState(""),
    [analyze, setAnalyze] = useState(false),
    [choice, setChoice] = useState("");
  const serial = useRef(0),
    pending = useRef(null);
  const stage = w.stages[stageIndex],
    signs = script?.writing?.signs || [];
  useEffect(() => {
    const c = new AbortController();
    setScript(null);
    if (w.script.id && w.script.revision)
      languageRequest(
        world,
        `/${w.script.id}?age=${age}&revision=${w.script.revision}`,
        undefined,
        c.signal,
      )
        .then((next) => {
          if (!c.signal.aborted) setScript(next);
        })
        .catch((e) => {
          if (e.name !== "AbortError") setError(e.message);
        });
    return () => c.abort();
  }, [world, age, w.script.id, w.script.revision]);
  useEffect(() => {
    serial.current++;
    pending.current?.abort();
    setBusy(false);
    setResult(null);
    setChoice("");
  }, [definition, op, mode, lexeme, features, units, text, analyze]);
  useEffect(() => () => pending.current?.abort(), []);
  function change(next) {
    if (canEdit) onChange(next);
  }
  function updateStage(patch) {
    change({
      ...w,
      stages: w.stages.map((s, i) =>
        i === stageIndex ? { ...s, ...patch } : s,
      ),
    });
  }
  function updateRule(i, patch) {
    updateStage({
      rules: stage.rules.map((r, j) => (j === i ? { ...r, ...patch } : r)),
    });
  }
  async function run(selected = "") {
    const n = ++serial.current;
    pending.current?.abort();
    const controller = new AbortController();
    pending.current = controller;
    setBusy(true);
    setError("");
    try {
      const q = {
        revision: version,
        engine: report.engine,
        adapter: report.adapter,
        compiler: report.compiler,
        operation: op,
        policy: "all",
        limit: 128,
      };
      if (op === "spell") {
        if (mode === "composed") {
          q.lexeme = lexeme;
          q.features = features;
        } else {
          q.spelling = {
            units: units
              .trim()
              .split(/\s+/)
              .filter(Boolean)
              .map((v) =>
                v === "|"
                  ? { kind: "boundary", value: "morpheme" }
                  : { kind: "segment", value: v },
              ),
          };
        }
      } else if (op === "graphemes") q.text = text;
      else {
        q.spelling = { analyze, ...(selected ? { choice: selected } : {}) };
        if (w.stages.at(-1)?.outputLayer === "sign")
          q.spelling.units = JSON.parse(units || "[]");
        else q.text = text;
      }
      const response = await languageRequest(
        world,
        `/${id}/run`,
        { ageId: age, definitionRevision: version, request: q },
        controller.signal,
      );
      if (n === serial.current) {
        setResult(response.result);
        setChoice(selected);
      }
    } catch (e) {
      if (e.name !== "AbortError" && n === serial.current) setError(e.message);
    } finally {
      if (n === serial.current) setBusy(false);
    }
  }
  const enabled =
    report?.capabilities?.some(
      (c) => c.operation === op && c.support === "supported",
    ) &&
    report?.ready?.[op] &&
    fresh;
  return (
    <section
      className="glyph-workbench sequence-workbench"
      aria-label="Sequence orthography workbench"
    >
      <h3>Sequence orthography</h3>
      {report && (
        <section aria-label="Spelling readiness">
          <p>
            {report.capabilities
              ?.filter((c) =>
                ["spell", "unspell", "graphemes"].includes(c.operation),
              )
              .map(
                (c) =>
                  `${c.operation}: ${c.support === "unsupported" ? "unsupported" : report.ready?.[c.operation] ? "ready" : "not ready"}`,
              )
              .join(" · ")}
          </p>
          <ul>
            {report.diagnostics?.map((d, i) => (
              <li key={i}>
                <code>{d.code}</code> · {d.path} {d.ruleId} — {d.message}
              </li>
            ))}
          </ul>
        </section>
      )}
      <p>
        Finite authored rules, Unicode 15.0 grapheme clusters and pinned Script
        rendering. No automatic shaping, vertical layout, font export, prosody
        or recursive grammar.
      </p>
      <p>
        {w.version} · {w.segmentation} · {w.normalizer}. Saved revision:{" "}
        {version}.
      </p>
      <fieldset disabled={!canEdit}>
        <legend>Authored spelling source</legend>
        <label>
          Pinned vector Script revision
          <select
            value={w.script.id ? `${w.script.id}:${w.script.revision}` : ""}
            onChange={(e) => {
              const [id, revision] = e.target.value.split(":");
              change({
                ...w,
                script: { id: id || "", revision: revision || "" },
              });
            }}
          >
            <option value="">Choose Script 2 revision</option>
            {scripts.flatMap((s) =>
              s.script.revisions
                .filter((r) => r.schema === 2)
                .map((r) => (
                  <option key={`${s.id}:${r.hash}`} value={`${s.id}:${r.hash}`}>
                    {s.name} · {r.hash.slice(0, 12)}
                  </option>
                )),
            )}
          </select>
        </label>
        <label>
          Post-phonological input layer
          <select
            value={w.inputLayer}
            onChange={(e) => change({ ...w, inputLayer: e.target.value })}
          >
            {["surface", "surface-boundaries"].map((x) => (
              <option key={x}>{x}</option>
            ))}
          </select>
        </label>
        <label>
          Inverse policy
          <select
            value={w.inverse}
            onChange={(e) => change({ ...w, inverse: e.target.value })}
          >
            <option value="all">All bounded preimages</option>
            <option value="unsupported">Explicitly unsupported</option>
          </select>
        </label>
        <p>
          Normalization runs only in an authored normalization stage. Deletion
          and normalization disable inverse readiness.
        </p>
        <label>
          Selected spelling stage
          <select
            value={stageIndex}
            onChange={(e) => {
              setStageIndex(Number(e.target.value));
              setPage(0);
            }}
          >
            {w.stages.map((s, i) => (
              <option key={s.id} value={i}>
                {i + 1} · {s.id}
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          disabled={w.stages.length >= 8}
          onClick={() => {
            const input = w.stages.at(-1)?.outputLayer || w.inputLayer;
            change({
              ...w,
              stages: [
                ...w.stages,
                {
                  id: newID(),
                  kind: "rewrite",
                  inputLayer: input,
                  outputLayer: "grapheme",
                  unmatched: "reject",
                  normalization: "",
                  rules: [],
                },
              ],
            });
            setStageIndex(w.stages.length);
            setPage(0);
          }}
        >
          Add spelling stage
        </button>
        {stage && (
          <fieldset>
            <legend>Stage {stageIndex + 1}</legend>
            <label>
              Stable stage ID
              <input
                value={stage.id}
                onChange={(e) => updateStage({ id: e.target.value })}
                maxLength={80}
              />
            </label>
            <label>
              Stage kind
              <select
                value={stage.kind}
                onChange={(e) => {
                  if (e.target.value === "normalize" && stage.rules.length) {
                    setError(
                      "Add a separate normalization stage or explicitly remove these rules first. Authored rules have been retained.",
                    );
                    return;
                  }
                  updateStage(
                    e.target.value === "normalize"
                      ? {
                          kind: "normalize",
                          inputLayer: "grapheme",
                          outputLayer: "grapheme",
                          normalization: "NFC",
                          unmatched: "",
                          rules: [],
                        }
                      : {
                          kind: "rewrite",
                          normalization: "",
                          unmatched: "reject",
                        },
                  );
                }}
              >
                <option value="rewrite">Finite rewrite</option>
                <option value="normalize">Explicit normalization</option>
              </select>
            </label>
            <label>
              Stage input layer
              <select
                value={stage.inputLayer}
                onChange={(e) => updateStage({ inputLayer: e.target.value })}
              >
                {["surface", "surface-boundaries", "grapheme"].map((x) => (
                  <option key={x}>{x}</option>
                ))}
              </select>
            </label>
            <label>
              Stage output layer
              <select
                value={stage.outputLayer}
                onChange={(e) => updateStage({ outputLayer: e.target.value })}
              >
                {["grapheme", "sign"].map((x) => (
                  <option key={x}>{x}</option>
                ))}
              </select>
            </label>
            {stage.kind === "normalize" ? (
              <label>
                Normalization form
                <select
                  value={stage.normalization}
                  onChange={(e) =>
                    updateStage({ normalization: e.target.value })
                  }
                >
                  <option>NFC</option>
                  <option>NFD</option>
                </select>
              </label>
            ) : (
              <>
                <label>
                  Unmatched input
                  <select
                    value={stage.unmatched}
                    onChange={(e) => updateStage({ unmatched: e.target.value })}
                  >
                    <option value="reject">Report no mapping</option>
                    <option value="identity">
                      Preserve unchanged (same layer only)
                    </option>
                  </select>
                </label>
                {stage.rules.slice(page * 20, page * 20 + 20).map((r, j) => {
                  const i = page * 20 + j;
                  return (
                    <fieldset key={i} aria-label={`Spelling rule ${i + 1}`}>
                      <legend>Rule {i + 1}</legend>
                      <label>
                        Stable rule ID
                        <input
                          value={r.id}
                          maxLength={80}
                          onChange={(e) =>
                            updateRule(i, { id: e.target.value })
                          }
                        />
                      </label>
                      <label>
                        Priority
                        <input
                          type="number"
                          step="1"
                          min="-100000"
                          max="100000"
                          value={r.priority}
                          onChange={(e) => {
                            if (
                              e.target.value !== "" &&
                              Number.isInteger(Number(e.target.value))
                            )
                              updateRule(i, {
                                priority: Number(e.target.value),
                              });
                          }}
                        />
                      </label>
                      <UnitList
                        name="Input pattern"
                        value={r.input}
                        layer={stage.inputLayer}
                        onChange={(v) => updateRule(i, { input: v })}
                        signs={signs}
                      />
                      <UnitList
                        name="Left context"
                        value={r.left}
                        layer={stage.inputLayer}
                        onChange={(v) => updateRule(i, { left: v })}
                        signs={signs}
                        contextOnly
                      />
                      <UnitList
                        name="Right context"
                        value={r.right}
                        layer={stage.inputLayer}
                        onChange={(v) => updateRule(i, { right: v })}
                        signs={signs}
                        contextOnly
                      />
                      <UnitList
                        name="Output sequence"
                        value={r.output}
                        layer={stage.outputLayer}
                        onChange={(v) => updateRule(i, { output: v })}
                        signs={signs}
                      />
                      <button
                        type="button"
                        onClick={() =>
                          updateStage({
                            rules: stage.rules.filter((_, k) => k !== i),
                          })
                        }
                      >
                        Remove spelling rule {i + 1}
                      </button>
                    </fieldset>
                  );
                })}
                <div className="language-inline">
                  <button
                    type="button"
                    disabled={page === 0}
                    onClick={() => setPage(page - 1)}
                  >
                    Previous spelling rules
                  </button>
                  <span>
                    Rules page {page + 1} of{" "}
                    {Math.max(1, Math.ceil(stage.rules.length / 20))}
                  </span>
                  <button
                    type="button"
                    disabled={(page + 1) * 20 >= stage.rules.length}
                    onClick={() => setPage(page + 1)}
                  >
                    Next spelling rules
                  </button>
                </div>
                <button
                  type="button"
                  disabled={
                    w.stages.reduce((n, s) => n + s.rules.length, 0) >= 128
                  }
                  onClick={() => {
                    updateStage({
                      rules: [
                        ...stage.rules,
                        {
                          id: newID(),
                          priority: 0,
                          input: [],
                          left: [],
                          right: [],
                          output: [],
                        },
                      ],
                    });
                    setPage(Math.floor(stage.rules.length / 20));
                  }}
                >
                  Add spelling rule
                </button>
              </>
            )}
            <button
              type="button"
              disabled={stageIndex === 0}
              onClick={() => {
                const stages = copy(w.stages);
                [stages[stageIndex - 1], stages[stageIndex]] = [
                  stages[stageIndex],
                  stages[stageIndex - 1],
                ];
                change({ ...w, stages });
                setStageIndex(stageIndex - 1);
              }}
            >
              Move stage earlier
            </button>
            <button
              type="button"
              onClick={() => {
                change({
                  ...w,
                  stages: w.stages.filter((_, i) => i !== stageIndex),
                });
                setStageIndex(0);
                setPage(0);
              }}
            >
              Remove spelling stage
            </button>
          </fieldset>
        )}
      </fieldset>
      <h4>Spelling laboratory</h4>
      <p>
        Save and compile through the revision controls above. Execution uses
        that exact saved source and Script; edits cancel obsolete results.
      </p>
      <label>
        Sequence operation
        <select value={op} onChange={(e) => setOp(e.target.value)}>
          {["spell", "unspell", "graphemes"].map((x) => (
            <option key={x} value={x}>
              {x}
              {report?.ready?.[x] ? "" : " — needs dependencies / unsupported"}
            </option>
          ))}
        </select>
      </label>
      {op === "spell" && (
        <>
          <label>
            Spelling input mode
            <select value={mode} onChange={(e) => setMode(e.target.value)}>
              <option value="composed">Composed lexical form</option>
              <option value="units">Explicit surface units</option>
            </select>
          </label>
          {mode === "composed" ? (
            <>
              <label>
                Spelling lexeme
                <select
                  value={lexeme}
                  onChange={(e) => setLexeme(e.target.value)}
                >
                  <option value="">Choose lexeme</option>
                  {definition.lexicon.map((l) => (
                    <option key={l.id}>{l.id}</option>
                  ))}
                </select>
              </label>
              <StructuredEditor
                label="Spelling features"
                schema={{ type: "map" }}
                value={features}
                onChange={setFeatures}
              />
            </>
          ) : (
            <label>
              Source segment IDs and boundaries
              <input
                aria-label="Source segment IDs and boundaries"
                value={units}
                onChange={(e) => setUnits(e.target.value)}
                placeholder="p a d | a"
              />
              <span>
                Use | for an explicit morpheme boundary in surface-boundaries
                mode.
              </span>
            </label>
          )}
        </>
      )}
      {(op === "graphemes" ||
        (op === "unspell" && w.stages.at(-1)?.outputLayer !== "sign")) && (
        <label>
          Exact Unicode input
          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            maxLength={4096}
          />
        </label>
      )}
      {op === "unspell" && (
        <>
          {w.stages.at(-1)?.outputLayer === "sign" && (
            <label>
              Structured inverse sign units
              <textarea
                value={units}
                onChange={(e) => setUnits(e.target.value)}
                placeholder='[{"kind":"sign","sign":{"kind":"sign","sign":"authored-id","roles":["classifier"]}}]'
              />
            </label>
          )}
          <label>
            <input
              type="checkbox"
              checked={analyze}
              onChange={(e) => setAnalyze(e.target.checked)}
            />
            Run A2 after inverse spelling
          </label>
        </>
      )}
      <button type="button" disabled={!enabled || busy} onClick={() => run()}>
        Run sequence operation
      </button>
      {!fresh && <p>Save and compile this revision before running examples.</p>}
      {busy && <p role="status">Computing bounded spelling…</p>}
      {error && <p role="alert">{error}</p>}
      {result && (
        <section aria-label="Sequence operation result">
          <h4>{result.status}</h4>
          <p>
            {result.knownValid} valid analyses · {result.spelling?.steps}{" "}
            spelling steps
          </p>
          {result.diagnostics.map((d, i) => (
            <p key={i} role="alert">
              {d.code}: {d.message}
            </p>
          ))}
          {op === "graphemes" && (
            <ClusterText
              text={text}
              clusters={result.spelling?.clusters || []}
            />
          )}
          {result.spelling?.spellings.map((run) => (
            <section key={run.id}>
              <RenderedSpelling run={run} />
              <button
                type="button"
                onClick={() => {
                  setOp("unspell");
                  setUnits(
                    JSON.stringify(
                      run.output.map((v) => v.unit),
                      null,
                      2,
                    ),
                  );
                  setText(run.text);
                }}
              >
                Use output for inverse
              </button>
            </section>
          ))}
          {result.spelling?.preimages.map((p) => (
            <section key={p.id}>
              <button
                type="button"
                aria-pressed={choice === p.id}
                onClick={() => run(p.id)}
              >
                Choose preimage {p.id.slice(0, 12)}
              </button>
              <p>
                {p.units.map(label).join(" ")} ·{" "}
                {p.analyses.map((a) => `${a.lexeme}/${a.cell}`).join(", ") ||
                  "Raw preimage; lexical analysis not requested"}
              </p>
              <RenderedSpelling run={p.forward} />
            </section>
          ))}
        </section>
      )}
    </section>
  );
}
