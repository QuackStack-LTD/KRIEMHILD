export function generationSpec(spec) {
  return { ...spec, seed: spec.seed === "" ? crypto.randomUUID() : spec.seed };
}
