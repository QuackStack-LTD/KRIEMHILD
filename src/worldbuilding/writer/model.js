export const nodeTypes = [
  "book",
  "part",
  "chapter",
  "scene",
  "frontMatter",
  "backMatter",
  "note",
  "research",
  "outline",
];
export const statuses = [
  "Idea",
  "Outlined",
  "Drafting",
  "Needs revision",
  "Final",
];
export const privateTypes = new Set(["note", "research", "outline"]);
export function textContent(node) {
  if (!node) return "";
  if (node.type === "text") return node.text || "";
  if (node.type === "horizontalRule") return "\n***\n";
  if (node.type === "hardBreak") return "\n";
  return (
    (node.content || []).map(textContent).join("") +
    (["paragraph", "heading", "codeBlock"].includes(node.type) ? "\n" : "")
  );
}
export function words(text = "") {
  return (
    text
      .replace(/\*\*\*/g, "")
      .match(/[\p{L}\p{N}]+(?:['’-][\p{L}\p{N}]+)*/gu) || []
  ).length;
}
export function plainDocument(text = "") {
  return {
    type: "doc",
    content: text.split(/\r?\n/).map((line) => ({
      type: "paragraph",
      ...(line ? { content: [{ type: "text", text: line }] } : {}),
    })),
  };
}
export function orderedNodes(nodes, parent = "", depth = 0, seen = new Set()) {
  return nodes
    .filter((n) => !n.deleted && n.parentId === parent && !seen.has(n.id))
    .sort((a, b) => a.position - b.position || a.id.localeCompare(b.id))
    .flatMap((n) => {
      seen.add(n.id);
      return [{ ...n, depth }, ...orderedNodes(nodes, n.id, depth + 1, seen)];
    });
}
export function manuscriptNodes(nodes, selected = "", includePrivate = false) {
  const all = orderedNodes(nodes),
    excluded = new Set();
  for (const n of all)
    if (privateTypes.has(n.type) || excluded.has(n.parentId))
      excluded.add(n.id);
  const descendants = new Set(selected ? [selected] : all.map((n) => n.id));
  if (selected)
    for (const n of all) if (descendants.has(n.parentId)) descendants.add(n.id);
  return all.filter(
    (n) => descendants.has(n.id) && (includePrivate || !excluded.has(n.id)),
  );
}
export function newNode(
  projectId,
  type = "scene",
  parentId = "",
  position = 0,
  age = "",
) {
  return {
    id: crypto.randomUUID(),
    projectId,
    parentId,
    type,
    position,
    title: `Untitled ${type}`,
    synopsis: "",
    status: "Idea",
    tags: "",
    content: plainDocument(),
    contentSchema: 1,
    text: "",
    notes: "",
    purpose: "",
    conflict: "",
    outcome: "",
    storyAgeId: age,
    storyYear: null,
    povEntityId: "",
    locationEntityId: "",
    targetWords: 0,
    references: [],
    revision: 0,
  };
}
export function continuity(node, doc, worldId) {
  if (!node) return [];
  const result = [],
    age = node.storyAgeId;
  const refs = [
    ...(node.references || []),
    ...[node.povEntityId, node.locationEntityId]
      .filter(Boolean)
      .map((entityId) => ({ entityId, ageId: age, worldId })),
  ];
  for (const ref of refs) {
    const e = doc.entities.find(
      (e) => e.id === ref.entityId && e.ageId === (age || ref.ageId),
    );
    let reason = "";
    if (ref.worldId !== worldId)
      reason = "Reference belongs to a different world.";
    else if (!e) reason = "Entity is missing from this scene’s story Age.";
    else if (
      e.kind === "character" &&
      ["dead", "deceased"].includes(
        String(e.properties?.status || "").toLowerCase(),
      )
    )
      reason = "Character is recorded as dead in this Age.";
    else if (
      node.storyYear != null &&
      e.properties?.startYear != null &&
      String(e.properties.startYear).trim() !== "" &&
      node.storyYear < Number(e.properties.startYear)
    )
      reason = "Scene precedes the recorded start year.";
    else if (
      node.storyYear != null &&
      e.properties?.endYear != null &&
      String(e.properties.endYear).trim() !== "" &&
      node.storyYear > Number(e.properties.endYear)
    )
      reason = "Scene follows the recorded end year.";
    if (reason && !result.some((x) => x.id === ref.entityId))
      result.push({
        id: ref.entityId,
        ageId: age || ref.ageId,
        name: e?.name || ref.wording || ref.entityId,
        reason,
      });
  }
  return result;
}
export async function request(path = "", body, method) {
  const r = await fetch("/api/writer" + path, {
    method: method || (body ? "POST" : "GET"),
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  const data = await r.json();
  if (!r.ok) {
    const e = Error(data.error || "Writing request failed");
    e.status = r.status;
    throw e;
  }
  return data;
}
export function download(name, data, type = "text/plain;charset=utf-8") {
  const url = URL.createObjectURL(new Blob([data], { type }));
  const a = document.createElement("a");
  a.href = url;
  a.download = name.replace(/[<>:"/\\|?*]/g, "-");
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export function inlineReferences(content) {
  const refs = [];
  function visit(n) {
    for (const mark of n.marks || []) {
      if (mark.type === "worldReference") {
        const r = { ...mark.attrs, type: "inline", wording: n.text || "" };
        if (
          !refs.some(
            (x) =>
              x.worldId === r.worldId &&
              x.entityId === r.entityId &&
              x.ageId === r.ageId,
          )
        )
          refs.push(r);
      }
    }
    for (const child of n.content || []) visit(child);
  }
  if (content) visit(content);
  return refs;
}
