import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parse, compileScript } from "@vue/compiler-sfc";
import * as vue from "vue";
import dayjs from "dayjs";

// Run real component handlers with API boundaries replaced, including live permission revocation.
function panel(name, permissions) {
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
      get: (_, key) => async () => {
        calls.push(key);
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
          return { ...vue, onMounted() {}, onBeforeUnmount() {} };
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
  const state = component.setup({ wsId: 3 }, { expose() {}, emit() {} });
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
    key.startsWith("automation:") || key.startsWith("application_market:");
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
  .filter((line) => line.includes("wrapAction"))) {
  const expected =
    line.includes("http.MethodGet") && !line.includes("/copy-targets")
      ? 'withAnyPerm("automation:view", "automation:manage")'
      : 'withPerm("automation:manage")';
  assert.ok(line.includes(expected), line);
}
const catalog = read("internal/app/goteams/controller/enterprise.go");
for (const group of ["GroupKeyAutomation", "GroupKeyApplicationMarket"]) {
  assert.ok(
    catalog.includes(`permission.${group},`),
    `${group} must be configurable`,
  );
}
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
