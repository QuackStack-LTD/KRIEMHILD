import { useState, useEffect } from "react";
import {
  groups,
  roles,
  habitatKinds,
  blankRule,
  interactions,
} from "./biosphere.js";
import { label } from "./model.js";
export function CSVInput({ value = [], onChange, ...props }) {
  const joined = value.join(", "),
    [text, setText] = useState(joined);
  useEffect(() => setText(joined), [joined]);
  return (
    <input
      type="text"
      {...props}
      value={text}
      onChange={(e) => {
        setText(e.target.value);
        onChange(
          e.target.value
            .split(",")
            .map((s) => s.trim())
            .filter(Boolean),
        );
      }}
    />
  );
}
export function Choices({ values, value = [], onChange }) {
  return (
    <div className="bio-choices">
      {values.map((v) => (
        <label key={v}>
          <input
            type="checkbox"
            checked={value.includes(v)}
            onChange={(e) =>
              onChange(
                e.target.checked ? [...value, v] : value.filter((x) => x !== v),
              )
            }
          />
          {label(v)}
        </label>
      ))}
    </div>
  );
}
export function Tolerances({ value: r, onChange }) {
  return (
    <div>
      <label>
        Environment
        <select
          value={r.medium}
          onChange={(e) => onChange({ ...r, medium: e.target.value })}
        >
          {["terrestrial", "freshwater", "marine", "amphibious", "any"].map(
            (x) => (
              <option key={x}>{x}</option>
            ),
          )}
        </select>
      </label>
      <div className="bio-ranges">
        {[
          ["temperature", "Temperature °C"],
          ["moisture", "Soil moisture 0–1"],
          ["elevation", "Elevation m"],
          ["depth", "Water depth m"],
          ["salinity", "Salinity ‰"],
        ].map(([k, n]) => (
          <fieldset key={k}>
            <legend>{n}</legend>
            {["min", "max"].map((end) => (
              <label key={end}>
                {end}
                <input
                  required
                  type="number"
                  step="any"
                  value={r[k]?.[end] ?? 0}
                  onChange={(e) =>
                    onChange({
                      ...r,
                      [k]: { ...r[k], [end]: Number(e.target.value) },
                    })
                  }
                />
              </label>
            ))}
          </fieldset>
        ))}
      </div>
      <label>
        Minimum sunlight 0–1
        <input
          type="number"
          min="0"
          max="1"
          step=".01"
          value={r.minLight || 0}
          onChange={(e) => onChange({ ...r, minLight: Number(e.target.value) })}
        />
      </label>
      <label>
        Essential substrates (comma separated)
        <CSVInput
          value={r.substrates}
          onChange={(substrates) => onChange({ ...r, substrates })}
        />
      </label>
      <details>
        <summary>Native habitat restrictions</summary>
        <Choices
          values={habitatKinds}
          value={r.habitats}
          onChange={(habitats) => onChange({ ...r, habitats })}
        />
      </details>
    </div>
  );
}
export function TraitsEditor({ value = {}, onChange }) {
  return (
    <div>
      {Object.entries(value).map(([k, v], i) => (
        <div className="bio-trait" key={i}>
          <input
            type="text"
            aria-label="Trait name"
            required
            value={k}
            onChange={(e) => {
              const pairs = Object.entries(value);
              pairs[i][0] = e.target.value;
              onChange(Object.fromEntries(pairs));
            }}
          />
          <input
            type="text"
            aria-label="Trait description"
            value={v}
            onChange={(e) => onChange({ ...value, [k]: e.target.value })}
          />
          <button
            type="button"
            onClick={() =>
              onChange(
                Object.fromEntries(
                  Object.entries(value).filter(([key]) => key !== k),
                ),
              )
            }
          >
            Remove
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={() =>
          onChange({
            ...value,
            ["trait-" + (Object.keys(value).length + 1)]: "",
          })
        }
      >
        Add trait
      </button>
    </div>
  );
}
export function RulesEditor({
  value = [],
  onChange,
  habitats = [],
  species = [],
}) {
  const [open, setOpen] = useState("");
  const edit = (id, k, v) =>
    onChange(value.map((r) => (r.id === id ? { ...r, [k]: v } : r)));
  return (
    <div className="bio-rules">
      {value.map((r) => (
        <section key={r.id}>
          <div className="section-heading">
            <label>
              <input
                type="checkbox"
                checked={r.enabled}
                onChange={(e) => edit(r.id, "enabled", e.target.checked)}
              />{" "}
              {r.name || "Untitled rule"}
            </label>
            <button
              type="button"
              onClick={() => setOpen(open === r.id ? "" : r.id)}
            >
              {open === r.id ? "Collapse" : "Edit"}
            </button>
            <button
              type="button"
              onClick={() => onChange(value.filter((x) => x.id !== r.id))}
            >
              Remove
            </button>
          </div>
          <small>
            {r.provenance} · {r.strict ? "Requirement" : "Preference"} ·{" "}
            {r.targetGroup} ·{" "}
            {(r.habitats || []).join(", ") || "All habitat kinds"}
          </small>
          {open === r.id && (
            <div>
              <label>
                Rule name
                <input
                  type="text"
                  required
                  value={r.name}
                  onChange={(e) => edit(r.id, "name", e.target.value)}
                />
              </label>
              <label>
                Organism group
                <select
                  value={r.targetGroup}
                  onChange={(e) => edit(r.id, "targetGroup", e.target.value)}
                >
                  {groups.map((g) => (
                    <option key={g}>{g}</option>
                  ))}
                </select>
              </label>
              <details>
                <summary>Target ecological roles (empty means all)</summary>
                <Choices
                  values={roles}
                  value={r.targetRoles}
                  onChange={(v) => edit(r.id, "targetRoles", v)}
                />
              </details>
              <label>
                Specific lifeform (optional)
                <select
                  value={r.speciesId || ""}
                  onChange={(e) => edit(r.id, "speciesId", e.target.value)}
                >
                  <option value="">Generated members of this group</option>
                  {species
                    .filter((s) => s.biology?.group === r.targetGroup)
                    .map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name}
                      </option>
                    ))}
                </select>
              </label>
              <details>
                <summary>Habitat conditions</summary>
                <Choices
                  values={habitatKinds}
                  value={r.habitats}
                  onChange={(v) => edit(r.id, "habitats", v)}
                />
                <label>
                  Restrict to region
                  <select
                    value={r.habitatIds?.[0] || ""}
                    onChange={(e) =>
                      edit(
                        r.id,
                        "habitatIds",
                        e.target.value ? [e.target.value] : [],
                      )
                    }
                  >
                    <option value="">Any region</option>
                    {habitats.map((h) => (
                      <option key={h.id} value={h.id}>
                        {h.name}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Substrate
                  <input
                    type="text"
                    placeholder="volcanic, limestone, sand, geothermal…"
                    value={r.substrate || ""}
                    onChange={(e) => edit(r.id, "substrate", e.target.value)}
                  />
                </label>
              </details>
              <h4>Add / modify traits</h4>
              <TraitsEditor
                value={r.traits}
                onChange={(v) => edit(r.id, "traits", v)}
              />
              <label>
                Remove traits (comma separated)
                <CSVInput
                  value={r.removeTraits}
                  onChange={(v) => edit(r.id, "removeTraits", v)}
                />
              </label>
              <label>
                Required food / host group
                <select
                  value={r.foodGroup || ""}
                  onChange={(e) => edit(r.id, "foodGroup", e.target.value)}
                >
                  <option value="">Keep existing dependencies</option>
                  {groups.map((g) => (
                    <option key={g}>{g}</option>
                  ))}
                </select>
              </label>
              <label>
                Lifecycle effect
                <input
                  type="text"
                  value={r.lifecycle || ""}
                  onChange={(e) => edit(r.id, "lifecycle", e.target.value)}
                />
              </label>
              <label>
                Abundance multiplier
                <input
                  type="number"
                  min="0"
                  max="3"
                  step=".1"
                  value={r.abundance}
                  onChange={(e) =>
                    edit(r.id, "abundance", Number(e.target.value))
                  }
                />
              </label>
              <label>
                Priority
                <input
                  type="number"
                  value={r.priority}
                  onChange={(e) =>
                    edit(r.id, "priority", Number(e.target.value))
                  }
                />
              </label>
              <label>
                Conflicting traits
                <select
                  value={r.conflict}
                  onChange={(e) => edit(r.id, "conflict", e.target.value)}
                >
                  {["merge", "replace", "reject"].map((x) => (
                    <option key={x}>{x}</option>
                  ))}
                </select>
              </label>
              <label>
                <input
                  type="checkbox"
                  checked={r.strict}
                  onChange={(e) => edit(r.id, "strict", e.target.checked)}
                />{" "}
                Strict habitat / substrate requirement
              </label>
              <details>
                <summary>Explicit environmental override</summary>
                <p>
                  Changing tolerances permits unusual biology. Supply a
                  supporting trait or mechanism; unrelated traits remain
                  unchanged.
                </p>
                {r.tolerances ? (
                  <>
                    <Tolerances
                      value={r.tolerances}
                      onChange={(v) => edit(r.id, "tolerances", v)}
                    />
                    <button
                      type="button"
                      onClick={() => edit(r.id, "tolerances", null)}
                    >
                      Remove override
                    </button>
                  </>
                ) : (
                  <button
                    type="button"
                    onClick={() =>
                      edit(r.id, "tolerances", {
                        medium: "terrestrial",
                        temperature: { min: -20, max: 45 },
                        moisture: { min: 0, max: 1 },
                        elevation: { min: -500, max: 5000 },
                        depth: { min: 0, max: 0 },
                        salinity: { min: 0, max: 5 },
                        minLight: 0,
                      })
                    }
                  >
                    Define tolerance override
                  </button>
                )}
              </details>
            </div>
          )}
        </section>
      ))}
      <button
        type="button"
        onClick={() => {
          const r = blankRule();
          onChange([...value, r]);
          setOpen(r.id);
        }}
      >
        Add biological rule
      </button>
    </div>
  );
}
export function DependenciesEditor({ value = [], onChange }) {
  return (
    <div>
      {value.map((d, i) => (
        <div key={i} className="bio-dependency">
          <select
            aria-label="Dependency type"
            value={d.kind}
            onChange={(e) =>
              onChange(
                value.map((x, j) =>
                  j === i ? { ...x, kind: e.target.value } : x,
                ),
              )
            }
          >
            {interactions.map((k) => (
              <option key={k}>{k}</option>
            ))}
          </select>
          <select
            aria-label="Required group"
            value={d.group || ""}
            onChange={(e) =>
              onChange(
                value.map((x, j) =>
                  j === i ? { ...x, group: e.target.value } : x,
                ),
              )
            }
          >
            <option value="">Any group</option>
            {groups.map((k) => (
              <option key={k}>{k}</option>
            ))}
          </select>
          <select
            aria-label="Required role"
            value={d.role || ""}
            onChange={(e) =>
              onChange(
                value.map((x, j) =>
                  j === i ? { ...x, role: e.target.value } : x,
                ),
              )
            }
          >
            <option value="">Any role</option>
            {roles.map((k) => (
              <option key={k}>{k}</option>
            ))}
          </select>
          <input
            type="text"
            placeholder="Context / mechanism"
            value={d.description || ""}
            onChange={(e) =>
              onChange(
                value.map((x, j) =>
                  j === i ? { ...x, description: e.target.value } : x,
                ),
              )
            }
          />
          <button
            type="button"
            onClick={() => onChange(value.filter((_, j) => j !== i))}
          >
            Remove
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={() =>
          onChange([...value, { kind: "depends-on", role: "producer" }])
        }
      >
        Add required dependency
      </button>
    </div>
  );
}
