import { createContext, useContext, useEffect, useState } from "react";
import { accountRequest, setCSRF } from "./api.js";
import "./auth.css";
const AuthContext = createContext(null);
export const useAuth = () => useContext(AuthContext);
export default function AuthProvider({ children }) {
  const [account, setAccount] = useState(null),
    [state, setState] = useState("loading"),
    [error, setError] = useState("");
  async function load() {
    setState("loading");
    try {
      const data = await accountRequest("/api/me");
      setCSRF(data.csrfToken);
      setAccount(data);
      setState("authenticated");
    } catch (e) {
      setState(e.status === 401 ? "unauthenticated" : "error");
      setError(
        e.status === 404
          ? "Restart the KRIEMHILD backend to load account and collaboration support."
          : e.message,
      );
    }
  }
  useEffect(() => {
    void load();
    const expired = () => {
      setCSRF("");
      setAccount(null);
      setState("expired");
    };
    window.addEventListener("session-expired", expired);
    return () => window.removeEventListener("session-expired", expired);
  }, []);
  if (state !== "authenticated")
    return (
      <main className="account-welcome">
        <span className="eyebrow">KRIEMHILD</span>
        <h1>Your worlds, together</h1>
        {state === "loading" ? (
          <p role="status">Opening your workspace…</p>
        ) : state === "error" ? (
          <>
            <p role="alert">{error}</p>
            <button onClick={load}>Retry connection</button>
          </>
        ) : (
          <>
            <p>
              {state === "expired"
                ? "Your session expired. Sign in again to continue."
                : "Sign in to open your worlds and collaborate."}
            </p>
            <a className="account-signin" href="/auth/login">
              Sign in
            </a>
            <p>
              Registration, password recovery, and account verification are
              available through your identity provider.
            </p>
          </>
        )}
      </main>
    );
  return (
    <AuthContext.Provider
      value={{
        ...account,
        async logout() {
          const result = await accountRequest("/auth/logout", {});
          location.assign(result?.redirect || "/");
        },
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
