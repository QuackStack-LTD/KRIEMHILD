import Dialog from "./ui/Dialog.jsx";
import { useState } from "react";
import {
  activeEntities,
  characterRelationKinds,
  familyConflict,
  label,
  parentPair,
} from "./model.js";

export default function CharacterRelationEditor({
  doc,
  age,
  source,
  relation,
  genealogy,
  mixed = false,
  command,
  onClose,
}) {
  const initial = relation || {
    source,
    destination: "",
    kind: genealogy ? "child-of" : "friend",
  };
  const [value, setValue] = useState(initial),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const characters = activeEntities(doc, age).filter(
    (e) => e.kind === "character",
  );
  const parentRecords = (child) =>
    doc.relations.filter(
      (r) => r.ageId === age && parentPair(r)?.[1] === child,
    );
  const initialParents = (child) => [
    ...new Set(
      [
        initial.kind === "child-of" ? initial.destination : "",
        ...parentRecords(child).map((r) => parentPair(r)[0]),
      ].filter(Boolean),
    ),
  ];
  const [parents, setParents] = useState(initialParents(initial.source));
  const field = (k, v) => {
    setError("");
    setValue((x) => ({ ...x, [k]: v }));
    if (k === "source")
      setParents(parentRecords(v).map((r) => parentPair(r)[0]));
    if (k === "kind" && v === "child-of")
      setParents([
        ...new Set(
          [
            value.destination,
            ...parentRecords(value.source).map((r) => parentPair(r)[0]),
          ].filter(Boolean),
        ),
      ]);
  };
  const choices = (exclude) =>
    characters
      .filter((e) => e.id !== exclude)
      .map((e) => (
        <option key={e.id} value={e.id}>
          {e.name}
        </option>
      ));
  async function save(event) {
    event.preventDefault();
    setError("");
    let changes = [value],
      remove = [];
    if (value.kind === "child-of") {
      const ids = parents.filter(Boolean);
      if (!ids.length) {
        setError("Select at least one parent.");
        return;
      }
      if (ids.length > 2 || new Set(ids).size !== ids.length) {
        setError("Choose one or two different parents.");
        return;
      }
      const old = parentRecords(value.source);
      remove = old.map((r) => r.id);
      if (value.id && !remove.includes(value.id)) remove.push(value.id);
      changes = ids.map((id, index) => {
        const existing = old.find((r) => parentPair(r)[0] === id);
        if (existing && existing.kind !== "child-of")
          return {
            ...existing,
            description: value.description || "",
            notes: value.notes || "",
          };
        return {
          ...value,
          id: existing?.id,
          destination: id,
          sourcePort: index === 0 ? value.sourcePort : undefined,
          destinationPort: index === 0 ? value.destinationPort : undefined,
        };
      });
    }
    const trial = {
      ...doc,
      relations: doc.relations.filter(
        (r) => !remove.includes(r.id) || r.ageId !== age,
      ),
    };
    for (const r of changes) {
      if (!r.source || !r.destination) {
        setError("Choose both characters.");
        return;
      }
      if (genealogy || mixed) {
        const problem = familyConflict(trial, age, r);
        if (problem) {
          setError(problem);
          return;
        }
      }
      trial.relations = [
        ...trial.relations.filter((x) => x.id !== r.id || x.ageId !== age),
        { ...r, ageId: age },
      ];
    }
    setBusy(true);
    try {
      const d = await command({
        kind: "relations",
        ageId: age,
        relations: changes,
        removeRelationIds: remove,
      });
      if (d) onClose();
      else setError("Could not save. Check the page error and try again.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <Dialog
      title={<>{genealogy ? "Family connection" : "Character relationship"}</>}
      size="form"
      className="relation-dialog"
      onClose={onClose}
      busy={busy}
      onSubmit={save}
      actions={
        <>
          <button
            type="button"
            disabled={busy}
            onClick={onClose}
            data-dialog-dismiss
          >
            Cancel
          </button>
          {!value.automatic && (
            <>
              <button className="primary" disabled={busy}>
                Save connection
              </button>
              {relation?.id && (
                <button
                  type="button"
                  className="danger"
                  disabled={busy}
                  onClick={async () => {
                    setBusy(true);
                    try {
                      if (
                        await command({
                          kind: "delete-relation",
                          ageId: age,
                          edgeId: relation.id,
                        })
                      )
                        onClose();
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  Delete connection
                </button>
              )}
            </>
          )}
        </>
      }
    >
      <span className="eyebrow">
        {doc.ages.find((a) => a.id === age)?.name}
      </span>
      <p>The connection is saved when you confirm its type and description.</p>
      <fieldset disabled={busy || value.automatic}>
        <label>
          {value.kind === "child-of" ? "Child" : "From character"}
          <select
            required
            value={value.source || ""}
            onChange={(e) => field("source", e.target.value)}
          >
            <option value="">Choose a character</option>
            {choices("")}
          </select>
        </label>
        <label>
          Relationship type
          <select
            value={value.kind}
            onChange={(e) => field("kind", e.target.value)}
          >
            {value.automatic && (
              <option value={value.kind}>{label(value.kind)}</option>
            )}
            {(mixed
              ? [
                  ...characterRelationKinds(true),
                  ...characterRelationKinds(false),
                ]
              : characterRelationKinds(genealogy)
            ).map((k) => (
              <option key={k} value={k}>
                {k === "child-of" ? "Child of (choose parents)" : label(k)}
              </option>
            ))}
          </select>
        </label>
        {value.kind === "child-of" ? (
          <div className="field-pair">
            {[0, 1].map((i) => (
              <label key={i}>
                Parent {i + 1}
                {i === 1 ? " (optional)" : ""}
                <select
                  value={parents[i] || ""}
                  onChange={(e) => {
                    setError("");
                    setParents((p) => {
                      const next = [...p];
                      next[i] = e.target.value;
                      return next;
                    });
                  }}
                >
                  <option value="">Not specified</option>
                  {choices(value.source)}
                </select>
              </label>
            ))}
          </div>
        ) : (
          <label>
            To character
            <select
              required
              value={value.destination || ""}
              onChange={(e) => field("destination", e.target.value)}
            >
              <option value="">Choose a character</option>
              {choices(value.source)}
            </select>
          </label>
        )}
        <label>
          Description
          <textarea
            rows={3}
            value={value.description || ""}
            onChange={(e) => field("description", e.target.value)}
            placeholder={
              genealogy
                ? "Describe their family connection"
                : "Describe this relationship"
            }
          />
        </label>
        <label>
          Notes
          <textarea
            rows={2}
            value={value.notes || ""}
            onChange={(e) => field("notes", e.target.value)}
          />
        </label>
        <div className="field-pair">
          <label>
            Valid from
            <input
              type="text"
              value={value.validFrom || ""}
              onChange={(e) => field("validFrom", e.target.value)}
            />
          </label>
          <label>
            Valid to
            <input
              type="text"
              value={value.validTo || ""}
              onChange={(e) => field("validTo", e.target.value)}
            />
          </label>
        </div>
        <label>
          Context
          <select
            value={value.contextId || ""}
            onChange={(e) => field("contextId", e.target.value)}
          >
            <option value="">No additional context</option>
            {activeEntities(doc, age).map((e) => (
              <option key={e.id} value={e.id}>
                {e.name}
              </option>
            ))}
          </select>
        </label>
      </fieldset>
      {value.automatic && (
        <p>This relationship is derived from its source records.</p>
      )}
      {error && <p role="alert">{error}</p>}
    </Dialog>
  );
}
