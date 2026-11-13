import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { parse, compileScript, compileTemplate } from "@vue/compiler-sfc";
import { createMemoryHistory, createRouter } from "vue-router";
import { computed, reactive } from "vue";
import { buildCategoryOptions } from "../src/utils/requirementCategory.js";
import { sortFieldsByOrder } from "../src/utils/fieldOrder.js";
import issueRoutes from "../src/router/issues.js";
import {
  legacyIssueRoute,
  issueWorkItemRoute,
  workItemRoute,
} from "../src/utils/workItemRoutes.js";

const router = createRouter({
  history: createMemoryHistory(),
  routes: issueRoutes,
});
assert.equal(
  router.resolve("/issues/15/settings/templates/new").name,
  "IssueTemplateCreate",
);
assert.equal(
  router.resolve("/issues/15/settings/templates/12").name,
  "IssueTemplateSettings",
);
assert.equal(
  router.resolve("/issues/15/items/267/history/9").name,
  "IssueHistoryDetail",
);
assert.equal(
  router.resolve("/issues/15/items/267").meta.workItemSurface,
  "issue",
);
assert.deepEqual(workItemRoute("issue", "detail", "15", 267), {
  name: "IssueDetail",
  params: { wsId: "15", id: 267 },
  query: {},
});
assert.equal(
  workItemRoute(undefined, "detail", "15", 267).name,
  "WorkItemDetail",
);
assert.equal(workItemRoute("issue", "create", "15").name, "IssueCreate");
const source = {
  name: "RequirementBoard",
  params: { wsId: "15" },
  query: { tab: "issue-invite", Lang: "en-US" },
  hash: "#templates",
};
assert.equal(legacyIssueRoute(source), null);
assert.deepEqual(issueWorkItemRoute(source), {
  name: "IssueInviteManage",
  params: { wsId: "15" },
  query: { Lang: "en-US" },
  hash: "#templates",
  replace: true,
});
assert.equal(source.query.tab, "issue-invite");
assert.equal(
  legacyIssueRoute({ ...source, query: { tab: "requirement" } }),
  null,
);
assert.equal(
  legacyIssueRoute({
    ...source,
    name: "WorkspaceSettings",
    query: { section: "issue-invite" },
  }).name,
  "IssueSettings",
);

// Internal work-item destinations are independent from invitation management and public filling.
for (const [from, to] of Object.entries({
  RequirementBoard: "IssueList",
  CreateWorkItem: "IssueCreate",
  WorkItemDetail: "IssueDetail",
  WorkItemHistoryDetail: "IssueHistoryDetail",
})) {
  const target = {
    name: from,
    params: { wsId: "15", id: "268", historyId: "1088" },
    query: { tab: "requirement", detail: "268", returnTo: "/issues/15" },
    hash: "#content",
  };
  assert.deepEqual(issueWorkItemRoute(target), {
    ...target,
    name: to,
    replace: true,
  });
}
for (const [tab, name] of Object.entries({
  knowledge: "IssueKnowledge",
  planning: "IssuePlanning",
  iteration: "IssueIteration",
  gantt: "IssueGantt",
})) {
  const query = {
    tab,
    kbFile: "42",
    kbFolder: "43",
    kbCollapsed: "1",
    Lang: "en-US",
  };
  const target = issueWorkItemRoute({
    name: "RequirementBoard",
    params: { wsId: "15" },
    query,
    hash: "#content",
  });
  const { tab: _tab, ...expectedQuery } = query;
  assert.deepEqual(target, {
    name,
    params: { wsId: "15" },
    query: expectedQuery,
    hash: "#content",
    replace: true,
  });
  assert.equal(router.resolve(target).name, name);
  assert.equal(router.resolve(target).meta.boardTab, tab);
  assert.equal(query.tab, tab);
}
assert.deepEqual(
  router.resolve({ name: "IssueKnowledge", params: { wsId: "15" } }).meta
    .anyPermission,
  ["kb:view", "kb:manage"],
);
for (const tab of ["unknown", "__proto__"]) {
  assert.equal(
    issueWorkItemRoute({ name: "RequirementBoard", query: { tab } }),
    null,
  );
}
assert.equal(issueWorkItemRoute({ name: "IssueInviteManage" }), null);
assert.equal(issueWorkItemRoute({ name: "IssueInvite" }), null);
assert.equal(issueWorkItemRoute({ name: "IssueList" }), null);
assert.equal(
  issueWorkItemRoute({
    name: "RequirementBoard",
    params: { wsId: "15" },
    query: {},
  }).name,
  "IssueList",
);

// Dashboard routes and copied links retain the selected dashboard across the migration.
assert.equal(
  router.resolve("/issues/15/dashboard?dashboardId=42").name,
  "IssueDashboard",
);
assert.equal(
  router.resolve("/issues/15/dashboard").meta.permission,
  "dashboard:view",
);
const oldDashboard = {
  name: "RequirementBoard",
  params: { wsId: "15" },
  query: { tab: "dashboard", dashboardId: "42", Lang: "en-US" },
  hash: "#chart",
};
assert.deepEqual(issueWorkItemRoute(oldDashboard), {
  name: "IssueDashboard",
  params: { wsId: "15" },
  query: { dashboardId: "42", Lang: "en-US" },
  hash: "#chart",
  replace: true,
});
assert.equal(oldDashboard.query.tab, "dashboard");
const issueDashboardLink = workItemRoute(
  "issue",
  "dashboard",
  "15",
  undefined,
  { dashboardId: "42" },
);
assert.equal(
  router.resolve(issueDashboardLink).href,
  "/issues/15/dashboard?dashboardId=42",
);
assert.deepEqual(
  workItemRoute(undefined, "dashboard", "15", undefined, { dashboardId: "42" }),
  {
    name: "RequirementBoard",
    params: { wsId: "15" },
    query: { dashboardId: "42", tab: "dashboard" },
  },
);

const { usesIssuePages } = await import("../src/utils/workspaceType.js");
assert.equal(usesIssuePages("issue", []), true);
assert.equal(usesIssuePages("software", ["issue"]), false);
assert.equal(usesIssuePages("agile", ["issue"]), false);
assert.equal(usesIssuePages("", ["issue"]), true);
assert.equal(usesIssuePages("", []), false);

// Sidebar destinations follow the existing application cache, including switch updates.
const applicationsSource = readFileSync(
  new URL("../src/composables/useWorkspaceApplications.js", import.meta.url),
  "utf8",
)
  .replace(/import[\s\S]*?from\s+["'][^"']+["'];/g, "")
  .replace(/export (async )?function/g, "$1function");
const workspaceEntries = new Function(
  "reactive",
  "usesIssuePages",
  `${applicationsSource}\nreturn { workspaceEntryPath, syncWorkspaceApplications, setWorkspaceApplicationEnabled };`,
)((value) => value, usesIssuePages);
assert.equal(workspaceEntries.workspaceEntryPath(15), "/workspace/15/board");
workspaceEntries.syncWorkspaceApplications(15, [
  "issue",
  "knowledge_base",
  "dashboard",
]);
assert.equal(workspaceEntries.workspaceEntryPath(15), "/issues/15");
assert.notEqual(workspaceEntries.workspaceEntryPath(15), "/issues/15/invite");
workspaceEntries.syncWorkspaceApplications(16, ["defect", "planning"]);
assert.equal(workspaceEntries.workspaceEntryPath(16), "/workspace/16/board");
workspaceEntries.setWorkspaceApplicationEnabled(15, "issue", false);
assert.equal(workspaceEntries.workspaceEntryPath(15), "/workspace/15/board");

workspaceEntries.syncWorkspaceApplications(17, [], "issue");
assert.equal(workspaceEntries.workspaceEntryPath(17), "/issues/17");
workspaceEntries.setWorkspaceApplicationEnabled(17, "issue", false);
assert.equal(workspaceEntries.workspaceEntryPath(17), "/issues/17");
workspaceEntries.syncWorkspaceApplications(18, ["issue"], "software");
assert.equal(workspaceEntries.workspaceEntryPath(18), "/workspace/18/board");
workspaceEntries.syncWorkspaceApplications(19, ["issue"], "agile");
assert.equal(workspaceEntries.workspaceEntryPath(19), "/workspace/19/board");
assert.equal(workspaceEntries.workspaceEntryPath(20, "issue"), "/issues/20");

// A moved setup binding must still reach its template. Production compilation alone
// accepts an undefined _ctx property and would leave the affected control invisible.
const root = fileURLToPath(new URL("../src/", import.meta.url));
// Only template administrators choose types; other users inherit a valid template type.
const createWorkspaceSource = readFileSync(
  path.join(root, "components/layout/CreateWorkspaceModal.vue"),
  "utf8",
);
// Management permission must not bypass template selection.
const startFlowSource = createWorkspaceSource.match(
  /async function startFlow\(\) \{[\s\S]*?\n\}/,
)[0];
for (const canManage of [true, false]) {
  for (const templateList of [[{ id: "120", workspace_type: "issue" }], []]) {
    const state = {
      chooserVisible: { value: false },
      formVisible: { value: false },
      templates: { value: [] },
      workspaceTypes: { value: [] },
      canManageTypes: { value: false },
      resetFormState() {},
      closeFlow: assert.fail,
      listWorkspaceTypes: async () => ({ list: [], can_manage: canManage }),
      listWorkspaceTemplates: async () => templateList,
      createFromTemplate: () => {
        state.formVisible.value = true;
      },
    };
    await new Function(
      "state",
      `let flowSeq = 0;
      const { chooserVisible, formVisible, templates, workspaceTypes,
        canManageTypes, resetFormState, closeFlow, listWorkspaceTypes,
        listWorkspaceTemplates, createFromTemplate } = state;
      ${startFlowSource}
      return startFlow();`,
    )(state);
    assert.equal(state.canManageTypes.value, canManage);
    assert.equal(state.chooserVisible.value, templateList.length > 0);
    assert.equal(state.formVisible.value, templateList.length === 0);
    assert.deepEqual(
      state.templates.value,
      templateList.map((tpl) => ({ ...tpl, id: Number(tpl.id) })),
    );
  }
}
const selectTemplateSource = createWorkspaceSource.match(
  /function createFromTemplate\(tpl\) \{[\s\S]*?\n\}/,
)[0];
const createForm = {};
const createFormVisible = { value: false };
const templateChooserVisible = { value: true };
const availableWorkspaceTypes = {
  value: ["software", "agile", "issue", "custom_123456789abc"].map((key) => ({
    key,
  })),
};
const missingTemplateMessages = [];
const selectTemplate = new Function(
  "form",
  "formVisible",
  "chooserVisible",
  "workspaceTypes",
  "message",
  "$t",
  `${selectTemplateSource}\nreturn createFromTemplate;`,
)(
  createForm,
  createFormVisible,
  templateChooserVisible,
  availableWorkspaceTypes,
  { error: (text) => missingTemplateMessages.push(text) },
  (key) => key,
);
for (const type of ["software", "agile", "issue", "custom_123456789abc"]) {
  selectTemplate({ id: 120, name: "Renamed template", workspace_type: type });
  assert.equal(createForm.workspaceType, type);
  assert.equal(createForm.templateWorkspaceId, 120);
  assert.equal(createFormVisible.value, true);
  assert.equal(templateChooserVisible.value, false);
}
for (const type of [undefined, null, ""]) {
  createFormVisible.value = false;
  templateChooserVisible.value = true;
  selectTemplate({
    id: 121,
    name: "Incomplete template",
    workspace_type: type,
  });
  assert.equal(createForm.workspaceType, "software");
  assert.equal(createForm.templateWorkspaceId, 121);
  assert.equal(createFormVisible.value, true);
  assert.equal(templateChooserVisible.value, false);
}
assert.deepEqual(missingTemplateMessages, []);
createFormVisible.value = false;
templateChooserVisible.value = true;
selectTemplate({
  id: 122,
  name: "Invalid template",
  workspace_type: "unknown",
});
assert.equal(createForm.workspaceType, "software");
assert.equal(createForm.templateWorkspaceId, 121);
assert.equal(createFormVisible.value, false);
assert.equal(templateChooserVisible.value, true);
assert.deepEqual(missingTemplateMessages, [
  "workspace_templateTypeUnavailable",
]);
selectTemplate(null);
assert.equal(createForm.workspaceType, "software");
assert.equal(createForm.templateWorkspaceId, 0);
const createWorkspaceTemplate = parse(createWorkspaceSource).descriptor
  .template;
const workspaceTypeField = createWorkspaceTemplate.ast.children
  .flatMap((modal) => modal.children || [])
  .flatMap((form) => form.children || [])
  .find((field) =>
    field.props?.some(
      (prop) => prop.name === "name" && prop.value?.content === "workspaceType",
    ),
  );
assert.equal(
  workspaceTypeField.props.find((prop) => prop.name === "if")?.exp?.content,
  "canManageTypes",
);

function walk(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const target = path.join(directory, entry.name);
    return entry.isDirectory() ? walk(target) : [target];
  });
}
const files = [
  ...walk(path.join(root, "views/issues")),
  ...walk(path.join(root, "components/work-item")),
  ...walk(path.join(root, "components/dashboard")),
  ...[
    "layouts/IssueLayout.vue",
    "layouts/IssueBoardLayout.vue",
    "layouts/Sidebar.vue",
    "components/layout/CreateWorkspaceModal.vue",
    "views/workspace/BoardPage.vue",
    "views/workspace/create/CreateWorkItemPage.vue",
    "components/common/RequirementDetailDrawer.vue",
  ].map((file) => path.join(root, file)),
].filter((file) => file.endsWith(".vue"));
for (const file of files) {
  const { descriptor, errors } = parse(readFileSync(file, "utf8"));
  assert.deepEqual(errors, [], file);
  const script = compileScript(descriptor, { id: file });
  const template = compileTemplate({
    source: descriptor.template.content,
    filename: file,
    id: file,
    compilerOptions: { bindingMetadata: script.bindings },
  });
  assert.deepEqual(template.errors, [], file);
  const missing = [...template.code.matchAll(/_ctx\.([A-Za-z_]\w*)/g)].map(
    (match) => match[1],
  );
  assert.deepEqual(
    [...new Set(missing)],
    [],
    `Unbound template properties in ${path.relative(root, file)}`,
  );
}

// App navigation remains complete on issue pages.
const boardSource = readFileSync(
  path.join(root, "composables/useWorkItemBoard.js"),
  "utf8",
);

// Template columns must use the same preferences as the list settings.
const columnContext = reactive({
  activeTab: { value: "requirement" },
  fieldConfigsByType: {
    value: Object.fromEntries(
      ["requirement", "defect"].map((type) => [
        type,
        ["id", "title", "status", "version"].map((key, sort_order) => ({
          key,
          enabled: 1,
          sort_order,
        })),
      ]),
    ),
  },
  customFieldsByType: { value: { requirement: [], defect: [] } },
  listPreferences: { fields: [] },
  frozenColumnIndex: { value: -1 },
});
const columnModel = new Function(
  "computed",
  "sortFieldsByOrder",
  "context",
  `
  const { activeTab, fieldConfigsByType, customFieldsByType,
    listPreferences, frozenColumnIndex } = context;
  const t = (key) => key;
  const agingEnabled = { value: false };
  const slaEnabled = { value: false };
  const columnWidths = {};
  const columnWidth = (type, key, fallback) => key === "template" ? 220 : fallback;
  const editableCellProps = () => ({});
  const tableAreaWidth = { value: 1400 };
  const selectionColumnWidth = 48;
  ${boardSource.slice(boardSource.indexOf("  const builtinListFieldMeta ="), boardSource.indexOf("  const agingEnabled ="))}
  ${boardSource.slice(boardSource.indexOf("  const systemListFields ="), boardSource.indexOf("  function normalizeListSort("))}
  ${boardSource.slice(boardSource.indexOf("  const tableCols ="), boardSource.indexOf("  const tableScrollWidth ="))}
  return { systemListFields, tableCols };
`,
)(computed, sortFieldsByOrder, columnContext);
for (const type of ["requirement", "defect"]) {
  columnContext.activeTab.value = type;
  columnContext.listPreferences.fields = [];
  const fields = columnModel.systemListFields.value;
  const templateIndex = fields.findIndex(
    (field) => field.preferenceKey === "builtin:template",
  );
  assert.equal(fields[templateIndex - 1].columnKey, "status");
  assert.equal(fields[templateIndex].kind, "builtin");
  assert.equal(fields[templateIndex].enabled, true);
  assert.equal(fields[templateIndex].label, "settings_template");
  assert.equal(columnModel.tableCols.value[templateIndex].key, "template");
  assert.equal(columnModel.tableCols.value[templateIndex].width, 220);

  columnContext.listPreferences.fields = fields.map((field) => ({
    ...field,
    visible: field.columnKey !== "template",
  }));
  assert.equal(
    columnModel.tableCols.value.some((column) => column.key === "template"),
    false,
  );
  const template = { ...fields[templateIndex], visible: true };
  columnContext.listPreferences.fields = [
    template,
    ...fields.filter((field) => field.columnKey !== "template"),
  ];
  columnContext.frozenColumnIndex.value = 0;
  assert.equal(columnModel.tableCols.value[0].key, "template");
  assert.equal(columnModel.tableCols.value[0].fixed, "left");
  assert.equal(columnModel.tableCols.value[1].fixed, undefined);
  columnContext.frozenColumnIndex.value = -1;
}

const defaultTabsSource = boardSource.match(
  /const boardTabs = \[[\s\S]*?\n  \];/,
)[0];
const accessSource = boardSource.match(
  /function canAccessBoardTab\(tab\) \{[\s\S]*?(?=\n  function requestedBoardTab)/,
)[0];
const permissions = new Set(["requirement:view", "kb:view", "dashboard:view"]);
const navigationAvailability = new Function(
  "authStore",
  "enabledApplications",
  "canViewDashboard",
  `${defaultTabsSource}\n${accessSource}\nreturn availableBoardTabs;`,
)(
  { hasPermission: (key) => permissions.has(key) },
  { value: ["knowledge_base", "dashboard", "issue"] },
  { value: true },
);
assert.deepEqual(navigationAvailability(), [
  "requirement",
  "dashboard",
  "knowledge",
  "issue-invite",
]);
permissions.delete("kb:view");
assert.equal(navigationAvailability().includes("knowledge"), false);
const tabChangeSource = boardSource.match(
  /async function handleTabChange\(key\) \{[\s\S]*?(?=\n  async function activateBoardTab)/,
)[0];
const destinations = [];
const changeTab = new Function(
  "surface",
  "route",
  "router",
  "wsId",
  "workspaceMetadata",
  "enabledApplications",
  "usesIssuePages",
  `${tabChangeSource}\nreturn handleTabChange;`,
)(
  "issue",
  { name: "IssueInviteManage" },
  { push: (target) => destinations.push(target) },
  { value: "15" },
  { value: { workspace: { workspace_type: "issue" } } },
  { value: ["issue"] },
  usesIssuePages,
);
await changeTab("knowledge");
await changeTab("dashboard");
await changeTab("requirement");
await changeTab("issue-invite");
assert.deepEqual(destinations, [
  { name: "IssueKnowledge", params: { wsId: "15" } },
  { name: "IssueDashboard", params: { wsId: "15" } },
  { name: "IssueList", params: { wsId: "15" }, query: { tab: "requirement" } },
  { name: "IssueInviteManage", params: { wsId: "15" } },
]);

// Leaving a board while its initial template request is pending must not rewrite the new route.
const mountSource = boardSource.match(
  /onMounted\(async \(\) => \{([\s\S]*?)\n  \}\);/,
)[1];
const mountContinuation = mountSource.slice(mountSource.indexOf("    try {"));
const mountSequenceSetup = mountSource.slice(
  0,
  mountSource.indexOf("    window.addEventListener("),
);
let finishBoardTemplates;
let normalizedAfterLeaving = 0;
const pendingBoard = new Function(
  "loadWorkflowOptions",
  "loadTemplateOptions",
  "normalizeBoardTabQuery",
  `
  let workspaceLoadSequence = 0;
  const sharedDetailId = null;
  const isWorkItemTab = { value: false };
  const boardContextReady = { value: false };
  const fetchRequirements = () => {};
  const initialize = async () => {
    ${mountSequenceSetup}
    ${mountContinuation}
  };
  return { initialize, dispose: () => { workspaceLoadSequence += 1; } };
`,
)(
  async () => {},
  () =>
    new Promise((resolve) => {
      finishBoardTemplates = resolve;
    }),
  () => {
    normalizedAfterLeaving += 1;
  },
);
const initializingBoard = pendingBoard.initialize();
await Promise.resolve();
assert.equal(typeof finishBoardTemplates, "function");
pendingBoard.dispose();
finishBoardTemplates();
await initializingBoard;
assert.equal(normalizedAfterLeaving, 0);

// Only the visible issue subset is reordered; workspace-only tabs retain their slots.
const headerSource = readFileSync(
  path.join(root, "components/work-item/WorkItemBoardHeader.vue"),
  "utf8",
);
// The dashboard panel mounts after metadata; its exposed controls must not be captured as null.
const panelProps = reactive({
  model: { dashboardPanelRef: null, iterationPanelRef: null },
});
const panelBindingsSource = headerSource.match(
  /const dashboardPanelRef = computed[\s\S]*?(?=\nfunction openSettings)/,
)[0];
const panelBindings = new Function(
  "computed",
  "props",
  `${panelBindingsSource}\nreturn { dashboardPanelRef, iterationPanelRef };`,
)(computed, panelProps);
assert.equal(panelBindings.dashboardPanelRef.value, null);
let searchToggleCount = 0;
panelProps.model = {
  dashboardPanelRef: {
    toggleSearch: () => {
      searchToggleCount += 1;
    },
  },
  iterationPanelRef: { views: [] },
};
panelBindings.dashboardPanelRef.value.toggleSearch();
assert.equal(searchToggleCount, 1);
assert.deepEqual(panelBindings.iterationPanelRef.value.views, []);

const mergeSource = headerSource.match(
  /function mergeNavigationOrder\(order\) \{[\s\S]*?(?=\nfunction navigationPayload)/,
)[0];
const navRouter = {
  currentRoute: { value: { meta: { workItemSurface: "issue" } } },
};
const navigationOrder = {
  value: [
    "requirement",
    "planning",
    "knowledge",
    "issue-invite",
    "defect",
    "dashboard",
  ],
};
const mergeNavigationOrder = new Function(
  "router",
  "navigationOrder",
  "availableNavigationTabs",
  `${mergeSource}\nreturn mergeNavigationOrder;`,
)(navRouter, navigationOrder, {
  value: ["requirement", "defect", "issue-invite"],
});
assert.deepEqual(mergeNavigationOrder(["issue-invite", "requirement"]), [
  "issue-invite",
  "planning",
  "knowledge",
  "requirement",
  "dashboard",
]);
assert.deepEqual(navigationOrder.value, [
  "requirement",
  "planning",
  "knowledge",
  "issue-invite",
  "defect",
  "dashboard",
]);
navRouter.currentRoute.value.meta.workItemSurface = undefined;
assert.deepEqual(mergeNavigationOrder(["planning"]), ["planning"]);

// A save started in the previous workspace may finish after layout cleanup.
const storeSource = readFileSync(
  path.join(root, "stores/issueInvite.js"),
  "utf8",
)
  .replace(/import[\s\S]*?from\s+["'][^"']+["'];/g, "")
  .replace("export const", "const");
let resolveConfigSave;
let resolveTemplateSave;
const createInviteStore = new Function(
  "defineStore",
  "ref",
  "updateIssueInviteConfigApi",
  "updateIssueInviteTemplateApi",
  `${storeSource}\nreturn useIssueInviteStore;`,
)(
  (_name, setup) => setup,
  (value) => ({ value }),
  () =>
    new Promise((resolve) => {
      resolveConfigSave = resolve;
    }),
  () =>
    new Promise((resolve) => {
      resolveTemplateSave = resolve;
    }),
);
const inviteStore = createInviteStore();
const configSave = inviteStore.saveConfig("old", { guide_title: "old" });
const templateSave = inviteStore.saveTemplate("old", 1, { name: "old" });
inviteStore.reset();
inviteStore.config.value = { guide_title: "new" };
inviteStore.templates.value = [{ id: 1, name: "new" }];
resolveConfigSave({ guide_title: "old" });
resolveTemplateSave({ id: 1, name: "old" });
await Promise.all([configSave, templateSave]);
assert.equal(inviteStore.config.value.guide_title, "new");
assert.equal(inviteStore.templates.value[0].name, "new");

// Resolve the newer workspace before the old request: the old response must not
// replace options or load templates under the currently selected workspace.
const optionsSource = readFileSync(
  path.join(root, "composables/useCreateWorkItemOptions.js"),
  "utf8",
)
  .replace(/import[\s\S]*?from\s+["'][^"']+["'];/g, "")
  .replace("export function", "function");
const pending = new Map();
const useOptions = new Function(
  "getWorkspaceMetadata",
  "listRequirementCategories",
  "emptyCustomFieldValue",
  "currentProjectMembers",
  "buildCategoryOptions",
  `${optionsSource}\nreturn useCreateWorkItemOptions;`,
)(
  (wsId) => new Promise((resolve) => pending.set(wsId, resolve)),
  async () => ({ list: [] }),
  () => "",
  (members) => members || [],
  buildCategoryOptions,
);
const value = (initial) => ({ value: initial });
const context = {
  selectedWs: value("old"),
  type: value("requirement"),
  authStore: { consumeProjectMetadata: () => null },
  t: (key) => key,
  fieldEnabled: () => true,
  returnCategoryQuery: value(""),
  categoryId: value(0),
  versions: value([]),
  members: value([]),
  customFieldMembers: value([]),
  statuses: value([]),
  status: value(undefined),
  priorities: value([]),
  priority: value(undefined),
  fieldConfigs: value([]),
  categories: value([]),
  customFields: value([]),
  customValues: value({}),
};
const templateScopes = [];
context.loadTemplates = async () =>
  templateScopes.push(context.selectedWs.value);
const options = useOptions(context);
const oldRequest = options.loadOptions();
context.selectedWs.value = "new";
const newRequest = options.loadOptions();
pending.get("new")({ requirement: { statuses: [{ id: 2, name: "new" }] } });
await newRequest;
pending.get("old")({ requirement: { statuses: [{ id: 1, name: "old" }] } });
await oldRequest;
assert.equal(context.statuses.value[0].id, 2);
assert.deepEqual(templateScopes, ["new"]);
assert.deepEqual(
  buildCategoryOptions(
    [
      { id: 2, parent_id: 1, name: "Child" },
      { id: 1, parent_id: 0, name: "Parent" },
    ],
    "Uncategorized",
  ),
  [
    { value: 0, label: "Uncategorized" },
    { value: 1, label: "Parent" },
    { value: 2, label: "　Child" },
  ],
);
console.log(
  `Issue navigation and template bindings passed (${files.length} components)`,
);
