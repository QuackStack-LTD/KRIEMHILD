import { zipSync, strToU8 } from "fflate";
import { markdown } from "./editor.js";
import { textContent } from "./model.js";
const xml = (s) =>
  String(s || "").replace(
    /[&<>"']/g,
    (c) =>
      ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&apos;",
      })[c],
  );
export function exportText(nodes, format) {
  return nodes
    .map((n) =>
      format === "md"
        ? `# ${n.title}\n\n${markdown.serialize(n.content)}`
        : `${n.title}\n\n${textContent(n.content).trim()}`,
    )
    .join("\n\n");
}
// Minimal semantic DOCX, deliberately not a publishing/typesetting engine.
export function exportDocx(nodes) {
  const links = [];
  function runs(n) {
    if (n.type === "hardBreak") return "<w:r><w:br/></w:r>";
    if (n.type === "text") {
      const marks = (n.marks || [])
        .map((m) =>
          m.type === "bold"
            ? "<w:b/>"
            : m.type === "italic"
              ? "<w:i/>"
              : m.type === "strike"
                ? "<w:strike/>"
                : "",
        )
        .join("");
      const run = `<w:r><w:rPr>${marks}</w:rPr><w:t xml:space="preserve">${xml(n.text)}</w:t></w:r>`;
      const link = (n.marks || []).find(
        (m) =>
          m.type === "link" &&
          /^(https?:\/\/|mailto:)/i.test(m.attrs?.href || ""),
      );
      if (link) {
        links.push(link.attrs.href);
        return `<w:hyperlink r:id="link${links.length}">${run}</w:hyperlink>`;
      }
      return run;
    }
    return (n.content || []).map(runs).join("");
  }
  function blocks(n, list = false) {
    if (n.type === "horizontalRule")
      return "<w:p><w:r><w:t>***</w:t></w:r></w:p>";
    if (["paragraph", "heading", "codeBlock"].includes(n.type))
      return `<w:p>${n.type === "heading" ? `<w:pPr><w:pStyle w:val="Heading${n.attrs?.level || 1}"/></w:pPr>` : ""}${list ? "<w:r><w:t>• </w:t></w:r>" : ""}${runs(n)}</w:p>`;
    return (n.content || [])
      .map((c) => blocks(c, list || n.type === "listItem"))
      .join("");
  }
  const body = nodes
    .map(
      (n) =>
        `<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>${xml(n.title)}</w:t></w:r></w:p>` +
        blocks(n.content),
    )
    .join("");
  const files = {
    "[Content_Types].xml":
      '<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/></Types>',
    "_rels/.rels":
      '<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>',
    "word/_rels/document.xml.rels":
      '<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>',
    "word/styles.xml":
      '<?xml version="1.0"?><w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">' +
      [1, 2, 3]
        .map(
          (i) =>
            `<w:style w:type="paragraph" w:styleId="Heading${i}"><w:name w:val="heading ${i}"/><w:rPr><w:b/><w:sz w:val="${34 - i * 2}"/></w:rPr></w:style>`,
        )
        .join("") +
      "</w:styles>",
    "word/document.xml": `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>${body}<w:sectPr/></w:body></w:document>`,
  };
  files["word/_rels/document.xml.rels"] = files[
    "word/_rels/document.xml.rels"
  ].replace(
    "</Relationships>",
    links
      .map(
        (href, i) =>
          `<Relationship Id="link${i + 1}" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" TargetMode="External" Target="${xml(href)}"/>`,
      )
      .join("") + "</Relationships>",
  );
  return zipSync(
    Object.fromEntries(Object.entries(files).map(([k, v]) => [k, strToU8(v)])),
  );
}
