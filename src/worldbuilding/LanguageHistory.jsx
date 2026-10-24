import { useEffect, useState } from "react";
import { StructuredEditor, changesSchema } from "./LanguageEditors.jsx";
import { confirmAction } from "./ui/feedback.js";

export default function LanguageHistory({
  doc,
  age,
  definition,
  onChange,
  dirty,
  busy,
  onPreview,
  onApply,
  readOnly = false,
}) {
  const [preview, setPreview] = useState(null),
    key = JSON.stringify(definition);
  useEffect(() => setPreview(null), [key]);
  const h = definition.history;
  const displayName =
    doc.entities.find(
      (e) => e.id === definition.languageId && e.ageId === age.id,
    )?.name || definition.name;
  return (
    <section>
      <h3>Historical transformation</h3>
      <p>
        Pin an immutable source. Sound changes use this target's authored
        inventory, natural classes and phonotactic constraints. The source's
        lexical metadata is copied; target-only entries are retained. Applying
        appends a target revision.
      </p>
      <label>
        Historical source revision
        <select
          disabled={readOnly}
          value={
            h ? `${h.sourceAge}:${h.sourceLanguage}:${h.sourceRevision}` : ""
          }
          onChange={(e) => {
            if (!e.target.value) {
              onChange(null);
              return;
            }
            const [sourceAge, sourceLanguage, sourceRevision] =
              e.target.value.split(":");
            onChange({
              sourceAge,
              sourceLanguage,
              sourceRevision,
              changes: h?.changes || [],
              exceptions: h?.exceptions || [],
              notes: h?.notes || "",
            });
          }}
        >
          <option value="">No historical transformation</option>
          {doc.entities
            .filter((e) => e.kind === "language" && !e.deleted && e.language)
            .flatMap((e) =>
              e.language.revisions.map((r) => (
                <option
                  key={`${e.ageId}:${e.id}:${r.hash}`}
                  value={`${e.ageId}:${e.id}:${r.hash}`}
                >
                  {e.name} · {doc.ages.find((a) => a.id === e.ageId)?.name} ·{" "}
                  {r.hash.slice(0, 10)}
                </option>
              )),
            )}
        </select>
      </label>
      {h && (
        <>
          <StructuredEditor
            label="Changes"
            schema={changesSchema}
            value={h.changes}
            onChange={(v) => onChange({ ...h, changes: v })}
            context={{
              readOnly,
              segments: definition.phonology?.segments || [],
              classes: definition.phonology?.classes || [],
            }}
          />
          <StructuredEditor
            label="Historical exceptions"
            schema={{
              type: "array",
              item: {
                type: "object",
                fields: {
                  lexeme: { type: "text" },
                  ruleId: { type: "text" },
                  segments: { type: "tokens" },
                },
              },
              blank: () => ({
                lexeme: "",
                ruleId: crypto.randomUUID(),
                segments: [],
              }),
            }}
            value={h.exceptions}
            context={{ readOnly }}
            onChange={(v) => onChange({ ...h, exceptions: v })}
          />
          <label>
            Historical notes
            <textarea
              disabled={readOnly}
              value={h.notes}
              onChange={(e) => onChange({ ...h, notes: e.target.value })}
            />
          </label>
          <p>
            {dirty
              ? "Save this target definition before previewing."
              : `Previewing does not change ${age.name} or its source language.`}
          </p>
          <button
            disabled={dirty || busy}
            onClick={async () => {
              const result = await onPreview();
              if (result) setPreview(result);
            }}
          >
            Preview historical changes
          </button>
          {preview && (
            <div className="language-results">
              <h4>{preview.status}</h4>
              {preview.diagnostics.map((d, i) => (
                <p key={i}>
                  {d.code}: {d.message}
                </p>
              ))}
              <p>
                {preview.changes.length} lexical forms examined · source{" "}
                {preview.sourceRevision.slice(0, 10)} · target{" "}
                {preview.targetRevision.slice(0, 10)}
              </p>
              {preview.changes.map((c) => (
                <details key={c.lexeme}>
                  <summary>
                    {c.lexeme}: {c.before.join(" ")} → {c.after.join(" ")}
                  </summary>
                  <ol>
                    {c.trace.map((s, i) => (
                      <li key={i}>
                        {s.stage} · {s.ruleId}: {s.before.join(" ")} →{" "}
                        {s.after.join(" ")}
                      </li>
                    ))}
                  </ol>
                </details>
              ))}
              <button
                disabled={
                  readOnly || dirty || busy || preview.status !== "success"
                }
                onClick={async () => {
                  if (
                    await confirmAction(
                      `Apply these ${preview.changes.length} reviewed lexical changes to ${displayName} in ${age.name}? The source stays unchanged.`,
                      { accept: "Apply historical changes" },
                    )
                  ) {
                    await onApply(preview.previewHash);
                    setPreview(null);
                  }
                }}
              >
                Apply reviewed historical changes
              </button>
            </div>
          )}
        </>
      )}
    </section>
  );
}
