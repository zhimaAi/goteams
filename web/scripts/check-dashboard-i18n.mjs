import assert from "node:assert/strict";
import { createSSRApp, h, provide, ref } from "vue";
import { renderToString } from "@vue/server-renderer";
import { createI18n } from "vue-i18n";
import { useDashboardI18n } from "../src/composables/useDashboardI18n.js";
import zh from "../src/locales/zh-CN.js";
import en from "../src/locales/en-US.js";

const workspaceType = ref("issue");
const i18n = createI18n({
  legacy: false,
  locale: "zh-CN",
  messages: { "zh-CN": zh, "en-US": en },
});
let t;
const app = createSSRApp({
  setup() {
    provide("dashboardWorkspaceType", workspaceType);
    return () =>
      h({
        setup() {
          ({ t } = useDashboardI18n());
          return () => h("span", t("dashboard_scope_requirement"));
        },
      });
  },
});
app.use(i18n);
assert.equal(await renderToString(app), "<span>工单</span>");

for (const [locale, messages, term] of [
  ["zh-CN", zh, "工单"],
  ["en-US", en, "Ticket"],
]) {
  i18n.global.locale.value = locale;
  workspaceType.value = "issue";
  for (const key of [
    "dashboard_scope_requirement",
    "dashboard_metric_created_requirement_count",
    "dashboard_metric_completed_requirement_count",
    "dashboard_metric_overdue_requirement_count",
    "dashboard_chart_burndown",
    "dashboard_chart_scope_empty",
    "gantt_view_requirement_filters",
  ]) {
    assert.ok(t(key).toLowerCase().includes(term.toLowerCase()), key);
  }
  assert.equal(t("dashboard_scope_defect"), messages.dashboard_scope_defect);
  assert.equal(
    t("dashboard_metric_work_item_count"),
    messages.dashboard_metric_work_item_count,
  );
  assert.equal(
    t("dashboard_chart_undated", { count: 3 }),
    i18n.global.t("dashboard_chart_undated", { count: 3 }),
  );
  for (const type of ["software", "agile", "", undefined]) {
    workspaceType.value = type;
    assert.equal(
      t("dashboard_scope_requirement"),
      messages.dashboard_scope_requirement,
    );
    assert.equal(
      t("dashboard_metric_completed_requirement_count"),
      messages.dashboard_metric_completed_requirement_count,
    );
    assert.equal(
      t("gantt_view_requirement_filters"),
      messages.gantt_view_requirement_filters,
    );
  }
}
console.log(
  "Dashboard labels follow the explicit workspace type in both locales.",
);
