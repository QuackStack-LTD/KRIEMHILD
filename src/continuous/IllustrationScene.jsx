import { useRef } from "react";
import { anchor } from "./geometry.js";
import { riverWidth } from "./rivers.js";
import { useMapProjection } from "./MapProjection.js";
export default function IllustrationScene({
  config,
  objects,
  selected,
  onSelect,
  onOverride,
  position,
  state,
  pixel: sourcePixel,
  relief,
  coastOverlay,
  riverMask,
  landMask,
}) {
  const drag = useRef(null),
    projection = useMapProjection(),
    project = projection?.point || ((p) => p),
    scale = projection?.scale || 1;
  const worldBounds = {
    x: config.bounds[0],
    y: config.bounds[1],
    w: config.bounds[2],
    h: config.bounds[3],
  };
  const { x, y, w, h } = projection?.box(worldBounds) || worldBounds,
    pixel = sourcePixel * scale;
  function handlers(o, vertex) {
    return {
      onPointerDown: (e) => {
        e.stopPropagation();
        onSelect(o);
        if (
          config.layers.find((l) => l.id === o.layer)?.locked ||
          (o.source && o.kind !== "text")
        )
          return;
        e.currentTarget.setPointerCapture(e.pointerId);
        drag.current = { o, vertex, start: position(e) };
      },
      onPointerMove: (e) => {
        if (!drag.current) return;
        e.stopPropagation();
        update(e, false);
      },
      onPointerUp: (e) => {
        if (!drag.current) return;
        e.stopPropagation();
        update(e, true);
        drag.current = null;
      },
    };
  }
  function update(e, commit) {
    const { o, vertex, start } = drag.current,
      p = position(e),
      dx = p[0] - start[0],
      dy = p[1] - start[1];
    const override =
      vertex != null
        ? { points: o.points.map((q, i) => (i === vertex ? p : q)) }
        : o.kind === "text" || o.kind === "symbol"
          ? { position: [o.position[0] + dx, o.position[1] + dy] }
          : { points: o.points.map((q) => [q[0] + dx, q[1] + dy]) };
    onOverride(o, override, commit);
  }
  function draw(source) {
    const o = {
      ...source,
      points: source.points?.map(project),
      position: source.position ? project(source.position) : undefined,
      leaderFrom: source.leaderFrom ? project(source.leaderFrom) : undefined,
      size: source.size ? source.size * scale : undefined,
    };
    const color = o.color || "#503d2a",
      p = o.position || anchor(o.points || [[x, y]], o.geometry),
      size = o.size || pixel * (o.symbol === "capital" ? 7 : 5),
      shared = {
        ...handlers(source),
        stroke: color,
        strokeWidth: pixel * 1.2,
        fill: "none",
      };
    if (o.kind === "text")
      return (
        <g>
          {o.leaderFrom && (
            <line
              x1={o.leaderFrom[0]}
              y1={o.leaderFrom[1]}
              x2={p[0]}
              y2={p[1]}
              stroke={color}
              strokeWidth={pixel * 0.7}
              pointerEvents="none"
            />
          )}
          <text
            {...shared}
            x={p[0]}
            y={p[1]}
            fill={color}
            fontFamily="Georgia,serif"
            fontSize={o.size || pixel * 13}
            textAnchor="middle"
            dominantBaseline="middle"
            style={{ cursor: "move" }}
            paintOrder="stroke"
            stroke="#e9d9af"
            strokeWidth={pixel * 2.5}
            strokeLinejoin="round"
          >
            {o.text}
          </text>
        </g>
      );
    if (o.kind === "symbol")
      return (
        <g
          {...shared}
          transform={`translate(${p}) scale(${size})`}
          strokeWidth={0.16}
          fill="#e8d7b0"
        >
          {o.symbol === "mountain" ? (
            <>
              <path d="M-1,1 L-.15,-1.3 L1.2,1 M-.15,-1.3 L.1,.25 L.7,1" />
              <path d="M-.1,-.7 L-.5,.1 L.1,.25" />
            </>
          ) : o.symbol === "tree" ? (
            <>
              <path d="M0,1 V-.2 M-1,.4 L-.5,-.3 L-.8,-.3 L0,-1.5 L.8,-.3 L.5,-.3 L1,.4 Z" />
            </>
          ) : o.symbol === "marsh" ? (
            <path d="M-1,.6 H1 M-.6,.6 V-.5 M0,.6 V-.9 M.6,.6 V-.3" />
          ) : o.symbol === "capital" ? (
            <>
              <path d="M-1,1 V-1 H-.5 V-.5 H.1 V-1 H.8 V1 Z" />
              <path d="M-.2,1 V.1 H.3 V1" />
            </>
          ) : (
            <>
              <path d="M-1,0 L0,-1 L1,0 M-.7,-.2 V1 H.7 V-.2" />
              <path d="M-.2,1 V.3 H.2 V1" />
            </>
          )}
        </g>
      );
    const points = o.points.map((p) => p.join(",")).join(" ");
    return o.kind === "polygon" ? (
      <polygon
        {...shared}
        points={points}
        fill={o.layer === "countries" ? "#ac815418" : "#bea07650"}
        strokeDasharray={
          o.dash
            ? o.dash
                .split(" ")
                .map((v) => +v * pixel)
                .join(" ")
            : undefined
        }
      />
    ) : (
      <polyline
        {...shared}
        points={points}
        stroke={color}
        strokeWidth={
          o.river
            ? riverWidth(o.river, sourcePixel, state.continuous.spec.radiusKm) *
              scale
            : pixel * 1.2
        }
        strokeDasharray={
          (o.dash
            ? o.dash
                .split(" ")
                .map((v) => +v * pixel)
                .join(" ")
            : null) ||
          (o.layer === "routes" ? `${pixel * 4} ${pixel * 2}` : undefined)
        }
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    );
  }
  return (
    <g aria-label="Editable medieval illustration">
      <defs>
        <filter id="paper-grain">
          <feTurbulence
            type="fractalNoise"
            baseFrequency={0.7 / scale}
            numOctaves="3"
            seed="19"
          />
          <feColorMatrix type="saturate" values="0" />
          <feComponentTransfer>
            <feFuncA type="linear" slope=".08" />
          </feComponentTransfer>
        </filter>
      </defs>
      {[...config.layers]
        .sort((a, b) => a.order - b.order)
        .filter((l) => l.visible)
        .map((l) => (
          <g
            key={l.id}
            data-illustration-layer={l.id}
            opacity={l.opacity}
            mask={
              l.id === "waterways"
                ? riverMask
                : !["labels", "paper", "decoration", "annotations"].includes(
                      l.id,
                    )
                  ? landMask
                  : undefined
            }
          >
            {l.id === "relief" && relief}
            {l.id === "coasts" && coastOverlay}
            {l.id === "paper" && (
              <g pointerEvents="none">
                <rect
                  x={x}
                  y={y}
                  width={w}
                  height={h}
                  fill="#e9d9af"
                  opacity=".5"
                />
                <rect
                  x={x}
                  y={y}
                  width={w}
                  height={h}
                  filter="url(#paper-grain)"
                  opacity=".35"
                />
              </g>
            )}
            {objects
              .filter(
                (o) =>
                  o.layer === l.id &&
                  !o.hidden &&
                  !(l.id === "coasts" && coastOverlay),
              )
              .map((o) => (
                <g
                  key={o.id}
                  data-source={o.source}
                  data-illustration-id={o.id}
                >
                  {draw(o)}
                </g>
              ))}
            {l.id === "decoration" && (
              <g
                pointerEvents="none"
                fill="#503d2a"
                stroke="#503d2a"
                fontFamily="Georgia,serif"
              >
                <text
                  x={x + w / 2}
                  y={y + h * 0.075}
                  textAnchor="middle"
                  stroke="none"
                  fontSize={w / 52}
                >
                  {config.title}
                </text>
                <g
                  transform={`translate(${x + w * 0.94},${y + h * 0.15}) scale(${w * 0.025})`}
                  strokeWidth=".04"
                >
                  <path
                    d="M0,-1 L.2,-.15 L1,0 L.2,.15 L0,1 L-.2,.15 L-1,0 L-.2,-.15 Z"
                    fill="#e9d9af"
                  />
                  <path d="M0,-1 V1 M-1,0 H1" />
                  <text
                    y="-1.15"
                    textAnchor="middle"
                    stroke="none"
                    fontSize=".45"
                  >
                    N
                  </text>
                </g>
              </g>
            )}
          </g>
        ))}
      {selected && objects.find((o) => o.id === selected.id) && (
        <g
          aria-label="Illustration selection"
          stroke="#fff1a6"
          fill="none"
          strokeWidth={pixel * 2}
          pointerEvents="none"
        >
          {selected.points?.length ? (
            <polyline
              points={selected.points
                .map(project)
                .map((p) => p.join(","))
                .join(" ")}
            />
          ) : (
            <circle
              cx={selected.position && project(selected.position)[0]}
              cy={selected.position && project(selected.position)[1]}
              r={pixel * 12}
            />
          )}
        </g>
      )}
      {!selected?.source &&
        selected?.points?.map((p, i) => (
          <circle
            key={i}
            data-editor-handle="illustration-vertex"
            {...handlers(selected, i)}
            cx={project(p)[0]}
            cy={project(p)[1]}
            r={pixel * 4}
            fill="#fff1a6"
            stroke="#3e3022"
            strokeWidth={pixel}
          />
        ))}
    </g>
  );
}
