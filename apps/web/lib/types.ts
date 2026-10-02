export type Field = {
  key: string;
  type: "text" | "number" | "boolean" | "entity";
};
export type Pin = { entity: string; x: number; y: number };
export type Doc = {
  type: string;
  content?: Doc[];
  text?: string;
  attrs?: Record<string, unknown>;
  marks?: { type: string }[];
};
export type Entry = {
  id: string;
  kind:
    | "entity"
    | "relation"
    | "map"
    | "scene"
    | "schema"
    | "note"
    | "calendar"
    | "chronology"
    | "event";
  name: string;
  notes?: string;
  type?: string;
  status?: string;
  properties?: Record<string, unknown>;
  fields?: Field[];
  from?: string;
  to?: string;
  asset?: string;
  width?: number;
  height?: number;
  pins?: Pin[];
  document?: Doc;
  story?: string;
  order?: number;
  settingAge?: string;
  settingSnapshot?: string;
  references?: string[];
  terrain?: Terrain;
  features?: Feature[];
  calendar?: Calendar;
  chronology?: Chronology;
  event?: WorldEvent;
  settingDate?: string;
};
export type Age = {
  id: string;
  name: string;
  snapshot: string;
  sourceAge?: string;
  sourceSnapshot?: string;
  sourceRevision?: string;
  undo: string[];
  redo: string[];
};
export type World = { id: string; name: string; version: number };
export type State = {
  role?: "owner" | "editor" | "viewer";
  readOnly?: boolean;
  revision: string;
  root: {
    world: World;
    ages: Record<string, Age>;
    message: string;
    savedAt: string;
  };
  age: Age;
  records: Record<string, Entry>;
  recovered: boolean;
};
export type Difference = {
  id: string;
  change: string;
  before: Entry | null;
  after: Entry | null;
  current?: Entry;
  conflict: boolean;
};
export type Command = {
  action: string;
  record?: Entry;
  id?: string;
  name?: string;
  ids?: string[];
  incoming?: string;
  records?: Entry[];
  event?: Entry;
  chronology?: Chronology;
};
export type Terrain = {
  algorithm: string;
  seed: string;
  columns: number;
  rows: number;
  sea: number;
  heights: number[];
  water: number[];
  flow: number[];
  wrap: boolean;
  unit: string;
};
export type Feature = {
  id: string;
  name: string;
  kind: "river" | "route" | "border" | "lake" | "climate" | "biome";
  points: { x: number; y: number }[];
  entity?: string;
  notes?: string;
};
export type Calendar = {
  epoch: string;
  era: string;
  months: { name: string; days: number }[];
  week: number;
  yearZero: boolean;
  leapEvery: number;
  leapDays: number;
};
export type Chronology = {
  age: string;
  baseline: string;
  start?: string;
  end?: string;
  branch: string;
  calendar?: string;
};
export type WorldEvent = {
  age: string;
  date: { precision: string; tick?: string; end?: string; relative?: string };
  until?: string;
  track?: string;
  causes?: string[];
  changes?: { target: string; before?: string; after: string }[];
};
export type HistoricalState = {
  records: Record<string, Entry>;
  unresolved: string[];
  tick: string;
  label: string;
};
export const newID = () => crypto.randomUUID();
export const emptyDoc: Doc = { type: "doc", content: [{ type: "paragraph" }] };
export const textOf = (doc?: Doc): string =>
  doc
    ? (doc.text ??
      (doc.content ?? []).map(textOf).join(doc.type === "doc" ? "\n" : ""))
    : "";
export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch("/api/v1" + path, {
    ...init,
    headers: {
      ...(init?.body instanceof Blob
        ? {}
        : { "Content-Type": "application/json" }),
      ...init?.headers,
    },
  });
  if (!response.ok) {
    const body = await response
      .json()
      .catch(() => ({ error: response.statusText }));
    throw new Error(body.error || "Request failed");
  }
  return response.json();
}
