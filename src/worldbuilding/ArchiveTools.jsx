import { useState } from "react";
import Dialog from "./ui/Dialog.jsx";
import {
  archiveCategories,
  archiveDefaults,
  archivePurposes,
  archiveRequest,
} from "./archives.js";
import { label, registry } from "./model.js";

export function ArchiveOwnerField({ all, config, onChange }) {
  return (
    <label>
      Archive owner
      <select
        required
        value={config?.ownerId || ""}
        onChange={(e) =>
          onChange(archiveDefaults(all.find((x) => x.id === e.target.value)))
        }
      >
        <option value="">Choose its owner</option>
        {all
          .filter((e) => registry.types[e.kind]?.category !== "records")
          .map((e) => (
            <option key={e.id} value={e.id}>
              {e.name} — {label(e.kind)}
            </option>
          ))}
      </select>
      <small>Ownership is distinct from the archive's physical location.</small>
    </label>
  );
}
export function ArchiveSettings({ archive, doc, age, all, command, close }) {
  const [config, setConfig] = useState(
    structuredClone(archive.archive || archiveDefaults()),
  );
  const change = (key, value) => setConfig((c) => ({ ...c, [key]: value }));
  const sourceChange = (id, key, value) =>
    change(
      "sources",
      config.sources.map((s) => (s.id === id ? { ...s, [key]: value } : s)),
    );
  return (
    <Dialog
      title={`Archive Settings — ${archive.name}`}
      size="editor"
      className="archive-settings"
      onClose={close}
      onSubmit={async (e) => {
        e.preventDefault();
        const result = await command({
          kind: "entity",
          ageId: age.id,
          entity: { ...archive, archive: config },
        });
        if (!result) throw Error("Unable to save archive settings");
        close();
      }}
      actions={
        <>
          <button type="button" onClick={close} data-dialog-dismiss>
            Cancel
          </button>
          <button className="primary">Save settings</button>
        </>
      }
    >
      <ArchiveOwnerField
        all={all}
        config={config}
        onChange={(next) =>
          setConfig({
            ...next,
            sources: config.sources,
            coverage: config.coverage,
          })
        }
      />
      <div className="archive-form-grid">
        <label>
          Archive purpose
          <select
            value={config.purpose}
            onChange={(e) => change("purpose", e.target.value)}
          >
            {archivePurposes.map((v) => (
              <option key={v}>{v}</option>
            ))}
          </select>
        </label>
        <label>
          Documentation intensity
          <select
            value={config.intensity}
            onChange={(e) => change("intensity", e.target.value)}
          >
            {["none", "minimal", "moderate", "detailed", "exhaustive"].map(
              (v) => (
                <option key={v}>{v}</option>
              ),
            )}
          </select>
          <small>Controls recording detail, never access to information.</small>
        </label>
        <label>
          Knowledge scope
          <select
            value={config.knowledgeScope}
            onChange={(e) => change("knowledgeScope", e.target.value)}
          >
            <option value="direct">Direct knowledge only</option>
            <option value="institutional">Direct + dated administration</option>
            <option value="transmitted">
              Also allow explicit received sources
            </option>
          </select>
        </label>
        <label>
          Retention policy
          <select
            value={config.retention}
            onChange={(e) => change("retention", e.target.value)}
          >
            <option value="preserve">
              Preserve records and inherit with the archive
            </option>
            <option value="review">
              Review custody and losses at each Age
            </option>
          </select>
          <small>
            Loss, confiscation and destruction require an explicit action.
          </small>
        </label>
      </div>
      <fieldset>
        <legend>Document categories</legend>
        <div className="archive-checks">
          {archiveCategories.map((v) => (
            <label key={v}>
              <input
                type="checkbox"
                checked={config.categories.includes(v)}
                onChange={(e) =>
                  change(
                    "categories",
                    e.target.checked
                      ? [...config.categories, v]
                      : config.categories.filter((x) => x !== v),
                  )
                }
              />
              {v}
            </label>
          ))}
        </div>
      </fieldset>
      <p>
        Only one dated, authored governance or membership relationship grants
        institutional records. Proximity, spatial containment, allies, enemies
        and indirect connections do not grant access. Add evidence below for
        correspondence, observations or received histories. Dates in different
        Ages are never compared numerically.
      </p>
      <h3>Explicit knowledge sources</h3>
      {(config.sources || []).map((s) => (
        <fieldset key={s.id}>
          <legend>Received information</legend>
          <div className="archive-form-grid">
            <label>
              Source Age
              <select
                value={s.sourceAge}
                onChange={(e) => {
                  change(
                    "sources",
                    config.sources.map((x) =>
                      x.id === s.id
                        ? { ...x, sourceAge: e.target.value, entityId: "" }
                        : x,
                    ),
                  );
                }}
              >
                {doc.ages.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Subject entity
              <select
                required
                value={s.entityId}
                onChange={(e) => sourceChange(s.id, "entityId", e.target.value)}
              >
                <option value="">Select subject</option>
                {doc.entities
                  .filter(
                    (e) =>
                      e.ageId === s.sourceAge &&
                      !e.deleted &&
                      e.kind !== "archive",
                  )
                  .map((e) => (
                    <option key={e.id} value={e.id}>
                      {e.name}
                    </option>
                  ))}
              </select>
            </label>
            <label>
              Arrival Age
              <select
                value={s.arrivalAge}
                onChange={(e) =>
                  sourceChange(s.id, "arrivalAge", e.target.value)
                }
              >
                {doc.ages.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Channel
              <select
                value={s.channel}
                onChange={(e) => sourceChange(s.id, "channel", e.target.value)}
              >
                {[
                  "observation",
                  "messenger",
                  "diplomacy",
                  "trade",
                  "expedition",
                  "intelligence",
                  "inherited-record",
                ].map((v) => (
                  <option key={v}>{v}</option>
                ))}
              </select>
            </label>
            <label>
              Evidence strength
              <select
                value={s.strength}
                onChange={(e) => sourceChange(s.id, "strength", e.target.value)}
              >
                {["established", "corroborated", "incidental"].map((v) => (
                  <option key={v}>{v}</option>
                ))}
              </select>
            </label>
            {[
              ["fromYear", "Source information from year"],
              ["toYear", "Source information through year"],
              ["arrivalYear", "Received in arrival Age, year"],
            ].map(([key, title]) => (
              <label key={key}>
                {title}
                <input
                  required
                  type="number"
                  step="1"
                  value={s[key]}
                  onChange={(e) =>
                    sourceChange(
                      s.id,
                      key,
                      e.target.value === "" ? "" : Number(e.target.value),
                    )
                  }
                />
              </label>
            ))}
          </div>
          <label>
            Evidence / attributed source
            <textarea
              required
              value={s.evidence}
              onChange={(e) => sourceChange(s.id, "evidence", e.target.value)}
              placeholder="For example: the envoy's letter describes the treaty signed in year 42."
            />
          </label>
          <div className="archive-checks">
            {archiveCategories.map((v) => (
              <label key={v}>
                <input
                  type="checkbox"
                  checked={s.categories.includes(v)}
                  onChange={(e) =>
                    sourceChange(
                      s.id,
                      "categories",
                      e.target.checked
                        ? [...s.categories, v]
                        : s.categories.filter((x) => x !== v),
                    )
                  }
                />
                {v}
              </label>
            ))}
          </div>
          <button
            type="button"
            onClick={() =>
              change(
                "sources",
                config.sources.filter((x) => x.id !== s.id),
              )
            }
          >
            Remove source
          </button>
        </fieldset>
      ))}
      <button
        type="button"
        onClick={() =>
          change("sources", [
            ...(config.sources || []),
            {
              id: crypto.randomUUID(),
              sourceAge: age.id,
              arrivalAge: age.id,
              entityId: "",
              channel: "messenger",
              strength: "established",
              fromYear: 0,
              toYear: 0,
              arrivalYear: 0,
              evidence: "",
              categories: ["chronicles"],
            },
          ])
        }
      >
        Add knowledge source
      </button>
    </Dialog>
  );
}
export function GatherDocumentsDialog({
  doc,
  age,
  archive,
  onDocument,
  close,
  reviseId = "",
}) {
  const [request, setRequest] = useState({
    scope: "age",
    sourceAge: age.id,
    fromYear: "",
    toYear: "",
    recordingYear: Math.max(
      0,
      ...(doc.years || [])
        .filter((y) => y.ageId === age.id)
        .map((y) => y.number),
      ...(doc.libraryFiles || [])
        .filter((f) => f.ageId === age.id && f.record?.recordingAge === age.id)
        .map((f) => f.record.recordingYear),
      ...(doc.libraryFiles || [])
        .filter((f) => f.ageId === age.id)
        .flatMap((f) =>
          (f.custody || [])
            .filter((c) => c.ageId === age.id)
            .map((c) => c.year),
        ),
    ),
  });
  const [preview, setPreview] = useState(null),
    [saved, setSaved] = useState(false);
  const update = (key, value) => {
    setRequest({ ...request, [key]: value });
    setPreview(null);
    setSaved(false);
  };
  const run = async (previewOnly) => {
    const body = {
      ...request,
      revision: doc.world.revision,
      ageId: age.id,
      archiveId: archive.id,
      reviseId,
      preview: previewOnly,
      recordingYear: Number(request.recordingYear),
      fromYear: request.fromYear === "" ? null : Number(request.fromYear),
      toYear: request.toYear === "" ? null : Number(request.toYear),
    };
    const value = await archiveRequest(doc.world.id, "gather", body);
    setPreview(value.result);
    setSaved(!previewOnly);
    if (!previewOnly) {
      onDocument(value.document);
      setSaved(true);
    }
  };
  return (
    <Dialog
      title={reviseId ? "Revise generated document" : "Gather Documents"}
      size="editor"
      className="archive-settings"
      trackChanges={false}
      onClose={close}
      onSubmit={async (e) => {
        e.preventDefault();
        await run(!preview || saved);
      }}
      actions={
        <>
          <button type="button" onClick={close} data-dialog-dismiss>
            {saved ? "Close" : "Cancel"}
          </button>
          <button className="primary">
            {saved
              ? "Check for new information"
              : preview
                ? reviseId
                  ? "Save new file version"
                  : "Generate and save"
                : "Preview eligible documents"}
          </button>
        </>
      }
    >
      <p>
        Gathering uses the information available to {archive.name}. Undated
        facts are recorded as observations in the selected recording year, never
        as historical estimates. Your authored files remain unchanged. Revision
        creates a new file version.
      </p>
      <div className="archive-form-grid">
        <label>
          History scope
          <select
            value={request.scope}
            onChange={(e) => update("scope", e.target.value)}
          >
            <option value="age">Selected Age</option>
            <option value="range">Year range in selected Age</option>
            <option value="history">
              Available history and received sources
            </option>
          </select>
        </label>
        {request.scope !== "history" && (
          <label>
            Source Age
            <select
              value={request.sourceAge}
              onChange={(e) => update("sourceAge", e.target.value)}
            >
              {doc.ages.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </select>
          </label>
        )}
        {request.scope === "range" &&
          ["fromYear", "toYear"].map((key) => (
            <label key={key}>
              {key === "fromYear" ? "From year" : "Through year"}
              <input
                type="number"
                required
                step="1"
                value={request[key]}
                onChange={(e) => update(key, e.target.value)}
              />
            </label>
          ))}
        <label>
          Recording year in {age.name}
          <input
            required
            type="number"
            step="1"
            value={request.recordingYear}
            onChange={(e) => update("recordingYear", e.target.value)}
          />
        </label>
      </div>
      {preview && (
        <section aria-live="polite">
          <h3>
            {saved ? "Gathering complete" : "Preview"}: {preview.created.length}{" "}
            {saved ? "saved" : "eligible"}, {preview.skipped.length} skipped
          </h3>
          <ul>
            {preview.created.map((x) => (
              <li key={x.sourceId}>
                <strong>{x.title}</strong>
                <p>{x.reason}</p>
              </li>
            ))}
          </ul>
          <details>
            <summary>Skipped information and reasons (author's audit)</summary>
            <ul>
              {preview.skipped.map((x, i) => (
                <li key={i}>
                  <strong>{x.title}</strong> — {x.reason}
                </li>
              ))}
            </ul>
          </details>
        </section>
      )}
    </Dialog>
  );
}
export function ArchiveCustodyDialog({
  doc,
  age,
  file,
  archives,
  onDocument,
  close,
}) {
  const [action, setAction] = useState("copy"),
    [destination, setDestination] = useState(""),
    [year, setYear] = useState(""),
    [reason, setReason] = useState("");
  const move = ["copy", "transfer", "confiscate", "inherit"].includes(action);
  return (
    <Dialog
      title="Document preservation & custody"
      className="archive-settings"
      onClose={close}
      onSubmit={async (e) => {
        e.preventDefault();
        const d = await archiveRequest(doc.world.id, "custody", {
          revision: doc.world.revision,
          ageId: age.id,
          documentId: file.id,
          action,
          destination,
          year: Number(year),
          reason,
        });
        onDocument(d);
        close();
      }}
      actions={
        <>
          <button type="button" onClick={close} data-dialog-dismiss>
            Cancel
          </button>
          <button>Record action</button>
        </>
      }
    >
      <p>
        A copy receives its own physical identity and keeps its source
        references. Loss and destruction preserve historical metadata and past
        file versions.
      </p>
      <label>
        Action
        <select value={action} onChange={(e) => setAction(e.target.value)}>
          {[
            "copy",
            "transfer",
            "confiscate",
            "inherit",
            "lose",
            "destroy",
            "recover",
          ].map((v) => (
            <option key={v}>{v}</option>
          ))}
        </select>
      </label>
      {move && (
        <label>
          Destination archive
          <select
            required
            value={destination}
            onChange={(e) => setDestination(e.target.value)}
          >
            <option value="">Choose archive</option>
            {archives
              .filter((a) => a.id !== file.archiveId)
              .map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
          </select>
        </label>
      )}
      <label>
        Year in {age.name}
        <input
          required
          type="number"
          step="1"
          value={year}
          onChange={(e) => setYear(e.target.value)}
        />
      </label>
      <label>
        Historical reason
        <textarea
          required
          value={reason}
          onChange={(e) => setReason(e.target.value)}
        />
      </label>
    </Dialog>
  );
}
export function DocumentProvenance({ file, doc, onProfile }) {
  const r = file.record;
  if (!r && !file.custody?.length) return null;
  return (
    <details className="archive-provenance">
      <summary>
        Historical sources & preservation — {file.status || "present"}
      </summary>
      {r && (
        <>
          <p>
            {r.artifact} · {r.category} · attributed to {r.authorName}
          </p>
          <p>
            Covered year {r.fromYear}
            {r.toYear !== r.fromYear ? `–${r.toYear}` : ""} in{" "}
            {doc.ages.find((a) => a.id === r.sourceAge)?.name ||
              "historical Age"}
            . Created {r.creationYear}; recorded {r.recordingYear} in{" "}
            {doc.ages.find((a) => a.id === r.recordingAge)?.name ||
              "historical Age"}
            .
          </p>
          <p>
            {r.knowledge} · {r.reliability}. {r.evidence}
          </p>
          <p>
            {r.authoredEdits && "This version includes authored edits. "}Editing
            this file does not change world history or its source references.
          </p>
          <div className="library-toolbar">
            {r.relatedEntities.map((id) => {
              const e = doc.entities.find(
                (e) => e.ageId === r.sourceAge && e.id === id,
              );
              return (
                e && (
                  <button key={id} onClick={() => onProfile(e)}>
                    {e.name}
                  </button>
                )
              );
            })}
          </div>
          <small>Source identifiers: {r.sourceIds.join(", ")}</small>
        </>
      )}
      {file.copyOf && <p>Physical copy of document {file.copyOf}</p>}
      <ul>
        {file.custody?.map((c, i) => (
          <li key={i}>
            {c.action} · year {c.year} · {c.reason}
          </li>
        ))}
      </ul>
    </details>
  );
}
