import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parse, compileScript } from "@vue/compiler-sfc";
import * as vue from "vue";

const { descriptor } = parse(
  readFileSync(
    new URL("../src/views/workspace/settings/SLATab.vue", import.meta.url),
    "utf8",
  ),
);
const source = compileScript(descriptor, { id: "SLATab" })
  .content.replace(
    /^import\s+([\s\S]*?)\s+from\s+["']([^"']+)["'];?\n/gm,
    (_, binding, name) =>
      `const ${binding.startsWith("{") ? binding.replace(/\bas\b/g, ":") : `{ default: ${binding.trim()} }`} = modules[${JSON.stringify(name)}];\n`,
  )
  .replace("export default", "return");
const warnings = [];
const saved = [];
const savedStages = [];
const options = {
  priorities: [{ id: 1, name: "High", color: "#ff4d4f" }],
  categories: [],
  statuses: [],
  roles: [
    { id: 2, name: "Support" },
    { id: 3, name: "Manager" },
  ],
};
const modules = {
  "@/constants/robot": await import("../src/constants/robot.js"),
  "@/components/notification/RobotMentionModal.vue": { default: {} },
  "@/components/notification/RobotUserMappingModal.vue": { default: {} },
  vue,
  "vue-i18n": {
    useI18n: () => ({ t: (key, params) => JSON.stringify({ key, ...params }) }),
  },
  "ant-design-vue": {
    message: {
      warning: (text) => warnings.push(JSON.parse(text)),
      success() {},
    },
  },
  "@ant-design/icons-vue": {},
  "@/stores/auth": { useAuthStore: () => ({ hasPermission: () => true }) },
  "@/api/sla": {
    getSLASettings: async () => ({ enabled: true, timezone: "Asia/Shanghai" }),
    getSLAMetadata: async () => options,
    listSLAStages: async () => ({
      list: [{ id: 10, name: "First response" }],
      total: 1,
    }),
    listSLAPolicies: async () => ({ list: [], total: 0 }),
    saveSLAPolicy: async (_, data) => saved.push(data),
    saveSLAStage: async (_, data) => savedStages.push(data),
  },
};
const scope = vue.effectScope();
const state = scope.run(() =>
  new Function("modules", source)(modules).setup({ wsId: 18 }, { expose() {} }),
);
await new Promise((resolve) => setImmediate(resolve));
state.editStage();
state.stageDraft.name = "Any status stage";
await state.submitStage();
assert.equal(savedStages.length, 0, "empty status selectors must block saving");
assert.equal(state.stageStatusOptions.value[0].value, "any");
for (const side of ["from", "to"]) {
  assert.equal(state.stageSelectorMissing(state.stageDraft.config[side]), true);
  state.updateStageStatuses(side, [1, 2]);
  state.updateStageStatuses(side, [1, 2, "any"]);
  assert.deepEqual(state.stageDraft.config[side], {
    mode: "any",
    status_ids: [],
  });
  assert.equal(
    state.stageSelectorMissing(state.stageDraft.config[side]),
    false,
  );
  state.updateStageStatuses(side, ["any", 2]);
  assert.deepEqual(state.stageDraft.config[side], {
    mode: "selected",
    status_ids: [2],
  });
  state.updateStageStatuses(side, [2, "any"]);
  state.updateStageStatuses(side, []);
  assert.equal(state.stageSelectorMissing(state.stageDraft.config[side]), true);
  state.updateStageStatuses(side, ["any"]);
}
await state.submitStage();
assert.equal(savedStages.length, 1);
const stage = savedStages[0];
assert.deepEqual(stage.config, {
  from: { mode: "any", status_ids: [] },
  to: { mode: "any", status_ids: [] },
});
state.editStage({ ...stage, id: 20 });
assert.equal(
  state.transitionText(state.stageDraft.config).includes("sla_selector_any"),
  true,
);
await state.submitStage();
assert.equal(savedStages.length, 2);
assert.deepEqual(
  savedStages[1].config,
  stage.config,
  "editing must preserve any status",
);

for (const [from, to, overlap] of [
  [[1], [1], true],
  [[1, 2], [2, 3], true],
  [[1, 2], [3, 4], false],
  [["any"], [1], false],
  [[1], ["any"], false],
]) {
  for (const editing of [false, true]) {
    state.editStage(editing ? { ...stage, id: 20 } : undefined);
    state.stageDraft.name = "Status overlap check";
    state.updateStageStatuses("from", []);
    state.updateStageStatuses("to", []);
    state.updateStageStatuses("from", from);
    state.updateStageStatuses("to", to);
    assert.equal(state.stageStatusesOverlap.value, overlap);
    const count = savedStages.length;
    await state.submitStage();
    assert.equal(savedStages.length, count + (overlap ? 0 : 1));
  }
}

function valid() {
  state.editPolicy();
  state.policyDraft.name = "Test SLA";
  for (const priority of state.targetPriorities.value)
    state.targetCell(priority.id, 10).value = 30;
}
async function invalid(change, expected) {
  valid();
  change();
  const count = saved.length;
  await state.submitPolicy();
  assert.equal(saved.length, count, "invalid form was saved");
  assert.equal(warnings.at(-1).key, expected);
}
await invalid(() => {
  state.policyDraft.name = " ";
}, "sla_policyNameRequired");
await invalid(() => {
  state.selectedStages.value = [];
}, "sla_targetStagesRequired");
await invalid(() => {
  state.targetCell(1, 10).value = undefined;
}, "sla_targetLimitRequired");
assert.equal(warnings.at(-1).priority, "High");
assert.equal(warnings.at(-1).stage, "First response");
await invalid(() => {
  state.slots.value = [];
}, "sla_workSlotsRequired");
await invalid(() => {
  state.slots.value[0].weekdays = [];
}, "sla_slotWeekdaysRequired");
await invalid(() => {
  state.slots.value[0].end = "08:00";
}, "sla_slotTimeInvalid");
await invalid(() => {
  state.addAction();
  state.actionDrafts.value[0].stage_id = 99;
}, "sla_actionStageRequired");
await invalid(() => {
  state.addAction();
  state.actionDrafts.value[0].position = undefined;
}, "sla_actionTimingRequired");
await invalid(() => {
  state.addAction();
  state.actionDrafts.value[0].value = null;
}, "sla_actionOffsetInvalid");
await invalid(() => {
  state.addAction();
  state.actionDrafts.value[0].value = 31;
}, "sla_actionOffsetExceedsLimit");
await invalid(() => {
  state.addAction();
  state.actionDrafts.value[0].recipients = [];
}, "sla_actionRecipientsRequired");
await invalid(() => {
  state.addAction();
  state.addAction();
  state.actionDrafts.value[1].recipients = ["role:999"];
}, "sla_actionRecipientsRequired");
assert.equal(warnings.at(-1).index, 2);

valid();
await state.submitPolicy();
assert.deepEqual(saved.at(-1).config.actions, [], "actions must be optional");
valid();
state.addAction();
state.actionDrafts.value[0].recipients = ["assignees", "role:2", "role:3"];
await state.submitPolicy();
const policy = saved.at(-1);
assert.deepEqual(policy.config.actions[0].recipients, [
  { type: "assignees" },
  { type: "workspace_role", role_id: 2 },
  { type: "workspace_role", role_id: 3 },
]);
state.editPolicy({ ...policy, id: 1 });
assert.deepEqual(
  [...state.actionDrafts.value[0].recipients],
  ["assignees", "role:2", "role:3"],
);
await state.submitPolicy();
assert.deepEqual(
  saved.at(-1).config.actions,
  policy.config.actions,
  "editing must preserve every recipient and action key",
);
state.actionDrafts.value = [];
await state.submitPolicy();
assert.deepEqual(
  saved.at(-1).config.actions,
  [],
  "removing all actions must be saved",
);
valid();
state.addAction();
const webhookAction = state.actionDrafts.value[0];
webhookAction.channel = "webhook";
webhookAction.recipients = [];
await state.submitPolicy();
assert.equal(warnings.at(-1).key, "sla_webhookRequired");
webhookAction.webhooks = [
  {
    platform: "feishu",
    url: "https://open.feishu.cn/open-apis/bot/v2/hook/test",
    at_mode: "user_id",
    secret: "test",
    at_user_ids: [1],
    at_roles: ["assignee"],
    at_all: false,
  },
];
await state.submitPolicy();
const webhookPolicy = saved.at(-1);
assert.equal(webhookPolicy.config.actions[0].channel, "webhook");
assert.deepEqual(webhookPolicy.config.actions[0].recipients, []);
state.editPolicy({ ...webhookPolicy, id: 2 });
await state.submitPolicy();
assert.deepEqual(
  saved.at(-1).config.actions,
  webhookPolicy.config.actions,
  "webhook configuration must survive editing",
);
scope.stop();
console.log(
  "SLA stage any-status round-trip, form validation, optional actions and multiple-recipient round-trip passed",
);
