// Renders one recorded description change as two highlighted panes.
//
// The stored description is rich text, so the diff never touches markup: both sides are sanitized
// first, then every text node is rewritten into a sequence of plain text, <del> and <ins> nodes.
// Formatting, images and media therefore survive untouched, and the highlight tags are created
// through the DOM instead of by string surgery, so a tag can never be cut in half.

import { diffChars } from "diff";
import {
  markdownToHtml,
  sanitizeStoredHtml,
} from "@/components/common/rich-text-editor/markdownContent";

// jsdiff aborts the character diff and returns undefined once it exceeds this budget; the panes
// then fall back to a plain side-by-side view instead of freezing the page.
const DIFF_TIMEOUT_MS = 2000;

// Only the innermost block gets a whole-block tint, so nested wrappers do not stack backgrounds.
const BLOCK_SELECTOR =
  "p, div, li, h1, h2, h3, h4, h5, h6, blockquote, pre, td, th, figcaption";

// Class names applied to a block whose visible text changed completely.
const REMOVED_BLOCK_CLASS = "is-removed";
const ADDED_BLOCK_CLASS = "is-added";

/**
 * Turns one stored description value into sanitized HTML the diff can operate on.
 * markdown mode goes through the markdown renderer; the rich-text branch sanitizes the stored value.
 */
export function toDescriptionHtml(value, descriptionMode) {
  if (!value) return "";
  return descriptionMode === "markdown"
    ? markdownToHtml(value)
    : sanitizeStoredHtml(value);
}

/**
 * Builds the two panes of one description change.
 * Returns { before, after, diffSkipped }: each side is sanitized HTML carrying <del>/<ins> highlights,
 * and diffSkipped tells the caller the character diff hit its time budget and stayed unhighlighted.
 */
export function buildDescriptionDiff(oldValue, newValue, descriptionMode) {
  const oldHtml = toDescriptionHtml(oldValue, descriptionMode);
  const newHtml = toDescriptionHtml(newValue, descriptionMode);
  // A missing side has nothing to compare against; the page renders its own empty placeholder.
  if (!oldHtml || !newHtml) {
    return { before: oldHtml, after: newHtml, diffSkipped: false };
  }

  const beforeRoot = parseHtml(oldHtml);
  const afterRoot = parseHtml(newHtml);
  const beforeNodes = collectTextNodes(beforeRoot);
  const afterNodes = collectTextNodes(afterRoot);
  const oldText = beforeNodes.map((node) => node.nodeValue).join("");
  const newText = afterNodes.map((node) => node.nodeValue).join("");

  const parts = diffChars(oldText, newText, { timeout: DIFF_TIMEOUT_MS });
  if (!parts) {
    return { before: oldHtml, after: newHtml, diffSkipped: true };
  }

  rewriteTextNodes(beforeNodes, parts, "old");
  rewriteTextNodes(afterNodes, parts, "new");
  tintWholeBlocks(beforeRoot, "del", REMOVED_BLOCK_CLASS);
  tintWholeBlocks(afterRoot, "ins", ADDED_BLOCK_CLASS);

  return {
    before: serializeFragment(beforeRoot),
    after: serializeFragment(afterRoot),
    diffSkipped: false,
  };
}

function parseHtml(html) {
  const template = document.createElement("template");
  template.innerHTML = html;
  return template.content;
}

// DocumentFragment has no innerHTML, so the parsed children are moved into a container to be
// serialized back. Moving instead of cloning is safe because the fragment is done at this point.
function serializeFragment(fragment) {
  const holder = document.createElement("div");
  holder.appendChild(fragment);
  return holder.innerHTML;
}

function collectTextNodes(root) {
  const nodes = [];
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  while (walker.nextNode()) {
    nodes.push(walker.currentNode);
  }
  return nodes;
}

/**
 * Rewrites every text node of one side so that characters belonging to a change are wrapped.
 * side is "old" or "new": the old side keeps removed characters, the new side keeps added ones.
 */
function rewriteTextNodes(nodes, parts, side) {
  const spans = buildSpans(parts, side);
  if (!spans.length) return;

  // Walk back to front: replacing a node never shifts the offsets of the nodes before it.
  let nodeStart = nodes.reduce(
    (total, node) => total + node.nodeValue.length,
    0,
  );
  for (let index = nodes.length - 1; index >= 0; index -= 1) {
    const node = nodes[index];
    nodeStart -= node.nodeValue.length;
    const pieces = buildNodePieces(node, nodeStart, spans);
    if (pieces) node.replaceWith(...pieces);
  }
}

/**
 * Flattens the diff into the character ranges this side owns, in order.
 * A range with no kind is unchanged text; "del" and "ins" mark the highlighted ranges.
 */
function buildSpans(parts, side) {
  const spans = [];
  let cursor = 0;
  for (const part of parts) {
    const belongs = side === "old" ? !part.added : !part.removed;
    if (!belongs) continue;
    let kind = "";
    if (part.removed) kind = "del";
    if (part.added) kind = "ins";
    spans.push({ start: cursor, end: cursor + part.value.length, kind });
    cursor += part.value.length;
  }
  return spans;
}

/**
 * Rebuilds one text node into plain text plus <del>/<ins> wrappers.
 * Returns null when nothing inside the node changed, leaving the original node untouched.
 */
function buildNodePieces(node, nodeStart, spans) {
  const text = node.nodeValue;
  if (!text) return null;
  const nodeEnd = nodeStart + text.length;
  const pieces = [];
  let changed = false;
  let cursor = nodeStart;

  for (const span of spans) {
    if (span.end <= cursor) continue;
    if (span.start >= nodeEnd) break;
    const start = Math.max(span.start, cursor);
    const end = Math.min(span.end, nodeEnd);
    if (start > cursor) {
      pieces.push(
        document.createTextNode(
          text.slice(cursor - nodeStart, start - nodeStart),
        ),
      );
    }
    const slice = text.slice(start - nodeStart, end - nodeStart);
    if (slice) {
      if (span.kind) {
        const wrapper = document.createElement(span.kind);
        wrapper.textContent = slice;
        pieces.push(wrapper);
        changed = true;
      } else {
        pieces.push(document.createTextNode(slice));
      }
    }
    cursor = end;
    if (cursor >= nodeEnd) break;
  }

  // Defensive: keep any tail the spans did not cover so no character is ever dropped.
  if (cursor < nodeEnd) {
    pieces.push(document.createTextNode(text.slice(cursor - nodeStart)));
  }
  return changed ? pieces : null;
}

/**
 * Tints a block whose visible text changed completely, so a wholly added or removed paragraph
 * reads as one coloured block instead of a run of small inline marks.
 */
function tintWholeBlocks(root, tagName, className) {
  const blocks = Array.from(root.querySelectorAll(BLOCK_SELECTOR));
  for (const block of blocks) {
    // Skip wrappers: their inner blocks carry the tint themselves.
    if (block.querySelector(BLOCK_SELECTOR)) continue;
    const total = block.textContent.replace(/\s+/g, "").length;
    if (!total) continue;
    const marked = Array.from(block.querySelectorAll(tagName))
      .map((mark) => mark.textContent.replace(/\s+/g, "").length)
      .reduce((sum, length) => sum + length, 0);
    if (marked >= total) block.classList.add(className);
  }
}
