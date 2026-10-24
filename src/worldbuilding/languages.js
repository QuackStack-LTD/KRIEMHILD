export const ENGINE = "languages/1";
export const ADAPTER = "token-relation/1";
export function definitionVersions(d) {
  return d?.schema === 2
    ? {
        engine: "languages/2",
        adapter: "finite-compose/1",
        compiler: "languages-compiler/2",
      }
    : { engine: ENGINE, adapter: ADAPTER };
}
export function matchingCompile(c, hash, d) {
  const versions = definitionVersions(d);
  return (
    c.revision === hash &&
    c.engine === versions.engine &&
    c.adapter === versions.adapter &&
    (c.compiler || "") === (versions.compiler || "")
  );
}
export const emptyProductive = () => ({
  version: "productive/1",
  features: [],
  paradigms: [],
  stratum: { id: "", boundaryPolicy: "retain-until-surface", rules: [] },
});
export const sections = [
  "Overview",
  "Phonology",
  "Lexicon",
  "Morphology",
  "Syntax",
  "Scripts",
  "History",
  "Laboratory",
];
export const newID = () => crypto.randomUUID().replaceAll("-", "");
// Age selection is client-local in the existing shell. A save response carries
// the server's default Age; it must not move an editor away from its selection.
export function retainLanguageAge(document, ageId) {
  return document.ages.some((a) => a.id === ageId)
    ? { ...document, world: { ...document.world, currentAge: ageId } }
    : document;
}
export const blankDefinition = (name, id = newID()) => ({
  schema: 1,
  languageId: id,
  name,
  notes: "",
  phonology: null,
  lexicon: [],
  morphology: null,
  syntax: null,
  orthography: null,
  history: null,
  tests: [],
});
export const blankScript = (name, id = newID()) => ({
  schema: 1,
  scriptId: id,
  name,
  notes: "",
  direction: "undecided",
  graphemes: [],
});
export const emptyPhonology = () => ({
  features: [],
  segments: [],
  classes: [],
  templates: [],
  allowed: [],
  forbidden: [],
  rules: [],
});
export const emptyMorphology = () => ({ features: [], paradigms: [] });
export const emptySyntax = () => ({
  types: [],
  structures: [],
  constructions: [],
});
export const metadataFor = (record) => {
  const state = record?.language || record?.script;
  return {
    name: record?.name || "",
    kind: state?.kind || "",
    status: state?.status || "",
    description: state?.description || "",
    links: structuredClone(state?.links || []),
  };
};
export const stateMetadata = ({
  kind = "",
  status = "",
  description = "",
  links = [],
} = {}) => ({ kind, status, description, links });
export function definitionStats(d) {
  return {
    segments: d?.phonology?.segments?.length || 0,
    lexemes: d?.lexicon?.length || 0,
    paradigms: (d?.productive || d?.morphology)?.paradigms?.length || 0,
    constructions: d?.syntax?.constructions?.length || 0,
  };
}
export async function languageRequest(world, path, body, signal) {
  const response = await fetch(`/api/worlds/${world}/languages${path}`, {
    method: body === undefined ? "GET" : "POST",
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal,
  });
  const result = await response.json();
  if (!response.ok) {
    const error = new Error(result.error || "Language request failed");
    error.status = response.status;
    throw error;
  }
  return result;
}
export function diffDefinitions(before, after, path = "definition") {
  if (JSON.stringify(before) === JSON.stringify(after)) return [];
  if (
    before &&
    after &&
    !Array.isArray(before) &&
    !Array.isArray(after) &&
    typeof before === "object" &&
    typeof after === "object"
  )
    return [...new Set([...Object.keys(before), ...Object.keys(after)])]
      .sort()
      .flatMap((key) =>
        diffDefinitions(before[key], after[key], `${path}.${key}`),
      );
  return [{ path, before, after }];
}
