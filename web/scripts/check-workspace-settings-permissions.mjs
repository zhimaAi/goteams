import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parse, compileScript } from "@vue/compiler-sfc";
import * as vue from "vue";
import dayjs from "dayjs";

// Run real component handlers with API boundaries replaced, including live permission revocation.
function panel(name, permissions, apiOverrides = {}, props = { wsId: 3 }) {
  const { descriptor } = parse(
    readFileSync(
      new URL(`../src/views/workspace/settings/${name}.vue`, import.meta.url),
      "utf8",
    ),
  );
  const calls = [];
  let confirmation;
  const apis = new Proxy(
    {},
    {
      get:
        (_, key) =>
        async (...args) => {
          calls.push(key);
          if (apiOverrides[key]) return apiOverrides[key](...args);
          if (key === "listWorkspaceApplications")
            return [{ app_key: "planning", enabled: 1 }];
          if (key.startsWith("listAutomation")) return [];
          return {};
        },
    },
  );
  const modules = new Proxy(
    {},
    {
      get: (_, source) => {
        if (source === "vue")
          return {
            ...vue,
            onMounted() {},
            onBeforeMount() {},
            onBeforeUnmount() {},
          };
        if (source === "dayjs") return { default: dayjs };
        if (source === "vue-i18n")
          return { useI18n: () => ({ t: (key) => key }) };
        if (source === "vue-router")
          return {
            useRoute: () => ({ query: {} }),
            useRouter: () => ({ push() {}, replace() {} }),
          };
        if (source === "@/stores/auth")
          return {
            useAuthStore: () => ({
              hasPermission: (key) => permissions.has(key),
            }),
          };
        if (source === "@/stores/locale")
          return { useLocaleStore: () => ({ currentLang: "zh-CN" }) };
        if (source === "ant-design-vue")
          return {
            message: { success() {}, warning() {} },
            Modal: {
              confirm: (config) => {
                confirmation = config;
                return { destroy() {} };
              },
            },
          };
        if (source === "@/composables/useWorkspaceApplications")
          return {
            useWorkspaceApplications: () => ({
              defectAppEnabled: vue.ref(true),
            }),
            syncWorkspaceApplications() {},
            setWorkspaceApplicationEnabled() {},
          };
        if (source.startsWith("@/api/")) return apis;
        return {};
      },
    },
  );
  let code = compileScript(descriptor, { id: name })
    .content.replace(
      /^import\s+([\s\S]*?)\s+from\s+["']([^"']+)["'];?\n/gm,
      (_, binding, source) =>
        `const ${binding.startsWith("{") ? binding.replace(/\bas\b/g, ":") : `{ default: ${binding.trim()} }`} = __modules[${JSON.stringify(source)}];\n`,
    )
    .replace("export default", "return");
  const component = new Function("__modules", code)(modules);
  const state = component.setup(props, { expose() {}, emit() {} });
  return { state, calls, confirm: () => confirmation };
}

const permissions = vue.reactive(
  new Set(["automation:view", "application_market:view"]),
);
const automation = panel("AutomationTab", permissions);
assert.equal(automation.state.canManage.value, false);
await automation.state.loadRules();
await automation.state.loadExecutions();
assert.deepEqual(automation.calls, [
  "listAutomationRules",
  "listAutomationExecutions",
]);
automation.calls.length = 0;
for (const method of [
  "openRule",
  "saveRule",
  "toggleRule",
  "openCopyRule",
  "submitCopyRule",
  "removeRule",
])
  await automation.state[method]({ id: 1 });
assert.deepEqual(automation.calls, []);
permissions.delete("automation:view");
permissions.add("automation:manage");
await automation.state.toggleRule({ id: 1 }, true);
await automation.state.removeRule({ id: 1 });
assert.deepEqual(automation.calls, [
  "setAutomationRuleEnabled",
  "deleteAutomationRule",
  "listAutomationRules",
]);
permissions.delete("automation:manage");
assert.equal(automation.state.canManage.value, false);

const market = panel("ApplicationMarketPanel", permissions);
await vue.nextTick();
assert.ok(market.calls.includes("listWorkspaceApplications"));
const app = { app_key: "planning", enabled: true };
market.state.handleToggle(app, false);
assert.equal(market.confirm(), undefined);
permissions.delete("application_market:view");
permissions.add("application_market:manage");
market.state.handleToggle(app, false);
assert.ok(market.confirm());
await market.confirm().onOk();
assert.ok(market.calls.includes("updateWorkspaceApplication"));
// Revoking management while the confirmation is open must not submit a write.
market.calls.length = 0;
market.state.handleToggle(app, true);
permissions.delete("application_market:manage");
await market.confirm().onOk();
assert.deepEqual(market.calls, []);
assert.equal(app.enabled, false);
assert.equal(app.confirming, false);

// Exercise the real satisfaction handlers with each grant combination.
const satisfactionConfig = () => ({
  enabled: true,
  rating_mode: 2,
  auto_status_ids: [1],
  auto_creator_kinds: [1],
  options: [
    { key: "good", value: 2, label: "Good", tags: [{ key: "a", label: "A" }] },
  ],
});
const settle = async () => {
  await Promise.resolve();
  await Promise.resolve();
  await vue.nextTick();
};
const deferred = () => {
  let resolve;
  const promise = new Promise((done) => {
    resolve = done;
  });
  return { promise, resolve };
};
for (const grants of [
  [],
  ["satisfaction:view"],
  ["satisfaction:manage"],
  ["satisfaction:view", "satisfaction:manage"],
]) {
  const permissions = vue.reactive(new Set(grants));
  const props = vue.reactive({ wsId: 3 });
  let readPending;
  let savePending;
  const satisfaction = panel(
    "SatisfactionTab",
    permissions,
    {
      getSatisfactionConfig: () => readPending?.promise || satisfactionConfig(),
      getWorkspaceMetadata: () => ({
        requirement: { statuses: [{ id: 1, name: "Done" }] },
      }),
      listIssueInviteTemplates: () => [{ id: 2, name: "Template", enabled: 1 }],
      saveSatisfactionConfig: () =>
        savePending?.promise || satisfactionConfig(),
    },
    props,
  );
  const { state, calls } = satisfaction;
  await settle();
  assert.equal(state.canView.value, grants.length > 0);
  assert.equal(state.canManage.value, grants.includes("satisfaction:manage"));
  if (!grants.length) {
    assert.deepEqual(calls, []);
    assert.equal(state.config.value, null);
    await state.save();
    state.changeMode(3);
    assert.equal(satisfaction.confirm(), undefined);
    continue;
  }
  assert.equal(state.failed.value, false);
  assert.equal(state.templates.value[0].value, 2);
  calls.length = 0;
  const option = state.config.value.options[0];
  state.addTag(option);
  state.removeTag(option, 0);
  state.changeMode(3);
  if (!state.canManage.value) {
    assert.equal(option.tags.length, 1);
    assert.equal(option.tags[0].key, "a");
    assert.equal(satisfaction.confirm(), undefined);
    await state.save();
    assert.deepEqual(calls, []);
  } else {
    assert.ok(satisfaction.confirm());
    await satisfaction.confirm().onOk();
    assert.equal(state.config.value.rating_mode, 3);
    await state.save();
    assert.deepEqual(calls, ["saveSatisfactionConfig"]);
    // A confirmation opened with management must not write after revocation.
    state.changeMode(4);
    const confirmation = satisfaction.confirm();
    permissions.add("satisfaction:view");
    permissions.delete("satisfaction:manage");
    await settle();
    confirmation.onOk();
    assert.equal(state.config.value.rating_mode, 2);
    calls.length = 0;
    await state.save();
    assert.deepEqual(calls, []);
  }
  // In-flight reads and writes must not restore data after losing access.
  readPending = deferred();
  const loading = state.load();
  permissions.clear();
  assert.equal(state.config.value, null);
  readPending.resolve(satisfactionConfig());
  await loading;
  assert.equal(state.config.value, null);
  assert.equal(state.loading.value, false);
  // Switching space invalidates a save response from the previous space.
  readPending = undefined;
  permissions.add("satisfaction:manage");
  await settle();
  savePending = deferred();
  const saving = state.save();
  props.wsId = 4;
  await settle();
  savePending.resolve({ ...satisfactionConfig(), rating_mode: 5 });
  await saving;
  assert.equal(state.config.value.rating_mode, 2);
}

// Keep server read/write boundaries explicit; copy targets remain a management helper.
const root = new URL("../../", import.meta.url);
const read = (path) => readFileSync(new URL(path, root), "utf8");
// Either permission opens its module; view alone must never grant management.
const board = read("web/src/composables/useWorkItemBoard.js");
const settingsRoute = read("web/src/router/index.js").split(
  'name: "WorkspaceSettings",',
)[1];
const routePermissions = JSON.parse(
  settingsRoute
    .match(/anyPermission:\s*(\[[\s\S]*?\])/)[1]
    .replace(/,\s*]/g, "]"),
);
const frame = read("web/src/components/work-item/WorkspaceSettingsFrame.vue");
for (const key of [
  "automation:view",
  "automation:manage",
  "application_market:view",
  "application_market:manage",
  "satisfaction:view",
  "satisfaction:manage",
  "requirement:view",
  "defect:view",
]) {
  const authStore = { hasPermission: (permission) => permission === key };
  const entry = new Function(
    "computed",
    "authStore",
    `${board.match(/const canViewProjectSettings = computed\([\s\S]*?\n  \);/)[0]} return canViewProjectSettings.value;`,
  )(vue.computed, authStore);
  const expected =
    key.startsWith("automation:") ||
    key.startsWith("application_market:") ||
    key.startsWith("satisfaction:");
  assert.equal(entry, expected, `${key}: settings entry`);
  assert.equal(
    routePermissions.includes(key),
    expected,
    `${key}: settings route`,
  );
  if (expected) {
    const module = key.split(":")[0];
    const menuKeys = JSON.parse(
      frame.match(new RegExp(`hasAny\\((\\["${module}:view"[^\\]]*\\])\\)`))[1],
    );
    assert.ok(menuKeys.some(authStore.hasPermission), `${key}: settings menu`);
  }
}
for (const line of read("internal/app/goteams/route/automation.go")
  .split("\n")
  .filter(
    (line) => line.includes("wrapAction") && line.includes("automation:"),
  )) {
  const expected =
    line.includes("http.MethodGet") &&
    !line.includes("/copy-targets") &&
    !line.includes("/user-mappings")
      ? 'withAnyPerm("automation:view", "automation:manage")'
      : 'withPerm("automation:manage")';
  assert.ok(line.includes(expected), line);
}
const catalog = read("internal/app/goteams/controller/enterprise.go");
for (const group of [
  "GroupKeyAutomation",
  "GroupKeyApplicationMarket",
  "GroupKeySatisfaction",
]) {
  assert.ok(
    catalog.includes(`permission.${group}`),
    `${group} must be configurable`,
  );
}
const satisfactionRoutes = read("internal/app/goteams/route/satisfaction.go");
assert.match(
  satisfactionRoutes,
  /withAnyPerm\("satisfaction:view", "satisfaction:manage"\), controller.GetSatisfactionConfig/,
);
assert.match(
  satisfactionRoutes,
  /withPerm\("satisfaction:manage"\), controller.SaveSatisfactionConfig/,
);
assert.match(
  catalog,
  /slices.Insert\(projectGroupKeys, automationIndex\+1, permission.GroupKeySLA, permission.GroupKeyWarning, permission.GroupKeyAutoAssignment, permission.GroupKeySatisfaction\)/,
);
const routes = read("internal/app/goteams/route/workspace_application.go");
assert.match(
  routes,
  /withAnyPerm\("application_market:view", "application_market:manage"\), controller.GetWorkspaceApplications/,
);
assert.match(
  routes,
  /withPerm\("application_market:manage"\), controller.UpdateWorkspaceApplication/,
);
console.log(
  "Workspace settings permissions: read-only, management, revocation and API boundaries passed.",
);
