import assert from "node:assert/strict";
import { createSSRApp, h, provide, ref } from "vue";
import { renderToString } from "@vue/server-renderer";
import { createI18n } from "vue-i18n";
import { createServer } from "vite";
import { useDashboardWorkItems } from "../src/composables/useDashboardWorkItems.js";
import {
  categoryDateRange,
  chartEntries,
  defaultChartConfig,
} from "../src/components/dashboard/dashboardCharts.js";
import zh from "../src/locales/zh-CN.js";
import en from "../src/locales/en-US.js";

const categoryEntry = chartEntries.find(
  (entry) => entry.kind === "category_stats",
);
const categoryConfig = defaultChartConfig(categoryEntry);
const workloadEntry = chartEntries.find(
  (entry) => entry.kind === "workload_stats",
);
const workloadConfig = defaultChartConfig(workloadEntry);
const workloadCard = { card_type: "chart", config: workloadConfig };
const categoryCard = { card_type: "chart", config: categoryConfig };
const [customStart, customEnd] = categoryDateRange("custom", [
  "2026-10-31",
  "2026-11-02",
]);
assert.equal(customEnd.add(1, "day").diff(customStart, "day"), 3);
assert.equal(categoryConfig.data_scope, "requirement");

const workspaceType = ref("issue");
const workItems = useDashboardWorkItems(workspaceType);
const filters = {
  common: { logic: "and", conditions: [] },
  work_item_types: ["requirement", "defect"],
  requirement: {
    logic: "and",
    conditions: [{ field: "status", operator: "in", values: ["1"] }],
  },
  defect: {
    logic: "and",
    conditions: [{ field: "status", operator: "in", values: ["2"] }],
  },
};
assert.deepEqual(workItems.workItemTypes.value, ["requirement"]);
assert.equal(workItems.isCardVisible(categoryCard), true);
assert.equal(workItems.isCardVisible(workloadCard), true);
assert.equal(workItems.dataScope("all"), "requirement");
assert.equal(workItems.dataScope("defect"), "");
assert.equal(workItems.dataScope(""), "");
assert.deepEqual(workItems.normalizeFilters(filters).work_item_types, [
  "requirement",
]);
assert.deepEqual(workItems.normalizeFilters(filters).defect.conditions, []);
assert.deepEqual(
  workItems.normalizeFilters(filters).requirement,
  filters.requirement,
);
assert.equal(
  filters.defect.conditions.length,
  1,
  "stored filters remain intact",
);
for (const card_type of ["metric", "chart"]) {
  assert.equal(
    workItems.isCardVisible({ card_type, config: { data_scope: "defect" } }),
    false,
  );
  assert.equal(
    workItems.isCardVisible({ card_type, config: { data_scope: "all" } }),
    true,
  );
}
for (const type of ["software", "agile", "", undefined]) {
  workspaceType.value = type;
  assert.equal(workItems.isCardVisible(categoryCard), false);
  assert.equal(workItems.isCardVisible(workloadCard), false);
  assert.equal(workItems.dataScope("all"), "all");
  assert.equal(workItems.dataScope("defect"), "defect");
  assert.deepEqual(workItems.normalizeFilters(filters), filters);
  assert.equal(
    workItems.isCardVisible({
      card_type: "chart",
      config: { data_scope: "defect" },
    }),
    true,
  );
}

// Render real Vue components so hidden controls, legends and percentages are checked together.
const server = await createServer({
  server: { middlewareMode: true, ws: false },
  appType: "custom",
});
try {
  const { default: Config } = await server.ssrLoadModule(
    "/src/components/dashboard/DashboardChartConfig.vue",
  );
  const { default: Chart } = await server.ssrLoadModule(
    "/src/components/dashboard/DashboardChart.vue",
  );
  const { default: Filter } = await server.ssrLoadModule(
    "/src/components/dashboard/DashboardFilterPanel.vue",
  );
  const { default: Types } = await server.ssrLoadModule(
    "/src/components/dashboard/DashboardCardTypes.vue",
  );
  const { default: CategoryStats } = await server.ssrLoadModule(
    "/src/components/dashboard/DashboardStatsTable.vue",
  );
  async function render(component, props, type, locale) {
    const app = createSSRApp({
      setup() {
        provide("dashboardWorkspaceType", ref(type));
        return () => h(component, props);
      },
    });
    app.use(
      createI18n({
        legacy: false,
        locale,
        messages: { "zh-CN": zh, "en-US": en },
      }),
    );
    const stub = {
      setup:
        (_, { slots }) =>
        () =>
          h("div", slots.default?.()),
    };
    for (const name of [
      "a-config-provider",
      "a-empty",
      "a-divider",
      "a-input",
      "a-radio-group",
      "a-radio",
      "a-radio-button",
      "a-checkbox-group",
      "a-checkbox",
      "a-input-number",
      "a-select",
      "a-button",
      "a-badge",
      "a-range-picker",
      "a-avatar",
      "a-table-summary-row",
      "a-table-summary-cell",
      "a-popover",
      "a-tooltip",
      "a-tag",
      "a-date-picker",
      "a-input-group",
    ])
      app.component(name, stub);
    app.component("a-table", {
      props: ["dataSource", "columns"],
      setup: (table) => () => {
        const [parent] = table.dataSource;
        const rate = table.columns.find(
          (column) => column.dataIndex === "resolution_rate",
        );
        return h("div", [
          parent.employee || parent.date || parent.name,
          parent.children?.[0]?.name,
          rate.customRender({ record: parent }),
        ]);
      },
    });
    return renderToString(app);
  }
  const entries = chartEntries.filter((entry) =>
    [
      "status_bar",
      "priority_bar",
      "completion_line",
      "status_pie",
      "daily_created_line",
    ].includes(entry.kind),
  );
  for (const locale of ["zh-CN", "en-US"]) {
    const messages = locale === "zh-CN" ? zh : en;
    const defect = locale === "zh-CN" ? "缺陷" : "Defect";
    assert.ok(
      (await render(Types, {}, "issue", locale)).includes(
        messages.dashboard_chart_category,
      ),
    );
    assert.ok(
      !(await render(Types, {}, "software", locale)).includes(
        messages.dashboard_chart_category,
      ),
    );
    const categoryForm = await render(
      Config,
      { config: categoryConfig },
      "issue",
      locale,
    );
    assert.ok(categoryForm.includes(messages.dashboard_category_by_category));
    assert.ok(categoryForm.includes(messages.dashboard_category_all_members));
    assert.ok(!categoryForm.includes(defect));
    const categoryTable = await render(
      CategoryStats,
      {
        filters: { preset: "week" },
        rows: [
          {
            id: 1,
            parent_id: 0,
            name: "Parent category",
            created: 12,
            resolved: 8,
          },
          {
            id: 2,
            parent_id: 1,
            name: "Child category",
            created: 5,
            resolved: 4,
          },
        ],
      },
      "issue",
      locale,
    );
    assert.ok(categoryTable.includes("Parent categoryChild category66.7%"));
    assert.ok(categoryTable.includes('role="radiogroup"'));
    assert.ok(categoryTable.includes('button-style="solid"'));
    const workloadForm = await render(
      Config,
      { config: workloadConfig },
      "issue",
      locale,
    );
    assert.ok(workloadForm.includes(messages.dashboard_workload_by_member));
    assert.ok(workloadForm.includes(messages.dashboard_workload_by_date));
    const workloadTable = await render(
      CategoryStats,
      {
        workload: "member",
        filters: { preset: "month30" },
        members: [{ user_id: 7, user_name: "Member 7" }],
        rows: [
          {
            group: "7",
            created: 9,
            resolved: 4,
            assigned: 6,
            assigned_resolved: 4,
          },
        ],
      },
      "issue",
      locale,
    );
    assert.ok(workloadTable.includes("Member 7"));
    assert.ok(workloadTable.includes("66.7%"));
    assert.ok(workloadTable.includes('role="radiogroup"'));
    assert.ok(workloadTable.includes('button-style="solid"'));
    for (const entry of entries) {
      const config = defaultChartConfig(entry);
      const issue = await render(Config, { config }, "issue", locale);
      assert.ok(
        !issue.includes(defect),
        `${entry.kind}: issue controls contain no defects`,
      );
      const ordinary = await render(Config, { config }, "software", locale);
      assert.ok(
        ordinary.includes(defect),
        `${entry.kind}: ordinary controls retain defects`,
      );
      const result = {
        buckets: [
          {
            id: 1,
            type: "requirement",
            name: "ticket-status",
            count: 2,
            timestamp: 1793318400,
          },
          {
            id: 2,
            type: "defect",
            name: "defect-status",
            count: 98,
            timestamp: 1793318400,
          },
        ],
      };
      const chart = await render(Chart, { config, result }, "issue", locale);
      assert.ok(!chart.includes("defect-status"));
      assert.ok(!chart.includes(defect));
      if (["status_pie", "priority_bar"].includes(entry.kind))
        assert.ok(
          chart.includes("100.00%"),
          "percentages exclude hidden defects",
        );
    }
    for (const compact of [true, false]) {
      const issue = await render(
        Filter,
        { modelValue: filters, compact, defaultCollapsed: false },
        "issue",
        locale,
      );
      assert.ok(!issue.includes(defect), "filter controls contain no defects");
      const ordinary = await render(
        Filter,
        { modelValue: filters, compact, defaultCollapsed: false },
        "software",
        locale,
      );
      if (!compact) assert.ok(ordinary.includes(defect));
    }
  }
} finally {
  await server.close();
}
console.log(
  "Dashboard work item scopes, controls, filters and percentages passed in both locales.",
);
