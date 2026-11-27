import assert from "node:assert/strict";
import { createServer } from "vite";
import { fileURLToPath } from "node:url";
import { readFileSync } from "node:fs";
import { parse, compileScript } from "@vue/compiler-sfc";
import * as vue from "vue";

const server = await createServer({
  configFile: false,
  resolve: {
    alias: { "@": fileURLToPath(new URL("../src", import.meta.url)) },
  },
  server: { middlewareMode: true, ws: false },
});
try {
  const { useStatusTransitionFieldOrder } = await server.ssrLoadModule(
    "/src/composables/useStatusTransitionFieldOrder.js",
  );
  const {
    statusTransitionForm,
    buildStatusTransitionListUpdate,
    statusTransitionFields,
    statusTransitionFieldOptions,
  } = await server.ssrLoadModule("/src/utils/statusTransition.js");
  const { createStatusTransitionDraft, isStatusTransitionFieldEmpty } =
    await server.ssrLoadModule("/src/utils/statusTransitionDraft.js");
  const keys = ["severity_id", "estimated_work_hours", "responsible_ids"];
  const form = {
    enabled: 1,
    fields: keys.map((key) => ({ key, required: 1 })),
  };
  assert.equal(
    statusTransitionForm({ defect_status_transition: form }, "defect").fields
      .length,
    3,
  );
  assert.equal(
    statusTransitionForm({ requirement_status_transition: form }, "requirement")
      .fields.length,
    0,
  );
  assert.equal(
    statusTransitionForm(
      { defect_status_transition: form },
      "defect",
      [],
      [{ key: "severity_id", enabled: 0 }],
    ).fields.length,
    2,
  );
  for (const value of [null, undefined, ""]) {
    assert.equal(
      isStatusTransitionFieldEmpty({ key: "estimated_work_hours" }, value),
      true,
    );
  }
  assert.equal(
    isStatusTransitionFieldEmpty({ key: "estimated_work_hours" }, 0),
    false,
  );
  assert.equal(
    isStatusTransitionFieldEmpty({ key: "responsible_ids" }, []),
    true,
  );
  assert.equal(
    isStatusTransitionFieldEmpty({ key: "responsible_ids" }, [7]),
    false,
  );
  const item = {
    type: "defect",
    severity_id: 4,
    estimated_work_hours: 2.5,
    responsible_ids: [7],
    responsible_names: ["Seven"],
    assignee_ids: [9],
  };
  const draft = createStatusTransitionDraft(item, []);
  assert.deepEqual(draft.responsible_ids, [7]);
  assert.equal(draft.estimated_work_hours, 2.5);
  assert.equal(draft.severity_id, 4);
  const patch = buildStatusTransitionListUpdate(
    item,
    {
      status_id: 2,
      severity_id: 5,
      estimated_work_hours: 0,
      responsible_ids: [8],
    },
    new Set(keys),
    {
      defect: { severities: [{ id: 5, label: "Fatal", color: "red" }] },
      members: [{ user_id: 8, user_name: "Eight" }],
    },
  );
  assert.equal(patch.severity_name, "Fatal");
  assert.equal(patch.severity_color, "red");
  assert.equal(patch.estimated_work_hours, 0);
  assert.deepEqual(patch.responsible_names, ["Eight"]);
  assert.deepEqual(patch.assignee_ids, [9]);
  const unchanged = buildStatusTransitionListUpdate(
    item,
    { status_id: 2, responsible_ids: [] },
    new Set(),
    {},
  );
  assert.deepEqual(unchanged.responsible_ids, [7]);
  const settings = {
    requirement_status_transition: {
      enabled: 1,
      fields: [{ key: "comment", required: 1 }],
      custom_enabled: 1,
      status_rules: [
        { status_id: 10, fields: [{ key: "version", required: 1 }] },
        { status_id: 20, fields: [] },
        { status_id: 30, fields: [{ key: "custom_5", required: 1 }] },
        { status_id: 40, fields: [{ key: "severity_id", required: 1 }] },
      ],
    },
  };
  const resolved = statusTransitionForm(settings, "requirement", [], []);
  assert.deepEqual(statusTransitionFields(resolved, "10"), [
    { key: "version", required: 1 },
  ]);
  for (const id of [20, 30, 40])
    assert.deepEqual(statusTransitionFields(resolved, id), []);
  assert.deepEqual(statusTransitionFields(resolved, 99), [
    { key: "comment", required: 1 },
  ]);
  assert.deepEqual(
    statusTransitionFields({ ...resolved, custom_enabled: 0 }, 20),
    resolved.fields,
  );
  assert.deepEqual(statusTransitionFields({ ...resolved, enabled: 0 }, 10), []);
  const old = statusTransitionForm({}, "requirement");
  assert.equal(old.custom_enabled, 0);
  assert.deepEqual(old.status_rules, []);
  assert.equal(
    settings.requirement_status_transition.status_rules[2].fields.length,
    1,
    "parsing must not mutate source config",
  );
  const options = statusTransitionFieldOptions(
    "requirement",
    [
      { key: "version", enabled: 1 },
      { key: "severity_id", enabled: 1 },
    ],
    [
      { id: 5, name: "Extra", is_enabled: 1 },
      { id: 6, name: "Hidden", is_enabled: 0 },
    ],
    (key) => key,
  );
  assert.deepEqual(
    options.map((field) => field.key),
    ["version", "comment", "custom_5"],
  );
  const descriptor = parse(
    readFileSync(
      new URL(
        "../src/views/workspace/settings/CustomStatusTransitionCard.vue",
        import.meta.url,
      ),
      "utf8",
    ),
  ).descriptor;
  const componentCode = compileScript(descriptor, {
    id: "custom-transition-test",
  })
    .content.replace(
      /^import\s+([\s\S]*?)\s+from\s+["']([^"']+)["'];?\n/gm,
      (_, binding, source) =>
        `const ${binding.startsWith("{") ? binding.replace(/\bas\b/g, ":") : `{default: ${binding.trim()}}`} = modules[${JSON.stringify(source)}];\n`,
    )
    .replace("export default", "return");
  const component = new Function("modules", componentCode)({
    vue,
    "vue-i18n": { useI18n: () => ({ t: (key) => key }) },
    "@ant-design/icons-vue": {},
    "./WorkflowOptionHandle.vue": { default: {} },
    "@/composables/useStatusTransitionFieldOrder": {
      useStatusTransitionFieldOrder,
    },
  });
  const componentProps = vue.reactive({
    form: resolved,
    statuses: [
      { id: 10, name: "First" },
      { id: 20, name: "Second" },
      { id: 50, name: "New" },
    ],
    options,
    canManage: true,
    saving: false,
  });
  const events = [];
  const scope = vue.effectScope();
  const state = scope.run(() =>
    component.setup(componentProps, {
      expose() {},
      emit: (...args) => events.push(args),
    }),
  );
  assert.deepEqual(
    state.availableStatuses.value.map((item) => item.value),
    [50],
  );
  state.startAdd();
  state.newStatusId.value = 50;
  state.add();
  assert.deepEqual(events.at(-1)[1].status_rules.at(-1), {
    status_id: 50,
    fields: [],
  });
  state.startAdd();
  state.editNewFields();
  assert.equal(state.editingNewFields.value, false, "select a status first");
  state.newStatusId.value = 50;
  const beforeNewFields = events.length;
  state.editNewFields();
  state.select("comment", true);
  state.setRequired("comment", true);
  state.saveFields();
  assert.equal(
    events.length,
    beforeNewFields,
    "field selection only updates the row draft",
  );
  assert.equal(state.editingNewFields.value, false);
  assert.deepEqual(state.newFields.value, [{ key: "comment", required: 1 }]);
  state.editNewFields();
  state.select("version", true);
  state.cancelEdit();
  assert.deepEqual(
    state.newFields.value,
    [{ key: "comment", required: 1 }],
    "cancel keeps the previous row draft",
  );
  state.add();
  assert.deepEqual(
    events.at(-1)[1].status_rules.at(-1),
    {
      status_id: 50,
      fields: [{ key: "comment", required: 1 }],
    },
    "new status and required fields must be saved in one request",
  );
  state.startAdd();
  assert.deepEqual(
    state.newFields.value,
    [],
    "a new row must not inherit abandoned fields",
  );
  assert.equal(
    componentProps.form.status_rules.length,
    4,
    "draft must not mutate saved rules",
  );
  const eventCount = events.length;
  state.newStatusId.value = 10;
  state.add();
  assert.equal(events.length, eventCount, "duplicate status must not be saved");
  state.edit(componentProps.form.status_rules[0]);
  state.select("comment", true);
  state.setRequired("comment", true);
  state.cancelEdit();
  assert.equal(
    componentProps.form.status_rules[0].fields.length,
    1,
    "cancel must leave saved fields unchanged",
  );
  state.edit(componentProps.form.status_rules[0]);
  state.select("version", false);
  state.saveFields();
  assert.deepEqual(events.at(-1)[1].status_rules[0].fields, []);
  state.edit(componentProps.form.status_rules[0]);
  state.select("comment", true);
  state.setRequired("comment", true);
  const beforeSort = events.length;
  await state.moveField("comment", -1);
  assert.deepEqual(
    state.fields.value.map((field) => field.key),
    ["comment", "version"],
  );
  assert.equal(
    state.fields.value[0].required,
    1,
    "required flag must follow the moved field",
  );
  assert.equal(
    events.length,
    beforeSort,
    "sorting modal fields must not save before confirmation",
  );
  state.cancelEdit();
  state.edit(componentProps.form.status_rules[0]);
  assert.deepEqual(
    state.fields.value.map((field) => field.key),
    ["version"],
    "cancel discards sorted draft",
  );
  state.select("comment", true);
  await state.moveField("comment", -1);
  state.saveFields();
  assert.deepEqual(
    events.at(-1)[1].status_rules[0].fields.map((field) => field.key),
    ["comment", "version"],
  );
  assert.deepEqual(
    events.at(-1)[1].status_rules[1].fields,
    [],
    "other status rules stay unchanged",
  );
  state.startAdd();
  state.newStatusId.value = 50;
  state.editNewFields();
  state.select("version", true);
  state.select("comment", true);
  await state.moveField("comment", -1);
  state.saveFields();
  state.add();
  assert.deepEqual(
    events
      .at(-1)[1]
      .status_rules.at(-1)
      .fields.map((field) => field.key),
    ["comment", "version"],
    "new row preserves sorted fields",
  );
  const sortedSettings = statusTransitionForm(
    { requirement_status_transition: events.at(-1)[1] },
    "requirement",
  );
  assert.deepEqual(
    statusTransitionFields(sortedSettings, 50).map((field) => field.key),
    ["comment", "version"],
    "runtime form uses persisted order",
  );
  componentProps.canManage = false;
  const beforeDenied = events.length;
  state.remove(10);
  assert.equal(
    events.length,
    beforeDenied,
    "permission revocation must block writes",
  );
  componentProps.canManage = true;
  componentProps.saving = true;
  state.remove(10);
  assert.equal(
    events.length,
    beforeDenied,
    "saving must block duplicate writes",
  );
  componentProps.saving = false;
  componentProps.form = { ...resolved, status_rules: [] };
  await vue.nextTick();
  assert.equal(state.editingStatusId.value, null);
  assert.equal(state.adding.value, false);
  scope.stop();
  function loadPanel(name, modules) {
    const descriptor = parse(
      readFileSync(
        new URL(`../src/views/workspace/settings/${name}.vue`, import.meta.url),
        "utf8",
      ),
    ).descriptor;
    const code = compileScript(descriptor, { id: name })
      .content.replace(
        /^import\s+([\s\S]*?)\s+from\s+["']([^"']+)["'];?\n/gm,
        (_, binding, source) =>
          `const ${binding.startsWith("{") ? binding.replace(/\bas\b/g, ":") : `{default: ${binding.trim()}}`} = modules[${JSON.stringify(source)}];\n`,
      )
      .replace("export default", "return");
    return new Function("modules", code)(modules);
  }
  const constants = await server.ssrLoadModule(
    "/src/constants/statusTransition.js",
  );
  const utils = await server.ssrLoadModule("/src/utils/statusTransition.js");
  const requests = [];
  let saveBehavior = async () => {};
  const panelProps = vue.reactive({
    wsId: 1,
    typeKey: "requirement",
    canManage: true,
    reloadKey: 0,
    statuses: [],
  });
  const parent = loadPanel("AdvancedStatusTransitionCard", {
    vue: { ...vue, onBeforeUnmount() {} },
    "ant-design-vue": { message: { success() {} } },
    "@ant-design/icons-vue": {},
    "vue-i18n": { useI18n: () => ({ t: (key) => key }) },
    "@/api/workspace": {
      getWorkspaceSettings: async (id) => ({
        requirement_status_transition: {
          enabled: 1,
          fields:
            id === 1
              ? [
                  { key: "assignee", required: 0 },
                  { key: "comment", required: 1 },
                ]
              : [{ key: "priority", required: 0 }],
        },
      }),
      listCustomFields: async () => [],
      listFieldConfigs: async () => [
        { key: "assignee", enabled: 1 },
        { key: "priority", enabled: 1 },
      ],
      updateWorkspaceSettings: async (id, body) => {
        requests.push({ id, body });
        await saveBehavior();
      },
    },
    "@/constants/statusTransition": constants,
    "@/utils/statusTransition": utils,
    "./CustomStatusTransitionCard.vue": { default: {} },
    "./WorkflowOptionHandle.vue": { default: {} },
    "@/composables/useStatusTransitionFieldOrder": {
      useStatusTransitionFieldOrder,
    },
  });
  const parentScope = vue.effectScope();
  const parentState = parentScope.run(() =>
    parent.setup(panelProps, { expose() {}, emit() {} }),
  );
  const flush = () => new Promise((resolve) => setImmediate(resolve));
  await flush();
  assert.equal(parentState.ready.value, true);
  await parentState.moveField("comment", -1);
  assert.deepEqual(
    requests
      .at(-1)
      .body.requirement_status_transition.fields.map((field) => field.key),
    ["comment", "assignee"],
  );
  assert.equal(parentState.transitionForm.value.fields[0].required, 1);
  saveBehavior = async () => {
    throw new Error("expected save failure");
  };
  await parentState.moveField("comment", 1);
  assert.deepEqual(
    parentState.transitionForm.value.fields.map((field) => field.key),
    ["comment", "assignee"],
    "failed default sort restores saved order",
  );
  const requestCount = requests.length;
  panelProps.canManage = false;
  await parentState.moveField("comment", 1);
  assert.equal(
    requests.length,
    requestCount,
    "keyboard sorting respects revoked permission",
  );
  panelProps.canManage = true;
  let release;
  saveBehavior = () =>
    new Promise((resolve) => {
      release = resolve;
    });
  const moving = parentState.moveField("comment", 1);
  await parentState.moveField("comment", 1);
  assert.equal(
    requests.length,
    requestCount + 1,
    "in-flight sorting blocks duplicate requests",
  );
  panelProps.wsId = 2;
  await flush();
  release();
  await moving;
  assert.deepEqual(
    parentState.transitionForm.value.fields.map((field) => field.key),
    ["priority"],
    "old save does not overwrite new workspace",
  );
  assert.equal(requests.at(-1).id, 1);
  parentScope.stop();
  const orderSource = readFileSync(
    new URL(
      "../src/composables/useStatusTransitionFieldOrder.js",
      import.meta.url,
    ),
    "utf8",
  )
    .replace(/import[\s\S]*?from\s+["'][^"']+["'];/g, "")
    .replace("export function", "function");
  let dragOptions;
  let created = 0,
    destroyed = 0,
    restores = 0;
  const sortableStub = {
    create(_element, options) {
      created++;
      dragOptions = options;
      return {
        destroy() {
          destroyed++;
        },
      };
    },
  };
  const orderWithStub = new Function(
    "ref",
    "watchPostEffect",
    "Sortable",
    `${orderSource}\nreturn useStatusTransitionFieldOrder;`,
  )(vue.ref, vue.watchPostEffect, sortableStub);
  const dragFields = vue.ref([
    { key: "assignee", required: 1 },
    { key: "comment", required: 0 },
  ]);
  const dragDisabled = vue.ref(false);
  const firstRow = { matches: () => true },
    secondRow = { matches: () => true };
  const dragContainer = {
    children: [secondRow, firstRow],
    insertBefore(item, before) {
      restores++;
      assert.equal(item, firstRow);
      assert.equal(before, secondRow);
    },
  };
  const dragScope = vue.effectScope();
  const dragState = dragScope.run(() =>
    orderWithStub(dragFields, () => dragContainer, dragDisabled),
  );
  await vue.nextTick();
  assert.equal(created, 1);
  dragOptions.onEnd({
    oldDraggableIndex: 0,
    newDraggableIndex: 1,
    item: firstRow,
    from: dragContainer,
  });
  assert.equal(restores, 1, "drag must restore DOM before Vue reorders rows");
  assert.deepEqual(
    dragFields.value.map((field) => field.key),
    ["comment", "assignee"],
  );
  assert.equal(dragFields.value[1].required, 1);
  await flush();
  const beforeBoundary = dragFields.value;
  await dragState.moveField("comment", -1);
  await dragState.moveField("assignee", 1);
  assert.equal(
    dragFields.value,
    beforeBoundary,
    "out-of-bounds keyboard movement is ignored",
  );
  dragDisabled.value = true;
  await vue.nextTick();
  assert.equal(created, destroyed, "disabling sorting destroys its instance");
  await dragState.moveField("comment", 1);
  assert.equal(dragFields.value, beforeBoundary);
  dragDisabled.value = false;
  await vue.nextTick();
  assert.equal(created, destroyed + 1);
  dragScope.stop();
  assert.equal(created, destroyed, "scope cleanup leaves no Sortable instance");
  console.log("status transition field checks passed");
} finally {
  await server.close();
}
