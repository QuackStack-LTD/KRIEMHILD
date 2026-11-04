// Run in the collaborative browser's preview_evaluate on the disposable server.
// Real wheel gestures; capture the resulting camera, never infer it from count.
(() => {
  const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
  const svg = () => document.querySelector(".studio-stage svg");
  const camera = () => svg().getAttribute("viewBox").split(" ").map(Number);
  window.deepzoomResults = [];
  window.deepzoomStep = async (name, width) => {
    const start = performance.now();
    if (width === 2)
      [...document.querySelectorAll("button")]
        .find((b) => b.textContent === "Fit world")
        .click();
    else {
      for (let n = 0; n < 40; n++) {
        const c = camera();
        if (Math.abs(c[2] - width) < 1e-10) break;
        const point = new DOMPoint(1.3, 0.22).matrixTransform(
          svg().getScreenCTM(),
        );
        svg().dispatchEvent(
          new WheelEvent("wheel", {
            bubbles: true,
            cancelable: true,
            clientX: point.x,
            clientY: point.y,
            deltaY: Math.log(width / c[2]) / 0.0015,
          }),
        );
        await wait(25);
      }
    }
    await wait(200);
    for (let n = 0; n < 1800; n++) {
      const log = window.__terrainViewportLog || [];
      const req = log.filter((e) => e.event === "request").at(-1);
      const frame = document.querySelector(
        '[aria-label="Committed terrain viewport"]',
      );
      if (
        (frame?.dataset.phase === "final" &&
          +frame.dataset.generation === req?.generation) ||
        document.querySelector("footer")?.innerText.includes("Retry")
      )
        break;
      await wait(100);
    }
    const r = svg().getBoundingClientRect();
    const state = {
      name,
      duration: performance.now() - start,
      camera: camera(),
      viewport: { width: r.width, height: r.height },
      dpr: devicePixelRatio,
      footer: document.querySelector("footer")?.innerText,
      heap: performance.memory?.usedJSHeapSize,
      images: [...svg().querySelectorAll("[data-terrain-style] image")].map(
        (e) => ({
          style: e.parentNode.dataset.terrainStyle,
          ...e.dataset,
          bounds: ["x", "y", "width", "height"].map((k) => +e.getAttribute(k)),
        }),
      ),
      log: (window.__terrainViewportLog || []).filter((e) => e.time >= start),
      network: performance
        .getEntriesByType("resource")
        .filter(
          (e) => e.startTime >= start && e.name.includes("/terrain/tiles/"),
        )
        .map((e) => ({
          url: e.name,
          ms: e.duration,
          bytes: e.transferSize,
          serverTiming: e.serverTiming,
        })),
    };
    window.deepzoomResults.push(state);
    return {
      name,
      duration: state.duration,
      camera: state.camera,
      footer: state.footer,
      lod: state.images[0]?.lod,
    };
  };
  return "Deep-zoom recorder ready";
})();
