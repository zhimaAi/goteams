import assert from "node:assert/strict";
import { getSchema } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import { EditorImage } from "../src/components/common/rich-text-editor/richTextNodes.js";

const schema = getSchema([StarterKit, EditorImage.configure({ inline: true })]);
const original = schema.nodes.image.create({ src: "/one.png", alt: "one" });
let updates = 0;
const createNodeView = EditorImage.config.addNodeView.call({
  parent: () => () => ({
    update() {
      updates++;
      assert.equal(this, nodeView);
      return true;
    },
  }),
});
const nodeView = createNodeView({ node: original });

assert.equal(nodeView.update(original, [], null), true);
assert.equal(updates, 1);
for (const attrs of [
  { src: "/two.png" },
  { alt: "two" },
  { title: "two" },
  { width: 240, height: 120 },
]) {
  assert.equal(
    nodeView.update(
      schema.nodes.image.create({ ...original.attrs, ...attrs }),
      [],
      null,
    ),
    false,
    "changed image attributes must rebuild the view instead of keeping stale DOM",
  );
}
assert.equal(nodeView.update(schema.nodes.paragraph.create(), [], null), false);
assert.equal(updates, 1, "rejected updates must not mutate the existing view");
assert.equal(EditorImage.config.addNodeView.call({ parent: () => null }), null);
console.log("editor image node view regressions: ok");
