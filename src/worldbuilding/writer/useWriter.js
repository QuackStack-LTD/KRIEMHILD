import { useEffect, useRef, useState } from "react";
import { registerNavigationGuard, confirmAction } from "../ui/feedback.js";
import { request, orderedNodes, inlineReferences } from "./model.js";
import { useAuth } from "../../auth/AuthProvider.jsx";
export default function useWriter(worldId) {
  const account = useAuth();
  const [projects, setProjects] = useState([]),
    [project, setProject] = useState(null),
    [nodes, setNodes] = useState([]),
    [draft, setDraft] = useState(null),
    [dirty, setDirty] = useState(false),
    [status, setStatus] = useState("Loading…"),
    [error, setError] = useState(""),
    [recovery, setRecovery] = useState(null),
    [epoch, setEpoch] = useState(0);
  const room = useRef(null);
  const state = useRef({}),
    flight = useRef(null),
    generation = useRef(0),
    base = useRef(null);
  state.current = { project, draft, dirty, status };
  const key = (n) =>
    "kriemhild.writer.recovery." +
    (account?.user.id ? account.user.id + "." : "") +
    n.projectId +
    "." +
    n.id;
  function accept(n) {
    base.current = n;
    setDraft(n);
    setDirty(false);
    state.current = { ...state.current, draft: n, dirty: false };
    setEpoch((x) => x + 1);
    setStatus(n ? "Saved" : "No scene selected");
    setRecovery(null);
    if (n) {
      try {
        const local = JSON.parse(localStorage.getItem(key(n)) || "null");
        if (local && JSON.stringify(local) !== JSON.stringify(n))
          setRecovery(local);
      } catch {
        setError(
          "Local recovery storage is unavailable. Keep this tab open until saved.",
        );
      }
    }
  }
  async function refreshProjects() {
    const p = await request("/projects?world=" + encodeURIComponent(worldId));
    setProjects(p);
    return p;
  }
  async function load(id) {
    const b = await request("/projects/" + id);
    setProject(b.project);
    state.current.project = b.project;
    setNodes(b.nodes);
    const ordered = orderedNodes(b.nodes);
    accept(ordered.find((n) => n.type === "scene") || ordered[0] || null);
  }
  useEffect(() => {
    let live = true;
    refreshProjects()
      .then(async (p) => {
        if (live) {
          if (p.length) await load(p[0].id);
          else setStatus("Create a writing project");
        }
      })
      .catch((e) => setError(e.message));
    return () => {
      live = false;
    };
  }, [worldId]);
  function edit(patch) {
    const n = { ...state.current.draft, ...patch };
    if (patch.content)
      n.references = [
        ...(n.references || []).filter((r) => r.type !== "inline"),
        ...inlineReferences(patch.content),
      ];
    generation.current++;
    state.current = { ...state.current, draft: n, dirty: true };
    setDraft(n);
    setDirty(true);
    setStatus("Unsaved changes");
    try {
      localStorage.setItem(key(n), JSON.stringify(n));
    } catch {
      setError(
        "Local recovery storage is full or unavailable. Save to the server or download a backup.",
      );
    }
  }
  async function flush(label = "") {
    if (flight.current) {
      const ok = await flight.current;
      if (!ok) return false;
      if (state.current.dirty || label) return flush(label);
      return true;
    }
    if (!state.current.draft || (!state.current.dirty && !label)) return true;
    const n = state.current.draft,
      g = generation.current;
    setStatus("Saving…");
    setError("");
    flight.current = (async () => {
      try {
        if (room.current) await room.current.flush();
        let saved;
        try {
          saved = await request("/projects/" + n.projectId + "/nodes", {
            node: n,
            label,
          });
        } catch (e) {
          if (e.status !== 409 || !room.current) throw e;
          const bundle = await request("/projects/" + n.projectId),
            current = bundle.nodes.find((x) => x.id === n.id);
          if (!current || current.deleted) throw e;
          const ignore = new Set([
            "revision",
            "updatedAt",
            "createdAt",
            "authorId",
            "content",
            "text",
            "references",
          ]);
          const patch = Object.fromEntries(
            Object.entries(n).filter(
              ([k, v]) =>
                !ignore.has(k) &&
                JSON.stringify(v) !== JSON.stringify(base.current?.[k]),
            ),
          );
          if (
            Object.keys(patch).some(
              (k) =>
                JSON.stringify(current[k]) !==
                  JSON.stringify(base.current?.[k]) &&
                JSON.stringify(current[k]) !== JSON.stringify(patch[k]),
            )
          )
            throw e;
          saved = await request("/projects/" + n.projectId + "/nodes", {
            node: {
              ...current,
              ...patch,
              content: n.content,
              text: n.text,
              references: n.references,
            },
            label,
          });
        }
        base.current = saved;
        setNodes((prev) => [...prev.filter((x) => x.id !== saved.id), saved]);
        if (generation.current === g) {
          state.current = { ...state.current, draft: saved, dirty: false };
          setDraft(saved);
          setDirty(false);
          setStatus("Saved");
          try {
            localStorage.removeItem(key(n));
          } catch {}
        } else {
          const newer = {
            ...state.current.draft,
            revision: saved.revision,
            createdAt: saved.createdAt,
          };
          state.current.draft = newer;
          setDraft(newer);
          try {
            localStorage.setItem(key(n), JSON.stringify(newer));
          } catch {}
          setStatus("Unsaved changes");
        }
        return true;
      } catch (e) {
        setStatus(
          e.status === 409
            ? "Conflict needs attention"
            : "Offline / save failed",
        );
        setError(e.message);
        return false;
      } finally {
        flight.current = null;
      }
    })();
    const ok = await flight.current;
    if (ok && state.current.dirty) return flush();
    return ok;
  }
  useEffect(() => {
    if (!dirty) return;
    const timer = setTimeout(() => void flush(), 1100);
    return () => clearTimeout(timer);
  }, [draft, dirty]);
  useEffect(() => {
    const remove = registerNavigationGuard(async () => {
      if (await flush()) return true;
      return confirmAction(
        "The server has not saved your draft. Leave with local recovery retained?",
        { accept: "Leave with recovery" },
      );
    });
    const unload = (e) => {
      if (state.current.dirty) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    const online = () => void flush();
    window.addEventListener("beforeunload", unload);
    window.addEventListener("online", online);
    return () => {
      remove();
      window.removeEventListener("beforeunload", unload);
      window.removeEventListener("online", online);
    };
  }, []);
  async function choose(n) {
    if (!(await flush())) return false;
    accept(n);
    return true;
  }
  async function chooseProject(id) {
    if (!(await flush())) return;
    await load(id);
  }
  async function saveNode(n, label) {
    const before = state.current.draft;
    const patch =
      before?.id === n.id
        ? Object.fromEntries(
            Object.entries(n).filter(
              ([k, v]) =>
                !["revision", "updatedAt", "createdAt"].includes(k) &&
                JSON.stringify(v) !== JSON.stringify(before[k]),
            ),
          )
        : null;
    if (!(await flush())) return null;
    if (patch && n.id === state.current.draft?.id)
      n = { ...state.current.draft, ...patch };
    const saved = await request("/projects/" + n.projectId + "/nodes", {
      node: n,
      label,
    });
    setNodes((prev) => [...prev.filter((x) => x.id !== n.id), saved]);
    accept(saved.deleted ? null : saved);
    return saved;
  }
  async function saveProject(p) {
    if (!(await flush())) return;
    const saved = await request("/projects", p);
    setProject(saved);
    state.current.project = saved;
    await refreshProjects();
    return saved;
  }
  async function reload() {
    if (
      !(await confirmAction(
        "Reload the server version? Your local draft remains available in recovery.",
      ))
    )
      return;
    const id = state.current.draft?.id;
    const b = await request("/projects/" + state.current.project.id);
    setNodes(b.nodes);
    accept(b.nodes.find((n) => n.id === id) || null);
  }
  function recover() {
    const n = { ...recovery, revision: state.current.draft.revision };
    setRecovery(null);
    edit(n);
    setEpoch((x) => x + 1);
  }
  async function saveCopy() {
    const original = state.current.draft;
    const copy = {
      ...original,
      id: crypto.randomUUID(),
      revision: 0,
      title: original.title + " (recovered copy)",
      createdAt: 0,
    };
    const n = await request("/projects/" + copy.projectId + "/nodes", {
      node: copy,
      label: "Recovered local edits",
    });
    setNodes((prev) => [...prev, n]);
    try {
      localStorage.removeItem(key(original));
    } catch {}
    accept(n);
  }
  const visible = nodes.map((n) => (n.id === draft?.id ? draft : n));
  return {
    accept: (n) => {
      setNodes((prev) => [...prev.filter((x) => x.id !== n.id), n]);
      accept(n);
    },
    room,
    projects,
    project,
    nodes: visible,
    draft,
    dirty,
    status,
    error,
    setError,
    recovery,
    setRecovery,
    epoch,
    edit,
    flush,
    choose,
    chooseProject,
    saveNode,
    saveProject,
    refreshProjects,
    load,
    reload,
    recover,
    saveCopy,
    base,
  };
}
