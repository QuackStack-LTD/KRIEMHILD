export const ROOT = "root";
export const INWORLD = "inworld";
export const isFolder = (n) =>
  ["folder", "book", "part", "chapter"].includes(n.type);
export const baseName = (name) =>
  name.replace(/\.(docx|pdf|txt|md|markdown)$/i, "");
export const fileURL = (n, version = "") =>
  `/api/writer/projects/${encodeURIComponent(n.projectId)}/files/${encodeURIComponent(n.id)}${version ? "?version=" + version : ""}`;
export const libraryURL = (world, f) =>
  `/api/worlds/${world}/library/${f.id}?age=${f.ageId}`;
export const imageURL = (world, hash) =>
  `/api/worlds/${world}/writer-archive/files/${hash}`;
export function archiveItems(data, world) {
  const rootId = "archive-" + world,
    projects = new Map(data.projects.map((p) => [p.id, p]));
  return [
    ...data.projects
      .filter((p) => p.id !== rootId)
      .map((p) => ({
        ...p,
        key: "project/" + p.id,
        parent: ROOT,
        title: p.name,
        type: "folder",
        project: true,
        projectId: p.id,
      })),
    ...data.nodes.map((n) => ({
      ...n,
      key: "node/" + n.id,
      parent: n.parentId
        ? "node/" + n.parentId
        : n.projectId === rootId
          ? ROOT
          : "project/" + n.projectId,
      projectDeleted: !!projects.get(n.projectId)?.deleted,
    })),
  ];
}
export function inworldItems(doc) {
  if (!doc) return [];
  const key = (age, id) => `inworld/${age}/${id}`;
  const entities = (doc.entities || []).filter((e) => !e.deleted),
    byKey = new Map(entities.map((e) => [key(e.ageId, e.id), e]));
  const files = (doc.libraryFiles || []).filter(
    (f) => f.versions?.length && byKey.has(key(f.ageId, f.id)),
  );
  const fileIds = new Set(files.map((f) => key(f.ageId, f.id)));
  const archives = entities.filter((e) => e.kind === "archive");
  const resourceFolders = archives
    .filter((e) => !e.properties?.resourceFolder)
    .map((a) => {
      const saved = archives.find(
        (e) =>
          e.ageId === a.ageId &&
          e.properties?.parentArchive === a.id &&
          e.properties?.resourceFolder === "resources",
      );
      return {
        owner: a,
        saved,
        key: saved ? key(a.ageId, saved.id) : key(a.ageId, `resources/${a.id}`),
      };
    });
  const resourceByKey = new Map(resourceFolders.map((r) => [r.key, r]));
  return [
    {
      key: INWORLD,
      parent: ROOT,
      title: "Inworld Archive",
      type: "folder",
      inworld: true,
      description: "Linked to the Library in every Age",
    },
    ...(doc.ages || []).map((a) => ({
      key: key(a.id, ""),
      parent: INWORLD,
      title: a.name,
      type: "folder",
      inworld: true,
      ageId: a.id,
    })),
    ...resourceFolders.flatMap((r) => [
      ...(!r.saved
        ? [
            {
              key: r.key,
              parent: key(r.owner.ageId, r.owner.id),
              title: "Resources",
              type: "folder",
              inworld: true,
              ageId: r.owner.ageId,
              id: `resources/${r.owner.id}`,
              resourceArchiveId: r.owner.id,
            },
          ]
        : []),
      {
        key: r.key + "/review",
        parent: r.key,
        title: "Review Terrain Resources",
        type: "resource-review",
        inworld: true,
        ageId: r.owner.ageId,
        id: r.owner.id,
        resourceArchiveId: r.owner.id,
        resourceReview: true,
        description: "Saved areas, current knowledge and historical charts",
      },
      {
        key: r.key + "/documents",
        parent: r.key,
        title: "Resource documents",
        type: "folder",
        inworld: true,
        ageId: r.owner.ageId,
        id: r.saved?.id || "",
        resourceArchiveId: r.owner.id,
        description: "Reports organized by Age and year",
      },
    ]),
    ...entities.map((e) => {
      const parentArchive =
        e.kind === "archive"
          ? e.properties?.parentArchive
          : files.find((f) => f.ageId === e.ageId && f.id === e.id)?.archiveId;
      let parent =
        parentArchive && byKey.has(key(e.ageId, parentArchive))
          ? key(e.ageId, parentArchive)
          : key(e.ageId, "");
      if (resourceByKey.has(parent)) parent += "/documents";
      return {
        key: key(e.ageId, e.id),
        parent,
        title: e.name,
        type: e.kind === "archive" ? "folder" : "record",
        inworld: true,
        ageId: e.ageId,
        id: e.id,
        entity: e,
        file: files.find((f) => f.ageId === e.ageId && f.id === e.id),
        hasFile: fileIds.has(key(e.ageId, e.id)),
      };
    }),
  ];
}
export function descendants(items, key) {
  const ids = new Set([key]);
  for (let changed = true; changed; ) {
    changed = false;
    for (const item of items)
      if (ids.has(item.parent) && !ids.has(item.key)) {
        ids.add(item.key);
        changed = true;
      }
  }
  return ids;
}
export function crumbs(items, folder) {
  const out = [],
    seen = new Set(),
    byId = new Map(items.map((n) => [n.key, n]));
  while (folder && folder !== ROOT && !seen.has(folder)) {
    seen.add(folder);
    const n = byId.get(folder);
    if (!n) break;
    out.unshift(n);
    folder = n.parent;
  }
  return out;
}
export const sizeLabel = (bytes) =>
  bytes == null
    ? "—"
    : bytes < 1024
      ? `${bytes} B`
      : bytes < 1024 * 1024
        ? `${(bytes / 1024).toFixed(1)} KB`
        : `${(bytes / 1024 / 1024).toFixed(1)} MB`;
export function kindLabel(n) {
  if (n.resourceReview) return "Resource maps & charts";
  if (isFolder(n)) return n.inworld ? "Linked folder" : "Folder";
  const mime =
    n.mime || n.mimeType || n.MIME || n.file?.versions?.at(-1)?.mime || "";
  if (mime.startsWith("image/")) return "Image";
  if (n.type === "asset")
    return (n.fileName?.split(".").pop() || "File").toUpperCase();
  return (
    n.format?.toUpperCase() || (n.inworld ? "Library document" : "Document")
  );
}
export async function archiveRequest(world, path = "", body) {
  const r = await fetch(`/api/worlds/${world}/writer-archive${path}`, {
    method: body === undefined ? "GET" : "POST",
    headers:
      body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const value = await r.json();
  if (!r.ok) throw Error(value.error || "Archive request failed");
  return value;
}
export async function uploadArchiveFile(node, bytes, name) {
  const form = new FormData();
  form.append("metadata", JSON.stringify(node));
  form.append("file", new Blob([bytes]), name);
  const r = await fetch(`/api/writer/projects/${node.projectId}/files`, {
    method: "POST",
    body: form,
  });
  const value = await r.json();
  if (!r.ok) throw Error(value.error || "Unable to import file");
  return value;
}
