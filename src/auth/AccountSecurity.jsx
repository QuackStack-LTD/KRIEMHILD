import { useEffect, useState } from "react";
import { accountRequest } from "./api.js";
import {
  allowNavigation,
  confirmAction,
} from "../worldbuilding/ui/feedback.js";

export default function AccountSecurity({ mode }) {
  const [account, setAccount] = useState(null),
    [error, setError] = useState(""),
    [pending, setPending] = useState(false),
    [confirmation, setConfirmation] = useState("");
  useEffect(() => {
    let active = true;
    accountRequest("/api/account")
      .then((v) => {
        if (active) setAccount(v);
      })
      .catch((e) => {
        if (active) setError(e.message);
      });
    return () => {
      active = false;
    };
  }, []);
  async function perform(close) {
    if (!(await allowNavigation())) return;
    if (
      !(await confirmAction(
        close
          ? "Close your KRIEMHILD account? Your access will end on every device. Shared content and revision attribution are retained. Your identity-provider account is managed separately."
          : "Sign out of KRIEMHILD on every device?",
      ))
    )
      return;
    setPending(true);
    setError("");
    try {
      await accountRequest(
        close ? "/api/account" : "/api/account/sessions/revoke",
        close ? { confirmation } : {},
        close ? "DELETE" : "POST",
      );
      window.dispatchEvent(new Event("session-expired"));
    } catch (e) {
      setError(e.message);
      setPending(false);
    }
  }
  if (mode === "local") return null;
  return (
    <section className="account-security" aria-label="Account security">
      <h3>Sessions and account</h3>
      {error && <p role="alert">{error}</p>}
      {account && (
        <>
          <p>{account.activeSessions} active login sessions.</p>
          <button disabled={pending} onClick={() => perform(false)}>
            Sign out everywhere
          </button>
          <p>
            Closing this account removes your access and profile information.
            Shared content and its author ID remain in history. Your
            identity-provider account is managed separately.
          </p>
          {account.ownedWorlds.length > 0 ? (
            <>
              <p>
                Transfer ownership or explicitly delete these worlds before
                closing your account:
              </p>
              <ul>
                {account.ownedWorlds.map((w) => (
                  <li key={w.id}>{w.name}</li>
                ))}
              </ul>
            </>
          ) : (
            <label>
              Type CLOSE to close your account
              <input
                value={confirmation}
                onChange={(e) => setConfirmation(e.target.value)}
                autoComplete="off"
              />
            </label>
          )}
          <button
            className="danger"
            disabled={
              pending ||
              account.ownedWorlds.length > 0 ||
              confirmation !== "CLOSE"
            }
            onClick={() => perform(true)}
          >
            Close account
          </button>
        </>
      )}
    </section>
  );
}
