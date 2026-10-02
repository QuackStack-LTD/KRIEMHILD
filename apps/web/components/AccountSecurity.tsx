"use client";
import { useState } from "react";
import { api } from "../lib/types";

export default function AccountSecurity({
  administrator = false,
}: {
  administrator?: boolean;
}) {
  const [current, setCurrent] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [target, setTarget] = useState("");
  const [reset, setReset] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [status, setStatus] = useState("");
  return (
    <details className="card">
      <summary>Password and sessions</summary>
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          setError("");
          setStatus("");
          if (password !== confirm) {
            setError("The new passwords do not match.");
            return;
          }
          setBusy(true);
          try {
            const result = await api<{ signInRequired: boolean }>(
              reset ? "/accounts/reset-password" : "/password",
              {
                method: "POST",
                body: JSON.stringify({
                  current,
                  password,
                  ...(reset ? { user: target } : {}),
                }),
              },
            );
            setCurrent("");
            setPassword("");
            setConfirm("");
            if (result.signInRequired) location.href = "/";
            else
              setStatus(
                "Password reset. All sessions for that account have been signed out.",
              );
          } catch (e) {
            setError((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      >
        <p>
          Changing a password signs that account out on every device. Save any
          shared writing drafts first.
        </p>
        {administrator && (
          <label>
            <input
              type="checkbox"
              checked={reset}
              onChange={(e) => setReset(e.target.checked)}
            />{" "}
            Reset another account's password
          </label>
        )}
        {reset && (
          <label>
            Account to reset
            <input
              required
              autoComplete="off"
              value={target}
              onChange={(e) => setTarget(e.target.value)}
            />
          </label>
        )}
        <label>
          Your current password
          <input
            required
            type="password"
            autoComplete="current-password"
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
          />
        </label>
        <label>
          New password
          <input
            required
            type="password"
            autoComplete="new-password"
            minLength={12}
            maxLength={512}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>
        <label>
          Confirm new password
          <input
            required
            type="password"
            autoComplete="new-password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
          />
        </label>
        <button disabled={busy}>
          {reset ? "Reset password" : "Change password and sign out"}
        </button>
        {error && <p role="alert">{error}</p>}
        {status && <p role="status">{status}</p>}
      </form>
    </details>
  );
}
