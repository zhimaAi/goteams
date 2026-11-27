import DOMPurify from "dompurify";
import { Marked } from "marked";

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

// Keep Windows separators literal inside existing Markdown destinations.
function destination(token) {
  const windows = token.raw?.match(
    /\]\(\s*(?:<([A-Za-z]:[\\/][^>]+)>|([A-Za-z]:[\\/][\s\S]*?))(?:\s+(?:"[^"]*"|'[^']*'|\([^)]*\)))?\s*\)$/,
  );
  return windows ? windows[1] || windows[2] : token.href || "";
}

function isLocalPath(value) {
  if (/^(?:file:|[A-Za-z]:[\\/])/i.test(value)) return true;
  return !/^(?:[A-Za-z][A-Za-z0-9+.-]*:|\/\/)/.test(value);
}

function normalizedPath(value, encoded = false, directory = "") {
  let path = String(value || "").trim();
  if (encoded) {
    if (/^file:/i.test(path)) {
      try {
        const url = new URL(path);
        if (url.hostname && url.hostname !== "localhost") return "";
        path = url.pathname;
        if (/^\/[A-Za-z]:\//.test(path)) path = path.slice(1);
      } catch {
        return "";
      }
    } else {
      path = path.split(/[?#]/, 1)[0];
    }
    try {
      path = decodeURIComponent(path);
    } catch {
      return "";
    }
  }
  path = `${directory}${path}`.replace(/\\/g, "/");
  if (!path || /[\0\r\n]/.test(path) || path.startsWith("//")) return "";
  const parts = [];
  for (const part of path.split("/")) {
    if (part === ".") continue;
    if (part === "..") {
      if (!parts.length || parts.at(-1) === "") return "";
      parts.pop();
    } else parts.push(part);
  }
  const result = parts.join("/");
  return /^[A-Za-z]:\//.test(result) ? result.toLowerCase() : result;
}

function documentForPath(value, documents, sourcePath) {
  const path = normalizedPath(value, true);
  const absolute = /^(?:\/|[a-z]:\/)/i.test(path);
  const candidates = [path];
  if (!absolute && sourcePath) {
    candidates.unshift(
      normalizedPath(value, true, sourcePath.replace(/[^/]*$/, "")),
    );
  }
  for (const candidate of candidates) {
    const matches = documents.filter(
      (doc) =>
        candidate &&
        candidate ===
          normalizedPath(absolute ? doc.source_abs_path : doc.rel_path),
    );
    if (matches.length === 1) return matches[0];
    if (matches.length > 1) return null;
  }
  return null;
}

function documentAnchor(doc, label) {
  const id = Number(doc.document_id);
  if (!Number.isSafeInteger(id) || id <= 0) return docAnchor(label);
  return `<a class="markdown-doc-reference" role="link" tabindex="0" data-doc-id="${id}">${escapeHtml(label)}</a>`;
}

function localImageHtml(href, label, documents, options) {
  const doc = documentForPath(href, documents, options.sourcePath);
  const id = Number(doc?.document_id) || 0;
  const status = id ? options.loading : options.notSynced;
  return `<span class="markdown-document-image" data-image-document-id="${id}"><button type="button" class="markdown-image-preview" disabled aria-label="${escapeHtml(options.preview || label)}"><img alt="${escapeHtml(label)}" hidden><span class="markdown-image-status" role="status">${escapeHtml(status || "")}</span></button><button type="button" class="markdown-image-retry" hidden>${escapeHtml(options.retry || "")}</button></span>`;
}

function sanitizeMessageHtml(value) {
  return DOMPurify.sanitize(value, {
    USE_PROFILES: { html: true },
    FORBID_TAGS: ["iframe", "object", "embed", "form", "style", "script"],
  });
}

function resolveRenderedResources(html, documents, options) {
  if (typeof document === "undefined") return html;
  const template = document.createElement("template");
  template.innerHTML = html;
  template.content.querySelectorAll("img[src]").forEach((image) => {
    const href = image.getAttribute("src") || "";
    if (!isLocalPath(href)) return;
    const replacement = document.createElement("template");
    replacement.innerHTML = localImageHtml(href, image.alt, documents, options);
    image.replaceWith(replacement.content);
  });
  template.content.querySelectorAll("a[href]").forEach((anchor) => {
    const href = anchor.getAttribute("href") || "";
    if (!/^(?:https?:|mailto:|\/)/i.test(href)) return;
    anchor.setAttribute("target", "_blank");
    anchor.setAttribute("rel", "noopener noreferrer");
  });
  return template.innerHTML;
}

// Resolve only explicit references and Markdown links/images; prose and code stay literal.
export function renderConversationMarkdown(
  value,
  documents = [],
  options = {},
) {
  const source = normalizeMarkdown(value);
  if (!source) return "";
  const list = Array.isArray(documents) ? documents : [];
  const parser = new Marked({
    async: false,
    gfm: true,
    breaks: true,
    renderer: {
      image(token) {
        const href = destination(token);
        return isLocalPath(href)
          ? localImageHtml(href, token.text || "", list, options)
          : false;
      },
      link(token) {
        const href = destination(token);
        if (!isLocalPath(href) || href.startsWith("#")) return false;
        const doc = documentForPath(href, list, options.sourcePath);
        return doc
          ? documentAnchor(doc, token.text)
          : this.parser.parseInline(token.tokens);
      },
    },
    extensions: [
      {
        name: "documentReference",
        level: "inline",
        start(src) {
          return src.indexOf("#[");
        },
        tokenizer(src) {
          const match = /^#\[([^\]\n]{1,200})\]/.exec(src);
          if (match)
            return {
              type: "documentReference",
              raw: match[0],
              text: match[1].split(" · ")[0].trim(),
            };
        },
        renderer(token) {
          const doc = findDocumentByName(list, token.text);
          return doc ? documentAnchor(doc, token.text) : docAnchor(token.text);
        },
      },
    ],
  });
  try {
    return resolveRenderedResources(
      sanitizeMessageHtml(parser.parse(source)),
      list,
      options,
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
