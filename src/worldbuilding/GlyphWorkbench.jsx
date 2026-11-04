import { useEffect, useRef, useState } from "react";
import { StructuredEditor } from "./LanguageEditors.jsx";
import { languageRequest, newID } from "./languages.js";
import {
  VECTOR_VERSIONS,
  signRoles,
  emptyWriting,
  blankGlyph,
  blankPath,
  safeGlyph,
  commandPath,
  commandSizes,
  boundedHistory,
} from "./vector-script.js";

const text = { type: "text" },
  readingSchema = {
    type: "array",
    blank: () => ({ id: newID(), text: "", when: {} }),
    item: { type: "object", fields: { id: text, text, when: { type: "map" } } },
  },
  associationSchema = {
    type: "array",
    blank: () => ({ kind: "meaning", value: "" }),
    item: {
      type: "object",
      fields: {
        kind: {
          type: "choice",
          options: [
            "phoneme",
            "grapheme",
            "lexeme",
            "morpheme",
            "meaning",
            "classifier",
          ],
        },
        value: text,
      },
    },
  };
const copy = (value) => structuredClone(value);
function NumberField({ label, value, onChange }) {
  const [draft, setDraft] = useState(String(value)),
    [error, setError] = useState(false);
  useEffect(() => setDraft(String(value)), [value]);
  return (
    <label>
      {label}
      <input
        type="number"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={() => {
          const next = Number(draft);
          if (draft.trim() && Number.isFinite(next)) {
            setError(false);
            if (next !== value) onChange(next);
          } else setError(true);
        }}
        aria-invalid={error}
      />
      {error && <span role="alert">Enter a finite number.</span>}
    </label>
  );
}
export function GlyphPreview({ glyph, size = 64, guides = false, ...props }) {
  if (!safeGlyph(glyph))
    return <span role="alert">Invalid geometry; preview is unavailable.</span>;
  return (
    <svg
      {...props}
      width={size}
      height={size}
      viewBox={glyph.viewBox.join(" ")}
      role="img"
      aria-label={`${glyph.name || glyph.id} glyph at ${size} pixels`}
    >
      <rect
        x={glyph.viewBox[0]}
        y={glyph.viewBox[1]}
        width={glyph.viewBox[2]}
        height={glyph.viewBox[3]}
        fill="#f5edda"
      />
      {glyph.paths.map((p) => (
        <path
          key={p.id}
          d={commandPath(p.commands)}
          fill={p.fill}
          stroke={p.stroke}
          strokeWidth={p.strokeWidth}
        />
      ))}
      {guides && (
        <line
          x1={glyph.viewBox[0]}
          x2={glyph.viewBox[0] + glyph.viewBox[2]}
          y1={glyph.baseline}
          y2={glyph.baseline}
          stroke="#b88b57"
          strokeWidth={glyph.viewBox[2] / 300}
        />
      )}
    </svg>
  );
}
function RoleChoices({ value, onChange, allowed = signRoles }) {
  return (
    <div className="glyph-role-choices">
      {allowed.map((role) => (
        <label key={role}>
          <input
            type="checkbox"
            checked={value.includes(role)}
            onChange={(e) =>
              onChange(
                e.target.checked
                  ? [...value, role]
                  : value.filter((r) => r !== role),
              )
            }
          />
          {role}
        </label>
      ))}
    </div>
  );
}
function SequenceNodes({
  items,
  onChange,
  signs,
  depth = 0,
  path = "sequence",
}) {
  const update = (i, patch) =>
    onChange(items.map((node, j) => (j === i ? { ...node, ...patch } : node)));
  return (
    <div className="glyph-sequence-nodes">
      {items.map((node, i) => {
        const sign = signs.find((s) => s.id === node.sign);
        return (
          <fieldset key={i} aria-label={`${path} node ${i + 1}`}>
            <legend>
              {node.kind} {i + 1}
            </legend>
            {node.kind === "sign" && (
              <>
                <label>
                  Sign
                  <select
                    value={node.sign || ""}
                    onChange={(e) =>
                      update(i, {
                        sign: e.target.value,
                        reading: "",
                        roles: [],
                        context: {},
                      })
                    }
                  >
                    <option value="">Choose a sign</option>
                    {signs.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.label || s.id}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Reading
                  <select
                    value={node.reading || ""}
                    onChange={(e) => update(i, { reading: e.target.value })}
                  >
                    <option value="">
                      Explicit choice / silent semantic sign
                    </option>
                    {sign?.readings.map((r) => (
                      <option key={r.id} value={r.id}>
                        {r.text || r.id} · {r.id}
                      </option>
                    ))}
                  </select>
                </label>
                <RoleChoices
                  value={node.roles || []}
                  allowed={sign?.roles || []}
                  onChange={(roles) => update(i, { roles })}
                />
                <StructuredEditor
                  label="Authored context"
                  schema={{ type: "map" }}
                  value={node.context || {}}
                  onChange={(context) => update(i, { context })}
                />
              </>
            )}
            {node.kind === "group" && depth < 4 && (
              <SequenceNodes
                items={node.children || []}
                signs={signs}
                depth={depth + 1}
                path={`${path}.${i + 1}`}
                onChange={(children) => update(i, { children })}
              />
            )}
            <button
              type="button"
              onClick={() => onChange(items.filter((_, j) => j !== i))}
            >
              Remove node {i + 1}
            </button>
            {i > 0 && (
              <button
                type="button"
                onClick={() => {
                  const next = [...items];
                  [next[i - 1], next[i]] = [next[i], next[i - 1]];
                  onChange(next);
                }}
              >
                Move node {i + 1} earlier
              </button>
            )}
          </fieldset>
        );
      })}
      <div className="language-inline">
        <button
          type="button"
          onClick={() =>
            onChange([
              ...items,
              { kind: "sign", sign: "", reading: "", roles: [], context: {} },
            ])
          }
        >
          Add sign
        </button>
        {depth < 4 && (
          <button
            type="button"
            onClick={() =>
              onChange([...items, { kind: "group", children: [] }])
            }
          >
            Add group
          </button>
        )}
        {depth === 0 && (
          <button
            type="button"
            onClick={() => onChange([...items, { kind: "break" }])}
          >
            Add line break
          </button>
        )}
      </div>
    </div>
  );
}
export default function GlyphWorkbench({
  definition,
  onChange,
  onDirection,
  world,
  age,
  id,
  version,
  compiled,
  dirty,
  canEdit,
}) {
  const writing = definition.writing || emptyWriting(),
    [selected, setSelected] = useState(writing.glyphs[0]?.id || ""),
    [selectedSign, setSelectedSign] = useState(writing.signs[0]?.id || ""),
    [sequence, setSequence] = useState(writing.sequences[0]?.id || ""),
    [filter, setFilter] = useState(""),
    [page, setPage] = useState(1),
    [pathID, setPathID] = useState(""),
    [commandPage, setCommandPage] = useState(1),
    [tool, setTool] = useState("draw"),
    [past, setPast] = useState([]),
    [future, setFuture] = useState([]),
    [svgSource, setSVGSource] = useState(""),
    [imported, setImported] = useState(null),
    [message, setMessage] = useState(""),
    [busy, setBusy] = useState(false),
    [result, setResult] = useState(null),
    [outputURL, setOutputURL] = useState("");
  const canvas = useRef(null),
    drag = useRef(null),
    pending = useRef(null),
    serial = useRef(0),
    active = useRef(writing);
  active.current = writing;
  const glyph = writing.glyphs.find((g) => g.id === selected),
    sign = writing.signs.find((s) => s.id === selectedSign),
    phrase = writing.sequences.find((s) => s.id === sequence),
    path = glyph?.paths.find((p) => p.id === pathID) || glyph?.paths[0];
  const commandPages = Math.max(
      1,
      Math.ceil((path?.commands.length || 0) / 20),
    ),
    currentCommandPage = Math.min(commandPage, commandPages);
  useEffect(() => setCommandPage(1), [path?.id]);
  useEffect(() => {
    serial.current++;
    pending.current?.abort();
    setBusy(false);
    setResult(null);
    setImported(null);
  }, [definition, sequence, selected]);
  useEffect(() => {
    if (!result?.svg) {
      setOutputURL("");
      return;
    }
    const url = URL.createObjectURL(
      new Blob([result.svg], { type: "image/svg+xml" }),
    );
    setOutputURL(url);
    return () => URL.revokeObjectURL(url);
  }, [result]);
  useEffect(() => () => pending.current?.abort(), []);
  function change(next, remember = true) {
    if (!canEdit) return;
    if (remember) {
      setPast((old) => boundedHistory([...old, copy(active.current)]));
      setFuture([]);
    }
    onChange(next);
  }
  function updateGlyph(patch, remember = true) {
    change(
      {
        ...active.current,
        glyphs: active.current.glyphs.map((g) =>
          g.id === selected ? { ...g, ...patch } : g,
        ),
      },
      remember,
    );
  }
  function updatePath(patch, remember = true) {
    updateGlyph(
      {
        paths: glyph.paths.map((p) =>
          p.id === path.id ? { ...p, ...patch } : p,
        ),
      },
      remember,
    );
  }
  function updateSign(patch) {
    change({
      ...writing,
      signs: writing.signs.map((s) =>
        s.id === selectedSign ? { ...s, ...patch } : s,
      ),
    });
  }
  function updatePhrase(patch) {
    change({
      ...writing,
      sequences: writing.sequences.map((s) =>
        s.id === sequence ? { ...s, ...patch } : s,
      ),
    });
  }
  function undo() {
    if (!past.length) return;
    setFuture((old) => boundedHistory([...old, copy(writing)]));
    onChange(past.at(-1));
    setPast((old) => old.slice(0, -1));
  }
  function redo() {
    if (!future.length) return;
    setPast((old) => boundedHistory([...old, copy(writing)]));
    onChange(future.at(-1));
    setFuture((old) => old.slice(0, -1));
  }
  function addGlyph() {
    if (writing.glyphs.length >= 256) {
      setMessage("The script is limited to 256 glyphs.");
      return;
    }
    const g = blankGlyph(newID());
    change({ ...writing, glyphs: [...writing.glyphs, g] });
    setSelected(g.id);
    setPathID("");
  }
  function addPath(commands = []) {
    if (!glyph || glyph.paths.length >= 128) return;
    const p = { ...blankPath(newID()), commands };
    updateGlyph({ paths: [...glyph.paths, p] });
    setPathID(p.id);
  }
  function addShape(kind) {
    const [x, y, w, h] = glyph.viewBox,
      a = x + w * 0.2,
      b = y + h * 0.2,
      c = x + w * 0.8,
      d = y + h * 0.8;
    let commands;
    if (kind === "rectangle")
      commands = [
        { op: "M", values: [a, b] },
        { op: "L", values: [c, b] },
        { op: "L", values: [c, d] },
        { op: "L", values: [a, d] },
        { op: "Z", values: [] },
      ];
    else {
      const cx = x + w / 2,
        cy = y + h / 2,
        rx = w * 0.3,
        ry = h * 0.3,
        k = 0.5522847498307936;
      commands = [
        { op: "M", values: [cx + rx, cy] },
        {
          op: "C",
          values: [cx + rx, cy + k * ry, cx + k * rx, cy + ry, cx, cy + ry],
        },
        {
          op: "C",
          values: [cx - k * rx, cy + ry, cx - rx, cy + k * ry, cx - rx, cy],
        },
        {
          op: "C",
          values: [cx - rx, cy - k * ry, cx - k * rx, cy - ry, cx, cy - ry],
        },
        {
          op: "C",
          values: [cx + k * rx, cy - ry, cx + rx, cy - k * ry, cx + rx, cy],
        },
        { op: "Z", values: [] },
      ];
    }
    addPath(commands);
  }
  function coordinates(event) {
    const point = canvas.current.createSVGPoint();
    point.x = event.clientX;
    point.y = event.clientY;
    const p = point.matrixTransform(canvas.current.getScreenCTM().inverse());
    return [Math.round(p.x * 100) / 100, Math.round(p.y * 100) / 100];
  }
  function draw(event) {
    if (!canEdit || tool !== "draw" || event.button !== 0 || !glyph) return;
    const point = coordinates(event);
    if (!path || path.commands.at(-1)?.op === "Z") {
      addPath([{ op: "M", values: point }]);
    } else
      updatePath({
        commands: [
          ...path.commands,
          { op: path.commands.length ? "L" : "M", values: point },
        ],
      });
  }
  function moveHandle(event, ci, vi) {
    if (!drag.current) return;
    const point = coordinates(event);
    updatePath(
      {
        commands: path.commands.map((c, j) =>
          j === ci
            ? {
                ...c,
                values: c.values.map((n, k) =>
                  k === vi ? point[0] : k === vi + 1 ? point[1] : n,
                ),
              }
            : c,
        ),
      },
      false,
    );
  }
  async function job(action, body, receive) {
    const current = ++serial.current;
    pending.current?.abort();
    const controller = new AbortController();
    pending.current = controller;
    setBusy(true);
    setMessage("");
    try {
      const data = await languageRequest(
        world,
        `/${id}/${action}`,
        { ageId: age, definitionRevision: version, ...body },
        controller.signal,
      );
      if (current === serial.current) receive(data);
    } catch (error) {
      if (error.name !== "AbortError" && current === serial.current)
        setMessage(error.message);
    } finally {
      if (current === serial.current) setBusy(false);
    }
  }
  const filtered = writing.glyphs.filter((g) =>
      `${g.name} ${g.id}`.toLowerCase().includes(filter.toLowerCase()),
    ),
    pages = Math.max(1, Math.ceil(filtered.length / 20)),
    currentPage = Math.min(page, pages);
  return (
    <section
      className="glyph-workbench"
      aria-label="Custom writing system workbench"
    >
      <h3>Custom glyphs and sign sequences</h3>
      <p>
        Normalized vector artwork, authored readings and roles. No font
        installation, automatic transliteration, vertical layout or sequence
        spelling rules are implemented.
      </p>
      <fieldset disabled={!canEdit}>
        <div className="language-inline">
          <button type="button" disabled={!past.length} onClick={undo}>
            Undo glyph edit
          </button>
          <button type="button" disabled={!future.length} onClick={redo}>
            Redo glyph edit
          </button>
        </div>
        <div className="language-grid">
          <label>
            Writing profile
            <select
              value={writing.profile}
              onChange={(e) => change({ ...writing, profile: e.target.value })}
            >
              {[
                "alphabetic",
                "abjad",
                "abugida",
                "syllabic",
                "logographic",
                "mixed",
              ].map((p) => (
                <option key={p}>{p}</option>
              ))}
            </select>
          </label>
          <label>
            Writing direction
            <select
              value={definition.direction}
              onChange={(e) => onDirection(e.target.value)}
            >
              <option value="ltr">Left to right</option>
              <option value="rtl">Right to left</option>
              {!["ltr", "rtl"].includes(definition.direction) && (
                <option value={definition.direction}>
                  {definition.direction} · unsupported
                </option>
              )}
            </select>
          </label>
          <NumberField
            label="Units per em"
            value={writing.unitsPerEm}
            onChange={(unitsPerEm) => change({ ...writing, unitsPerEm })}
          />
          <NumberField
            label="Line height"
            value={writing.lineHeight}
            onChange={(lineHeight) => change({ ...writing, lineHeight })}
          />
          <NumberField
            label="Sign spacing"
            value={writing.spacing}
            onChange={(spacing) => change({ ...writing, spacing })}
          />
        </div>
        <h4>Glyph library</h4>
        <label>
          Search glyphs
          <input
            value={filter}
            onChange={(e) => {
              setFilter(e.target.value);
              setPage(1);
            }}
          />
        </label>
        <button type="button" onClick={addGlyph}>
          New glyph
        </button>
        <div className="glyph-library">
          {filtered.slice((currentPage - 1) * 20, currentPage * 20).map((g) => (
            <button
              type="button"
              key={g.id}
              aria-label={g.name || "Unnamed glyph"}
              aria-pressed={selected === g.id}
              onClick={() => {
                setSelected(g.id);
                setPathID("");
              }}
            >
              <GlyphPreview glyph={g} size={48} />
              <span>{g.name || "Unnamed glyph"}</span>
            </button>
          ))}
        </div>
        {pages > 1 && (
          <div className="language-inline">
            <button
              type="button"
              disabled={currentPage === 1}
              onClick={() => setPage(currentPage - 1)}
            >
              Previous glyphs
            </button>
            <span>
              Glyph page {currentPage} of {pages}
            </span>
            <button
              type="button"
              disabled={currentPage === pages}
              onClick={() => setPage(currentPage + 1)}
            >
              Next glyphs
            </button>
          </div>
        )}
        {glyph && (
          <>
            <label>
              Glyph name
              <input
                value={glyph.name}
                maxLength={160}
                onChange={(e) => updateGlyph({ name: e.target.value })}
              />
            </label>
            <label>
              Stable glyph ID
              <input readOnly value={glyph.id} />
            </label>
            <button
              type="button"
              disabled={writing.glyphs[0]?.id === glyph.id}
              onClick={() => {
                const next = [...writing.glyphs],
                  i = next.findIndex((g) => g.id === glyph.id);
                [next[i - 1], next[i]] = [next[i], next[i - 1]];
                change({ ...writing, glyphs: next });
              }}
            >
              Move glyph earlier
            </button>
            <div className="language-grid">
              {["x", "y", "width", "height"].map((label, i) => (
                <NumberField
                  key={label}
                  label={`Glyph viewBox ${label}`}
                  value={glyph.viewBox[i]}
                  onChange={(v) =>
                    updateGlyph({
                      viewBox: glyph.viewBox.map((n, j) => (i === j ? v : n)),
                    })
                  }
                />
              ))}
              {["baseline", "advance", "bearing", "scale"].map((field) => (
                <NumberField
                  key={field}
                  label={`Glyph ${field}`}
                  value={glyph[field]}
                  onChange={(value) => updateGlyph({ [field]: value })}
                />
              ))}
            </div>
            <div className="language-inline">
              <button type="button" onClick={() => addShape("rectangle")}>
                Add rectangle
              </button>
              <button type="button" onClick={() => addShape("ellipse")}>
                Add ellipse
              </button>
              <button
                type="button"
                onClick={() =>
                  addPath([
                    {
                      op: "M",
                      values: [
                        glyph.viewBox[0] + glyph.viewBox[2] / 2,
                        glyph.viewBox[1] + glyph.viewBox[3] / 2,
                      ],
                    },
                  ])
                }
              >
                New path
              </button>
              <label>
                Drawing tool
                <select value={tool} onChange={(e) => setTool(e.target.value)}>
                  <option value="draw">Click to add line points</option>
                  <option value="handles">Edit points and curve handles</option>
                </select>
              </label>
            </div>
            {safeGlyph(glyph) ? (
              <svg
                ref={canvas}
                className="glyph-canvas"
                role="img"
                aria-label="Glyph drawing area"
                viewBox={glyph.viewBox.join(" ")}
                onPointerDown={draw}
              >
                <rect
                  x={glyph.viewBox[0]}
                  y={glyph.viewBox[1]}
                  width={glyph.viewBox[2]}
                  height={glyph.viewBox[3]}
                  fill="#f5edda"
                />
                <line
                  x1={glyph.viewBox[0]}
                  x2={glyph.viewBox[0] + glyph.viewBox[2]}
                  y1={glyph.baseline}
                  y2={glyph.baseline}
                  stroke="#b88b57"
                  strokeWidth={glyph.viewBox[2] / 300}
                />
                {glyph.paths.map((p) => (
                  <path
                    key={p.id}
                    d={commandPath(p.commands)}
                    fill={p.fill}
                    stroke={p.stroke}
                    strokeWidth={p.strokeWidth}
                    pointerEvents="none"
                  />
                ))}
                {tool === "handles" &&
                  path?.commands.flatMap((c, ci) =>
                    c.values.flatMap((_, vi) =>
                      vi % 2
                        ? []
                        : [
                            <circle
                              key={`${ci}-${vi}`}
                              cx={c.values[vi]}
                              cy={c.values[vi + 1]}
                              r={glyph.viewBox[2] / 60}
                              fill="#1d829d"
                              stroke="#ffffff"
                              strokeWidth={glyph.viewBox[2] / 400}
                              tabIndex={canEdit ? 0 : -1}
                              role="button"
                              aria-label={`Move command ${ci + 1} point ${vi / 2 + 1}`}
                              onPointerDown={(e) => {
                                e.stopPropagation();
                                e.currentTarget.setPointerCapture(e.pointerId);
                                drag.current = copy(writing);
                              }}
                              onPointerMove={(e) => {
                                if (drag.current) {
                                  e.stopPropagation();
                                  moveHandle(e, ci, vi);
                                }
                              }}
                              onPointerUp={(e) => {
                                e.stopPropagation();
                                if (drag.current) {
                                  const before = drag.current;
                                  setPast((old) =>
                                    boundedHistory([...old, before]),
                                  );
                                  setFuture([]);
                                  drag.current = null;
                                }
                              }}
                              onPointerCancel={() => {
                                if (drag.current) onChange(drag.current);
                                drag.current = null;
                              }}
                              onKeyDown={(e) => {
                                const delta = {
                                  ArrowLeft: [-1, 0],
                                  ArrowRight: [1, 0],
                                  ArrowUp: [0, -1],
                                  ArrowDown: [0, 1],
                                }[e.key];
                                if (!delta) return;
                                e.preventDefault();
                                const step = e.shiftKey ? 10 : 1;
                                updatePath({
                                  commands: path.commands.map((command, j) =>
                                    j === ci
                                      ? {
                                          ...command,
                                          values: command.values.map(
                                            (v, k) =>
                                              v +
                                              (k === vi
                                                ? delta[0] * step
                                                : k === vi + 1
                                                  ? delta[1] * step
                                                  : 0),
                                          ),
                                        }
                                      : command,
                                  ),
                                });
                              }}
                            />,
                          ],
                    ),
                  )}
              </svg>
            ) : (
              <p role="alert">
                Invalid vector geometry. Correct the numeric commands below
                before previewing.
              </p>
            )}
            <p>
              Draw by clicking, or use New path and the numeric command editor.
              Curve control points support pointer dragging and arrow keys
              (Shift: 10 units).
            </p>
            <div className="glyph-size-previews">
              {[24, 64, 192].map((size) => (
                <GlyphPreview key={size} glyph={glyph} size={size} guides />
              ))}
            </div>
            <label>
              Path
              <select
                value={path?.id || ""}
                onChange={(e) => setPathID(e.target.value)}
              >
                {glyph.paths.map((p, i) => (
                  <option key={p.id} value={p.id}>
                    Path {i + 1} · {p.id}
                  </option>
                ))}
              </select>
            </label>
            {path && (
              <details open>
                <summary>Numeric path and curve editor</summary>
                <div className="language-grid">
                  <label>
                    Path fill
                    <input
                      value={path.fill}
                      onChange={(e) => updatePath({ fill: e.target.value })}
                    />
                  </label>
                  <label>
                    Path stroke
                    <input
                      value={path.stroke}
                      onChange={(e) => updatePath({ stroke: e.target.value })}
                    />
                  </label>
                  <NumberField
                    label="Path stroke width"
                    value={path.strokeWidth}
                    onChange={(strokeWidth) => updatePath({ strokeWidth })}
                  />
                </div>
                {path.commands
                  .slice((currentCommandPage - 1) * 20, currentCommandPage * 20)
                  .map((c, offset) => {
                    const i = (currentCommandPage - 1) * 20 + offset;
                    return (
                      <div className="glyph-command" key={i}>
                        <label>
                          Command {i + 1}
                          <select
                            value={c.op}
                            onChange={(e) => {
                              const op = e.target.value;
                              updatePath({
                                commands: path.commands.map((v, j) =>
                                  i === j
                                    ? {
                                        op,
                                        values: Array.from(
                                          { length: commandSizes[op] },
                                          (_, k) => c.values[k] ?? 0,
                                        ),
                                      }
                                    : v,
                                ),
                              });
                            }}
                          >
                            {Object.keys(commandSizes).map((op) => (
                              <option key={op}>{op}</option>
                            ))}
                          </select>
                        </label>
                        {c.values.map((v, vi) => (
                          <NumberField
                            key={vi}
                            label={`Command ${i + 1} coordinate ${vi + 1}`}
                            value={v}
                            onChange={(value) =>
                              updatePath({
                                commands: path.commands.map((command, j) =>
                                  j === i
                                    ? {
                                        ...command,
                                        values: command.values.map((n, k) =>
                                          k === vi ? value : n,
                                        ),
                                      }
                                    : command,
                                ),
                              })
                            }
                          />
                        ))}
                        <button
                          type="button"
                          onClick={() =>
                            updatePath({
                              commands: path.commands.filter((_, j) => j !== i),
                            })
                          }
                        >
                          Remove command {i + 1}
                        </button>
                      </div>
                    );
                  })}
                {commandPages > 1 && (
                  <div className="language-inline">
                    <button
                      type="button"
                      disabled={currentCommandPage === 1}
                      onClick={() => setCommandPage(currentCommandPage - 1)}
                    >
                      Previous commands
                    </button>
                    <span>
                      Command page {currentCommandPage} of {commandPages}
                    </span>
                    <button
                      type="button"
                      disabled={currentCommandPage === commandPages}
                      onClick={() => setCommandPage(currentCommandPage + 1)}
                    >
                      Next commands
                    </button>
                  </div>
                )}
                <div className="language-inline">
                  {["L", "Q", "C", "Z"].map((op) => (
                    <button
                      type="button"
                      key={op}
                      onClick={() =>
                        updatePath({
                          commands: [
                            ...path.commands,
                            {
                              op,
                              values: Array(commandSizes[op]).fill(
                                glyph.viewBox[2] / 2,
                              ),
                            },
                          ],
                        })
                      }
                    >
                      Add {op} command
                    </button>
                  ))}
                </div>
                <button
                  type="button"
                  onClick={() =>
                    updateGlyph({
                      paths: glyph.paths.filter((p) => p.id !== path.id),
                    })
                  }
                >
                  Remove selected path
                </button>
              </details>
            )}
            <details>
              <summary>Import safe SVG artwork</summary>
              <p>
                Maximum 64 KiB. Passive paths/shapes only; transforms, styles,
                active content and resource links are rejected. Review
                normalized geometry before using it.
              </p>
              <label>
                SVG file
                <input
                  type="file"
                  accept=".svg,image/svg+xml"
                  onChange={async (e) => {
                    const file = e.target.files?.[0];
                    if (!file) return;
                    if (file.size > 65536) {
                      setMessage("SVG exceeds 64 KiB.");
                      return;
                    }
                    setSVGSource(await file.text());
                  }}
                />
              </label>
              <label>
                SVG source
                <textarea
                  value={svgSource}
                  maxLength={65536}
                  onChange={(e) => setSVGSource(e.target.value)}
                />
              </label>
              <button
                type="button"
                disabled={!svgSource || busy}
                onClick={() =>
                  job(
                    "import-svg",
                    {
                      definition,
                      svg: svgSource,
                      glyphId: glyph.id,
                      glyphName: glyph.name,
                    },
                    (data) => setImported(data.import),
                  )
                }
              >
                Normalize SVG
              </button>
            </details>
          </>
        )}
      </fieldset>
      {imported && (
        <section aria-label="Normalized SVG review">
          <h4>Normalized SVG review</h4>
          <GlyphPreview glyph={imported.glyph} size={192} />
          <ul>
            {imported.warnings.map((warning, i) => (
              <li key={i}>{warning}</li>
            ))}
          </ul>
          <button
            type="button"
            disabled={!canEdit || !glyph}
            onClick={() => {
              updateGlyph({
                ...imported.glyph,
                id: glyph.id,
                name: glyph.name,
              });
              setImported(null);
              setSVGSource("");
            }}
          >
            Use imported artwork
          </button>
          <details>
            <summary>Normalized geometry</summary>
            <pre>{JSON.stringify(imported.glyph, null, 2)}</pre>
          </details>
        </section>
      )}
      <fieldset disabled={!canEdit}>
        <h4>Signs, readings and associations</h4>
        <p>
          A sign references artwork independently of Unicode.
          Linguistic/semantic associations are authored annotations; they do not
          infer etymologies, translations or external record links.
        </p>
        <button
          type="button"
          onClick={() => {
            if (writing.signs.length >= 512) {
              setMessage("The script is limited to 512 signs.");
              return;
            }
            const s = {
              id: newID(),
              label: "",
              glyph: glyph?.id || "",
              grapheme: "",
              roles: [],
              readings: [],
              associations: [],
            };
            change({ ...writing, signs: [...writing.signs, s] });
            setSelectedSign(s.id);
          }}
        >
          New sign
        </button>
        <label>
          Selected sign
          <select
            value={selectedSign}
            onChange={(e) => setSelectedSign(e.target.value)}
          >
            <option value="">Choose sign</option>
            {writing.signs.map((s) => (
              <option key={s.id} value={s.id}>
                {s.label || s.id}
              </option>
            ))}
          </select>
        </label>
        {sign && (
          <>
            <label>
              Sign label
              <input
                value={sign.label}
                maxLength={160}
                onChange={(e) => updateSign({ label: e.target.value })}
              />
            </label>
            <label>
              Stable sign ID
              <input readOnly value={sign.id} />
            </label>
            <label>
              Sign glyph
              <select
                value={sign.glyph}
                onChange={(e) => updateSign({ glyph: e.target.value })}
              >
                <option value="">Choose glyph</option>
                {writing.glyphs.map((g) => (
                  <option key={g.id} value={g.id}>
                    {g.name || g.id}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Authored Unicode/grapheme association (optional)
              <input
                value={sign.grapheme}
                maxLength={256}
                onChange={(e) => updateSign({ grapheme: e.target.value })}
              />
            </label>
            <RoleChoices
              value={sign.roles}
              onChange={(roles) => updateSign({ roles })}
            />
            <StructuredEditor
              label="Readings"
              schema={readingSchema}
              value={sign.readings}
              onChange={(readings) => updateSign({ readings })}
            />
            <StructuredEditor
              label="Associations"
              schema={associationSchema}
              value={sign.associations}
              onChange={(associations) => updateSign({ associations })}
            />
          </>
        )}
        <h4>Structured sequences</h4>
        <button
          type="button"
          onClick={() => {
            if (writing.sequences.length >= 128) {
              setMessage("The script is limited to 128 sequences.");
              return;
            }
            const s = { id: newID(), name: "", items: [] };
            change({ ...writing, sequences: [...writing.sequences, s] });
            setSequence(s.id);
          }}
        >
          New sequence
        </button>
        <label>
          Selected sequence
          <select
            value={sequence}
            onChange={(e) => setSequence(e.target.value)}
          >
            <option value="">Choose sequence</option>
            {writing.sequences.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name || s.id}
              </option>
            ))}
          </select>
        </label>
        {phrase && (
          <>
            <label>
              Sequence name
              <input
                value={phrase.name}
                maxLength={160}
                onChange={(e) => updatePhrase({ name: e.target.value })}
              />
            </label>
            <SequenceNodes
              items={phrase.items}
              signs={writing.signs}
              onChange={(items) => updatePhrase({ items })}
            />
          </>
        )}
      </fieldset>
      <div className="language-inline">
        <button
          type="button"
          disabled={!phrase || busy}
          onClick={() =>
            job(
              "render-script",
              {
                definition,
                scriptRequest: {
                  ...VECTOR_VERSIONS,
                  revision: "",
                  sequenceId: sequence,
                },
              },
              (data) => setResult(data.scriptResult),
            )
          }
        >
          Preview draft sequence
        </button>
        <button
          type="button"
          disabled={!phrase || busy || dirty || !compiled}
          onClick={() =>
            job(
              "run",
              {
                scriptRequest: {
                  ...VECTOR_VERSIONS,
                  revision: version,
                  sequenceId: sequence,
                },
              },
              (data) => setResult(data.scriptResult),
            )
          }
        >
          Render saved sequence
        </button>
      </div>
      {busy && <p role="status">Computing bounded script preview…</p>}
      {message && <p role="alert">{message}</p>}
      {result && (
        <section className="glyph-output" aria-label="Script render result">
          <h4>{result.status}</h4>
          {outputURL && (
            <>
              <img src={outputURL} alt="Rendered authored sign sequence" />
              <a href={outputURL} download="authored-sign-sequence.svg">
                Download rendered SVG
              </a>
            </>
          )}
          <p>
            Revision {result.revision} · {result.engine} · {result.adapter} ·{" "}
            {result.compiler}
          </p>
          {result.diagnostics.map((d, i) => (
            <p key={i} role="alert">
              {d.code} · {d.path}: {d.message}
            </p>
          ))}
          {result.choices.map((c) => (
            <p key={c.path}>
              Choose reading for {c.sign}:{" "}
              {c.readings.join(", ") || "No compatible authored readings"}
            </p>
          ))}
          <details>
            <summary>Sign identities, readings, roles and layout trace</summary>
            <pre>
              {JSON.stringify(
                {
                  sequence: result.sequence,
                  signs: result.signs,
                  trace: result.trace,
                },
                null,
                2,
              )}
            </pre>
          </details>
        </section>
      )}
    </section>
  );
}
