import { unzipSync, zipSync, strToU8, strFromU8 } from "fflate";
import { MarkdownManager } from "@tiptap/markdown";
import { archiveExtensions } from "./archive-extensions.js";
import { plainDocument, textContent } from "./model.js";
import { openDocx } from "../library-files.js";
import { exportDocx } from "./export.js";
const markdown = new MarkdownManager({ extensions: archiveExtensions });
const wordNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main";
export const mimeFor = (format) =>
  ({
    docx: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    pdf: "application/pdf",
    txt: "text/plain;charset=utf-8",
    md: "text/markdown;charset=utf-8",
  })[format];
export async function importDocument(bytes, name) {
  const ext = name.split(".").pop().toLowerCase();
  if (["txt", "md", "markdown"].includes(ext)) {
    const text = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
    return {
      content: ext === "txt" ? plainDocument(text) : markdown.parse(text),
      format: ext === "markdown" ? "md" : ext,
    };
  }
  if (ext === "docx") {
    const model = openDocx(bytes),
      content = model.nodes.map((p) => {
        const runs = [...p.getElementsByTagNameNS(wordNS, "r")].flatMap((r) => {
          const marks = [
            ["b", "bold"],
            ["i", "italic"],
            ["u", "underline"],
            ["strike", "strike"],
          ]
            .filter(([tag]) => {
              const v = r.getElementsByTagNameNS(wordNS, tag)[0];
              return (
                v &&
                !["0", "false", "none"].includes(
                  v.getAttributeNS(wordNS, "val"),
                )
              );
            })
            .map(([, type]) => ({ type }));
          return [...r.children].flatMap((el) =>
            el.localName === "t" && el.textContent
              ? [
                  {
                    type: "text",
                    text: el.textContent,
                    ...(marks.length ? { marks } : {}),
                  },
                ]
              : el.localName === "br"
                ? [{ type: "hardBreak" }]
                : el.localName === "tab"
                  ? [{ type: "text", text: "\t" }]
                  : [],
          );
        });
        const style =
          p
            .getElementsByTagNameNS(wordNS, "pStyle")[0]
            ?.getAttributeNS(wordNS, "val") || "";
        const level = Number(style.match(/heading\s*([1-6])/i)?.[1]);
        return {
          type: level ? "heading" : "paragraph",
          ...(level ? { attrs: { level } } : {}),
          ...(runs.length ? { content: runs } : {}),
        };
      });
    return {
      content: {
        type: "doc",
        content: content.length ? content : plainDocument().content,
      },
      format: "docx",
      notice:
        "DOCX imported as editable text. The original file keeps its complete layout and embedded objects.",
    };
  }
  if (ext === "pdf") {
    const pdfjs = await import("pdfjs-dist"),
      worker = await import("pdfjs-dist/build/pdf.worker.min.mjs?url");
    pdfjs.GlobalWorkerOptions.workerSrc = worker.default;
    const task = pdfjs.getDocument({
      data: bytes.slice(),
      isEvalSupported: false,
    });
    try {
      const pdf = await task.promise;
      if (pdf.numPages > 500) return null;
      const lines = [];
      for (let i = 1; i <= pdf.numPages; i++) {
        const page = await pdf.getPage(i),
          data = await page.getTextContent();
        let line = "",
          previousY = null;
        for (const item of data.items) {
          if (!("str" in item)) continue;
          const y = item.transform[5];
          if (previousY != null && Math.abs(y - previousY) > 3 && line) {
            lines.push(line);
            line = "";
          }
          line += item.str + (item.hasEOL ? "" : " ");
          previousY = y;
          if (item.hasEOL) {
            lines.push(line);
            line = "";
          }
        }
        if (line) lines.push(line);
        lines.push("");
        page.cleanup();
      }
      if (!lines.join("").trim()) return null;
      return {
        content: plainDocument(lines.join("\n")),
        format: "pdf",
        notice:
          "PDF text is editable here. The original PDF is preserved; page layout may differ after editing. Scanned pages need OCR before their text can be edited.",
      };
    } finally {
      await task.destroy();
    }
  }
  return null;
}
let fontBytes;
async function pdfFont(pdf) {
  const [{ default: fontkit }, { default: fontURL }] = await Promise.all([
    import("./pdf-fontkit.js"),
    import("../../assets/fonts/NotoSans-Regular.ttf?url"),
  ]);
  fontBytes ||= fetch(fontURL)
    .then((r) => {
      if (!r.ok) throw Error("Unable to load the document font");
      return r.arrayBuffer();
    })
    .catch((e) => {
      fontBytes = null;
      throw e;
    });
  pdf.registerFontkit(fontkit);
  return pdf.embedFont(await fontBytes, { subset: true });
}
export async function exportPDF(node) {
  const { PDFDocument, rgb } = await import("pdf-lib"),
    pdf = await PDFDocument.create(),
    font = await pdfFont(pdf);
  pdf.setTitle(node.title);
  pdf.setCreator("KRIEMHILD Writer's Archive");
  let page = pdf.addPage([595.28, 841.89]),
    y = 780;
  const next = () => {
    page = pdf.addPage([595.28, 841.89]);
    y = 780;
  };
  const line = (text, size) => {
    if (y < 60) next();
    page.drawText(text, { x: 58, y, size, font, color: rgb(0.12, 0.14, 0.12) });
    y -= size * 1.55;
  };
  async function block(n) {
    if (n.type === "image") {
      const r = await fetch(n.attrs.src);
      if (!r.ok) throw Error("Unable to export an embedded image");
      const bytes = await r.arrayBuffer(),
        mime = r.headers.get("Content-Type") || "";
      const img = mime.includes("jpeg")
        ? await pdf.embedJpg(bytes)
        : mime.includes("png")
          ? await pdf.embedPng(bytes)
          : await pdf.embedPng(await rasterImage(bytes, mime));
      const scale = Math.min(479 / img.width, 650 / img.height, 1),
        width = img.width * scale,
        height = img.height * scale;
      if (y - height < 60) next();
      page.drawImage(img, { x: 58, y: y - height, width, height });
      y -= height + 20;
      return;
    }
    if (["paragraph", "heading", "codeBlock"].includes(n.type)) {
      const size = n.type === "heading" ? 25 - (n.attrs?.level || 1) * 2 : 11,
        text = textContent(n).replace(/\n$/, "");
      for (const raw of text.split("\n")) {
        let current = "";
        for (const char of raw) {
          if (font.widthOfTextAtSize(current + char, size) > 479 && current) {
            line(current, size);
            current = "";
          }
          current += char;
        }
        line(current, size);
      }
      y -= 6;
      return;
    }
    if (n.type === "horizontalRule") {
      if (y < 70) next();
      page.drawLine({
        start: { x: 58, y },
        end: { x: 537, y },
        color: rgb(0.65, 0.65, 0.6),
        thickness: 0.5,
      });
      y -= 18;
      return;
    }
    for (const child of n.content || []) await block(child);
  }
  await block(node.content);
  return pdf.save();
}
async function rasterImage(bytes, mime) {
  const bitmap = await createImageBitmap(new Blob([bytes], { type: mime }));
  try {
    const canvas = document.createElement("canvas");
    canvas.width = bitmap.width;
    canvas.height = bitmap.height;
    canvas.getContext("2d").drawImage(bitmap, 0, 0);
    return new Uint8Array(
      await (
        await new Promise((resolve) => canvas.toBlob(resolve, "image/png"))
      ).arrayBuffer(),
    );
  } finally {
    bitmap.close();
  }
}
async function docxWithImages(node) {
  const images = [],
    content = structuredClone(node.content);
  function walk(n) {
    if (n.type === "image") {
      const id = images.push(structuredClone(n));
      n.type = "paragraph";
      n.content = [{ type: "text", text: `KRIEMHILDIMAGE${id}PLACEHOLDER` }];
      delete n.attrs;
    } else for (const child of n.content || []) walk(child);
  }
  walk(content);
  const files = unzipSync(exportDocx([{ ...node, title: "", content }]));
  let xml = strFromU8(files["word/document.xml"]).replace(
    /<w:p><w:pPr><w:pStyle w:val="Heading1"\/><\/w:pPr><w:r><w:t><\/w:t><\/w:r><\/w:p>/,
    "",
  );
  let rels = strFromU8(files["word/_rels/document.xml.rels"]),
    types = strFromU8(files["[Content_Types].xml"]);
  for (let i = 0; i < images.length; i++) {
    const r = await fetch(images[i].attrs.src);
    if (!r.ok) throw Error("Unable to export an embedded image");
    let bytes = new Uint8Array(await r.arrayBuffer()),
      mime = r.headers.get("Content-Type") || "";
    const jpeg = mime.includes("jpeg");
    if (!jpeg && !mime.includes("png")) bytes = await rasterImage(bytes, mime);
    const ext = jpeg ? "jpg" : "png",
      name = `image${i + 1}.${ext}`;
    files["word/media/" + name] = bytes;
    const bitmap = await createImageBitmap(
      new Blob([bytes], { type: jpeg ? "image/jpeg" : "image/png" }),
    );
    const scale = Math.min(575 / bitmap.width, 750 / bitmap.height, 1),
      cx = Math.round(bitmap.width * scale * 9525),
      cy = Math.round(bitmap.height * scale * 9525);
    bitmap.close();
    const drawing = `<w:drawing><wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"><wp:extent cx="${cx}" cy="${cy}"/><wp:docPr id="${i + 1}" name="Image ${i + 1}"/><a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:nvPicPr><pic:cNvPr id="${i + 1}" name="${name}"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="archiveImage${i}"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="${cx}" cy="${cy}"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing>`;
    xml = xml.replace(
      new RegExp(`<w:t[^>]*>KRIEMHILDIMAGE${i + 1}PLACEHOLDER<\\/w:t>`),
      drawing,
    );
    rels = rels.replace(
      "</Relationships>",
      `<Relationship Id="archiveImage${i}" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/${name}"/></Relationships>`,
    );
    if (!types.includes(`Extension="${ext}"`))
      types = types.replace(
        "</Types>",
        `<Default Extension="${ext}" ContentType="image/${jpeg ? "jpeg" : "png"}"/></Types>`,
      );
  }
  files["word/document.xml"] = strToU8(xml);
  files["word/_rels/document.xml.rels"] = strToU8(rels);
  files["[Content_Types].xml"] = strToU8(types);
  return zipSync(files);
}
export async function exportDocument(node, format) {
  if (format === "pdf") return exportPDF(node);
  if (format === "docx") return docxWithImages(node);
  return new TextEncoder().encode(
    format === "md"
      ? markdown.serialize(node.content)
      : textContent(node.content),
  );
}
