import DOMPurify from "dompurify";
import { marked } from "marked";
import TurndownService from "turndown";
import { gfm } from "turndown-plugin-gfm";
import {
  highlightCodeHtml,
  normalizeCodeBlockLanguage,
  normalizeCodeBlockTheme,
  normalizeVideoUrl,
} from "./richTextNodes";

const turndownService = new TurndownService({
  headingStyle: "atx",
  bulletListMarker: "-",
  codeBlockStyle: "fenced",
});
turndownService.use(gfm);
turndownService.addRule("editorFile", {
  filter: (node) =>
    ["A", "DIV"].includes(node.nodeName) &&
    node.hasAttribute("data-editor-file"),
  replacement: (_content, node) => {
    const name =
      node.getAttribute("data-file-name") ||
      node.querySelector(".editor-file-name")?.textContent?.trim() ||
      node.textContent?.trim() ||
      "File";
    const href =
      node.getAttribute("data-file-href") ||
      node.getAttribute("href") ||
      node.querySelector("a")?.getAttribute("href") ||
      "";
    return `\n\n[${name.replace(/[\[\]]/g, "")}](${href})\n\n`;
  },
});
turndownService.addRule("editorVideo", {
  filter: (node) =>
    node.nodeName === "DIV" && node.hasAttribute("data-editor-video"),
  replacement: (_content, node) => `\n\n${node.outerHTML}\n\n`,
});
turndownService.addRule("editorCodeBlock", {
  filter: (node) =>
    node.nodeName === "DIV" && node.hasAttribute("data-editor-code-block"),
  replacement: (_content, node) => `\n\n${node.outerHTML}\n\n`,
});

function escapeHtml(value) {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

function normalizeEditorContent(value) {
  if (!value) return "";
  if (
    /<(?:p|div|br|ul|ol|li|strong|b|em|i|u|s|strike|del|a|img|video|iframe|h[1-6]|blockquote|pre|code|hr|table|thead|tbody|tr|th|td|input)\b/i.test(
      value,
    )
  )
    return value;
  return value
    .split(/\r?\n/)
    .map((line) => `<p>${escapeHtml(line) || "<br>"}</p>`)
    .join("");
}

function isSafeFileUrl(value) {
  if (/^\/uploads\//.test(value)) return true;
  try {
    const url = new URL(value);
    return ["http:", "https:"].includes(url.protocol);
  } catch {
    return false;
  }
}

function sanitizeEditorHtml(value) {
  const sanitized = DOMPurify.sanitize(value, {
    USE_PROFILES: { html: true },
    ADD_TAGS: ["iframe", "video", "source"],
    ADD_ATTR: [
      "allow",
      "allowfullscreen",
      "controls",
      "download",
      "loading",
      "preload",
      "referrerpolicy",
      "target",
      "data-editor-file",
      "data-editor-code-block",
      "data-editor-video",
      "data-code-language",
      "data-code-theme",
      "data-file-name",
      "data-file-size",
      "data-file-href",
      "data-file-kind",
      "data-video-original-url",
      "data-video-provider",
      "data-video-src",
      // 评论 @人 标签；data-invite 标记该标签指向邀填人。
      "data-mention",
      "data-invite",
    ],
  });
  if (typeof document === "undefined") return sanitized;

  const template = document.createElement("template");
  template.innerHTML = sanitized;

  template.content.querySelectorAll("[data-editor-file]").forEach((node) => {
    const href =
      node.getAttribute("data-file-href") ||
      node.getAttribute("href") ||
      node.querySelector("a")?.getAttribute("href") ||
      "";
    if (!isSafeFileUrl(href)) {
      node.replaceWith(document.createTextNode(node.textContent || ""));
      return;
    }
    if (node.nodeName === "A") {
      node.setAttribute("target", "_blank");
      node.setAttribute("rel", "noopener noreferrer");
      return;
    }
    node.setAttribute("data-file-href", href);
    const download = node.querySelector("a");
    if (download) {
      download.setAttribute("href", href);
      download.setAttribute("target", "_blank");
      download.setAttribute("rel", "noopener noreferrer");
    }
  });

  template.content
    .querySelectorAll("div[data-editor-code-block]")
    .forEach((node) => {
      const theme = normalizeCodeBlockTheme(
        node.getAttribute("data-code-theme"),
      );
      const language = normalizeCodeBlockLanguage(
        node.getAttribute("data-code-language"),
      );
      const source = node.querySelector("code")?.textContent || "";
      node.setAttribute("data-code-theme", theme);
      node.setAttribute("data-code-language", language);
      node.classList.add("editor-code-block-node");
      node.replaceChildren();
      const pre = document.createElement("pre");
      const code = document.createElement("code");
      code.className = `language-${language} hljs`;
      code.innerHTML = highlightCodeHtml(source, language);
      pre.append(code);
      node.append(pre);
    });

  template.content
    .querySelectorAll("div[data-editor-video]")
    .forEach((node) => {
      const normalized = normalizeVideoUrl(
        node.getAttribute("data-video-original-url") ||
          node.getAttribute("data-video-src"),
      );
      if (!normalized) {
        node.remove();
        return;
      }
      node.setAttribute("data-video-provider", normalized.provider);
      node.setAttribute("data-video-src", normalized.src);
      node.setAttribute("data-video-original-url", normalized.originalUrl);
      node.classList.add("editor-video-node");
      node.replaceChildren();
      if (normalized.provider === "direct") {
        const video = document.createElement("video");
        video.src = normalized.src;
        video.controls = true;
        video.preload = "metadata";
        node.append(video);
        return;
      }
      const iframe = document.createElement("iframe");
      iframe.src = normalized.src;
      iframe.title = "Embedded video";
      iframe.loading = "lazy";
      iframe.allow =
        "accelerometer; autoplay; encrypted-media; picture-in-picture";
      iframe.allowFullscreen = true;
      iframe.referrerPolicy = "strict-origin-when-cross-origin";
      node.append(iframe);
    });

  template.content.querySelectorAll("iframe, video, source").forEach((node) => {
    if (!node.closest("div[data-editor-video]")) node.remove();
  });
  return template.innerHTML;
}

export function toEditorContent(value) {
  return sanitizeEditorHtml(normalizeEditorContent(value));
}

export function markdownToHtml(value) {
  if (!value.trim()) return "";
  return sanitizeEditorHtml(
    marked.parse(value, { async: false, gfm: true, breaks: false }),
  );
}

export function htmlToMarkdown(value) {
  if (!value) return "";
  return turndownService.turndown(toEditorContent(value)).trimEnd();
}

export function sanitizeStoredHtml(value) {
  return sanitizeEditorHtml(normalizeEditorContent(value));
}
