let csrfToken = "";
export function setCSRF(value) {
  csrfToken = value || "";
}
export function installAuthenticatedFetch() {
  const original = window.fetch.bind(window),
    mapRevisions = new Map();
  window.fetch = async (input, init = {}) => {
    const url = new URL(
      typeof input === "string" ? input : input.url,
      location.href,
    );
    if (url.origin !== location.origin || !/^\/(api|auth)\//.test(url.pathname))
      return original(input, init);
    const headers = new Headers(
      init.headers || (input instanceof Request ? input.headers : undefined),
    );
    const method = (init.method || input.method || "GET").toUpperCase();
    const session = url.pathname.match(/^\/api\/sessions\/([^/]+)/)?.[1];
    if (!["GET", "HEAD", "OPTIONS"].includes(method)) {
      if (csrfToken) headers.set("X-CSRF-Token", csrfToken);
      if (session && mapRevisions.has(session))
        headers.set("If-Match", String(mapRevisions.get(session)));
    }
    const response = await original(input, {
      ...init,
      headers,
      credentials: "same-origin",
    });
    if (response.status === 401)
      window.dispatchEvent(new Event("session-expired"));
    if (
      response.ok &&
      session &&
      (method !== "GET" || url.pathname.endsWith("/world/state"))
    ) {
      const revision = response.headers.get("X-Map-Revision");
      if (revision !== null) mapRevisions.set(session, Number(revision));
    }
    if (
      response.ok &&
      (url.pathname === "/api/sessions" ||
        /\/api\/projects\/(import|[^/]+\/open)$/.test(url.pathname))
    ) {
      const data = await response.clone().json();
      if (data.id) mapRevisions.set(data.id, data.mapRevision ?? 0);
    }
    return response;
  };
}
export async function accountRequest(path, body, method) {
  const response = await fetch(path, {
    method: method || (body === undefined ? "GET" : "POST"),
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (response.status === 204) return null;
  const data = await response.json();
  if (!response.ok) {
    const error = new Error(
      data.error || `Request failed (${response.status})`,
    );
    error.status = response.status;
    throw error;
  }
  return data;
}
