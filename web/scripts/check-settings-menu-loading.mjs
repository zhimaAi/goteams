import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parse, compileScript } from "@vue/compiler-sfc";
import * as vue from "vue";

const read = (path) =>
  readFileSync(new URL(`../src/${path}`, import.meta.url), "utf8");
const pending = [];
const preloaded = new Map();
const permissions = vue.reactive(new Set(["sla:view", "automation:view"]));
const authStore = {
  hasPermission: (key) => permissions.has(key),
  peekProjectMetadata: (id) => preloaded.get(String(id)),
  refreshProjectPermissions: async () => {},
};
const metadata = (type, apps = []) => ({
  workspace: { workspace_type: type },
  enabled_applications: apps,
});
const getWorkspaceMetadata = (id) =>
  new Promise((resolve, reject) => pending.push({ id, resolve, reject }));
const source = read("composables/useWorkspaceApplications.js")
  .replace(/import[\s\S]*?from\s+["'][^"']+["'];/g, "")
  .replace(/export (async )?function/g, "$1function");
const applications = new Function(
  "computed",
  "reactive",
  "watch",
  "getWorkspaceMetadata",
  "useAuthStore",
  `${source}\nreturn { useWorkspaceApplications, ensureWorkspaceApplications, syncWorkspaceApplications, setWorkspaceApplicationEnabled };`,
)(vue.computed, vue.reactive, vue.watch, getWorkspaceMetadata, () => authStore);

const { descriptor } = parse(
  read("components/work-item/WorkspaceSettingsFrame.vue"),
);
const componentSource = compileScript(descriptor, { id: "SettingsFrame" })
  .content.replace(
    /^import\s+([\s\S]*?)\s+from\s+["']([^"']+)["'];?\n/gm,
    (_, binding, name) =>
      `const ${binding.startsWith("{") ? binding.replace(/\bas\b/g, ":") : `{ default: ${binding.trim()} }`} = modules[${JSON.stringify(name)}];\n`,
  )
  .replace("export default", "return");
function frame(id, section = "sla") {
  const route = vue.reactive({
    params: { wsId: id },
    query: { section },
    meta: {},
  });
  const replacements = [];
  const modules = {
    vue,
    "vue-router": {
      useRoute: () => route,
      useRouter: () => ({ replace: (target) => replacements.push(target) }),
    },
    "vue-i18n": { useI18n: () => ({ t: (key) => key }) },
    "ant-design-vue": {},
    "@ant-design/icons-vue": {},
    "@/stores/auth": { useAuthStore: () => authStore },
    "@/stores/workspace": { useWorkspaceStore: () => ({}) },
    "@/api/workspace": { getWorkspaceMetadata },
    "@/composables/useWorkspaceApplications": applications,
    "@/composables/useWorkItemTypeAttributes": {
      useWorkItemTypeAttributes: () => ({ attributes: vue.ref({}) }),
    },
  };
  const scope = vue.effectScope();
  const state = scope.run(() =>
    new Function("modules", componentSource)(modules).setup(
      {},
      { expose() {} },
    ),
  );
  return { state, route, replacements, stop: () => scope.stop() };
}

// Board navigation reuses cached type synchronously, without another request.
applications.syncWorkspaceApplications("18", ["issue"], "issue");
const cached = frame("18");
assert.equal(cached.state.canSLA.value, true);
assert.equal(cached.state.activeSection.value, "sla");
assert.equal(pending.length, 0);
permissions.delete("sla:view");
assert.equal(cached.state.canSLA.value, false);
permissions.add("sla:manage");
assert.equal(cached.state.canSLA.value, true);

// Direct navigation reuses route-guard metadata, including the type check for owners.
preloaded.set("21", metadata("software"));
const software = frame("21");
assert.equal(software.state.canSLA.value, false);
assert.equal(software.state.canAutomation.value, true);
assert.equal(pending.length, 0);
software.stop();

// Cold loads share one request; preserve deep links until the type is known.
const cold = frame("30");
assert.equal(cold.state.applicationsLoading.value, true);
assert.equal(cold.state.activeSection.value, "");
assert.deepEqual(cold.replacements, []);
const loaded = applications.ensureWorkspaceApplications("30");
assert.equal(pending.length, 1);
pending.shift().resolve(metadata("issue", ["issue"]));
await loaded;
await vue.nextTick();
assert.equal(cold.state.canSLA.value, true);
assert.equal(cold.state.activeSection.value, "sla");
assert.deepEqual(cold.replacements, []);

// Switching away while the old request resolves must not show another project's SLA.
cached.route.params.wsId = "40";
await vue.nextTick();
const oldRequest = pending.shift();
const oldLoaded = applications.ensureWorkspaceApplications("40");
cached.route.params.wsId = "21";
await vue.nextTick();
assert.equal(cached.state.workspaceType.value, "software");
oldRequest.resolve(metadata("issue"));
await oldLoaded;
await vue.nextTick();
assert.equal(cached.state.canSLA.value, false);

// Refreshes keep the confirmed menu visible and ignore responses for old routes.
const refresh = cold.state.loadEnabledApplications();
assert.equal(cold.state.canSLA.value, true);
pending.shift().reject(new Error("offline"));
await refresh;
assert.equal(cold.state.canSLA.value, true);
const staleRefresh = cold.state.loadEnabledApplications();
cold.route.params.wsId = "21";
await vue.nextTick();
pending.shift().resolve(metadata("issue"));
await staleRefresh;
assert.equal(cold.state.canSLA.value, false);

// Failed cold loads stop the skeleton and leave independent menus usable; retry works.
const failed = frame("50");
const failedLoad = applications.ensureWorkspaceApplications("50");
pending.shift().reject(new Error("offline"));
await failedLoad;
await vue.nextTick();
assert.equal(failed.state.applicationsLoading.value, false);
assert.equal(failed.state.canAutomation.value, true);
assert.equal(failed.state.canSLA.value, false);
const retry = applications.ensureWorkspaceApplications("50");
pending.shift().resolve(metadata("issue"));
await retry;
assert.equal(failed.state.canSLA.value, true);
for (const instance of [cached, cold, failed]) instance.stop();
assert.equal(pending.length, 0);
console.log(
  "Settings menus: cache, shared loading, permissions, isolation and retry passed.",
);
