// Shared clipboard helpers for work items (requirements/defects).
// Both the requirement detail drawer and the workspace board list reuse these
// helpers so the two entry points stay in lockstep.

import {
  markdownToHtml,
  sanitizeStoredHtml,
  toEditorContent,
} from "@/components/common/rich-text-editor/markdownContent";

// writeClipboardText writes plain text to the system clipboard.
export async function writeClipboardText(text) {
  if (!text) return;
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  try {
    textarea.select();
    if (!document.execCommand("copy")) throw new Error("copy failed");
  } finally {
    textarea.remove();
  }
}

// writeClipboardHtmlFallback copies rich HTML content via a temporary DOM
// selection; this is the legacy path used when the modern clipboard API fails.
function writeClipboardHtmlFallback(html) {
  const container = document.createElement("div");
  container.innerHTML = html;
  container.style.position = "fixed";
  container.style.left = "-9999px";
  container.style.opacity = "0";
  document.body.appendChild(container);
  const selection = window.getSelection();
  const range = document.createRange();
  range.selectNodeContents(container);
  selection.removeAllRanges();
  selection.addRange(range);
  try {
    if (!document.execCommand("copy")) throw new Error("copy failed");
  } finally {
    selection.removeAllRanges();
    container.remove();
  }
}

// writeClipboardRichContent writes both plain text and HTML to the clipboard so
// rich-text editors preserve formatting when the user pastes elsewhere.
export async function writeClipboardRichContent({ html, text }) {
  if (navigator.clipboard?.write && window.ClipboardItem) {
    try {
      await navigator.clipboard.write([
        new window.ClipboardItem({
          "text/html": new Blob([html], { type: "text/html" }),
          "text/plain": new Blob([text], { type: "text/plain" }),
        }),
      ]);
      return;
    } catch {
      // Some browsers expose ClipboardItem but reject HTML writes; fall back to selection copy.
    }
  }
  if (!html) return;
  writeClipboardHtmlFallback(html);
}

// descriptionContent normalizes a stored description into the { html, text }
// payload used by the clipboard helpers.
export function descriptionContent(value, mode) {
  if (!value) return "";
  const html =
    mode === "markdown"
      ? markdownToHtml(value)
      : sanitizeStoredHtml(toEditorContent(value));
  if (!html) return "";
  const container = document.createElement("div");
  container.innerHTML = html;
  container
    .querySelectorAll(
      "img[src], video[src], source[src], iframe[src], [poster]",
    )
    .forEach((node) => {
      ["src", "poster"].forEach((attribute) => {
        const attributeValue = node.getAttribute(attribute);
        if (!attributeValue || /^(?:data|blob):/i.test(attributeValue)) return;
        try {
          node.setAttribute(
            attribute,
            new URL(attributeValue, window.location.origin).toString(),
          );
        } catch {
          // Keep malformed legacy URLs unchanged so other content can still be copied.
        }
      });
    });
  const blockTags = new Set([
    "ADDRESS",
    "ARTICLE",
    "ASIDE",
    "BLOCKQUOTE",
    "DIV",
    "FIGCAPTION",
    "FIGURE",
    "H1",
    "H2",
    "H3",
    "H4",
    "H5",
    "H6",
    "LI",
    "P",
    "PRE",
    "SECTION",
    "TABLE",
    "TR",
  ]);
  const walk = (node) => {
    if (node.nodeType === Node.TEXT_NODE) return node.nodeValue || "";
    if (node.nodeType !== Node.ELEMENT_NODE) return "";
    if (node.tagName === "BR") return "\n";
    if (node.tagName === "IMG") {
      const src = node.getAttribute("src") || "";
      return src ? `\n${src}\n` : "";
    }
    const content = Array.from(node.childNodes).map(walk).join("");
    return blockTags.has(node.tagName) ? `${content}\n` : content;
  };
  return {
    html: container.innerHTML,
    text: walk(container)
      .replace(/[ \t]+\n/g, "\n")
      .replace(/\n{3,}/g, "\n\n")
      .trim(),
  };
}

// combineDescriptionContent produces the rich-text payload for "copy title, link
// and description": a short heading, the source link, and the description HTML.
export function combineDescriptionContent(title, link, description) {
  const container = document.createElement("div");
  if (title) {
    const titleNode = document.createElement("h3");
    titleNode.textContent = title;
    container.appendChild(titleNode);
  }
  if (link) {
    const linkNode = document.createElement("p");
    const anchor = document.createElement("a");
    anchor.href = link;
    anchor.textContent = link;
    linkNode.appendChild(anchor);
    container.appendChild(linkNode);
  }
  if (description?.html) {
    const descriptionNode = document.createElement("div");
    descriptionNode.innerHTML = description.html;
    container.append(...descriptionNode.childNodes);
  }
  return {
    html: container.innerHTML,
    text: [title ? `### ${title}` : "", link, description?.text]
      .filter(Boolean)
      .join("\n"),
  };
}

// buildWorkItemPageUrl points at the standalone work-item detail page.
export function buildWorkItemPageUrl(wsId, requirementId, surface) {
  return new URL(
    surface === "issue"
      ? `/issues/${wsId}/items/${requirementId}`
      : `/workspace/${wsId}/work-items/${requirementId}`,
    window.location.origin,
  ).toString();
}
