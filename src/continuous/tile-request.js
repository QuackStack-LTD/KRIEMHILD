// Three attempts, bounded deadlines and abortable backoff. Obsolete camera work
// never retries; current work can recover without a refresh or another gesture.
function backoff(ms, signal) {
  return new Promise((resolve, reject) => {
    const cancel = () => {
      clearTimeout(timer);
      reject(signal.reason);
    };
    const timer = setTimeout(() => {
      signal.removeEventListener("abort", cancel);
      resolve();
    }, ms);
    signal.addEventListener("abort", cancel, { once: true });
    if (signal.aborted) cancel();
  });
}

export async function requestTile(
  url,
  revision,
  signal,
  {
    attempts = 3,
    timeout = 15000,
    delays = [250, 750],
    record = () => {},
  } = {},
) {
  for (let attempt = 0; attempt < attempts; attempt++) {
    signal.throwIfAborted();
    const deadline = new AbortController();
    const timer = setTimeout(
      () =>
        deadline.abort(
          new DOMException("Tile deadline exceeded", "TimeoutError"),
        ),
      timeout,
    );
    const started = performance.now();
    let status,
      retry = true;
    record({ event: "start", url, attempt, time: started });
    try {
      const r = await fetch(url, {
        signal: AbortSignal.any([signal, deadline.signal]),
      });
      status = r.status;
      retry = status === 408 || status === 429 || status >= 500;
      if (!r.ok) throw Error(`Terrain tile HTTP ${status}`);
      retry = false;
      if (r.headers.get("X-Terrain-Revision") !== String(revision))
        throw Error("Terrain revision mismatch");
      // A timeout reading the body is transient too.
      retry = true;
      const blob = await r.blob();
      signal.throwIfAborted();
      record({
        event: "complete",
        url,
        attempt,
        status,
        bytes: blob.size,
        revision,
        time: performance.now(),
        duration: performance.now() - started,
        cache: r.headers.get("X-Terrain-Cache"),
        timing: r.headers.get("Server-Timing"),
      });
      return blob;
    } catch (error) {
      record({
        event: signal.aborted ? "obsolete" : "error",
        url,
        attempt,
        status,
        error: error.name,
        message: error.message,
        time: performance.now(),
      });
      if (signal.aborted || !retry || attempt === attempts - 1) throw error;
    } finally {
      clearTimeout(timer);
    }
    await backoff(delays[Math.min(attempt, delays.length - 1)], signal);
  }
}
