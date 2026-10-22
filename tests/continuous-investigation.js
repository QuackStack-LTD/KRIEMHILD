(() => {
  const pause = (ms) => new Promise((r) => setTimeout(r, ms));
  const svg = () => document.querySelector(".studio-stage svg");
  const button = (name) =>
    [...document.querySelectorAll("button")].find(
      (e) => e.textContent.trim().toLowerCase() === name.toLowerCase(),
    );
  window.terrainAudit = {
    state() {
      const s = svg(),
        r = s.getBoundingClientRect(),
        inv = s.getScreenCTM().inverse();
      const tiles = [
        ...s.querySelectorAll('g[data-terrain-style="natural"] image'),
      ].map((e) => ({
        tile: e.dataset.tile,
        revision: e.dataset.revision,
        x: +e.getAttribute("x"),
        y: +e.getAttribute("y"),
        span: +e.getAttribute("width"),
        height: +e.getAttribute("height"),
        z: +(e.dataset.lod ?? e.dataset.tile?.split("/")[0]),
      }));
      const maxZ = Math.max(...tiles.map((t) => t.z));
      let missing = 0;
      for (let y = 1; y < r.height; y += 17)
        for (let x = 1; x < r.width; x += 17) {
          const p = new DOMPoint(r.left + x, r.top + y).matrixTransform(inv);
          if (p.x < 0 || p.x >= 2 || p.y < 0 || p.y >= 1) continue;
          if (
            !tiles.some(
              (t) =>
                t.z === maxZ &&
                p.x >= t.x &&
                p.x < t.x + t.span &&
                p.y >= t.y &&
                p.y < t.y + t.height,
            )
          )
            missing++;
        }
      return {
        view: s.getAttribute("viewBox"),
        rect: r.toJSON(),
        maxZ,
        missing,
        tiles,
        footer: document.querySelector("footer")?.innerText,
        log: window.__terrainViewportLog || window.__terrainTileLog || [],
        fault: window.auditFault,
      };
    },
    async settle() {
      for (let i = 0; i < 200; i++) {
        if (document.querySelector("footer")?.innerText.endsWith("Ready")) {
          await pause(350);
          return this.state();
        }
        await pause(100);
      }
      return this.state();
    },
    async fit() {
      button("Fit world").click();
      await pause(100);
      return this.settle();
    },
    async zoom(n, delay = 700, delta = -266) {
      for (let i = 0; i < n; i++) {
        const s = svg(),
          r = s.getBoundingClientRect();
        s.dispatchEvent(
          new WheelEvent("wheel", {
            clientX: r.left + r.width * 0.42,
            clientY: r.top + r.height * 0.4,
            deltaY: delta,
            bubbles: true,
            cancelable: true,
          }),
        );
        await pause(delay);
      }
      return this.settle();
    },
    async pan(dx, dy) {
      const s = svg(),
        r = s.getBoundingClientRect(),
        capture = s.setPointerCapture;
      s.setPointerCapture = () => {};
      const init = {
        clientX: r.left + r.width * 0.1,
        clientY: r.top + r.height * 0.1,
        button: 0,
        buttons: 1,
        pointerId: 99,
        bubbles: true,
      };
      s.dispatchEvent(new PointerEvent("pointerdown", init));
      s.dispatchEvent(
        new PointerEvent("pointermove", {
          ...init,
          clientX: init.clientX + dx,
          clientY: init.clientY + dy,
        }),
      );
      s.dispatchEvent(
        new PointerEvent("pointerup", {
          ...init,
          buttons: 0,
          clientX: init.clientX + dx,
          clientY: init.clientY + dy,
        }),
      );
      s.setPointerCapture = capture;
      await pause(100);
      return this.settle();
    },
    failOnce() {
      const real = window.fetch;
      window.auditFault = { attempts: 0, url: null };
      window.fetch = (u, o) => {
        const f = window.auditFault,
          url = String(u);
        if (
          url.includes("/terrain/tiles/") &&
          url.includes("style=natural") &&
          (!f.url || url === f.url)
        ) {
          f.url = url;
          f.attempts++;
          if (f.attempts === 1)
            return Promise.resolve(
              new Response("fixture 503", { status: 503 }),
            );
        }
        return real(u, o);
      };
    },
    async capture(debug = false) {
      const src = svg(),
        s = src.cloneNode(true),
        r = src.getBoundingClientRect();
      if (!debug)
        s.querySelectorAll('[aria-label="Tile diagnostics"]').forEach((e) =>
          e.remove(),
        );
      s.setAttribute("xmlns", "http://www.w3.org/2000/svg");
      s.setAttribute("width", r.width);
      s.setAttribute("height", r.height);
      await Promise.all(
        [...s.querySelectorAll("image")].map(async (e) => {
          const b = await (await fetch(e.getAttribute("href"))).blob();
          e.setAttribute(
            "href",
            await new Promise((resolve) => {
              const f = new FileReader();
              f.onload = () => resolve(f.result);
              f.readAsDataURL(b);
            }),
          );
        }),
      );
      const img = new Image();
      img.src =
        "data:image/svg+xml;base64," +
        btoa(unescape(encodeURIComponent(s.outerHTML)));
      await img.decode();
      const c = document.createElement("canvas");
      c.width = r.width;
      c.height = r.height;
      c.getContext("2d").drawImage(img, 0, 0);
      window.terrainCapture = c.toDataURL("image/png").split(",")[1];
      return {
        bytes: terrainCapture.length,
        view: src.getAttribute("viewBox"),
      };
    },
  };
  return terrainAudit.state();
})();
