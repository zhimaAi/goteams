import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import * as vue from "vue";
import dayjs from "dayjs";
import * as templateUtils from "../src/utils/workItemTemplate.js";
import {
  currentProjectMembers,
  resolveTemplateDateDefault,
  templateAssigneeIds,
  templateFieldIsVisible,
} from "../src/utils/workItemTemplate.js";

const now = new Date(2026, 8, 2, 15, 30);
assert.equal(
  resolveTemplateDateDefault({ date_type: "current" }, false, now),
  "2026-09-02",
);
assert.equal(
  resolveTemplateDateDefault(
    { date_type: "after", offset_days: 3 },
    false,
    now,
  ),
  "2026-09-05",
);
assert.equal(
  resolveTemplateDateDefault(
    { date_type: "before", offset_days: 2 },
    true,
    now,
  ),
  Math.floor(new Date(2026, 7, 31).getTime() / 1000),
);

const fields = [
  {
    field_key: "category",
    linkages: [{ value: 7, field_keys: ["version", "custom:3"] }],
  },
  { field_key: "version" },
  { field_key: "custom:3" },
  { field_key: "priority" },
];
assert.equal(
  templateFieldIsVisible(fields, "version", () => 7),
  true,
);
assert.equal(
  templateFieldIsVisible(fields, "custom:3", () => 8),
  false,
);
assert.equal(
  templateFieldIsVisible(fields, "custom:3", () => [6, 7]),
  true,
);
assert.equal(
  templateFieldIsVisible(fields, "priority", () => undefined),
  true,
);

const members = [
  { user_id: "2", user_status: "1" },
  { user_id: "2", user_status: "1" },
  { user_id: "9", user_status: "1", removed: true },
  { user_id: "3", user_status: "0" },
  { user_id: "1", user_status: "1" },
];
assert.deepEqual(
  currentProjectMembers(members).map((member) => member.user_id),
  ["2", "1"],
);
assert.deepEqual(templateAssigneeIds([2, 9, 2, "1", 3, 0], members), [2, 1]);
assert.deepEqual(templateAssigneeIds(null, members), []);

const chain = [
  {
    field_key: "custom:1",
    linkages: [{ value: "technical", field_keys: ["custom:2"] }],
  },
  {
    field_key: "custom:2",
    linkages: [{ value: "yes", field_keys: ["custom:3"] }],
  },
  { field_key: "custom:3" },
];
const chainValues = { "custom:1": "purchase", "custom:2": "yes" };
assert.equal(
  templateFieldIsVisible(chain, "custom:3", (key) => chainValues[key]),
  false,
  "hidden fields cannot reveal downstream fields using stale values",
);
chainValues["custom:1"] = "technical";
assert.equal(
  templateFieldIsVisible(chain, "custom:3", (key) => chainValues[key]),
  true,
);
const cycle = [
  {
    field_key: "custom:1",
    linkages: [{ value: "yes", field_keys: ["custom:2"] }],
  },
  {
    field_key: "custom:2",
    linkages: [{ value: "yes", field_keys: ["custom:1"] }],
  },
];
assert.equal(
  templateFieldIsVisible(cycle, "custom:1", () => "yes"),
  false,
  "cyclic rules terminate",
);

// Run the actual composables with in-memory API responses; no browser or framework needed.
const vite = await createServer({
  root: fileURLToPath(new URL("..", import.meta.url)),
  appType: "custom",
  logLevel: "silent",
  server: { middlewareMode: true, hmr: false },
});
let customUtils;
try {
  customUtils = await vite.ssrLoadModule("/src/utils/customField.js");
} finally {
  await vite.close();
}
function loadComposable(file, dependencies) {
  const source = readFileSync(
    new URL(`../src/composables/${file}.js`, import.meta.url),
    "utf8",
  )
    .replace(
      /import\s+([\s\S]*?)\s+from\s+["']([^"']+)["'];?/g,
      (_match, names, path) =>
        `const ${names} = dependencies[${JSON.stringify(path)}];`,
    )
    .replace("export function", "return function");
  return new Function("dependencies", source)({ vue, ...dependencies });
}

let response = {
  enabled: true,
  items: [
    {
      id: 1,
      name: "Equipment",
      title: "equipment title",
      description: "equipment body",
      fields: [
        { field_key: "priority", default_value: 7 },
        {
          field_key: "custom:1",
          default_value: "technical",
          linkages: [{ value: "technical", field_keys: ["custom:2"] }],
        },
        { field_key: "custom:2", default_value: "OVER", required: true },
        { field_key: "custom:3", default_value: "13800138000" },
        { field_key: "custom:4", default_value: "required default" },
      ],
    },
    {
      id: 2,
      name: "Purchasing",
      fields: [{ field_key: "custom:1", default_value: "purchase" }],
    },
  ],
};
const ctx = Object.fromEntries(
  Object.entries({
    selectedWs: "19",
    type: "requirement",
    title: "",
    description: "",
    descriptionMode: "rich-text",
    status: 1,
    priority: undefined,
    versionId: undefined,
    categoryId: 0,
    returnCategoryQuery: "",
    assigneeIds: [],
    plannedStartDate: null,
    plannedEndDate: null,
    estimatedWorkHours: 0,
    statuses: [{ id: 1 }],
    priorityOptions: [{ id: 7 }],
    verOptions: [],
    categoryOptions: [],
    memberOpts: [],
    customFields: [
      { id: 1, name: "Type", field_type: "select_single", is_enabled: 1 },
      { id: 2, name: "OVER", field_type: "text", is_enabled: 0 },
      { id: 3, name: "Phone", field_type: "phone", is_enabled: 0 },
      {
        id: 4,
        name: "Workspace required",
        field_type: "text",
        is_enabled: 1,
        is_required: 1,
      },
      { id: 5, name: "Outside template", field_type: "text", is_enabled: 1 },
    ],
    customValues: {},
  }).map(([key, value]) => [key, vue.ref(value)]),
);
Object.assign(ctx, {
  fieldEnabled: (key) => key !== "priority",
  fieldEditable: () => true,
  copiedOptionId: (options, value) =>
    options.find((option) => option.id === Number(value))?.id,
  copiedDate: (value) => dayjs.unix(value),
  copiedCustomValue: customUtils.parseCustomFieldValue,
  setDraftSuppress() {},
  bumpEditorRevision() {},
  keepTitleOnApply: () => false,
  builtinLabel: (key) => key,
});
const useTemplate = loadComposable("useCreateWorkItemTemplate", {
  "@/api/workspace": {
    listWorkItemTemplates: async () => structuredClone(response),
  },
  "@/utils/workItemTemplate": templateUtils,
});
const scope = vue.effectScope();
const template = scope.run(() => {
  const state = useTemplate(ctx);
  vue.watch(state.templateId, state.onTemplateIdChange);
  return state;
});
await template.loadTemplates();
await vue.nextTick();
assert.equal(ctx.title.value, "equipment title");
assert.equal(ctx.description.value, "equipment body");
assert.equal(
  ctx.priority.value,
  7,
  "template defaults apply even when the attribute switch is off",
);
assert.equal(template.isTemplateFieldVisible("priority"), true);
assert.equal(template.isTemplateFieldVisible("custom:2"), true);
assert.equal(template.isTemplateFieldVisible("custom:5"), false);
assert.equal(template.isTemplateFieldVisible("status"), false);
assert.equal(ctx.customValues.value[2], "OVER");
ctx.customValues.value[1] = "purchase";
assert.equal(template.isTemplateFieldVisible("custom:2"), false);
assert.equal(template.isTemplateRequired("custom:2"), false);
template.templateId.value = 2;
await vue.nextTick();
assert.equal(ctx.title.value, "");
assert.equal(
  ctx.customValues.value[2],
  undefined,
  "switching restores the pre-template baseline",
);
assert.equal(
  template.isTemplateFieldVisible("custom:4"),
  true,
  "old templates retain workspace-required fields",
);
assert.equal(template.isTemplateRequired("custom:4"), true);
assert.equal(template.isTemplateFieldVisible("custom:2"), false);

// Exercise the page's actual field projection and payload serialization.
const pageSource = readFileSync(
  new URL("../src/composables/useWorkItemCreate.js", import.meta.url),
  "utf8",
);
const projection = pageSource.slice(
  pageSource.indexOf("  const propertyItems ="),
  pageSource.indexOf("  const { loadOptions }"),
);
const propertyState = new Function(
  "context",
  `
  const { computed, builtinPropertyKeys, type, requirementWorkHourKeys, fieldConfigs,
    builtinPropertyLabel, fieldEditable, customFields, customTemplateFieldKey,
    selectedTemplate, isTemplateFieldVisible, isTemplateRequired, serializeCustomFieldValue, customValues } = context;
  ${projection}
  return { propertyItems, customFieldPayload };
`,
)({
  ...ctx,
  ...template,
  computed: vue.computed,
  builtinPropertyKeys: ["status", "priority", "version", "category"],
  requirementWorkHourKeys: new Set(),
  fieldConfigs: vue.ref([]),
  builtinPropertyLabel: (key) => key,
  customTemplateFieldKey: templateUtils.customTemplateFieldKey,
  serializeCustomFieldValue: customUtils.serializeCustomFieldValue,
});
assert.deepEqual(
  propertyState.propertyItems.value.map((field) => field.key),
  ["custom:1", "custom:4"],
);

const warnings = [];
const requests = [];
const useSubmit = loadComposable("useCreateWorkItemSubmit", {
  "ant-design-vue": {
    message: { warning: (value) => warnings.push(value), success() {} },
  },
  "@/api/workspace": {
    createRequirement: async (_ws, payload) => {
      requests.push(payload);
      return { id: 123 };
    },
  },
  "@/utils/localDraftRecord": { createDraftSessionId: () => "next-session" },
  "@/utils/workItemActionToast": { showWorkItemActionSuccess() {} },
  "@/utils/customField": customUtils,
});
const submit = useSubmit({
  ...ctx,
  ...template,
  ...propertyState,
  t: (key) => key,
  canCreateCurrentType: vue.ref(true),
  titleTouched: vue.ref(false),
  imageFileIds: vue.ref([]),
  parentId: vue.ref(undefined),
  planningId: vue.ref(0),
  creating: vue.ref(false),
  draft: {
    async flush() {},
    async markPosted() {},
    async startNew() {},
    async list() {},
  },
  goBoard() {},
});
ctx.title.value = "Purchasing request";
await submit.submitCreate(false);
assert.equal(
  requests.length,
  0,
  "workspace-required fields cannot be bypassed by old templates",
);
assert.equal(warnings.pop(), "settings_template_field_required");
ctx.customValues.value[4] = "applicant";
ctx.customValues.value[3] = "invalid hidden phone";
ctx.customValues.value[5] = "stale value";
await submit.submitCreate(false);
assert.deepEqual(requests.at(-1).custom_fields, {
  1: "purchase",
  4: "applicant",
});
assert.equal(requests.at(-1).template_id, 2);
assert.equal(requests.at(-1).priority, null);
assert.equal(requests.at(-1).status_id, 0);
ctx.categoryId.value = 99;
ctx.returnCategoryQuery.value = "7";
await submit.submitCreate(false);
assert.equal(
  requests.at(-1).category_id,
  7,
  "hidden category retains the route context, not stale form input",
);

// Reload from the saved response instead of reusing an earlier template object.
response.items[0].fields[1].default_value = "updated";
await template.loadTemplates();
await vue.nextTick();
assert.equal(ctx.customValues.value[1], "updated");
assert.equal(template.isTemplateFieldVisible("custom:2"), false);
await submit.submitCreate(true);
assert.equal(requests.at(-1).template_id, 1);
assert.equal(requests.at(-1).custom_fields[2], undefined);
assert.equal(
  ctx.customValues.value[1],
  "updated",
  "continue-create reapplies the selected template",
);
response.enabled = false;
await template.loadTemplates();
await vue.nextTick();
assert.equal(template.selectedTemplate.value, undefined);
assert.equal(template.isTemplateFieldVisible("priority"), false);
assert.equal(template.isTemplateFieldVisible("custom:2"), false);
assert.equal(template.isTemplateFieldVisible("custom:5"), true);
const copied = useTemplate({ ...ctx, copyFromId: 123 });
response.enabled = true;
ctx.title.value = "copied title";
await copied.loadTemplates();
assert.equal(copied.selectedTemplate.value, undefined);
assert.equal(
  ctx.title.value,
  "copied title",
  "copy creation does not auto-apply a template",
);
scope.stop();

console.log("work item template regression checks passed");
