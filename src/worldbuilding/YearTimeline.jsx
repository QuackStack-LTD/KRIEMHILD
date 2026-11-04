import { useEffect, useRef, useState } from "react";
import { entityIn, ageYears } from "./model.js";

export default function YearTimeline({
  doc,
  age,
  initialYear,
  onProfile,
  onNewAge,
  onGraph,
  busy,
}) {
  const years = ageYears(doc, age.id),
    [selected, setSelected] = useState(initialYear || years[0]?.id || ""),
    activeRef = useRef(null);
  const year = years.find((y) => y.id === selected) || years[0],
    index = years.indexOf(year);
  useEffect(() => {
    activeRef.current?.scrollIntoView({
      block: "nearest",
      inline: "center",
      behavior: "smooth",
    });
  }, [year?.id]);
  const started = year
    ? doc.ages.filter(
        (a) =>
          a.startedFromYear?.ageId === age.id &&
          a.startedFromYear?.yearId === year.id,
      )
    : [];
  return (
    <main className="world-page workspace year-workspace">
      <div className="workspace-heading">
        <div>
          <span className="eyebrow">
            {doc.world.name} / {age.name}
          </span>
          <h1>Year Timeline</h1>
          <p>
            Events recorded within this Age, drawn from record start and end
            years.
          </p>
        </div>
        <button onClick={() => onGraph()}>Open Time Graph</button>
      </div>
      {!years.length ? (
        <section className="workspace-empty">
          <h2>This Age has no recorded years yet</h2>
          <p>
            Enter a start or end year on a record profile and save it. Events
            with the same year will appear together here.
          </p>
        </section>
      ) : (
        <>
          <nav className="year-navigation" aria-label="Years in this Age">
            <button
              disabled={index <= 0}
              onClick={() => setSelected(years[index - 1].id)}
              aria-label="Previous recorded year"
            >
              ←
            </button>
            <div className="year-track" role="list">
              {years.map((y) => (
                <button
                  role="listitem"
                  ref={y.id === year?.id ? activeRef : null}
                  key={y.id}
                  className={`year-node ${y.id === year?.id ? "selected" : ""}`}
                  aria-current={y.id === year?.id ? "date" : undefined}
                  onClick={() => setSelected(y.id)}
                >
                  <span className="year-dot" />
                  <strong>Year {y.number}</strong>
                  <small>
                    {y.events.length}{" "}
                    {y.events.length === 1 ? "event" : "events"}
                  </small>
                </button>
              ))}
            </div>
            <button
              disabled={index >= years.length - 1}
              onClick={() => setSelected(years[index + 1].id)}
              aria-label="Next recorded year"
            >
              →
            </button>
          </nav>
          <section className="year-events" aria-labelledby="selected-year">
            <div className="section-heading">
              <div>
                <span className="eyebrow">{age.name}</span>
                <h2 id="selected-year">Year {year.number}</h2>
              </div>
              <button
                className="primary"
                disabled={busy}
                onClick={() => onNewAge(year)}
              >
                Start new Age here
              </button>
            </div>
            <p className="year-note">
              Years are ordered chronologically; spacing does not represent
              elapsed time.
            </p>
            {year.events.length ? (
              <ol className="year-event-list">
                {year.events.map((event) => {
                  const entity = entityIn(doc, age.id, event.entityId);
                  return (
                    <li key={event.id}>
                      <span className={`year-event-kind ${event.kind}`}>
                        {event.kind === "start" ? "Beginning" : "Ending"}
                      </span>
                      <div>
                        <h3>{event.title}</h3>
                        {event.description && <p>{event.description}</p>}
                        {entity && !entity.deleted && (
                          <button onClick={() => onProfile(entity)}>
                            Open {entity.name}
                          </button>
                        )}
                      </div>
                    </li>
                  );
                })}
              </ol>
            ) : (
              <p>
                No current events. This year remains available as a historical
                anchor after its source dates were changed.
              </p>
            )}
            {started.length > 0 && (
              <aside className="year-successors">
                <h3>Ages started here</h3>
                {started.map((a) => (
                  <button key={a.id} onClick={() => onGraph(a.id)}>
                    {a.name} → Time Graph
                  </button>
                ))}
              </aside>
            )}
          </section>
        </>
      )}
    </main>
  );
}
