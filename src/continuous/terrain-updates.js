// Preserve the identity of unchanged drawables and remove server tombstones.
export function mergeTerrainEntities(current, changes) {
  if (!changes?.length) return current;
  const pending = new Map(changes.map((e) => [e.id, e]));
  const next = [];
  for (const entity of current) {
    const change = pending.get(entity.id);
    if (!change) next.push(entity);
    else if (!change.deleted) next.push(change);
    pending.delete(entity.id);
  }
  for (const entity of pending.values()) if (!entity.deleted) next.push(entity);
  return next;
}

// Metadata and drawable-only revisions do not change raster pixels. Requests
// still carry the latest server revision when the camera subsequently moves.
export function terrainRenderRevision(revision, invalidations = []) {
  let current = revision;
  for (let i = invalidations.length - 1; i >= 0; i--) {
    const change = invalidations[i];
    if (change.revision > current) continue;
    if (change.revision !== current || change.raster !== false) break;
    current--;
  }
  return current;
}

export function patchRiverNames(rivers, names) {
  if (!names) return rivers;
  const bySource = new Map(
    names.filter((n) => n.sourceId).map((n) => [n.sourceId, n]),
  );
  let changed = false;
  const next = rivers.map((r) => {
    const n = bySource.get(r.id),
      name = n?.name || "",
      entityId = n?.logicalId || "";
    if ((r.name || "") === name && (r.entityId || "") === entityId) return r;
    changed = true;
    return { ...r, name, entityId };
  });
  return changed ? next : rivers;
}

export function patchTerrainNames(entities, names) {
  const byID = new Map(names.map((n) => [n.id, n]));
  let changed = false;
  const next = entities.map((e) => {
    const n = byID.get(e.id);
    if (
      !n ||
      e.recordStatus === "draft" ||
      (e.name === n.name && e.labelName === n.labelName)
    )
      return e;
    changed = true;
    return { ...e, name: n.name, labelName: n.labelName };
  });
  return changed ? next : entities;
}
