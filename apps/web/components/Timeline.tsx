"use client";
import { useEffect, useState } from "react";
import {
  api,
  Calendar,
  Command,
  Entry,
  HistoricalState,
  newID,
  State,
} from "../lib/types";
import { MapPicture } from "./Terrain";
type Props = {
  state: State;
  run: (c: Command) => Promise<boolean>;
  busy: boolean;
};
export default function Timeline({ state, run, busy }: Props) {
  const records = Object.values(state.records),
    chron = records.find((r) => r.kind === "chronology")?.chronology;
  const [start, setStart] = useState(chron?.start || ""),
    [end, setEnd] = useState(chron?.end || ""),
    [branch, setBranch] = useState(chron?.branch || "canon"),
    [calendar, setCalendar] = useState(chron?.calendar || "");
  const [name, setName] = useState(""),
    [when, setWhen] = useState(""),
    [precision, setPrecision] = useState("exact"),
    [rangeEnd, setRangeEnd] = useState(""),
    [track, setTrack] = useState("History"),
    [notes, setNotes] = useState(""),
    [cause, setCause] = useState("");
  const [tick, setTick] = useState("0"),
    [historical, setHistorical] = useState<HistoricalState | null>(null),
    [error, setError] = useState(""),
    [filter, setFilter] = useState("");
  const [calName, setCalName] = useState(""),
    [epoch, setEpoch] = useState("0"),
    [months, setMonths] = useState("First Rain:30, High Sun:30, Long Night:30"),
    [week, setWeek] = useState(7),
    [yearZero, setYearZero] = useState(false),
    [leapEvery, setLeapEvery] = useState(0),
    [leapDays, setLeapDays] = useState(0);
  const events = records
    .filter((r) => r.kind === "event" && r.event?.age === state.age.id)
    .sort((a, b) => {
      const x = a.event!.date.tick,
        y = b.event!.date.tick;
      return x && y
        ? BigInt(x) < BigInt(y)
          ? -1
          : BigInt(x) > BigInt(y)
            ? 1
            : a.name.localeCompare(b.name)
        : x
          ? -1
          : y
            ? 1
            : a.name.localeCompare(b.name);
    });
  useEffect(() => {
    setStart(chron?.start || "");
    setEnd(chron?.end || "");
    setBranch(chron?.branch || "canon");
    setCalendar(chron?.calendar || "");
  }, [chron?.start, chron?.end, chron?.branch, chron?.calendar]);
  useEffect(() => setHistorical(null), [state.revision]);
  return (
    <div className="timeline-page">
      <div className="eyebrow">Time / {state.age.name}</div>
      <h2>A history you can trace.</h2>
      <p className="muted">
        Fictional dates use integer day ticks. Empty dates remain unknown.
        Computer save times never become world history.
      </p>
      <section className="card">
        <h3>Age opening & branch</h3>
        <p>
          {chron
            ? "The opening baseline is pinned. Changing these dates changes its asserted validity; it does not replace its contents."
            : "Set a start tick to assert that the current saved world is the opening state of this Age. Undated edits made afterward affect only its overview."}
        </p>
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            if (
              await run({
                action: "configure-time",
                chronology: {
                  age: state.age.id,
                  baseline: "",
                  start,
                  end,
                  branch,
                  calendar,
                },
              })
            )
              setHistorical(null);
          }}
        >
          <div className="two-col">
            <label>
              Age start tick
              <input
                value={start}
                onChange={(e) => setStart(e.target.value)}
                placeholder="0"
              />
            </label>
            <label>
              Age end tick (exclusive)
              <input value={end} onChange={(e) => setEnd(e.target.value)} />
            </label>
            <label>
              Branch label
              <select
                value={branch}
                onChange={(e) => setBranch(e.target.value)}
              >
                {["canon", "alternate", "experiment", "abandoned"].map((b) => (
                  <option key={b}>{b}</option>
                ))}
              </select>
            </label>
            <label>
              Display calendar
              <select
                value={calendar}
                onChange={(e) => setCalendar(e.target.value)}
              >
                <option value="">Integer ticks</option>
                {records
                  .filter((r) => r.kind === "calendar")
                  .map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.name}
                    </option>
                  ))}
              </select>
            </label>
          </div>
          <button disabled={busy}>
            {chron ? "Save chronology" : "Pin opening baseline"}
          </button>
        </form>
      </section>
      <section className="card">
        <h3>Age ancestry</h3>
        <ul className="ancestry">
          {Object.values(state.root.ages).map((a) => (
            <li key={a.id}>
              <strong>{a.name}</strong>{" "}
              {a.sourceAge
                ? `← ${state.root.ages[a.sourceAge]?.name}`
                : "· first Age"}
              {a.id === state.age.id
                ? ` · ${chron?.branch || "unlabelled"}`
                : ""}
              {a.sourceSnapshot && (
                <small> · source {a.sourceSnapshot.slice(0, 10)}</small>
              )}
            </li>
          ))}
        </ul>
      </section>
      <details className="card">
        <summary>Define a calendar</summary>
        <p className="muted">
          One tick is one day. Leap days extend the final month every N years
          from the epoch. Definitions live in Age snapshots, so pinned scene
          calendars stay preserved.
        </p>
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            setError("");
            const list = months.split(",").map((s) => {
              const pair = s.trim().split(":");
              return { name: pair[0]?.trim(), days: Number(pair[1]) };
            });
            const definition: Calendar = {
              epoch,
              era: calName,
              months: list,
              week,
              yearZero,
              leapEvery,
              leapDays,
            };
            if (
              await run({
                action: "put",
                record: {
                  id: newID(),
                  kind: "calendar",
                  name: calName,
                  calendar: definition,
                },
              })
            )
              setCalName("");
          }}
        >
          <div className="two-col">
            <label>
              Calendar name / era
              <input
                required
                value={calName}
                onChange={(e) => setCalName(e.target.value)}
              />
            </label>
            <label>
              Epoch tick
              <input value={epoch} onChange={(e) => setEpoch(e.target.value)} />
            </label>
            <label>
              Months (name:days, …)
              <input
                required
                value={months}
                onChange={(e) => setMonths(e.target.value)}
              />
            </label>
            <label>
              Days per week
              <input
                type="number"
                min="1"
                max="100"
                value={week}
                onChange={(e) => setWeek(Number(e.target.value))}
              />
            </label>
            <label>
              Leap every N years (0 disables)
              <input
                type="number"
                min="0"
                max="1000"
                value={leapEvery}
                onChange={(e) => setLeapEvery(Number(e.target.value))}
              />
            </label>
            <label>
              Added leap days
              <input
                type="number"
                min="0"
                max="100"
                value={leapDays}
                onChange={(e) => setLeapDays(Number(e.target.value))}
              />
            </label>
            <label className="check-label">
              <input
                type="checkbox"
                checked={yearZero}
                onChange={(e) => setYearZero(e.target.checked)}
              />
              Include year zero
            </label>
          </div>
          <button disabled={busy}>Create calendar</button>
        </form>
      </details>
      <section className="card">
        <h3>Record an event</h3>
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            if (
              await run({
                action: "record-event",
                event: {
                  id: newID(),
                  kind: "event",
                  name,
                  notes,
                  event: {
                    age: state.age.id,
                    date: {
                      precision,
                      tick:
                        precision === "unknown" || precision === "relative"
                          ? undefined
                          : when,
                      end: precision === "range" ? rangeEnd : undefined,
                      relative: precision === "relative" ? when : undefined,
                    },
                    track,
                    causes: cause ? [cause] : [],
                  },
                },
              })
            ) {
              setName("");
              setNotes("");
            }
          }}
        >
          <div className="two-col">
            <label>
              Event name
              <input
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </label>
            <label>
              Date precision
              <select
                value={precision}
                onChange={(e) => setPrecision(e.target.value)}
              >
                {[
                  "exact",
                  "approximate",
                  "before",
                  "after",
                  "range",
                  "unknown",
                  "relative",
                ].map((p) => (
                  <option key={p}>{p}</option>
                ))}
              </select>
            </label>
            <label>
              {precision === "relative"
                ? "Relative date description"
                : "Event date tick"}
              <input
                disabled={precision === "unknown"}
                value={when}
                onChange={(e) => setWhen(e.target.value)}
              />
            </label>
            {precision === "range" && (
              <label>
                Range end tick
                <input
                  value={rangeEnd}
                  onChange={(e) => setRangeEnd(e.target.value)}
                />
              </label>
            )}
            <label>
              Timeline track
              <input value={track} onChange={(e) => setTrack(e.target.value)} />
            </label>
            <label>
              Caused by
              <select value={cause} onChange={(e) => setCause(e.target.value)}>
                <option value="">No asserted cause</option>
                {records
                  .filter((r) => r.kind === "event")
                  .map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.name}
                    </option>
                  ))}
              </select>
            </label>
            <label>
              Event explanation
              <textarea
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
              />
            </label>
          </div>
          <button disabled={busy}>Record event</button>
        </form>
      </section>
      <section className="card">
        <h3>Event timeline</h3>
        <label>
          Filter track
          <select value={filter} onChange={(e) => setFilter(e.target.value)}>
            <option value="">All tracks</option>
            {Array.from(
              new Set(events.map((r) => r.event?.track || "Unassigned")),
            ).map((t) => (
              <option key={t}>{t}</option>
            ))}
          </select>
        </label>
        {!events.length && <p>No events recorded in this Age.</p>}
        {events
          .filter((r) => !filter || (r.event?.track || "Unassigned") === filter)
          .map((r) => (
            <EventCard
              key={r.id + JSON.stringify(r.event)}
              entry={r}
              state={state}
              run={run}
              busy={busy}
            />
          ))}
      </section>
      <section className="card">
        <h3>Inspect a historical date</h3>
        <form
          className="map-tools"
          onSubmit={async (e) => {
            e.preventDefault();
            setError("");
            try {
              setHistorical(
                await api<HistoricalState>(
                  `/projects/${state.root.world.id}/historical?snapshot=${state.age.snapshot}&tick=${encodeURIComponent(tick)}`,
                ),
              );
            } catch (e) {
              setError((e as Error).message);
              setHistorical(null);
            }
          }}
        >
          <label>
            Inspect tick
            <input value={tick} onChange={(e) => setTick(e.target.value)} />
          </label>
          <button>Inspect date</button>
        </form>
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
        {historical && (
          <div>
            <h4>{historical.label}</h4>
            {historical.unresolved.map((s, i) => (
              <p className="notice" key={i}>
                {s}
              </p>
            ))}
            {Object.values(historical.records)
              .filter((r) => r.kind === "map")
              .map((r) => (
                <MapPicture
                  key={r.id}
                  map={r}
                  records={historical.records}
                  world={state.root.world.id}
                  label={`${r.name} at tick ${historical.tick}`}
                />
              ))}
            <ul>
              {Object.values(historical.records)
                .filter((r) => r.kind === "entity")
                .map((r) => (
                  <li key={r.id}>
                    {r.name} · {r.status || "unknown"}
                  </li>
                ))}
            </ul>
          </div>
        )}
      </section>
    </div>
  );
}
function EventCard({ entry, state, run, busy }: Props & { entry: Entry }) {
  const event = entry.event!;
  const [until, setUntil] = useState(event.until || "");
  return (
    <article className="timeline-event">
      <div className="eyebrow">
        {event.track || "Unassigned"} · {event.date.precision}{" "}
        {event.date.tick || event.date.relative || ""}
        {event.date.end ? ` – ${event.date.end}` : ""}
      </div>
      <h4>{entry.name}</h4>
      <p>{entry.notes}</p>
      {event.causes?.map((id) => (
        <p key={id}>Caused by: {state.records[id]?.name}</p>
      ))}
      {(event.changes || []).length > 0 && (
        <>
          <p>
            {event.changes?.length} recorded changes · valid [{event.date.tick},{" "}
            {event.until || "open"})
          </p>
          <ul>
            {event.changes?.map((ch) => (
              <li key={ch.target}>
                {state.records[ch.target]?.name || ch.target}{" "}
                <code>
                  {ch.before?.slice(0, 8) || "absent"} → {ch.after.slice(0, 8)}
                </code>
              </li>
            ))}
          </ul>
          <form
            className="map-tools"
            onSubmit={(e) => {
              e.preventDefault();
              void run({
                action: "put",
                record: { ...entry, event: { ...event, until } },
              });
            }}
          >
            <label>
              End validity for {entry.name}
              <input value={until} onChange={(e) => setUntil(e.target.value)} />
            </label>
            <button disabled={busy}>Update validity end</button>
          </form>
        </>
      )}
    </article>
  );
}
