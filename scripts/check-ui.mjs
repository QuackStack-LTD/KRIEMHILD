import { readFile, readdir } from "node:fs/promises";
import assert from "node:assert/strict";
import postcss from "postcss";

async function files(dir) {
  const entries = await readdir(dir, { withFileTypes: true });
  return (
    await Promise.all(
      entries.map((e) =>
        e.isDirectory() ? files(`${dir}/${e.name}`) : `${dir}/${e.name}`,
      ),
    )
  ).flat();
}
const source = (await files("src")).filter((f) => /\.(jsx|js|css)$/.test(f));
for (const path of source) {
  const text = await readFile(path, "utf8");
  assert.ok(!text.startsWith("\uFEFF"), `${path}: remove byte-order mark`);
  if (/\.(jsx|js)$/.test(path)) {
    assert.ok(
      !/\b(?:window\.)?(?:alert|prompt|confirm)\s*\(/.test(text),
      `${path}: use asynchronous UI feedback`,
    );
    assert.ok(
      !/className=["'](?:project-dialog-backdrop|dialog-backdrop)["']/.test(
        text,
      ),
      `${path}: use Dialog`,
    );
  }
  if (path.endsWith(".css")) {
    const css = postcss.parse(text);
    css.walkRules((rule) => {
      assert.ok(
        !/\.project-dialog\s*>\s*div/.test(rule.selector),
        `${path}: dialog content must not be styled as a footer`,
      );
      if (
        path === "src/style.css" &&
        !/keyframes/.test(rule.parent.name || "")
      ) {
        assert.ok(
          rule.selectors.every((s) => s.includes(".terrain-app")),
          `${path}: terrain styling escaped its root: ${rule.selector}`,
        );
      }
      if (rule.selector.includes(".institution-tabs"))
        assert.equal(
          path,
          "src/worldbuilding/ui.css",
          "Shared tabs belong in ui.css",
        );
    });
  }
}
const main = await readFile("src/main.jsx", "utf8");
assert.ok(
  main.includes("UIProvider") && main.includes("worldbuilding/ui.css"),
  "The application must mount shared feedback and styles",
);
const app = await readFile("src/App.jsx", "utf8");
assert.ok(
  !app.includes("FutureWorkspace"),
  "Remove the unused placeholder import",
);
assert.ok(app.includes("flora"), "Retain the legacy Biosphere route");
console.log(
  "UI boundaries, shared tabs, dialog migration and compatibility checks passed.",
);
