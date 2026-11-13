import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parse, compileScript } from "@vue/compiler-sfc";
import * as vue from "vue";

const { descriptor } = parse(
  readFileSync(
    new URL(
      "../src/components/notification/RobotUserMappingModal.vue",
      import.meta.url,
    ),
    "utf8",
  ),
);
const source = compileScript(descriptor, { id: "mapping" })
  .content.replace(
    /^import\s+([\s\S]*?)\s+from\s+["']([^"']+)["'];?\n/gm,
    (_, binding, name) =>
      `const ${binding.startsWith("{") ? binding.replace(/\bas\b/g, ":") : `{ default: ${binding.trim()} }`} = modules[${JSON.stringify(name)}];\n`,
  )
  .replace("export default", "return");
for (const enterprise of [false, true]) {
  const writes = [],
    reads = [],
    events = [];
  let fail = false;
  const list = async (...args) => {
    reads.push(args);
    if (fail) throw new Error("load failed");
    return [
      {
        user_id: 7,
        push: { feishu_user_id: "existing", dingtalk_at_user_ids: "ding" },
      },
    ];
  };
  const modules = {
    vue,
    "vue-i18n": { useI18n: () => ({ t: (key) => key }) },
    "ant-design-vue": { message: { success() {} } },
    "@/api/workspace": {
      listNotificationUserMappings: list,
      saveNotificationUserMappings: async (items) => writes.push(items),
    },
    "@/api/automation": {
      listAutomationUserMappings: list,
      saveAutomationUserMappings: async (ws, items) => {
        assert.equal(ws, 18);
        writes.push(items);
      },
    },
  };
  const member = enterprise
    ? { id: "7", name: "Member" }
    : { user_id: "7", user_name: "Member" };
  const scope = vue.effectScope();
  const state = scope.run(() =>
    new Function("modules", source)(modules).setup(
      { open: true, enterprise, wsId: 18, members: [member, member] },
      { expose() {}, emit: (...event) => events.push(event) },
    ),
  );
  await new Promise(setImmediate);
  assert.equal(reads.length, 1);
  assert.equal(
    state.mappingRows.value.length,
    1,
    "duplicate users must collapse",
  );
  assert.equal(
    state.mappingRows.value[0].feishu_user_id,
    "existing",
    "load shared values even when members have no user_push",
  );
  await state.saveMapping();
  assert.equal(writes.length, 0, "unchanged mappings must not be written");
  state.mappingRows.value[0].feishu_user_id = "";
  await state.saveMapping();
  assert.deepEqual(
    writes,
    [[{ user_id: 7, push: { feishu_user_id: "" } }]],
    "clear only the edited field; preserve other channels",
  );
  fail = true;
  await state.resetRows();
  await state.saveMapping();
  assert.equal(writes.length, 1, "load failure must prevent writes");
  assert.deepEqual(events.at(-1), ["update:open", false]);
  scope.stop();
}
console.log("Robot mapping regression checks passed");
