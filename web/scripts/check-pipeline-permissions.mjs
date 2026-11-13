import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parse, compileScript } from "@vue/compiler-sfc";
import * as vue from "vue";

// Execute the actual component setup with mocked API and authentication boundaries.
function panel(name, manage) {
  const filename = new URL(
    `../src/views/pipeline/${name}.vue`,
    import.meta.url,
  );
  const { descriptor } = parse(readFileSync(filename, "utf8"));
  const calls = [];
  const mounted = [];
  let failMembers = false;
  const apis = new Proxy(
    {},
    {
      get: (_, key) => async () => {
        calls.push(key);
        if (key === "listEnterpriseMembers") {
          if (failMembers) throw new Error("members unavailable");
          return [{ id: "2", name: "Member" }];
        }
        if (key === "listEnterpriseDepartments") {
          return { items: [{ id: "3", parent_id: "0", name: "Department" }] };
        }
        if (key === "listPipelines" || key === "listExpertGroups") {
          return { list: [{ id: 1, can_manage: manage }] };
        }
        return {
          id: 1,
          name: "Configuration",
          members: [],
          steps: [],
          agents: [],
        };
      },
    },
  );
  const modules = new Proxy(
    {},
    {
      get: (_, source) => {
        if (source === "vue")
          return { ...vue, onMounted: (fn) => mounted.push(fn) };
        if (source === "vue-i18n")
          return { useI18n: () => ({ t: (key) => key }) };
        if (source === "vue-router") return { useRoute: () => ({ query: {} }) };
        if (source === "@/stores/auth") {
          return {
            useAuthStore: () => ({
              hasEnterprisePermission: (key) =>
                manage && key === "pipeline:manage",
            }),
          };
        }
        if (source.startsWith("@/api/")) return apis;
        if (source.endsWith("/avatars")) {
          return {
            PIPELINE_AVATARS: [],
            PIPELINE_STEP_AVATARS: [],
            EXPERT_GROUP_AVATARS: [],
            AGENT_AVATARS: [],
          };
        }
        return {};
      },
    },
  );
  let code = compileScript(descriptor, { id: name }).content;
  code = code.replace(
    /^import\s+([\s\S]*?)\s+from\s+["']([^"']+)["'];?\n/gm,
    (_, binding, source) => {
      const target = binding.startsWith("{")
        ? binding.replace(/\bas\b/g, ":")
        : `{ default: ${binding.trim()} }`;
      return `const ${target} = __modules[${JSON.stringify(source)}];\n`;
    },
  );
  code = code.replace("export default", "return");
  const component = new Function("__modules", code)(modules);
  const state = component.setup({}, { expose() {} });
  return {
    state,
    calls,
    mounted,
    fail: (value) => {
      failMembers = value;
    },
  };
}

for (const name of ["PipelineSettingsPanel", "ExpertGroupSettingsPanel"]) {
  const create =
    name === "PipelineSettingsPanel" ? "showCreateModal" : "openCreateModal";
  const reader = panel(name, false);
  for (const mount of reader.mounted) await mount();
  assert.equal(reader.state.canManage.value, false);
  assert.ok(
    reader.calls.some(
      (key) => key === "getPipeline" || key === "getExpertGroup",
    ),
  );
  await reader.state[create]();
  await reader.state.openEditBasic();
  await reader.state.handleCreate({ name: "Denied" });
  assert.equal(
    reader.calls.filter((key) => key.startsWith("listEnterprise")).length,
    0,
  );
  assert.equal(reader.state.createModalVisible.value, false);
  assert.equal(reader.state.editBasicModalVisible.value, false);
  assert.ok(!reader.calls.some((key) => key.startsWith("create")));

  const manager = panel(name, true);
  for (const mount of manager.mounted) await mount();
  assert.equal(
    manager.calls.filter((key) => key.startsWith("listEnterprise")).length,
    0,
  );
  manager.fail(true);
  await manager.state[create]();
  assert.equal(manager.state.createModalVisible.value, false);
  assert.equal(manager.state.scopeOptionsLoading.value, false);
  manager.fail(false);
  const opening = manager.state[create]();
  const count = manager.calls.length;
  await manager.state[create]();
  assert.equal(
    manager.calls.length,
    count,
    "duplicate click must not reload options",
  );
  await opening;
  assert.equal(manager.state.createModalVisible.value, true);
  assert.equal(manager.state.memberOptions.value[0].id, 2);
  assert.equal(manager.state.departmentOptions.value[0].id, 3);
  await manager.state.openEditBasic();
  assert.equal(manager.state.editBasicModalVisible.value, true);
  console.log(
    `${name}: read-only, lazy loading, retry and duplicate click checks passed`,
  );
}
