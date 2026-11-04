// Evaluate in the collaborative browser on a disposable project's /build page.
window.previewUIAudit = { status: "idle", stages: [] };
window.runPreviewUIAudit = async () => {
  const audit = window.previewUIAudit;
  audit.status = "running";
  const until = async (fn) => {
    for (let i = 0; i < 1800; i++) {
      if (fn()) return;
      await new Promise((r) => setTimeout(r, 100));
    }
    throw Error("preview UI timeout");
  };
  const assert = (v, message) => {
    if (!v) throw Error(message);
  };
  const svg = () =>
    document.querySelector(".studio-stage svg") ||
    document.querySelector("svg[viewBox]");
  const button = (name) =>
    [...document.querySelectorAll("button")].find(
      (b) => b.textContent.trim() === name,
    );
  const quality = () =>
    [...document.querySelectorAll("label")]
      .find((l) => l.textContent.trim().startsWith("Preview Quality"))
      ?.querySelector("select");
  const change = (element, value) => {
    element.value = value;
    element.dispatchEvent(new Event("change", { bubbles: true }));
  };
  const camera = () => svg().getAttribute("viewBox");
  const frame = () =>
    document.querySelector('[aria-label="Committed terrain viewport"]');
  const ready = () =>
    frame()?.dataset.phase === "final" &&
    document.querySelector("footer")?.textContent.endsWith("Ready");
  const stable = async () => {
    await new Promise((r) => setTimeout(r, 150));
    await until(ready);
  };
  const save = (name) => {
    const g = frame(),
      images = [...g.querySelectorAll("image")];
    assert(
      new Set(images.map((i) => i.dataset.generation)).size === 1,
      "layer generation mismatch",
    );
    assert(
      new Set(images.map((i) => i.dataset.revision)).size === 1,
      "layer revision mismatch",
    );
    audit.stages.push({
      name,
      camera: camera(),
      generation: g.dataset.generation,
      revision: g.dataset.revision,
      pixels: [g.dataset.pixelWidth, g.dataset.pixelHeight],
      resolution: document.querySelector(
        '[aria-label="Effective preview resolution"]',
      )?.textContent,
      request: window.__terrainViewportLog.findLast(
        (e) => e.event === "request",
      ),
    });
  };
  try {
    const autosave = [...document.querySelectorAll('label')].find(l=>l.textContent.trim()==='Autosave accepted terrain after 30 seconds')?.querySelector('input');
    if (autosave?.checked) autosave.click();
    window.dispatchEvent(
      new CustomEvent("terrain-diagnostic-camera", {
        detail: { x: 1.29985, y: 0.219925, w: 0.0003, h: 0.00015 },
      }),
    );
    await stable();
    const original = camera(),
      revision = frame().dataset.revision;
    for (const preset of ["performance", "balanced", "high", "custom"]) {
      change(quality(), preset);
      await stable();
      if (preset === "custom") {
        const input = [...document.querySelectorAll("label")]
          .find((l) => l.textContent.trim().startsWith("Custom preview scale"))
          ?.querySelector("input");
        Object.getOwnPropertyDescriptor(
          HTMLInputElement.prototype,
          "value",
        ).set.call(input, "1.5");
        input.dispatchEvent(new Event("input", { bubbles: true }));
        input.dispatchEvent(new Event("change", { bubbles: true }));
        await stable();
      }
      const density = {
        performance: 0.5,
        balanced: 0.75,
        high: 1,
        custom: 1.5,
      }[preset];
      assert(
        +frame().dataset.pixelWidth === Math.round(svg().clientWidth * density),
        "preview width does not match preset",
      );
      assert(
        +frame().dataset.pixelHeight ===
          Math.round(svg().clientHeight * density),
        "preview height does not match preset",
      );
      save(preset);
      assert(camera() === original, "quality moved camera");
      assert(frame().dataset.revision === revision, "quality changed terrain");
    }
    const last = [...frame().querySelectorAll("image")].map((i) =>
      i.getAttribute("href"),
    );
    change(quality(), "performance");
    change(quality(), "high");
    change(quality(), "balanced");
    // A complete prior frame remains mounted during the render debounce.
    assert(
      [...frame().querySelectorAll("image")].every(
        (i, n) => i.getAttribute("href") === last[n],
      ),
      "pending replacement erased frame",
    );
    await stable();
    save("rapid-quality-final");
    assert(quality().value === "balanced", "quality preference lost");
    assert(camera() === original, "rapid quality moved camera");
    button("Map Builder")?.click();
    await stable();
    button("Medieval Illustrator").click();
    await stable();
    save("illustrator");
    assert(!quality(), "preview control leaked into Illustrator");
    const request = window.__terrainViewportLog.findLast(
      (e) => e.event === "request",
    );
    assert(!request.preview, "Illustrator inherited preview preference");
    assert(
      request.pixels.width ===
        Math.round(svg().clientWidth * Math.min(3, devicePixelRatio || 1)),
      "Illustrator density changed",
    );
    audit.status = "complete";
  } catch (e) {
    audit.error = e.stack;
    audit.status = "failed";
  }
};
