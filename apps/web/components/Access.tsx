"use client";
import { useEffect, useState } from "react";
import { api, State } from "../lib/types";
import AccountSecurity from "./AccountSecurity";
export function Login() {
  const [user, setUser] = useState(""),
    [password, setPassword] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  return (
    <main className="library-main">
      <h1>Welcome to KRIEMHILD.</h1>
      <form
        className="card"
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          try {
            await api("/login", {
              method: "POST",
              body: JSON.stringify({ user, password }),
            });
            location.reload();
          } catch (e) {
            setError((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      >
        <label>
          Account name
          <input
            required
            autoComplete="username"
            value={user}
            onChange={(e) => setUser(e.target.value)}
          />
        </label>
        <label>
          Password
          <input
            required
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>
        <button disabled={busy}>Sign in</button>
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
      </form>
    </main>
  );
}
export default function Access({ state }: { state: State }) {
  const [session, setSession] = useState<{ hosted?: boolean; user?: string }>(
      {},
    ),
    [members, setMembers] = useState<Record<string, string>>({}),
    [user, setUser] = useState(""),
    [password, setPassword] = useState(""),
    [role, setRole] = useState("viewer"),
    [error, setError] = useState(""),
    [status, setStatus] = useState("");
  const base = `/projects/${state.root.world.id}`;
  const refresh = () =>
    api<Record<string, string>>(base + "/members")
      .then(setMembers)
      .catch((e) => setError(e.message));
  useEffect(() => {
    api<typeof session>("/session").then(setSession);
    if (!state.role || state.role === "owner") void refresh();
  }, [state.root.world.id, state.role]);
  return (
    <section>
      <h2>Workspace access</h2>
      {!session.hosted ? (
        <p>
          This is a local workspace. Hosted accounts are enabled explicitly with
          the server's origin setting.
        </p>
      ) : (
        <>
          <p>Signed in as {session.user}</p>
          <AccountSecurity administrator={session.user === "admin"} />
          <button
            onClick={async () => {
              await api("/logout", { method: "POST" });
              location.href = "/";
            }}
          >
            Sign out
          </button>
          {(!state.role || state.role === "owner") && (
            <section className="card">
              <h3>World members</h3>
              <ul>
                {Object.entries(members).map(([name, role]) => (
                  <li key={name}>
                    {name} · {role}
                  </li>
                ))}
              </ul>
              <form
                onSubmit={async (e) => {
                  e.preventDefault();
                  setError("");
                  try {
                    await api(base + "/members", {
                      method: "POST",
                      body: JSON.stringify({ user, role }),
                    });
                    await refresh();
                    setStatus("Membership updated.");
                  } catch (e) {
                    setError((e as Error).message);
                  }
                }}
              >
                <label>
                  Member account
                  <input
                    required
                    value={user}
                    onChange={(e) => setUser(e.target.value)}
                  />
                </label>
                <label>
                  World role
                  <select
                    value={role}
                    onChange={(e) => setRole(e.target.value)}
                  >
                    {["viewer", "editor", "owner", "remove"].map((r) => (
                      <option key={r}>{r}</option>
                    ))}
                  </select>
                </label>
                <button>Update membership</button>
              </form>
            </section>
          )}
          {state.role && state.role !== "owner" && (
            <p>
              Your world role is {state.role}. Ask a world owner to change
              membership.
            </p>
          )}
          {session.user === "admin" && (
            <form
              className="card"
              onSubmit={async (e) => {
                e.preventDefault();
                try {
                  await api("/accounts", {
                    method: "POST",
                    body: JSON.stringify({ user, password }),
                  });
                  setPassword("");
                  setStatus(
                    "Account created. Add it to this world with a role.",
                  );
                } catch (e) {
                  setError((e as Error).message);
                }
              }}
            >
              <h3>Create an account</h3>
              <label>
                New account name
                <input
                  required
                  value={user}
                  onChange={(e) => setUser(e.target.value)}
                />
              </label>
              <label>
                Initial password
                <input
                  required
                  minLength={12}
                  type="password"
                  autoComplete="new-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </label>
              <button>Create account</button>
            </form>
          )}
        </>
      )}
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {status && <p role="status">{status}</p>}
    </section>
  );
}
