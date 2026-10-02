"use client";
import type { Entry } from "../lib/types";
import type { useRecordDraft } from "../lib/useRecordDraft";

export default function DraftRecovery({
  draft,
  kind,
  records = {},
  readOnly = false,
}: {
  draft: ReturnType<typeof useRecordDraft>;
  kind: string;
  records?: Record<string, Entry>;
  readOnly?: boolean;
}) {
  function describe(value: unknown): string {
    if (typeof value === "string") return records[value]?.name || value;
    if (value && typeof value === "object" && !Array.isArray(value)) {
      const q = value as Record<string, unknown>;
      if (Array.isArray(q.inputs) && Array.isArray(q.outputs)) {
        const flows = (items: unknown[]) =>
          items
            .map((item) => {
              const flow = item as {
                resource: string;
                amount: string;
                unit: string;
              };
              return `${flow.amount} ${flow.unit} ${records[flow.resource]?.name || "Missing resource"}`;
            })
            .join(", ") || "None";
        return `${q.batches} batches per period; priority ${q.priority}\nInputs: ${flows(q.inputs)}\nOutputs: ${flows(q.outputs)}`;
      }
      if (q.mode === "unknown") return "Unknown";
      if (q.mode === "qualitative") return String(q.text || "Unspecified");
      if (q.mode === "exact") return `${q.value ?? ""} ${q.unit ?? ""}`.trim();
      if (q.mode === "range")
        return `${q.min ?? "?"}–${q.max ?? "?"} ${q.unit ?? ""}`.trim();
    }
    return JSON.stringify(value);
  }
  return (
    <>
      {draft.recovery && (
        <div className="notice">
          <p>Unsaved {kind} recovered from this browser.</p>
          {draft.stale && (
            <p>
              The saved version changed after this draft began. Review the
              recovered fields before saving.
            </p>
          )}
          <details>
            <summary>Review recovered {kind}</summary>
            <h4>{draft.recovery.value.name}</h4>
            {draft.recovery.value.from && (
              <p>
                From:{" "}
                {records[draft.recovery.value.from]?.name || "Missing entity"}
              </p>
            )}
            {draft.recovery.value.to && (
              <p>
                To: {records[draft.recovery.value.to]?.name || "Missing entity"}
              </p>
            )}
            {draft.recovery.value.fields?.map((field) => (
              <p key={field.key}>
                {field.key}: {field.type}
              </p>
            ))}
            {draft.recovery.value.notes && <p>{draft.recovery.value.notes}</p>}
            <dl>
              {Object.entries(draft.recovery.value.properties || {})
                .filter(([key]) => !key.startsWith("_"))
                .map(([key, value]) => (
                  <div key={key}>
                    <dt>{key.replace(/ID$/, "")}</dt>
                    <dd style={{ whiteSpace: "pre-wrap" }}>
                      {describe(value)}
                    </dd>
                  </div>
                ))}
            </dl>
          </details>
          <button type="button" disabled={readOnly} onClick={draft.restore}>
            Restore {kind} draft for review
          </button>
          <button type="button" onClick={draft.discard}>
            Discard recovered {kind} draft
          </button>
        </div>
      )}
      {draft.storageError && <p role="alert">{draft.storageError}</p>}
    </>
  );
}
