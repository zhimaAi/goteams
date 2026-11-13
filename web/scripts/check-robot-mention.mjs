import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parse, compileScript } from "@vue/compiler-sfc";
import * as vue from "vue";
const { descriptor } = parse(
  readFileSync(
    new URL(
      "../src/components/notification/RobotMentionModal.vue",
      import.meta.url,
    ),
    "utf8",
  ),
);
const source = compileScript(descriptor, { id: "mention" })
  .content.replace(
    /^import\s+\{([^}]+)\}\s+from\s+["']vue["'];?\n/gm,
    (_, names) => `const {${names.replace(/\bas\b/g, ":")}} = vue;\n`,
  )
  .replace("export default", "return");
const events = [];
const props = vue.reactive({
  open: true,
  value: { at_all: true, at_user_ids: [7], at_roles: ["assignee"] },
  members: [],
});
const scope = vue.effectScope();
const state = scope.run(() =>
  new Function("vue", source)(vue).setup(props, {
    expose() {},
    emit: (...args) => events.push(args),
  }),
);
state.users.value.push(9);
state.roles.value.push("created_by");
assert.deepEqual(
  props.value.at_user_ids,
  [7],
  "draft editing must not mutate parent",
);
assert.deepEqual(props.value.at_roles, ["assignee"]);
state.confirm();
assert.deepEqual(events[0], [
  "confirm",
  { at_all: true, at_user_ids: [7, 9], at_roles: ["assignee", "created_by"] },
]);
assert.deepEqual(events[1], ["update:open", false]);
props.open = false;
await vue.nextTick();
props.value = { at_user_ids: [11] };
props.open = true;
await vue.nextTick();
assert.deepEqual(state.users.value, [11], "reopen loads current values");
assert.deepEqual(state.roles.value, []);
scope.stop();
console.log("Robot mention draft and confirm checks passed");
