import { registry } from "./model.js";
export default function EntityGraphIcon({ kind }) {
  const category = registry.types[kind]?.category;
  const paths = {
    settlements: "M4 29V14L16 3l12 11v15H4m9 0V19h6v10M2 14h28",
    politics: "M4 11l7 6 5-13 5 13 7-6-3 17H7L4 11m4 12h16",
    geography: "M2 28L12 6l7 12 4-7 8 17H2m6-13 4 3 3-3",
    organizations: "M3 11l13-8 13 8H3m3 4v10m7-10v10m7-10v10m7-10v10M2 29h28",
    culture:
      "M16 8C10 3 5 4 2 6v22c5-3 9-2 14 1 5-3 9-4 14-1V6c-5-2-9-3-14 2v21",
    records:
      kind === "archive"
        ? "M2 8h11l3 4h14v17H2V8m0 7h28"
        : "M7 2h13l7 7v21H7V2m13 0v8h7M11 16h12m-12 5h12",
    systems: "M16 2l4 10 11 4-11 4-4 11-4-11L1 16l11-4 4-10",
    biosphere: "M5 28C0 7 17 7 28 3c-1 18-8 26-23 25L23 8",
  };
  return (
    <svg width="32" height="32" viewBox="0 0 32 32" aria-hidden="true">
      <path
        d={paths[category] || paths.records}
        fill="none"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinejoin="round"
      />
    </svg>
  );
}
