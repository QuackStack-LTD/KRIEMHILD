"use client";
import { useState } from "react";
import { api, Command, Entry, State } from "../lib/types";
type Preview = { revision: string; records: Entry[]; warnings: string[] };
export default function TextImport({
  state,
  run,
  busy,
}: {
  state: State;
  run: (c: Command) => Promise<boolean>;
  busy: boolean;
}) {
  const [format, setFormat] = useState("csv"),
    [name, setName] = useState("Imported material"),
    [ageName, setAgeName] = useState("Import review"),
    [text, setText] = useState(""),
    [preview, setPreview] = useState<Preview | null>(null),
    [error, setError] = useState(""),
    [working, setWorking] = useState(false);
  return (
    <section className="card">
      <h3>Import text into a new Age</h3>
      <p>
        CSV and JSON create independent entities or notes. Markdown is preserved
        as editable scene text, including its original syntax. Preview shows
        every entry before acceptance.
      </p>
      <div className="two-col">
        <label>
          Text import format
          <select
            aria-label="Text import format"
            value={format}
            onChange={(e) => {
              setFormat(e.target.value);
              setPreview(null);
            }}
          >
            <option value="csv">
              CSV with name, type, notes and optional property columns
            </option>
            <option value="json">JSON array of entries</option>
            <option value="markdown">Markdown / plain scene text</option>
          </select>
        </label>
        <label>
          Import source name
          <input
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              setPreview(null);
            }}
          />
        </label>
        <label>
          New import Age name
          <input value={ageName} onChange={(e) => setAgeName(e.target.value)} />
        </label>
        <label>
          Text file
          <input
            type="file"
            accept=".csv,.json,.md,.txt"
            onChange={async (e) => {
              const file = e.target.files?.[0];
              if (!file) return;
              if (file.size > 2 * 1024 * 1024) {
                setError("Choose a text file up to 2 MiB");
                return;
              }
              setText(await file.text());
              setName(file.name);
              setPreview(null);
            }}
          />
        </label>
      </div>
      <label>
        Import text
        <textarea
          aria-label="Import text"
          rows={8}
          value={text}
          onChange={(e) => {
            setText(e.target.value);
            setPreview(null);
          }}
        />
      </label>
      <button
        disabled={busy || working || !text}
        onClick={async () => {
          setWorking(true);
          setError("");
          try {
            setPreview(
              await api<Preview>(
                `/projects/${state.root.world.id}/content-preview`,
                {
                  method: "POST",
                  body: JSON.stringify({
                    expected: state.revision,
                    age: state.age.id,
                    format,
                    name,
                    text,
                  }),
                },
              ),
            );
          } catch (e) {
            setError((e as Error).message);
          } finally {
            setWorking(false);
          }
        }}
      >
        Preview text import
      </button>
      {error && <p role="alert">{error}</p>}
      {preview && (
        <div>
          <ul>
            {preview.warnings.map((w) => (
              <li key={w}>{w}</li>
            ))}
          </ul>
          <details open>
            <summary>{preview.records.length} entries</summary>
            <ul>
              {preview.records.map((r) => (
                <li key={r.id}>
                  {r.name} · {r.kind} · {r.type}
                </li>
              ))}
            </ul>
          </details>
          <button
            disabled={
              busy ||
              state.readOnly ||
              !ageName.trim() ||
              state.revision !== preview.revision
            }
            onClick={async () => {
              if (
                await run({
                  action: "import-records",
                  name: ageName,
                  records: preview.records,
                })
              )
                setPreview(null);
            }}
          >
            Accept import as a new Age
          </button>
          {state.revision !== preview.revision && (
            <p>The world changed. Preview again before importing.</p>
          )}
          <button onClick={() => setPreview(null)}>Discard text preview</button>
        </div>
      )}
    </section>
  );
}
