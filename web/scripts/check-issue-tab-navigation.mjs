import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import * as vue from "vue";
import { compileScript, parse } from "@vue/compiler-sfc";
import {
  createMemoryHistory,
  createRouter,
  RouterView,
  useRoute,
  useRouter,
} from "vue-router";
import issueRoutes from "../src/router/issues.js";
import {
  issueWorkItemRoute,
  legacyIssueRoute,
} from "../src/utils/workItemRoutes.js";
import { usesIssuePages } from "../src/utils/workspaceType.js";
import { getRememberedTab } from "../src/utils/rememberedTab.js";

const read = (file) =>
  readFileSync(new URL(`../src/${file}`, import.meta.url), "utf8");
const boardSource = read("composables/useWorkItemBoard.js");
const initialTabs = boardSource.slice(
  boardSource.indexOf("  const enabledApplications ="),
  boardSource.indexOf("  // The board tab's secondary-menu title"),
);
// App-only roles must wait for metadata instead of mounting an unauthorized requirement list.
for (const [keys, requested, expected] of [
  [
    ["ticket:copy_link", "ticket:manage", "gantt:view"],
    "issue-invite",
    "issue-invite",
  ],
  [["gantt:view"], "gantt", "gantt"],
  [["defect:view"], "defect", "defect"],
  [[], "requirement", "issue-invite"],
  [["requirement:view"], "requirement", "requirement"],
]) {
  const permissions = new Set(keys);
  const model = new Function(
    "context",
    `const { ref, computed, route, authStore, getRememberedTab } = context;
     const options = {};
     const surface = undefined;
     const issueMetadata = null;
     const wsId = ref("21");
     const setRememberedTab = () => {};
     ${initialTabs}
     return { activeTab, enabledApplications, resolveBoardTab };`,
  )({
    ...vue,
    route: { name: "RequirementBoard", query: { tab: requested } },
    authStore: {
      userInfo: { id: 112 },
      hasPermission: (key) => permissions.has(key),
    },
    getRememberedTab,
  });
  assert.equal(
    model.activeTab.value,
    permissions.has("requirement:view") ? "requirement" : "",
  );
  model.enabledApplications.value = ["issue", "gantt", "defect"];
  assert.equal(model.resolveBoardTab(), expected);
}
const routeWatch = boardSource.slice(
  boardSource.indexOf("  // The issue header outlives"),
  boardSource.indexOf("  // ======================== View handlers"),
);
const metadata = {
  enabled_applications: [
    "defect",
    "dashboard",
    "issue",
    "knowledge_base",
    "planning",
    "iteration",
    "gantt",
  ],
  workspace: { name: "Issue workspace", color: "#123456" },
};
let boardMounts = 0;
const models = [];
function useBoard() {
  boardMounts += 1;
  const route = useRoute();
  const wsId = vue.ref(route.params.wsId);
  const model = new Function(
    "context",
    `
    const { ref, computed, watch, route, wsId, issueMetadata } = context;
    const options = {};
    const surface = route.meta.workItemSurface;
    const authStore = { userInfo: { id: 1 }, hasPermission: () => true };
    const setRememberedTab = () => {};
    const getRememberedTab = (_user, _scope, tabs, fallback) => tabs.includes(fallback) ? fallback : tabs[0];
    ${initialTabs}
    const activateBoardTab = async (key) => { activeTab.value = key; };
    ${routeWatch}
    return { activeTab, boardTabs, enabledApplications,
      isWorkItemTab: computed(() => ["requirement", "defect"].includes(activeTab.value)),
      visibleTopTabs: computed(availableBoardTabs) };
  `,
  )({ ...vue, route, wsId, issueMetadata: { value: metadata } });
  Object.assign(model, {
    wsId,
    boardPageRef: vue.ref(null),
    t: (key) => key,
    workItemAttributes: { displayName: (key) => key },
    boardTabTitle: {},
    workspaceName: vue.ref(metadata.workspace.name),
    projectColor: vue.ref(metadata.workspace.color),
    projectChar: vue.ref("I"),
    workspaceMetadata: vue.ref(metadata),
    searchFieldsForType: () => [],
    handleTabChange: new Function(
      "surface",
      "route",
      "router",
      "wsId",
      "activateBoardTab",
      "workspaceMetadata",
      "enabledApplications",
      "usesIssuePages",
      `${boardSource.match(/async function handleTabChange\(key\) \{[\s\S]*?(?=\n  async function activateBoardTab)/)[0]}\nreturn handleTabChange;`,
    )(
      route.meta.workItemSurface,
      route,
      useRouter(),
      wsId,
      async (key) => {
        model.activeTab.value = key;
      },
      vue.ref(metadata),
      model.enabledApplications,
      usesIssuePages,
    ),
  });
  models.push(model);
  return model;
}

// Compile the real layout/header; replace only their external UI and API dependencies.
function component(file, modules) {
  const { descriptor } = parse(read(file));
  const compiled = compileScript(descriptor, {
    id: file,
    inlineTemplate: true,
  }).content;
  const code = compiled
    .replace(
      /import\s+([\s\S]*?)\s+from\s+["']([^"']+)["'];?/g,
      (_match, bindings, name) =>
        bindings.startsWith("{")
          ? `const ${bindings.replace(/\bas\b/g, ":")} = modules[${JSON.stringify(name)}];`
          : `const ${bindings} = modules[${JSON.stringify(name)}].default;`,
    )
    .replaceAll("import.meta.env", '({ BASE_URL: "/" })')
    .replace("export default", "return");
  return new Function("modules", code)({ vue, ...modules });
}
let navigationRequests = 0;
let tabMounts = 0;
let displayedTabs;
let selectTab;
const Tabs = vue.defineComponent({
  props: ["tabs", "style", "activeKey"],
  setup(props, { emit }) {
    tabMounts += 1;
    displayedTabs = props;
    selectTab = (key) => emit("tab-change", key);
    return () => vue.h("nav", props.tabs.map((tab) => tab.key).join(","));
  },
});
const Empty = vue.defineComponent({ setup: () => () => vue.h("span") });
const Header = component("components/work-item/WorkItemBoardHeader.vue", {
  "ant-design-vue": { message: { success() {} } },
  "@ant-design/icons-vue": { SettingOutlined: Empty, FilterOutlined: Empty },
  "./BoardTopTabs.vue": { default: Tabs },
  "./ViewDropdown.vue": { default: Empty },
  "@/api/workspaceNavigation": {
    getWorkspaceNavigationSettings: async () => {
      navigationRequests += 1;
      return {
        main_nav: [
          "issue-invite",
          "knowledge",
          "planning",
          "iteration",
          "gantt",
          "dashboard",
          "defect",
          "requirement",
        ],
        style: "minimal",
      };
    },
  },
});
const Layout = component("layouts/IssueBoardLayout.vue", {
  "@/composables/useWorkItemBoard": { useWorkItemBoard: useBoard },
  "@/components/work-item/WorkItemBoardHeader.vue": { default: Header },
});
let childModel;
const Child = vue.defineComponent({
  setup() {
    childModel = vue.inject("issueBoard");
    return () => vue.h("article", childModel.activeTab.value);
  },
});
let searchToggles = 0;
const DashboardPage = component("views/issues/IssueDashboardPage.vue", {
  "vue-router": { useRouter },
  "@/components/dashboard/DashboardPanel.vue": {
    default: vue.defineComponent({
      setup(_props, { expose }) {
        expose({
          toggleSearch: () => {
            searchToggles += 1;
          },
        });
        return () => vue.h("section");
      },
    }),
  },
});
const DashboardChild = vue.defineComponent({
  setup() {
    childModel = vue.inject("issueBoard");
    return () => vue.h(DashboardPage);
  },
});
let renderedApplication;
const applicationPanel = (tab) => ({
  default: vue.defineComponent({
    props: ["wsId", "showToolbar"],
    setup(props, { expose }) {
      const route = useRoute();
      renderedApplication = {
        tab,
        wsId: props.wsId,
        query: { ...route.query },
      };
      expose({ views: [], toggleSearch() {} });
      return () => vue.h("section", tab);
    },
  }),
});
const ApplicationPage = component("views/issues/IssueApplicationPage.vue", {
  "vue-router": { useRoute, useRouter },
  "@/views/knowledgebase/KnowledgeBaseView.vue": applicationPanel("knowledge"),
  "@/views/workspace/board/planning/PlanningPanel.vue":
    applicationPanel("planning"),
  "@/views/workspace/board/gantt/GanttPanel.vue": applicationPanel("gantt"),
  "@/components/iteration/IterationPanel.vue": applicationPanel("iteration"),
});
const ApplicationChild = vue.defineComponent({
  setup() {
    childModel = vue.inject("issueBoard");
    return () => vue.h(ApplicationPage);
  },
});
// IssueLayout keys its router outlet by workspace ID.
const WorkspaceOutlet = vue.defineComponent({
  setup() {
    const route = useRoute();
    return () =>
      vue.h(RouterView, null, {
        default: ({ Component }) =>
          vue.h(Component, { key: route.params.wsId }),
      });
  },
});
function routes(records) {
  return records.map((record) => ({
    ...record,
    component: record.children
      ? record.path === "/issues/:wsId"
        ? WorkspaceOutlet
        : Layout
      : record.name === "IssueDashboard"
        ? DashboardChild
        : record.meta?.boardTab
          ? ApplicationChild
          : Child,
    children: record.children ? routes(record.children) : undefined,
  }));
}

const node = (type, text = "") => ({
  type,
  text,
  style: {},
  children: [],
  parent: null,
});
const renderer = vue.createRenderer({
  createElement: node,
  createText: (text) => node("text", text),
  createComment: (text) => node("comment", text),
  setText: (target, text) => {
    target.text = text;
  },
  setElementText: (target, text) => {
    target.text = text;
    target.children = [];
  },
  parentNode: (target) => target.parent,
  nextSibling: (target) =>
    target.parent?.children[target.parent.children.indexOf(target) + 1] || null,
  patchProp(target, key, _previous, value) {
    (target.props ||= {})[key] = value;
  },
  insert(target, parent, anchor = null) {
    if (target.parent)
      target.parent.children.splice(target.parent.children.indexOf(target), 1);
    const index = anchor ? parent.children.indexOf(anchor) : -1;
    parent.children.splice(
      index < 0 ? parent.children.length : index,
      0,
      target,
    );
    target.parent = parent;
  },
  remove(target) {
    target.parent?.children.splice(target.parent.children.indexOf(target), 1);
    target.parent = null;
  },
});
const flush = async () => {
  await Promise.resolve();
  await vue.nextTick();
};

for (const [entry, active] of [
  ["/issues/15/dashboard", "dashboard"],
  ["/issues/15/invite", "issue-invite"],
  ["/issues/15?tab=defect", "defect"],
  ["/issues/15/knowledge?kbFile=42&kbCollapsed=1", "knowledge"],
  ["/issues/15/planning", "planning"],
  ["/issues/15/iteration", "iteration"],
  ["/issues/15/gantt", "gantt"],
]) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: routes(issueRoutes),
  });
  await router.push(entry);
  const before = {
    boards: boardMounts,
    tabs: tabMounts,
    requests: navigationRequests,
  };
  const app = renderer.createApp(RouterView);
  app.use(router);
  app.config.globalProperties.$t = (key) => key;
  const Slot = vue.defineComponent({
    setup:
      (_props, { slots }) =>
      () =>
        vue.h("div", slots.default?.()),
  });
  for (const name of ["a-typography-title", "a-tooltip", "a-button"])
    app.component(name, Slot);
  app.mount(node("root"));
  await flush();
  const original = models.at(-1);
  assert.equal(
    original.activeTab.value,
    active,
    "direct entry must select the requested tab immediately",
  );
  if (active === "dashboard") {
    const before = searchToggles;
    childModel.dashboardPanelRef.value.toggleSearch();
    assert.equal(searchToggles, before + 1);
  }
  if (active === "knowledge") {
    assert.equal(renderedApplication.tab, "knowledge");
    assert.equal(renderedApplication.wsId, "15");
    assert.deepEqual(renderedApplication.query, {
      kbFile: "42",
      kbCollapsed: "1",
    });
  }
  assert.equal(
    childModel.activeTab,
    original.activeTab,
    "child pages share the header model",
  );
  assert.equal(displayedTabs.style, "minimal");
  const order = displayedTabs.tabs.map((tab) => tab.key);
  assert.deepEqual(order, [
    "issue-invite",
    "knowledge",
    "planning",
    "iteration",
    "gantt",
    "dashboard",
    "defect",
    "requirement",
  ]);
  for (const [path, expected] of [
    ["/issues/15?tab=requirement", "requirement"],
    ["/issues/15/dashboard", "dashboard"],
    ["/issues/15/knowledge", "knowledge"],
    ["/issues/15/planning", "planning"],
    ["/issues/15/iteration", "iteration"],
    ["/issues/15/gantt", "gantt"],
    ["/issues/15/invite", "issue-invite"],
    ["/issues/15?tab=defect", "defect"],
  ]) {
    await new Promise((resolve) => {
      const stop = router.afterEach(() => {
        stop();
        resolve();
      });
      selectTab(expected);
    });
    await flush();
    assert.equal(router.currentRoute.value.fullPath, path);
    assert.equal(original.activeTab.value, expected);
    assert.equal(displayedTabs.activeKey, expected);
    assert.deepEqual(
      displayedTabs.tabs.map((tab) => tab.key),
      order,
    );
    assert.equal(displayedTabs.style, "minimal");
    assert.equal(childModel.activeTab, original.activeTab);
    if (expected === "dashboard") {
      const before = searchToggles;
      childModel.dashboardPanelRef.value.toggleSearch();
      assert.equal(searchToggles, before + 1);
    } else {
      assert.equal(childModel.dashboardPanelRef.value, null);
    }
    if (["knowledge", "planning", "iteration", "gantt"].includes(expected)) {
      assert.equal(renderedApplication.tab, expected);
      assert.equal(renderedApplication.wsId, "15");
    }
    assert.equal(
      !!childModel.iterationPanelRef.value,
      expected === "iteration",
    );
  }
  const go = (delta) =>
    new Promise((resolve) => {
      const stop = router.afterEach(() => {
        stop();
        resolve();
      });
      router.go(delta);
    });
  await go(-1);
  await flush();
  assert.equal(original.activeTab.value, "issue-invite");
  await go(1);
  await flush();
  assert.equal(original.activeTab.value, "defect");
  await router.push("/issues/15/knowledge");
  await flush();
  await new Promise((resolve) => {
    const stop = router.afterEach((to) => {
      if (to.name !== "IssueList") return;
      stop();
      resolve();
    });
    original.enabledApplications.value = metadata.enabled_applications.filter(
      (key) => key !== "knowledge_base",
    );
  });
  await flush();
  assert.equal(router.currentRoute.value.name, "IssueList");
  assert.equal(
    displayedTabs.tabs.some((tab) => tab.key === "knowledge"),
    false,
  );
  original.enabledApplications.value = [...metadata.enabled_applications];
  await flush();
  assert.equal(
    boardMounts,
    before.boards + 1,
    "switching tabs must retain the board model",
  );
  assert.equal(
    tabMounts,
    before.tabs + 1,
    "switching tabs must retain the tab component",
  );
  assert.equal(
    navigationRequests,
    before.requests + 1,
    "switching tabs must not reload navigation settings",
  );
  await router.push("/issues/16/dashboard");
  await flush();
  assert.equal(boardMounts, before.boards + 2);
  assert.equal(tabMounts, before.tabs + 2);
  assert.equal(navigationRequests, before.requests + 2);
  assert.equal(models.at(-1).wsId.value, "16");
  assert.notEqual(childModel.activeTab, original.activeTab);
  app.unmount();
}
console.log(
  "Issue tab state, persistent header, navigation settings and browser history passed",
);

// A list setup that finishes after another tab or layout wins must not fetch stale data.
const activationSource = boardSource.match(
  /async function activateBoardTab\(key\) \{[\s\S]*?(?=\n  \/\/ The issue header)/,
)[0];
for (const leave of ["dashboard", "dispose", null]) {
  let ready;
  const pending = new Promise((resolve) => {
    ready = resolve;
  });
  const hydrated = [];
  const fetched = [];
  const state = Object.fromEntries(
    Object.entries({
      activeTab: "requirement",
      page: 4,
      selectedRowKeys: [1],
      kanbanNavigationItems: [1],
      requirements: [{ id: 1 }],
      total: 1,
      boardContextReady: true,
      wsId: "15",
    }).map(([key, value]) => [key, vue.ref(value)]),
  );
  Object.assign(state, {
    flushPendingSearchSave() {},
    setRememberedTab() {},
    authStore: { userInfo: { id: 1 } },
    boardTabStorageScope: () => "project:15",
    ensureViewsLoaded: () => pending,
    fetchVersions: async () => {},
    hydrateSearchState: (key) => hydrated.push(key),
    fetchRequirements: () => {
      if (["requirement", "defect"].includes(state.activeTab.value))
        fetched.push(state.activeTab.value);
    },
  });
  const activation = new Function(
    "state",
    `
    const { ${Object.keys(state).join(", ")} } = state;
    let workspaceLoadSequence = 1;
    let listRequestSequence = 0;
    ${activationSource}
    return { activate: activateBoardTab, dispose: () => { workspaceLoadSequence += 1; } };
  `,
  )(state);
  const switching = activation.activate("defect");
  assert.deepEqual(state.requirements.value, []);
  assert.deepEqual(state.selectedRowKeys.value, []);
  assert.equal(state.page.value, 1);
  if (leave === "dashboard") await activation.activate("dashboard");
  if (leave === "dispose") activation.dispose();
  ready();
  await switching;
  assert.deepEqual(hydrated, leave ? [] : ["defect"]);
  assert.deepEqual(fetched, leave ? [] : ["defect"]);
}
console.log(
  "Pending list initialization is discarded after tab changes or layout disposal",
);

const sharedDetailSource = boardSource.match(
  /async function openSharedDetail\(id\) \{[\s\S]*?(?=\n  \/\/ The drawer emits)/,
)[0];
for (const navigateAway of [false, true]) {
  let finish;
  const pending = new Promise((resolve) => {
    finish = resolve;
  });
  const state = {
    route: {
      name: "IssueList",
      fullPath: "/issues/15?detail=1",
      query: { detail: "1" },
    },
    wsId: vue.ref("15"),
    activeTab: vue.ref("requirement"),
    detailReqId: vue.ref(null),
    getRequirement: () => pending,
    setRememberedTab() {},
    boardTabStorageScope: () => "project:15",
    authStore: { userInfo: { id: 1 } },
    listPreferences: { previewMode: "drawer" },
  };
  const open = new Function(
    "state",
    `
    const { ${Object.keys(state).join(", ")} } = state;
    const surface = "issue";
    const workspaceLoadSequence = 1;
    ${sharedDetailSource}
    return openSharedDetail;
  `,
  )(state);
  const opening = open("1");
  if (navigateAway) {
    state.route.name = "IssueDashboard";
    state.route.fullPath = "/issues/15/dashboard";
    state.route.query = {};
  }
  finish({ id: 1, type: "defect" });
  await opening;
  assert.equal(state.activeTab.value, navigateAway ? "requirement" : "defect");
  assert.equal(state.detailReqId.value, navigateAway ? null : "1");
}
console.log("Pending shared-detail lookup cannot override a newer route");

// Invitation management is shared by agile and issue projects; the enclosing URL
// does not determine which board the other application tabs belong to.
const tabChangeSource = boardSource.match(
  /async function handleTabChange\(key\) \{[\s\S]*?(?=\n  async function activateBoardTab)/,
)[0];
const normalizeSource = boardSource.match(
  /function normalizeBoardTabQuery\(\) \{[\s\S]*?(?=\n  \/\/ App tabs depend)/,
)[0];
for (const workspaceType of ["software", "agile", "issue", ""]) {
  const destinations = [];
  const state = {
    surface: "issue",
    route: { name: "IssueInviteManage", query: {} },
    router: { push: (target) => destinations.push(target) },
    wsId: vue.ref("19"),
    workspaceMetadata: vue.ref({
      workspace: { workspace_type: workspaceType },
    }),
    enabledApplications: vue.ref(metadata.enabled_applications),
    usesIssuePages,
    activateBoardTab: () =>
      assert.fail("must navigate out of invitation management"),
  };
  const change = new Function(
    "state",
    `const { ${Object.keys(state).join(", ")} } = state;
     ${tabChangeSource}
     return handleTabChange;`,
  )(state);
  for (const key of [
    "requirement",
    "defect",
    "planning",
    "iteration",
    "gantt",
    "knowledge",
    "dashboard",
  ]) {
    await change(key);
    const target = destinations.at(-1);
    assert.equal(target.params.wsId, "19");
    if (["software", "agile"].includes(workspaceType)) {
      assert.equal(target.name, "RequirementBoard");
      assert.equal(target.query.tab, key);
    } else {
      assert.equal(
        target.name,
        {
          requirement: "IssueList",
          defect: "IssueList",
          planning: "IssuePlanning",
          iteration: "IssueIteration",
          gantt: "IssueGantt",
          knowledge: "IssueKnowledge",
          dashboard: "IssueDashboard",
        }[key],
      );
    }
  }
}

// A restored invitation tab stays inside the ordinary board.
for (const query of [{}, { tab: "issue-invite", Lang: "en-US" }]) {
  const targets = [];
  const state = {
    options: {},
    surface: undefined,
    route: { name: "RequirementBoard", query },
    activeTab: vue.ref("issue-invite"),
    wsId: vue.ref("19"),
    router: { replace: (target) => targets.push(target) },
    requestedBoardTab: () => query.tab || "",
  };
  const normalize = new Function(
    "state",
    `
    const { ${Object.keys(state).join(", ")} } = state;
    ${normalizeSource}
    return normalizeBoardTabQuery;
  `,
  )(state);
  normalize();
  assert.deepEqual(targets, [], "must not redirect away from the board");
}

const Menu = component("components/work-item/BoardNavigationMenu.vue", {
  "vue-i18n": { useI18n: () => ({ t: (key) => key }) },
  sortablejs: { default: {} },
  "@ant-design/icons-vue": Object.fromEntries(
    ["Check", "Close", "Down", "Edit", "Holder", "Plus"].map((name) => [
      `${name}Outlined`,
      Empty,
    ]),
  ),
});
const menuProps = vue.reactive({
  tabs: [{ key: "requirement", label: "Requirements" }],
  allApps: [{ key: "issue-invite", label: "Invitations" }],
  activeKey: "issue-invite",
  moreLabel: "Invitations",
});
const menuRoot = node("root");
const menuApp = renderer.createApp({ render: () => vue.h(Menu, menuProps) });
menuApp.component("a-button", Empty);
menuApp.component("a-checkbox", Empty);
menuApp.component("a-popover", {
  setup:
    (_props, { slots }) =>
    () =>
      vue.h("div", slots.default?.()),
});
menuApp.mount(menuRoot);
const findTrigger = (target) =>
  target.props?.class?.includes("board-navigation-trigger")
    ? target
    : target.children.map(findTrigger).find(Boolean);
assert.match(findTrigger(menuRoot).props.class, /\bactive\b/);
menuProps.activeKey = "requirement";
await flush();
assert.doesNotMatch(findTrigger(menuRoot).props.class, /\bactive\b/);
menuApp.unmount();
console.log(
  "Invitation routing, remembered entry and overflow selection passed",
);

// Exercise the real permission guard so both directions of canonicalization share
// the same workspace-type decision and finish without redirect loops.
const permissionSource = read("router/permission.js");
const permissionGuard = permissionSource.slice(
  permissionSource.indexOf("router.beforeEach"),
  permissionSource.indexOf("router.afterEach"),
);
let templateLoads = 0;
const invitationApplications = vue.ref(["issue"]);
const Invitation = component("views/issues/IssueInviteManagePage.vue", {
  "vue-router": { useRoute, useRouter },
  "vue-i18n": { useI18n: () => ({ t: (key) => key }) },
  "ant-design-vue": { message: {} },
  "@ant-design/icons-vue": Object.fromEntries(
    ["Copy", "Link", "Setting", "ShareAlt"].map((name) => [
      `${name}Outlined`,
      Empty,
    ]),
  ),
  "@/stores/auth": { useAuthStore: () => ({ hasPermission: () => false }) },
  "@/api/issueInvite": {
    getIssueInviteShare: () => assert.fail("copy-link permission is required"),
    listIssueInviteTemplates: async () => {
      templateLoads += 1;
      return [];
    },
  },
  "@/components/common/WorkItemTypePill.vue": { default: Empty },
  "@/composables/useWorkspaceApplications": {
    useWorkspaceApplications: () => ({ applications: invitationApplications }),
  },
});
const OrdinaryBoard = component("views/workspace/BoardPage.vue", {
  "@/composables/useWorkItemBoard": { useWorkItemBoard: useBoard },
  "@/components/work-item/WorkItemBoardHeader.vue": { default: Header },
  ...Object.fromEntries(
    [
      "@/components/work-item/WorkItemList.vue",
      "./board/planning/PlanningPanel.vue",
      "./board/gantt/GanttPanel.vue",
      "@/components/dashboard/DashboardPanel.vue",
      "@/views/knowledgebase/KnowledgeBaseView.vue",
      "@/components/iteration/IterationPanel.vue",
    ].map((name) => [name, { default: Empty }]),
  ),
  "@/views/issues/IssueInviteManagePage.vue": { default: Invitation },
});
for (const workspaceType of ["software", "agile", "issue", ""]) {
  const isIssue = usesIssuePages(workspaceType, metadata.enabled_applications);
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      ...routes(issueRoutes),
      {
        path: "/workspace/:wsId/board",
        name: "RequirementBoard",
        component: OrdinaryBoard,
        meta: { anyPermission: ["requirement:view", "ticket:manage"] },
      },
      { path: "/403", component: Empty },
    ],
  });
  const permissions = new Set(["workspace:view"]);
  const authStore = {
    token: "test-session",
    permissionWorkspaceId: "19",
    userInfo: { id: 1 },
    initializeSession: async () => {},
    fetchEnterprisePlanInfo() {},
    clearProjectMetadata() {},
    hasPermission: (key) => permissions.has(key),
  };
  const context = {
    router,
    issueWorkItemRoute,
    legacyIssueRoute,
    isIssueWorkspace: async () => isIssue,
    NProgress: { start() {} },
    useAuthStore: () => authStore,
    getWorkspaceAccess: async () => ({ is_member: true }),
    WHITE_LIST: [],
  };
  new Function(
    "context",
    `
    const { ${Object.keys(context).join(", ")} } = context;
    let sessionInitializedOnce = false;
    let initialNavigationPending = true;
    ${permissionGuard}
  `,
  )(context);
  for (const entry of [
    "/issues/19/invite?Lang=en-US#templates",
    "/workspace/19/board?tab=issue-invite&Lang=en-US#templates",
  ]) {
    await router.push(entry);
    assert.equal(
      router.currentRoute.value.name,
      isIssue ? "IssueInviteManage" : "RequirementBoard",
    );
    assert.equal(router.currentRoute.value.query.Lang, "en-US");
    assert.equal(router.currentRoute.value.hash, "#templates");
    assert.equal(
      router.currentRoute.value.query.tab,
      isIssue ? undefined : "issue-invite",
    );
  }
  if (!isIssue) {
    permissions.add("requirement:view");
    await router.push("/workspace/19/board?tab=requirement");
    const before = {
      boards: boardMounts,
      tabs: tabMounts,
      requests: navigationRequests,
    };
    const app = renderer.createApp(RouterView);
    app.use(router);
    app.config.globalProperties.$t = (key) => key;
    for (const name of [
      "a-typography-title",
      "a-tooltip",
      "a-button",
      "a-empty",
      "i18n-t",
    ])
      app.component(name, Empty);
    app.mount(node("root"));
    await flush();
    const original = models.at(-1);
    for (const key of [
      "issue-invite",
      "requirement",
      "knowledge",
      "issue-invite",
    ]) {
      const loads = templateLoads;
      await new Promise((resolve) => {
        const stop = router.afterEach(() => {
          stop();
          resolve();
        });
        selectTab(key);
      });
      await flush();
      assert.equal(router.currentRoute.value.name, "RequirementBoard");
      assert.equal(router.currentRoute.value.query.tab, key);
      assert.equal(displayedTabs.activeKey, key);
      assert.equal(models.at(-1), original);
      assert.equal(templateLoads, loads + (key === "issue-invite" ? 1 : 0));
    }
    const loads = templateLoads;
    invitationApplications.value = [];
    await flush();
    assert.equal(
      templateLoads,
      loads,
      "disabled invitations must not fetch templates",
    );
    invitationApplications.value = ["issue"];
    await flush();
    assert.equal(templateLoads, loads + 1);
    // Browser back/forward or another in-app link must also update the retained board.
    await router.push("/workspace/19/board?tab=requirement");
    await flush();
    assert.equal(original.activeTab.value, "requirement");
    for (const [delta, expected] of [
      [-1, "issue-invite"],
      [1, "requirement"],
    ]) {
      await new Promise((resolve) => {
        const stop = router.afterEach(() => {
          stop();
          resolve();
        });
        router.go(delta);
      });
      await flush();
      assert.equal(original.activeTab.value, expected);
      assert.equal(displayedTabs.activeKey, expected);
    }
    assert.equal(boardMounts, before.boards + 1);
    assert.equal(tabMounts, before.tabs + 1);
    assert.equal(navigationRequests, before.requests + 1);
    app.unmount();
    // The existing settings back button points at the old invitation route.
    permissions.add("ticket:manage");
    await router.push({ name: "IssueSettings", params: { wsId: "19" } });
    assert.equal(router.currentRoute.value.name, "IssueSettings");
    const back = read("components/work-item/WorkspaceSettingsFrame.vue").match(
      /function handleBack\(\) \{[\s\S]*?(?=\nfunction selectSection)/,
    )[0];
    new Function("router", "wsId", "issueSurface", `${back}\nhandleBack();`)(
      router,
      vue.ref("19"),
      vue.ref(true),
    );
    await new Promise((resolve) => {
      const stop = router.afterEach(() => {
        stop();
        resolve();
      });
    });
    assert.equal(router.currentRoute.value.name, "RequirementBoard");
    assert.equal(router.currentRoute.value.query.tab, "issue-invite");
    await router.push("/workspace/19/board?tab=requirement");
    permissions.delete("workspace:view");
    permissions.delete("ticket:manage");
    await router.push("/workspace/19/board?tab=issue-invite");
    assert.equal(router.currentRoute.value.path, "/403");
  }
  // Invitation capabilities must work without the unrelated workspace-view permission.
  for (const key of ["workspace:view", "ticket:copy_link", "ticket:manage"]) {
    permissions.clear();
    permissions.add(key);
    await router.push("/workspace/19/board?tab=issue-invite");
    assert.equal(
      router.currentRoute.value.name,
      isIssue ? "IssueInviteManage" : "RequirementBoard",
      `${workspaceType}: ${key}`,
    );
    await router.push("/issues/19/invite");
    assert.equal(
      router.currentRoute.value.name,
      isIssue ? "IssueInviteManage" : "RequirementBoard",
    );
    if (isIssue) {
      await router.push("/issues/19");
      assert.equal(router.currentRoute.value.name, "IssueInviteManage");
    }
    await router.push("/403");
  }
  permissions.clear();
  await router.push("/workspace/19/board?tab=issue-invite");
  assert.equal(router.currentRoute.value.path, "/403");
}
console.log(
  "Ordinary project invitation tabs retain the board and header; old links and permissions passed",
);
