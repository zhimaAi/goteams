import DOMPurify from "dompurify";
import { marked } from "marked";

const CODE_REGION_RE = /(```[\s\S]*?```|~~~[\s\S]*?~~~|`[^`\n]*`)/g;
const DOC_REFERENCE_RE = /#\[([^\]\n]{1,200})\]/g;
// Placeholders survive markdown parsing: they are plain ASCII with no syntax meaning,
// so a document path containing `*`, `_` or brackets cannot break the surrounding text.
const DOC_PLACEHOLDER_RE = /@@GTDOC(\d+)@@/g;
const MAX_LINKIFIED_PATH = 512;

function escapeHtml(value) {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

// 与客户端 MarkdownPreview 一致：折叠多余空行，单换行仍留给 breaks 保留。
function normalizeMarkdown(value) {
  return String(value || "")
    .replace(/\r\n/g, "\n")
    .replace(/\n{3,}/g, "\n\n")
    .replace(/[ \t]+\n/g, "\n")
    .trim();
}

function isCodeRegion(part) {
  return (
    part.startsWith("```") ||
    part.startsWith("~~~") ||
    (part.startsWith("`") && part.endsWith("`") && part.length > 1)
  );
}

function baseName(path) {
  const segments = String(path || "").split("/");
  return segments[segments.length - 1] || "";
}

function stripExtension(name) {
  return String(name || "").replace(/\.[^./]+$/, "");
}

function docAnchor(label) {
  return `<span class="markdown-doc-reference">${escapeHtml(label)}</span>`;
}

// `#[文件名]` is the client's persisted document reference. A document in this task's
// list becomes a link; anything else keeps the previous non-clickable label.
function renderDocReferences(part, documents, placeholders) {
  return part.replace(DOC_REFERENCE_RE, (match, name) => {
    const label = String(name).split(" · ")[0].trim();
    if (!label) return match;
    const doc = findDocumentByName(documents, label);
    if (!doc) return docAnchor(label);
    return addPlaceholder(placeholders, doc, label);
  });
}

function findDocumentByName(documents, name) {
  const target = String(name || "").toLowerCase();
  if (!target) return null;
  return (
    documents.find((doc) => String(doc.title || "").toLowerCase() === target) ||
    documents.find((doc) => baseName(doc.rel_path).toLowerCase() === target) ||
    documents.find(
      (doc) => stripExtension(baseName(doc.rel_path)).toLowerCase() === target,
    ) ||
    null
  );
}

// Literal matching only: no regex is built from user paths, so Windows separators,
// brackets and CJK characters cannot turn into a pattern.
function linkifyDocPaths(part, documents, placeholders) {
  const targets = [];
  documents.forEach((doc) => {
    const base = baseName(doc.rel_path);
    const candidates = [doc.source_abs_path, doc.rel_path];
    // A bare file name is only linkified when it still looks like a file name;
    // short names such as "a.md" would otherwise hit unrelated prose.
    if (base.length >= 6 && base.includes(".")) {
      candidates.push(base);
    }
    candidates.forEach((value) => {
      const path = String(value || "").trim();
      if (!path || path.length > MAX_LINKIFIED_PATH) return;
      targets.push({ path, doc });
    });
  });
  targets.sort((left, right) => right.path.length - left.path.length);
  return targets.reduce((text, target) => {
    if (!text.includes(target.path)) return text;
    const token = addPlaceholder(placeholders, target.doc, target.path);
    return text.split(target.path).join(token);
  }, part);
}

function addPlaceholder(placeholders, doc, label) {
  const token = `@@GTDOC${placeholders.length}@@`;
  placeholders.push({ doc, label });
  return token;
}

function applyDocPlaceholders(html, placeholders) {
  if (!placeholders.length) return html;
  return html.replace(DOC_PLACEHOLDER_RE, (match, rawIndex) => {
    const item = placeholders[Number(rawIndex)];
    if (!item) return match;
    const docKey = escapeHtml(String(item.doc.doc_key || ""));
    // No href: the click is handled by the host page, which asks the API for a
    // short-lived signed link instead of putting storage URLs in the DOM.
    return `<a class="markdown-doc-reference" role="link" tabindex="0" data-doc-id="${item.doc.document_id}" data-doc-key="${docKey}">${escapeHtml(item.label)}</a>`;
  });
}

function sanitizeMessageHtml(value) {
  return DOMPurify.sanitize(value, {
    USE_PROFILES: { html: true },
    FORBID_TAGS: ["iframe", "object", "embed", "form", "style", "script"],
  });
}

function openLinksInNewTab(html) {
  if (typeof document === "undefined") return html;
  const template = document.createElement("template");
  template.innerHTML = html;
  template.content.querySelectorAll("a[href]").forEach((anchor) => {
    const href = anchor.getAttribute("href") || "";
    if (!/^(?:https?:|mailto:|\/)/i.test(href)) return;
    anchor.setAttribute("target", "_blank");
    anchor.setAttribute("rel", "noopener noreferrer");
  });
  return template.innerHTML;
}

// 不使用 markdownToHtml：编辑器预览固定 breaks: false，这里要保留执行输出里的单换行。
// documents 是本任务的产出文档清单（document_id / doc_key / title / rel_path /
// source_abs_path）。只有清单内的路径会被链接化，代码块内一律不处理。
export function renderConversationMarkdown(value, documents = []) {
  const source = normalizeMarkdown(value);
  if (!source) return "";
  const list = Array.isArray(documents) ? documents : [];
  const placeholders = [];
  try {
    const prepared = source
      .split(CODE_REGION_RE)
      .map((part) => {
        if (isCodeRegion(part)) return part;
        const withReferences = renderDocReferences(part, list, placeholders);
        return list.length
          ? linkifyDocPaths(withReferences, list, placeholders)
          : withReferences;
      })
      .join("");
    const html = marked.parse(prepared, {
      async: false,
      gfm: true,
      breaks: true,
    });
    return openLinksInNewTab(
      sanitizeMessageHtml(applyDocPlaceholders(html, placeholders)),
    );
  } catch {
    return escapeHtml(source).replace(/\n/g, "<br>");
  }
}

// Resolves the document id behind a click inside rendered markdown; the host page
// calls this from its click handler so every markdown surface behaves the same.
export function documentIdFromEvent(event) {
  const target = event?.target;
  if (!target || typeof target.closest !== "function") return null;
  const anchor = target.closest("[data-doc-id]");
  if (!anchor) return null;
  const id = Number(anchor.getAttribute("data-doc-id"));
  return Number.isFinite(id) && id > 0 ? id : null;
}
