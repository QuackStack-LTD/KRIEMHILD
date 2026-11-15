// Name-only responses leave large, already loaded catalogues and unchanged
// record objects in place. Never apply an update across an unknown revision gap.
export function applyWorldNameChange(document, change) {
  if (
    !document ||
    !change?.world ||
    document.world.id !== change.world.id ||
    document.world.revision < change.base ||
    document.world.revision > change.world.revision
  )
    return null;
  const merge = (records, changes, project) => {
    if (!records || !changes?.length) return records;
    const byID = new Map(changes.map((r) => [`${r.ageId}/${r.id}`, r]));
    let changed = false;
    const next = records.map((record) => {
      const value = byID.get(`${record.ageId}/${record.id}`);
      if (!value) return record;
      changed = true;
      return project(record, value);
    });
    return changed ? next : records;
  };
  return {
    ...document,
    world: { ...change.world, currentAge: document.world.currentAge },
    entities: merge(document.entities, change.entities, (old, value) => ({
      ...old,
      name: value.name,
      names: value.names,
      updatedAt: value.updatedAt,
    })),
    representations: merge(
      document.representations,
      change.representations,
      (old, value) => ({
        ...old,
        shape: {
          ...old.shape,
          name: value.shape.name,
          labelName: value.shape.labelName,
        },
      }),
    ),
    maps: merge(document.maps, change.maps, (old, value) => ({
      ...old,
      name: value.name,
    })),
    terrains: merge(document.terrains, change.terrains, (old, value) => ({
      ...old,
      name: value.name,
    })),
  };
}

// Keep genuine profile drafts while rebasing server-owned name metadata. A
// clean rename must not manufacture an unsaved-changes navigation warning.
export function rebaseProfileName(draft, previous, next) {
  if (
    JSON.stringify(draft) === JSON.stringify(previous) ||
    JSON.stringify(draft) === JSON.stringify(next)
  )
    return next;
  const names =
    JSON.stringify(draft.names) === JSON.stringify(previous.names)
      ? next.names
      : draft.names?.map((n) =>
          n.type === "canonical" && n.name === previous.name
            ? { ...n, name: next.name }
            : n,
        );
  return {
    ...draft,
    name: draft.name === previous.name ? next.name : draft.name,
    updatedAt: next.updatedAt,
    names,
  };
}
