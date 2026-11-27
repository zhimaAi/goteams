import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
const source = readFileSync(
  new URL(
    "../src/views/workspace/settings/WarningNotificationTab.vue",
    import.meta.url,
  ),
  "utf8",
);
assert.ok(!source.includes("warning_module"));
const form = { value: { config: { type: "requirement", conditions: [{}] } } };
let calls = 0;
const change = runInNewContext(
  `(${source.slice(source.indexOf("function templateChanged("), source.indexOf("function field("))})`,
  {
    form,
    templates: { value: [{ id: 42, name: "Invite", type: "defect" }] },
    fields: { value: [{}] },
    metadata: () => calls++,
  },
);
change(42);
assert.equal(form.value.config.type, "defect");
assert.equal(form.value.config.template_id, 42);
assert.equal(form.value.config.template_name, "Invite");
assert.equal(form.value.config.conditions.length, 0);
assert.equal(calls, 1);
console.log("warning template checks passed");
