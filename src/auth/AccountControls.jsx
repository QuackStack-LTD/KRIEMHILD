import { useEffect, useState } from "react";
import { useAuth } from "./AuthProvider.jsx";
import LegacyDrafts from "./LegacyDrafts.jsx";
import AccountSecurity from "./AccountSecurity.jsx";
import { accountRequest } from "./api.js";
import Dialog from "../worldbuilding/ui/Dialog.jsx";
import {
  confirmAction,
  allowNavigation,
} from "../worldbuilding/ui/feedback.js";
export default function AccountControls({ worldId, revision, onRefresh }) {
  const auth = useAuth();
  const [members, setMembers] = useState([]),
    [role, setRole] = useState(""),
    [dialog, setDialog] = useState(""),
    [error, setError] = useState(""),
    [email, setEmail] = useState(""),
    [inviteRole, setInviteRole] = useState("editor"),
    [token, setToken] = useState(""),
    [invitations, setInvitations] = useState([]),
    [sentInvitations, setSentInvitations] = useState([]),
    [activity, setActivity] = useState([]),
    [revisions, setRevisions] = useState([]),
    [moreRevisions, setMoreRevisions] = useState(false),
    [online, setOnline] = useState([]),
    [connected, setConnected] = useState(false),
    [remoteRevision, setRemoteRevision] = useState(0),
    [pending, setPending] = useState(false);
  const manage = ["owner", "administrator"].includes(role);
  async function run(fn) {
    setError("");
    setPending(true);
    try {
      await fn();
    } catch (e) {
      setError(e.message);
    } finally {
      setPending(false);
    }
  }
  async function loadMembers() {
    const data = await accountRequest(`/api/worlds/${worldId}/members`);
    setMembers(data.members);
    setRole(data.role);
    setSentInvitations(
      ["owner", "administrator"].includes(data.role)
        ? await accountRequest(`/api/worlds/${worldId}/invitations`)
        : [],
    );
    window.dispatchEvent(
      new CustomEvent("world-role", { detail: { worldId, role: data.role } }),
    );
  }
  useEffect(() => {
    setRole("");
    setOnline([]);
    setRemoteRevision(0);
    if (worldId) void run(loadMembers);
    else
      void run(async () =>
        setInvitations(await accountRequest("/api/invitations")),
      );
  }, [worldId]);
  useEffect(() => {
    if (!worldId) return;
    let socket,
      timer,
      stopped = false;
    const connect = () => {
      socket = new WebSocket(
        `${location.protocol === "https:" ? "wss:" : "ws:"}//${location.host}/api/collaboration/worlds/${worldId}`,
      );
      socket.onopen = () => setConnected(true);
      socket.onmessage = (e) => {
        const data = JSON.parse(e.data);
        if (data.type === "presence")
          setOnline([
            ...new Map(data.members.map((m) => [m.userId, m])).values(),
          ]);
        if (data.type === "revision") setRemoteRevision(data.revision);
      };
      socket.onclose = () => {
        setConnected(false);
        if (!stopped)
          timer = setTimeout(() => {
            void loadMembers()
              .then(connect)
              .catch((e) => setError(e.message));
          }, 3000);
      };
    };
    connect();
    return () => {
      stopped = true;
      clearTimeout(timer);
      socket?.close();
    };
  }, [worldId]);
  if (!auth) return null;
  return (
    <div className="account-controls">
      {worldId && (
        <>
          <span className="account-role">{role || "Loading role…"}</span>
          <span
            className="connection-state"
            title={online.map((m) => m.name).join(", ")}
          >
            {connected ? `${online.length} online` : "Reconnecting…"}
          </span>
          {remoteRevision > revision && (
            <button onClick={() => run(onRefresh)}>
              Refresh shared changes
            </button>
          )}
          <div className="presence-avatars" aria-label="Active collaborators">
            {online.slice(0, 4).map((m) => (
              <span key={m.userId} className="presence-avatar" title={m.name}>
                {m.name.slice(0, 1).toUpperCase()}
              </span>
            ))}
          </div>
          <button
            onClick={() => {
              setDialog("share");
              void run(loadMembers);
            }}
          >
            Share
          </button>
          <button
            onClick={() => {
              setDialog("history");
              void run(async () => {
                const entries = await accountRequest(
                  `/api/worlds/${worldId}/revisions`,
                );
                setRevisions(entries);
                setMoreRevisions(entries.length === 200);
                setActivity(
                  manage
                    ? await accountRequest(`/api/worlds/${worldId}/activity`)
                    : [],
                );
              });
            }}
          >
            History
          </button>
        </>
      )}
      <button onClick={() => setDialog("account")}>{auth.user.name}</button>
      {!worldId && invitations.length > 0 && (
        <button onClick={() => setDialog("invitations")}>
          {invitations.length} invitations
        </button>
      )}
      {dialog && (
        <Dialog
          title={
            dialog === "share"
              ? "Share world"
              : dialog === "history"
                ? "Revision history"
                : dialog === "invitations"
                  ? "World invitations"
                  : "Your account"
          }
          onClose={() => setDialog("")}
          size="editor"
          trackChanges={false}
        >
          {error && <p role="alert">{error}</p>}
          {dialog === "account" && <LegacyDrafts />}
          {dialog === "account" && <AccountSecurity mode={auth.mode} />}
          {dialog === "account" && (
            <>
              <p>
                {auth.user.name}
                {auth.user.email && ` · ${auth.user.email}`}
              </p>
              <label>
                Workspace
                <select aria-label="Workspace">
                  {auth.workspaces.map((w) => (
                    <option key={w.id}>{w.name}</option>
                  ))}
                </select>
              </label>
              <p>
                {auth.mode === "local"
                  ? "Local workspace on this computer."
                  : "Account security and recovery are managed by your identity provider."}
              </p>
              {auth.mode !== "local" && (
                <button
                  disabled={pending}
                  onClick={() =>
                    run(async () => {
                      if (await allowNavigation()) await auth.logout();
                    })
                  }
                >
                  Sign out
                </button>
              )}
              <label>
                Accept an invitation
                <input
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  placeholder="Invitation token"
                />
              </label>
              <button
                disabled={!token || pending}
                onClick={() =>
                  run(async () => {
                    const result = await accountRequest(
                      "/api/invitations/accept",
                      { token },
                    );
                    location.assign(`/worlds/${result.worldId}`);
                  })
                }
              >
                Accept invitation
              </button>
            </>
          )}
          {dialog === "invitations" &&
            invitations.map((v) => (
              <div key={v.id} className="member-row">
                <span>
                  Invited as {v.role} · expires{" "}
                  {new Date(v.expiresAt).toLocaleDateString()}
                </span>
                <button
                  disabled={pending}
                  onClick={() =>
                    run(async () => {
                      const result = await accountRequest(
                        "/api/invitations/accept",
                        { token: v.id },
                      );
                      location.assign(`/worlds/${result.worldId}`);
                    })
                  }
                >
                  Accept
                </button>
                <button
                  disabled={pending}
                  onClick={() =>
                    run(async () => {
                      await accountRequest(
                        `/api/invitations/${v.id}`,
                        undefined,
                        "DELETE",
                      );
                      setInvitations((items) =>
                        items.filter((item) => item.id !== v.id),
                      );
                    })
                  }
                >
                  Decline
                </button>
              </div>
            ))}
          {dialog === "share" && (
            <>
              <p>
                Members have access to this world and its Ages, maps, Library,
                and manuscripts.
              </p>
              {members.map((m) => (
                <div key={m.userId} className="member-row">
                  <span>
                    {m.name}
                    {m.userId === auth.user.id ? " (you)" : ""}
                  </span>
                  {manage &&
                  m.role !== "owner" &&
                  (role === "owner" || m.role !== "administrator") ? (
                    <>
                      <select
                        aria-label={`Role for ${m.name}`}
                        value={m.role}
                        disabled={pending}
                        onChange={(e) =>
                          run(async () => {
                            await accountRequest(
                              `/api/worlds/${worldId}/members/${m.userId}`,
                              { role: e.target.value },
                              "PATCH",
                            );
                            await loadMembers();
                          })
                        }
                      >
                        {[
                          "viewer",
                          "commenter",
                          "editor",
                          ...(role === "owner" ? ["administrator"] : []),
                        ].map((v) => (
                          <option key={v}>{v}</option>
                        ))}
                      </select>
                      <button
                        disabled={pending}
                        onClick={() =>
                          run(async () => {
                            if (
                              !(await confirmAction(
                                `Remove ${m.name} from this world?`,
                              ))
                            )
                              return;
                            await accountRequest(
                              `/api/worlds/${worldId}/members/${m.userId}`,
                              undefined,
                              "DELETE",
                            );
                            await loadMembers();
                          })
                        }
                      >
                        Remove
                      </button>
                      {role === "owner" && (
                        <button
                          disabled={pending}
                          onClick={() =>
                            run(async () => {
                              if (
                                !(await confirmAction(
                                  `Transfer ownership to ${m.name}? You will become an administrator.`,
                                ))
                              )
                                return;
                              await accountRequest(
                                `/api/worlds/${worldId}/ownership`,
                                { userId: m.userId },
                              );
                              await loadMembers();
                            })
                          }
                        >
                          Transfer ownership
                        </button>
                      )}
                    </>
                  ) : (
                    <span>{m.role}</span>
                  )}
                </div>
              ))}
              {manage && (
                <div className="invite-form">
                  <label>
                    Collaborator email
                    <input
                      type="email"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                    />
                  </label>
                  <label>
                    Role
                    <select
                      value={inviteRole}
                      onChange={(e) => setInviteRole(e.target.value)}
                    >
                      {[
                        "viewer",
                        "commenter",
                        "editor",
                        ...(role === "owner" ? ["administrator"] : []),
                      ].map((v) => (
                        <option key={v}>{v}</option>
                      ))}
                    </select>
                  </label>
                  <button
                    disabled={pending || !email.includes("@")}
                    onClick={() =>
                      run(async () => {
                        const result = await accountRequest(
                          `/api/worlds/${worldId}/invitations`,
                          { email, role: inviteRole },
                        );
                        setToken(result.token);
                        await loadMembers();
                      })
                    }
                  >
                    Create invitation
                  </button>
                  {token && (
                    <label>
                      Share this invitation token
                      <input
                        readOnly
                        value={token}
                        onFocus={(e) => e.target.select()}
                      />
                      <small>
                        Valid for seven days. Only the verified recipient can
                        accept it.
                      </small>
                    </label>
                  )}
                </div>
              )}
              {manage && sentInvitations.length > 0 && (
                <section aria-label="Pending invitations">
                  <h3>Pending invitations</h3>
                  {sentInvitations.map((v) => (
                    <div className="member-row" key={v.id}>
                      <span>
                        {v.email} · {v.role} · expires{" "}
                        {new Date(v.expiresAt).toLocaleDateString()}
                      </span>
                      <button
                        disabled={
                          pending ||
                          (v.role === "administrator" && role !== "owner")
                        }
                        onClick={() =>
                          run(async () => {
                            await accountRequest(
                              `/api/worlds/${worldId}/invitations/${v.id}`,
                              undefined,
                              "DELETE",
                            );
                            setToken("");
                            await loadMembers();
                          })
                        }
                      >
                        Cancel invitation
                      </button>
                    </div>
                  ))}
                </section>
              )}
            </>
          )}
          {dialog === "history" && (
            <>
              {!revisions.length && <p>No recorded revisions yet.</p>}
              {revisions.map((v) => (
                <div className="member-row" key={v.revision}>
                  <span>
                    Revision {v.revision} ·{" "}
                    {v.kind === "world-restored"
                      ? "Restoration"
                      : v.kind === "world-created"
                        ? "World created"
                        : "Automatic recovery point"}{" "}
                    · {v.authorName} · {new Date(v.at).toLocaleString()}
                  </span>
                  {["owner", "administrator", "editor"].includes(role) && (
                    <button
                      disabled={pending}
                      onClick={() =>
                        run(async () => {
                          if (
                            !(await confirmAction(
                              `Restore revision ${v.revision}? This creates a new revision and preserves history.`,
                            ))
                          )
                            return;
                          if (!(await allowNavigation())) return;
                          await accountRequest(
                            `/api/worlds/${worldId}/revisions/${v.revision}/restore`,
                            { revision },
                          );
                          await onRefresh();
                          setDialog("");
                        })
                      }
                    >
                      Restore
                    </button>
                  )}
                </div>
              ))}
              {moreRevisions && (
                <button
                  disabled={pending}
                  onClick={() =>
                    run(async () => {
                      const entries = await accountRequest(
                        `/api/worlds/${worldId}/revisions?before=${revisions.at(-1).revision}`,
                      );
                      setRevisions((current) => [...current, ...entries]);
                      setMoreRevisions(entries.length === 200);
                    })
                  }
                >
                  Load older revisions
                </button>
              )}
              {manage && (
                <section aria-label="World activity">
                  <h3>Sharing activity</h3>
                  {activity.length === 0 && <p>No sharing activity yet.</p>}
                  {activity.map((v) => (
                    <div className="member-row" key={v.id}>
                      <span>
                        {v.kind.replaceAll("-", " ")} · {v.authorName} ·{" "}
                        {new Date(v.at).toLocaleString()}
                      </span>
                      <small>
                        {v.kind.startsWith("invitation-") &&
                        v.detail.includes("@")
                          ? v.detail
                          : members.find(
                              (m) => m.userId === v.detail.split(":")[0],
                            )?.name || ""}
                      </small>
                    </div>
                  ))}
                </section>
              )}
            </>
          )}
        </Dialog>
      )}
    </div>
  );
}
