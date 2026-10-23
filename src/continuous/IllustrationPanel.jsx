export default function IllustrationPanel({
  config,
  onChange,
  onScope,
  selected,
  onOverride,
  onAdd,
  entities,
  onSelect,
  objects,
  busy,
}) {
  return (
    <fieldset disabled={busy}>
      <h2>Medieval Illustrator</h2>
      <label>
        Map scope
        <select
          aria-label="Map scope"
          value={config.scope}
          onChange={(e) => onScope(e.target.value)}
        >
          {["world", "continent", "country", "city"].map((v) => (
            <option key={v}>{v}</option>
          ))}
        </select>
      </label>
      <label>
        Title
        <input
          value={config.title}
          onChange={(e) =>
            onChange({ ...config, title: e.target.value }, false)
          }
          onBlur={() => onChange(config, true)}
        />
      </label>
      <label>
        Visual language
        <select
          value={config.style}
          onChange={(e) => onChange({ ...config, style: e.target.value }, true)}
        >
          <option value="medieval">Illustrated Medieval</option>
          <option value="historical">Historical Atlas</option>
        </select>
      </label>
      <p>
        Drag labels and annotations. Linked borders and settlements follow Map
        Builder. Select a country or settlement below to focus its map.
      </p>
      <select
        aria-label="Focus geographical feature"
        value=""
        onChange={(e) => {
          const f = entities.find((x) => x.id === e.target.value);
          if (f)
            onScope(
              ["country", "state", "kingdom", "empire"].includes(f.kind)
                ? "country"
                : "city",
              f,
            );
        }}
      >
        <option value="">Open country or city map…</option>
        {entities
          .filter((e) =>
            [
              "country",
              "state",
              "kingdom",
              "empire",
              "capital",
              "city",
              "town",
              "village",
            ].includes(e.kind),
          )
          .map((e) => (
            <option key={e.id} value={e.id}>
              {e.name}
            </option>
          ))}
      </select>
      <button onClick={onAdd}>Add annotation</button>
      {objects.some((o) => o.hidden) && (
        <details>
          <summary>Hidden labels and objects</summary>
          {objects
            .filter((o) => o.hidden)
            .map((o) => (
              <div key={o.id}>
                <p>
                  {o.text || o.id}: {o.placementWarning || "Hidden manually"}
                </p>
                <button
                  disabled={config.layers.find((l) => l.id === o.layer)?.locked}
                  onClick={() => {
                    onOverride(
                      o,
                      { hidden: false, position: o.position },
                      true,
                    );
                    onSelect({ ...o, hidden: false });
                  }}
                >
                  Show for manual placement
                </button>
              </div>
            ))}
        </details>
      )}
      <details open>
        <summary>Illustration layers</summary>
        {[...config.layers]
          .sort((a, b) => a.order - b.order)
          .map((l) => (
            <div className="studio-layer" key={l.id}>
              <input
                type="checkbox"
                aria-label={`${l.name} illustration visibility`}
                checked={l.visible}
                onChange={(e) =>
                  onChange(
                    {
                      ...config,
                      layers: config.layers.map((x) =>
                        x.id === l.id ? { ...x, visible: e.target.checked } : x,
                      ),
                    },
                    true,
                  )
                }
              />
              <span>{l.name}</span>
              <button
                aria-label={`Raise illustration ${l.name}`}
                disabled={!l.order}
                onClick={() =>
                  onChange(
                    {
                      ...config,
                      layers: config.layers.map((x) =>
                        x.id === l.id
                          ? { ...x, order: x.order - 1 }
                          : x.order === l.order - 1
                            ? { ...x, order: x.order + 1 }
                            : x,
                      ),
                    },
                    true,
                  )
                }
              >
                ↑
              </button>
              <button
                aria-pressed={l.locked}
                onClick={() =>
                  onChange(
                    {
                      ...config,
                      layers: config.layers.map((x) =>
                        x.id === l.id ? { ...x, locked: !x.locked } : x,
                      ),
                    },
                    true,
                  )
                }
              >
                {l.locked ? "Locked" : "Lock"}
              </button>
              <input
                aria-label={`${l.name} opacity`}
                type="range"
                min="0"
                max="1"
                step=".1"
                value={l.opacity}
                onChange={(e) =>
                  onChange(
                    {
                      ...config,
                      layers: config.layers.map((x) =>
                        x.id === l.id ? { ...x, opacity: +e.target.value } : x,
                      ),
                    },
                    false,
                  )
                }
                onPointerUp={() => onChange(config, true)}
              />
            </div>
          ))}
      </details>
      {selected && (
        <fieldset
          disabled={config.layers.find((l) => l.id === selected.layer)?.locked}
        >
          <h3>Illustration object</h3>
          <p>
            {selected.source ? `Linked to ${selected.source}` : "Annotation"}
          </p>
          <label>
            Text
            <input
              value={selected.text || ""}
              onChange={(e) =>
                onOverride(selected, { text: e.target.value }, false)
              }
            />
          </label>
          <label>
            Ink
            <input
              type="color"
              value={selected.color || "#503d2a"}
              onChange={(e) =>
                onOverride(selected, { color: e.target.value }, false)
              }
            />
          </label>
          <label>
            Symbol
            <select
              value={selected.symbol || "city"}
              onChange={(e) =>
                onOverride(selected, { symbol: e.target.value }, false)
              }
            >
              {["capital", "city", "village", "mountain", "tree", "marsh"].map(
                (v) => (
                  <option key={v}>{v}</option>
                ),
              )}
            </select>
          </label>
          <label>
            Line style
            <select
              value={selected.dash || ""}
              onChange={(e) =>
                onOverride(selected, { dash: e.target.value }, false)
              }
            >
              <option value="">Solid</option>
              <option value="4 3">Dashed</option>
            </select>
          </label>
          <button onClick={() => onChange(config, true)}>
            Apply illustration properties
          </button>
          <button
            onClick={() => {
              onOverride(selected, { hidden: true }, true);
              onSelect(null);
            }}
          >
            Hide object
          </button>
          <p>
            Styles and label positions are saved separately from physical
            geography. Undo/redo uses the map history.
          </p>
        </fieldset>
      )}
    </fieldset>
  );
}
