import { useEffect, useState } from "react";
import { accountRequest } from "./api.js";
export default function LegacyDrafts() {
  const [projects, setProjects] = useState([]),
    [worlds, setWorlds] = useState([]),
    [project, setProject] = useState(""),
    [world, setWorld] = useState(""),
    [error, setError] = useState("");
  useEffect(() => {
    accountRequest("/api/writer/legacy")
      .then(setProjects)
      .catch((e) => {
        if (e.status !== 404) setError(e.message);
      });
    accountRequest("/api/worlds")
      .then(setWorlds)
      .catch((e) => setError(e.message));
  }, []);
  if (!projects.length && !error) return null;
  return (
    <section>
      <h3>Unassigned legacy drafts</h3>
      <p>
        Select their world explicitly. Existing text and versions will be
        preserved.
      </p>
      {error && <p role="alert">{error}</p>}
      <label>
        Writing project
        <select value={project} onChange={(e) => setProject(e.target.value)}>
          <option value="">Choose a project</option>
          {projects.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      </label>
      <label>
        World
        <select value={world} onChange={(e) => setWorld(e.target.value)}>
          <option value="">Choose a world</option>
          {worlds
            .filter((w) =>
              ["owner", "administrator", "editor"].includes(w.role),
            )
            .map((w) => (
              <option key={w.id} value={w.id}>
                {w.name}
              </option>
            ))}
        </select>
      </label>
      <button
        disabled={!project || !world}
        onClick={async () => {
          try {
            await accountRequest(
              `/api/writer/legacy/${encodeURIComponent(project)}/assign`,
              { worldId: world },
            );
            setProjects((p) => p.filter((x) => x.id !== project));
            setProject("");
          } catch (e) {
            setError(e.message);
          }
        }}
      >
        Assign to world
      </button>
    </section>
  );
}
