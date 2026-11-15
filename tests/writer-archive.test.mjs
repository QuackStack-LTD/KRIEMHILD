import test from "node:test";
import assert from "node:assert/strict";
import { JSDOM } from "jsdom";
import { unzipSync, strFromU8 } from "fflate";
import {
  archiveItems,
  inworldItems,
  descendants,
  crumbs,
  kindLabel,
} from "../src/worldbuilding/writer/archive-model.js";
import {
  importDocument,
  exportDocument,
} from "../src/worldbuilding/writer/archive-files.js";
import { plainDocument } from "../src/worldbuilding/writer/model.js";
const dom = new JSDOM("");
globalThis.DOMParser = dom.window.DOMParser;
globalThis.XMLSerializer = dom.window.XMLSerializer;
test("World archive keeps manuscript hierarchy, linked Ages, nested Library archives and images", () => {
  const data = {
    projects: [
      { id: "archive-w", worldId: "w" },
      { id: "novel", name: "Novel" },
    ],
    nodes: [
      {
        id: "folder",
        projectId: "archive-w",
        parentId: "",
        type: "folder",
        title: "Ideas",
      },
      {
        id: "note",
        projectId: "archive-w",
        parentId: "folder",
        type: "note",
        title: "A note",
      },
      {
        id: "chapter",
        projectId: "novel",
        parentId: "",
        type: "chapter",
        title: "Chapter",
      },
    ],
  };
  const own = archiveItems(data, "w");
  assert.equal(own.find((n) => n.id === "note").parent, "node/folder");
  assert.equal(own.find((n) => n.id === "chapter").parent, "project/novel");
  assert.equal(
    own.some((n) => n.key === "project/archive-w"),
    false,
  );
  assert.deepEqual(
    [...descendants(own, "node/folder")],
    ["node/folder", "node/note"],
  );
  assert.deepEqual(
    crumbs(own, "node/note").map((n) => n.title),
    ["Ideas", "A note"],
  );
  const linked = inworldItems({
    ages: [
      { id: "a", name: "First Age" },
      { id: "b", name: "Second Age" },
    ],
    entities: [
      {
        id: "arc",
        ageId: "a",
        kind: "archive",
        name: "Royal archive",
        properties: {},
      },
      {
        id: "sub",
        ageId: "a",
        kind: "archive",
        name: "Letters",
        properties: { parentArchive: "arc" },
      },
      {
        id: "seal",
        ageId: "a",
        kind: "certificate",
        name: "Seal",
        properties: {},
      },
    ],
    libraryFiles: [
      {
        id: "seal",
        ageId: "a",
        archiveId: "sub",
        versions: [{ name: "seal.png", mime: "image/png", size: 1 }],
      },
    ],
  });
  assert.equal(linked.find((n) => n.id === "seal").parent, "inworld/a/sub");
  assert.equal(kindLabel(linked.find((n) => n.id === "seal")), "Image");
  assert.ok(linked.find((n) => n.key === "inworld/b/"));
});
test("DOCX, Markdown and TXT exports contain edited content and DOCX reopens with formatting", async () => {
  const node = {
    title: "Edited document",
    content: {
      type: "doc",
      content: [
        {
          type: "heading",
          attrs: { level: 2 },
          content: [{ type: "text", text: "Opening" }],
        },
        {
          type: "paragraph",
          content: [
            {
              type: "text",
              text: "Світи & stories",
              marks: [{ type: "bold" }],
            },
          ],
        },
      ],
    },
  };
  const docx = await exportDocument(node, "docx"),
    files = unzipSync(docx),
    xml = strFromU8(files["word/document.xml"]);
  assert.match(xml, /Світи &amp; stories/);
  assert.match(xml, /<w:b\/>/);
  const reopened = await importDocument(docx, "story.docx");
  assert.equal(reopened.content.content[0].type, "heading");
  assert.equal(reopened.content.content[1].content[0].marks[0].type, "bold");
  const md = new TextDecoder().decode(await exportDocument(node, "md"));
  assert.match(md, /## Opening/);
  assert.match(md, /\*\*Світи &amp; stories\*\*/);
  const txt = new TextDecoder().decode(await exportDocument(node, "txt"));
  assert.match(txt, /Світи & stories/);
  const text = await importDocument(
    new TextEncoder().encode("first\nsecond"),
    "notes.txt",
  );
  assert.deepEqual(text.content, plainDocument("first\nsecond"));
  assert.equal(
    await importDocument(new Uint8Array([0, 255]), "drawing.bin"),
    null,
  );
});
