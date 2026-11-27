import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";

const source = readFileSync(
  new URL(
    "../src/views/workspace/settings/AutoAssignmentTab.vue",
    import.meta.url,
  ),
  "utf8",
)
  .split("<script setup>")[1]
  .split("</script>")[0];
let requests = [];
let complete;
const context = vm.createContext({
  ref: (value) => ({ value }),
  computed: (get) => ({
    get value() {
      return get();
    },
  }),
  watch: () => {},
  defineProps: () => ({ wsId: 5 }),
  useI18n: () => ({ t: (key) => key }),
  useAuthStore: () => ({ hasPermission: () => true }),
  message: { warning() {}, success() {} },
  saveAutoAssignment: (ws, body) => {
    requests.push({ ws, body });
    return new Promise((resolve, reject) => {
      complete = { resolve, reject };
    });
  },
});
vm.runInContext(
  source.replace(/^import .*;\n/gm, "") +
    "\nglobalThis.state = { config, revision, ready, draft, editorOpen, applyRule, toggleRule, deleteRule, searchName, filteredRules, pagination, changePage };",
  context,
);
const state = context.state;
state.ready.value = true;
state.editorOpen.value = true;
state.draft.value = {
  id: "rule-1",
  name: "Test",
  target: "members",
  member_ids: [1],
  modules: [
    {
      conditions: [{ field_key: "title", operator: "contains", value: "Test" }],
    },
  ],
};
const pending = state.applyRule();
assert.equal(requests.length, 1);
assert.equal(requests[0].ws, 5);
assert.equal(requests[0].body.revision, 0);
assert.equal(requests[0].body.config.rules.length, 1);
assert.equal(state.config.value.rules.length, 0);
assert.equal(state.editorOpen.value, true);
await state.applyRule();
assert.equal(requests.length, 1, "Repeated confirmation must not submit twice");
complete.resolve({ revision: 1, config: requests[0].body.config });
await pending;
assert.equal(state.revision.value, 1);
assert.equal(state.editorOpen.value, false);
assert.equal(state.config.value.rules.length, 1);
state.editorOpen.value = true;
state.draft.value = { ...state.draft.value, name: "Edited" };
const failed = state.applyRule();
assert.equal(requests[1].body.revision, 1);
assert.equal(requests[1].body.config.rules.length, 1);
complete.reject(new Error("Save failed"));
await failed;
assert.equal(state.editorOpen.value, true);
assert.equal(state.draft.value.name, "Edited");
assert.equal(state.config.value.rules[0].name, "Test");
const retry = state.applyRule();
complete.resolve({ revision: 2, config: requests[2].body.config });
await retry;
assert.equal(state.config.value.rules[0].name, "Edited");
assert.equal(state.config.value.rules.length, 1);
console.log("Auto-assignment confirmation persistence checks passed");

state.config.value.rules[0].enabled = true;
const toggled = state.toggleRule("rule-1", false);
assert.equal(requests[3].body.config.rules[0].enabled, false);
assert.equal(state.config.value.rules[0].enabled, true);
await state.toggleRule("rule-1", false);
assert.equal(requests.length, 4, "Repeated toggle must not submit twice");
complete.resolve({ revision: 3, config: requests[3].body.config });
await toggled;
assert.equal(state.config.value.rules[0].enabled, false);
const rejectedToggle = state.toggleRule("rule-1", true);
complete.reject(new Error("Save failed"));
await rejectedToggle;
assert.equal(state.config.value.rules[0].enabled, false);
const rejectedDelete = state.deleteRule("rule-1");
assert.equal(requests[5].body.config.rules.length, 0);
complete.reject(new Error("Delete failed"));
await rejectedDelete;
assert.equal(state.config.value.rules.length, 1);
const deleted = state.deleteRule("rule-1");
complete.resolve({ revision: 4, config: requests[6].body.config });
await deleted;
assert.equal(state.config.value.rules.length, 0);
console.log("Auto-assignment immediate toggle and deletion checks passed");

state.config.value.rules = Array.from({ length: 21 }, (_, i) => ({
  id: String(i),
  name: i === 0 ? "Support" : "Sales",
}));
assert.equal(state.pagination.value.total, 21);
assert.equal(state.pagination.value.pageSize, 10);
state.changePage({ current: 2, pageSize: 10 });
assert.equal(state.pagination.value.current, 2);
state.searchName.value = " SUPPORT ";
assert.equal(state.filteredRules.value.length, 1);
assert.equal(state.filteredRules.value[0].name, "Support");
assert.equal(state.config.value.rules.length, 21);
console.log("Auto-assignment search and pagination checks passed");
