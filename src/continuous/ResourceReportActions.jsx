import { useEffect, useRef, useState } from "react";
import Dialog from "../worldbuilding/ui/Dialog.jsx";
import { request } from "./api.js";
import { resourceReportParagraph } from "./resource-report-text.js";

export default function ResourceReportActions({ report, path }) {
  const [dialog, setDialog] = useState(false),
    [choices, setChoices] = useState(null),
    [archive, setArchive] = useState(""),
    [year, setYear] = useState(""),
    [format, setFormat] = useState("md"),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [saved, setSaved] = useState(null);
  const active = useRef(report.fingerprint);
  active.current = report.fingerprint;
  const base = path?.match(
    /^(sessions\/[^/]+\/terrain|worlds\/[^/]+\/settlements\/[^/]+)\/resources(?:\?|$)/,
  )?.[1];
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  useEffect(() => {
    setDialog(false);
    setChoices(null);
    setNotice("");
    setError("");
    setSaved(null);
  }, [report.fingerprint, path]);
  useEffect(() => {
    if (!dialog || !base) return;
    const controller = new AbortController();
    setChoices(null);
    setError("");
    request(
      `${base}/resource-archives?${new URLSearchParams({ longitude: report.longitude, latitude: report.latitude, radiusKm: report.radiusKm, age: report.context?.ageId || "" })}`,
      undefined,
      "GET",
      controller.signal,
    )
      .then((value) => {
        setChoices(value);
        setArchive(value.archives[0]?.id || "");
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(e.message);
      });
    return () => controller.abort();
  }, [dialog, base, report.fingerprint]);
  async function copy() {
    const fingerprint = report.fingerprint;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      let full = report;
      if (report.truncated && path) full = await request(`${path}&complete=1`);
      if (!mounted.current || active.current !== fingerprint) return;
      await navigator.clipboard.writeText(resourceReportParagraph(full));
      setNotice("All resource information copied as one paragraph.");
    } catch (e) {
      setError(`Could not copy: ${e.message}`);
    } finally {
      setBusy(false);
    }
  }
  async function save() {
    if (!archive) throw Error("Choose an archive.");
    const value = year.trim() === "" ? null : Number(year);
    if (
      value !== null &&
      (!/^-?\d+$/.test(year.trim()) || !Number.isSafeInteger(value))
    )
      throw Error("Enter a whole year, or leave it blank.");
    setBusy(true);
    setError("");
    try {
      const selection = new URLSearchParams(path.split("?")[1]);
      const result = await request(
        `${base}/resource-reports?${new URLSearchParams({ age: report.context?.ageId || "" })}`,
        {
          archiveId: archive,
          year: value,
          format,
          longitude: selection.has("longitude")
            ? Number(selection.get("longitude"))
            : report.longitude,
          latitude: selection.has("latitude")
            ? Number(selection.get("latitude"))
            : report.latitude,
          radiusKm: selection.has("radiusKm")
            ? Number(selection.get("radiusKm"))
            : report.radiusKm,
          fingerprint: report.fingerprint,
        },
      );
      setSaved(result);
      setDialog(false);
      setNotice(
        `Report archived. ${result.newCells} newly observed samples, ${result.changedCells} changed, ${result.unchangedCells} unchanged.`,
      );
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="resource-report-actions">
      <div className="resource-panel-controls">
        <button type="button" disabled={busy} onClick={copy}>
          Copy information
        </button>
        {base && (
          <button
            type="button"
            disabled={busy || !report.context}
            onClick={() => {
              setSaved(null);
              setDialog(true);
            }}
          >
            Archive information
          </button>
        )}
      </div>
      {notice && <p role="status">{notice}</p>}
      {error && !dialog && <p role="alert">{error}</p>}
      {saved && (
        <a
          href={`/worlds/${report.context.worldId}/library?age=${report.context.ageId}&record=${saved.resourcesFolderId}`}
        >
          Open archive resources
        </a>
      )}
      {dialog && (
        <Dialog
          title="Archive resource information"
          onClose={() => setDialog(false)}
          onSubmit={save}
          busy={busy}
          trackChanges={false}
          actions={
            <>
              <button
                type="button"
                disabled={busy}
                onClick={() => setDialog(false)}
              >
                Cancel
              </button>
              <button
                type="submit"
                className="primary"
                disabled={busy || !archive || !choices}
              >
                Confirm
              </button>
            </>
          }
        >
          <p>
            Save this circle’s complete resource report in an inworld archive.
            Archives with a known location are ordered from nearest to farthest.
          </p>
          {!choices && !error && <p role="status">Finding archives…</p>}
          {error && <p role="alert">{error}</p>}
          {choices && !choices.archives.length && (
            <p>
              Create an archive in the Library first, then return to this
              circle.
            </p>
          )}
          {choices?.archives.length > 0 && (
            <label>
              Archive
              <select
                aria-label="Resource report archive"
                required
                value={archive}
                onChange={(e) => setArchive(e.target.value)}
              >
                {choices.archives.map((a, i) => (
                  <option key={a.id} value={a.id}>
                    {a.path} —{" "}
                    {a.distanceKm == null
                      ? "location not recorded"
                      : `${a.distanceKm.toFixed(1)} km · ${a.location}`}
                    {i === 0 && a.distanceKm != null ? " · nearest" : ""}
                  </option>
                ))}
              </select>
            </label>
          )}
          <label>
            Year (optional)
            <input
              aria-label="Resource report year"
              inputMode="numeric"
              value={year}
              onChange={(e) => setYear(e.target.value)}
              placeholder="Undated"
            />
          </label>
          <p>
            {choices?.age?.name}. Leave the year blank to keep the report
            undated.
          </p>
          <label>
            Document format
            <select value={format} onChange={(e) => setFormat(e.target.value)}>
              <option value="md">Markdown (.md)</option>
              <option value="txt">Plain text (.txt)</option>
            </select>
          </label>
          <p>
            Stored in Resources → {choices?.age?.name || "Age"} →{" "}
            {year.trim() ? `Year ${year}` : "Undated"}. Shared geographic
            observations supply the archive’s maps and charts.
          </p>
        </Dialog>
      )}
    </div>
  );
}
