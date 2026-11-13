import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const componentPath = fileURLToPath(
  new URL("../src/components/common/RequirementComments.vue", import.meta.url),
);
const component = readFileSync(componentPath, "utf8");
const commentContentCss =
  component.match(/\.comment-content\s*\{([^}]*)\}/)?.[1] || "";
const whiteSpace =
  commentContentCss.match(/white-space:\s*([^;]+);/)?.[1]?.trim() || "normal";
const source = "first line\nsecond line";
const rendered = ["pre", "pre-wrap", "pre-line", "break-spaces"].includes(
  whiteSpace,
)
  ? source
  : source.replace(/\s+/g, " ");

if (rendered !== source) {
  console.error(`comment newlines collapsed with white-space: ${whiteSpace}`);
  process.exit(1);
}
console.log(`comment newlines preserved with white-space: ${whiteSpace}`);
