import { Mark, Node, mergeAttributes } from "@tiptap/core";
import Image from "@tiptap/extension-image";
import hljs from "highlight.js/lib/common";

export const EditorImage = Image.extend({
  addNodeView() {
    const createNodeView = this.parent();
    if (!createNodeView) return null;

    return (props) => {
      const nodeView = createNodeView(props);
      const update = nodeView.update.bind(nodeView);
      // Tiptap 3.27.3 accepts new image attributes without updating its DOM.
      // Rebuild changed images so paragraph splits cannot display a reused src.
      // Remove this guard when the upstream image view synchronizes attributes.
      nodeView.update = (node, decorations, innerDecorations) =>
        props.node.sameMarkup(node) &&
        update(node, decorations, innerDecorations);
      return nodeView;
    };
  },
});

const youtubeHosts = new Set([
  "youtube.com",
  "www.youtube.com",
  "m.youtube.com",
  "youtu.be",
  "www.youtube-nocookie.com",
]);
const bilibiliHosts = new Set([
  "bilibili.com",
  "www.bilibili.com",
  "m.bilibili.com",
  "player.bilibili.com",
]);
const codeBlockThemes = new Set(["default", "light", "dark"]);
const codeBlockLanguages = new Set([
  "plaintext",
  "javascript",
  "typescript",
  "html",
  "css",
  "json",
  "bash",
  "python",
  "java",
  "go",
  "sql",
  "c",
  "cpp",
  "csharp",
  "php",
  "ruby",
  "rust",
  "yaml",
  "markdown",
]);

function parseHttpUrl(value) {
  try {
    const url = new URL(String(value || "").trim());
    if (
      !["http:", "https:"].includes(url.protocol) ||
      url.username ||
      url.password
    )
      return null;
    return url;
  } catch {
    return null;
  }
}

function youtubeVideoId(url) {
  if (!youtubeHosts.has(url.hostname.toLowerCase())) return "";
  if (url.hostname.toLowerCase() === "youtu.be")
    return url.pathname.split("/").filter(Boolean)[0] || "";
  if (url.pathname === "/watch") return url.searchParams.get("v") || "";
  const match = url.pathname.match(/^\/(?:shorts|embed)\/([^/?#]+)/);
  return match?.[1] || "";
}

function bilibiliVideoId(url) {
  if (!bilibiliHosts.has(url.hostname.toLowerCase())) return "";
  const queryId = url.searchParams.get("bvid") || "";
  if (/^BV[0-9A-Za-z]+$/.test(queryId)) return queryId;
  return url.pathname.match(/\/video\/(BV[0-9A-Za-z]+)/i)?.[1] || "";
}

export function normalizeVideoUrl(value) {
  const rawValue = String(value || "").trim();
  if (
    /^\/uploads\/[^?#]+\.(?:mp4|webm)(?:[?#].*)?$/i.test(rawValue) &&
    !rawValue.includes("..")
  ) {
    return { provider: "direct", src: rawValue, originalUrl: rawValue };
  }
  const url = parseHttpUrl(rawValue);
  if (!url) return null;

  const youtubeId = youtubeVideoId(url);
  if (/^[0-9A-Za-z_-]{6,}$/.test(youtubeId)) {
    return {
      provider: "youtube",
      src: `https://www.youtube-nocookie.com/embed/${youtubeId}`,
      originalUrl: url.href,
    };
  }

  const bilibiliId = bilibiliVideoId(url);
  if (/^BV[0-9A-Za-z]+$/.test(bilibiliId)) {
    return {
      provider: "bilibili",
      src: `https://player.bilibili.com/player.html?bvid=${bilibiliId}`,
      originalUrl: url.href,
    };
  }

  if (/\.(?:mp4|webm)$/i.test(url.pathname)) {
    return { provider: "direct", src: url.href, originalUrl: url.href };
  }
  return null;
}

function fileAttributes(element) {
  return {
    href:
      element.getAttribute("data-file-href") ||
      element.getAttribute("href") ||
      element.querySelector("a")?.getAttribute("href") ||
      "",
    fileName:
      element.getAttribute("data-file-name") ||
      element.querySelector(".editor-file-name")?.textContent?.trim() ||
      element.textContent?.trim() ||
      "",
    fileSize: Number(element.getAttribute("data-file-size")) || 0,
  };
}

function fileKind(fileName) {
  const extension = String(fileName || "")
    .split(".")
    .pop()
    ?.toLowerCase();
  if (extension === "pdf") return "pdf";
  if (["jpg", "jpeg", "png", "gif", "webp"].includes(extension)) return "image";
  if (["ppt", "pptx"].includes(extension)) return "presentation";
  if (["mp4", "webm"].includes(extension)) return "video";
  if (["zip", "rar", "7z"].includes(extension)) return "archive";
  return "file";
}

export const EditorFile = Node.create({
  name: "editorFile",
  priority: 1000,
  group: "block",
  atom: true,
  selectable: true,
  isolating: true,

  addAttributes() {
    return {
      href: { default: "" },
      fileName: { default: "" },
      fileSize: { default: 0 },
    };
  },

  parseHTML() {
    return [
      {
        tag: "div[data-editor-file]",
        getAttrs: fileAttributes,
      },
      {
        tag: "a[data-editor-file]",
        getAttrs: fileAttributes,
      },
    ];
  },

  renderHTML({ node }) {
    const { href, fileName, fileSize } = node.attrs;
    const attributes = {
      "data-editor-file": "true",
      "data-file-href": href,
      "data-file-name": fileName,
      "data-file-size": String(fileSize || 0),
      "data-file-kind": fileKind(fileName),
      contenteditable: "false",
      class: "editor-file-node",
    };
    return [
      "div",
      mergeAttributes(attributes),
      ["span", { class: "editor-file-type-icon", "aria-hidden": "true" }],
      [
        "span",
        { class: "editor-file-meta" },
        ["span", { class: "editor-file-name" }, fileName],
        ["span", { class: "editor-file-size" }, formatFileSize(fileSize)],
      ],
      [
        "a",
        {
          class: "editor-file-download",
          href,
          target: "_blank",
          rel: "noopener noreferrer",
          download: fileName,
          title: fileName,
        },
        ["span", { class: "editor-file-download-icon", "aria-hidden": "true" }],
      ],
    ];
  },
});

export function normalizeCodeBlockTheme(value) {
  return codeBlockThemes.has(value) ? value : "default";
}

export function normalizeCodeBlockLanguage(value) {
  return codeBlockLanguages.has(value) ? value : "plaintext";
}

function escapeCodeHtml(value) {
  return String(value || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

export function highlightCodeHtml(value, language) {
  const code = String(value || "");
  const normalizedLanguage = normalizeCodeBlockLanguage(language);
  const highlightLanguage =
    normalizedLanguage === "html" ? "xml" : normalizedLanguage;
  if (
    normalizedLanguage === "plaintext" ||
    !hljs.getLanguage(highlightLanguage)
  )
    return escapeCodeHtml(code);
  return hljs.highlight(code, {
    language: highlightLanguage,
    ignoreIllegals: true,
  }).value;
}

function highlightedCodeElement(value, language) {
  if (typeof document === "undefined") return null;
  const code = document.createElement("code");
  code.className = `language-${language} hljs`;
  code.innerHTML = highlightCodeHtml(value, language);
  return code;
}

function codeBlockAttributes(element) {
  return {
    theme: normalizeCodeBlockTheme(element.getAttribute("data-code-theme")),
    language: normalizeCodeBlockLanguage(
      element.getAttribute("data-code-language"),
    ),
    code: element.querySelector("code")?.textContent || "",
  };
}

export const EditorCodeBlock = Node.create({
  name: "editorCodeBlock",
  priority: 1000,
  group: "block",
  atom: true,
  selectable: true,
  isolating: true,

  addAttributes() {
    return {
      theme: { default: "default" },
      language: { default: "plaintext" },
      code: { default: "" },
    };
  },

  parseHTML() {
    return [
      {
        tag: "div[data-editor-code-block]",
        getAttrs: codeBlockAttributes,
      },
    ];
  },

  renderHTML({ node }) {
    const theme = normalizeCodeBlockTheme(node.attrs.theme);
    const language = normalizeCodeBlockLanguage(node.attrs.language);
    const code = highlightedCodeElement(node.attrs.code, language);
    return [
      "div",
      {
        "data-editor-code-block": "true",
        "data-code-theme": theme,
        "data-code-language": language,
        contenteditable: "false",
        class: "editor-code-block-node",
      },
      [
        "pre",
        {},
        code || [
          "code",
          { class: `language-${language}` },
          node.attrs.code || "",
        ],
      ],
    ];
  },
});

export const EditorVideo = Node.create({
  name: "editorVideo",
  priority: 1000,
  group: "block",
  atom: true,
  selectable: true,
  draggable: true,

  addAttributes() {
    return {
      provider: { default: "" },
      src: { default: "" },
      originalUrl: { default: "" },
    };
  },

  parseHTML() {
    return [
      {
        tag: "div[data-editor-video]",
        getAttrs: (element) => {
          const normalized = normalizeVideoUrl(
            element.getAttribute("data-video-original-url") ||
              element.getAttribute("data-video-src"),
          );
          return normalized || false;
        },
      },
    ];
  },

  renderHTML({ node }) {
    const normalized = normalizeVideoUrl(
      node.attrs.originalUrl || node.attrs.src,
    );
    if (!normalized) return ["p", {}, ""];
    const attributes = {
      "data-editor-video": "true",
      "data-video-provider": normalized.provider,
      "data-video-src": normalized.src,
      "data-video-original-url": normalized.originalUrl,
      class: "editor-video-node",
    };
    if (normalized.provider === "direct") {
      return [
        "div",
        attributes,
        [
          "video",
          {
            src: normalized.src,
            controls: "controls",
            preload: "metadata",
          },
        ],
      ];
    }
    return [
      "div",
      attributes,
      [
        "iframe",
        {
          src: normalized.src,
          title: "Embedded video",
          loading: "lazy",
          allow: "accelerometer; autoplay; encrypted-media; picture-in-picture",
          allowfullscreen: "allowfullscreen",
          referrerpolicy: "strict-origin-when-cross-origin",
        },
      ],
    ];
  },
});

export function formatFileSize(value) {
  const size = Number(value) || 0;
  return `${(size / 1024 / 1024).toFixed(2)}MB`;
}

// 评论里的 @人 标签。标签只承载「@显示名」这段文本的样式，不存用户 ID——
// 后端仍按纯文本匹配 @name 解析被@的人；invite 属性用于标记「@了邀填人」。
export const MentionTag = Mark.create({
  name: "mentionTag",
  inclusive: false,
  addAttributes() {
    return {
      invite: {
        default: false,
        parseHTML: (element) => element.getAttribute("data-invite") === "true",
        renderHTML: (attributes) =>
          attributes.invite ? { "data-invite": "true" } : {},
      },
    };
  },
  parseHTML() {
    return [{ tag: "span[data-mention]" }];
  },
  renderHTML({ HTMLAttributes }) {
    return [
      "span",
      mergeAttributes(HTMLAttributes, {
        "data-mention": "true",
        class: "mention-tag",
      }),
      0,
    ];
  },
});
