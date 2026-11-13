import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const source = readFileSync(
  new URL("../src/composables/useWorkItemDetail.js", import.meta.url),
  "utf8",
);
const handler = source.match(
  /function onDocMouseDown\(e\) \{[\s\S]*?(?=\n  function onWindowResize)/,
)[0];
const props = { open: true, mode: "drawer" };
const descriptionEditing = { value: false };
let closes = 0;
let confirmations = 0;
let allowLeave = false;
const onMouseDown = new Function(
  "props",
  "descriptionEditing",
  "confirmLeaveEditing",
  "emit",
  `${handler}\nreturn onDocMouseDown;`,
)(
  props,
  descriptionEditing,
  () => {
    confirmations += 1;
    return allowLeave;
  },
  (event) => {
    assert.equal(event, "close");
    closes += 1;
  },
);

function mouseDown(ancestorClass) {
  const event = {
    target: {
      closest: (selectors) =>
        selectors
          .split(",")
          .some((selector) => selector.trim() === ancestorClass),
    },
    prevented: false,
    stopped: false,
    preventDefault() {
      this.prevented = true;
    },
    stopPropagation() {
      this.stopped = true;
    },
  };
  onMouseDown(event);
  return event;
}

for (const mode of ["drawer", "modal"]) {
  props.mode = mode;
  for (const editing of [false, true]) {
    descriptionEditing.value = editing;
    const before = closes;
    for (const ancestor of [
      ".work-item-detail-drawer",
      ".work-item-detail-modal",
      ".ant-tabs-dropdown",
      ".ant-select-dropdown",
      ".ant-picker-dropdown",
      ".ant-dropdown",
    ]) {
      const event = mouseDown(ancestor);
      assert.equal(
        closes,
        before,
        `${mode}: clicks inside ${ancestor} must keep detail open`,
      );
      assert.equal(
        event.prevented,
        false,
        "the popup item must still receive the click",
      );
      assert.equal(event.stopped, false);
    }
  }
}
assert.equal(
  confirmations,
  0,
  "popup interactions must not trigger the unsaved-draft prompt",
);
descriptionEditing.value = false;
mouseDown(".board-page");
assert.equal(closes, 1, "outside clicks must still close detail");
descriptionEditing.value = true;
assert.equal(mouseDown(".board-page").prevented, true);
assert.equal(closes, 1, "canceling the draft prompt must keep detail open");
allowLeave = true;
mouseDown(".board-page");
assert.equal(closes, 2);
props.mode = "page";
mouseDown(".board-page");
props.mode = "drawer";
props.open = false;
mouseDown(".board-page");
assert.equal(closes, 2, "full-page and closed details ignore outside clicks");
console.log(
  "Detail tab overflow clicks, outside closing and draft protection passed",
);
