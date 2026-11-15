export function generationSpec(spec) {
  return {
    ...spec,
    climateVersion: spec.climateVersion ?? 0,
    seed: spec.seed === "" ? crypto.randomUUID() : spec.seed,
  };
}
