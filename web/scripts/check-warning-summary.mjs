import assert from "node:assert/strict";
import { createI18n } from "vue-i18n";
import zh from "../src/locales/zh-CN.js";
import en from "../src/locales/en-US.js";
const i18n = createI18n({
  legacy: false,
  locale: "zh-CN",
  messages: { "zh-CN": zh, "en-US": en },
});
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";

const source = readFileSync(
  new URL(
    "../src/views/workspace/settings/WarningNotificationTab.vue",
    import.meta.url,
  ),
  "utf8",
);
const summary = source.slice(
  source.indexOf("function conditionSummary("),
  source.indexOf("async function load()"),
);
const conditionSummary = runInNewContext(`(${summary})`, {
  t: (key, params) => i18n.global.t(key, params ? { ...params } : {}),
  listFields: {
    value: {
      "requirement:1": [
        {
          key: "priority",
          label: "优先级",
          options: [{ key: "97", name: "紧急" }],
        },
      ],
      "defect:1": [
        {
          key: "priority",
          label: "优先级",
          options: [{ key: "97", name: "严重" }],
        },
      ],
    },
  },
});
const config = {
  type: "requirement",
  template_id: 1,
  relation: "AND",
  conditions: [
    {
      field_key: "title",
      field_label: "标题",
      operator: "contains",
      value: "测试",
    },
    { field_key: "priority", operator: "is", value: 97 },
  ],
};
assert.equal(conditionSummary(config), "标题包含测试且优先级是紧急");
assert.equal(
  conditionSummary({ ...config, type: "defect" }),
  "标题包含测试且优先级是严重",
);
assert.equal(
  conditionSummary({ ...config, relation: "OR" }),
  "标题包含测试或优先级是紧急",
);
i18n.global.locale.value = "en-US";
assert.equal(
  conditionSummary({ ...config, relation: "OR" }),
  "标题 Contains 测试 or 优先级 Is 紧急",
);
i18n.global.locale.value = "zh-CN";
config.conditions = [
  { field_key: "priority", operator: "in", value: ["97", "deleted"] },
];
assert.equal(conditionSummary(config), "优先级属于紧急、deleted");
config.conditions = [
  { field_key: "title", field_label: "标题", operator: "empty" },
];
assert.equal(conditionSummary(config), "标题为空");
config.conditions = [
  {
    field_key: "title",
    field_label: "标题",
    operator: "contains",
    value: "hello world",
  },
];
assert.equal(conditionSummary(config), "标题包含hello world");
console.log("Warning condition summary checks passed");
