import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const webRoot = fileURLToPath(new URL("..", import.meta.url));
const source = (path) =>
  readFile(new URL(`../${path}`, import.meta.url), "utf8");
const repoSource = (path) =>
  readFile(new URL(`../../${path}`, import.meta.url), "utf8");
const vite = await createServer({
  root: webRoot,
  appType: "custom",
  logLevel: "silent",
  server: { middlewareMode: true },
});

try {
  const customField = await vite.ssrLoadModule("/src/utils/customField.js");
  const { CUSTOM_FIELD_TYPE, NUMERIC_SEARCH_OPERATORS } =
    await vite.ssrLoadModule("/src/constants/customField.js");

  const single = {
    id: 1,
    field_type: CUSTOM_FIELD_TYPE.SELECT_SINGLE,
    options: [{ key: "opt_active", name: "有效选项" }],
  };
  const multiple = { ...single, field_type: CUSTOM_FIELD_TYPE.SELECT_MULTI };
  assert.equal(
    customField.optionFieldControlValue(single, "opt_deleted"),
    undefined,
  );
  assert.equal(
    customField.optionFieldControlValue(single, "opt_active"),
    "opt_active",
  );
  assert.deepEqual(
    customField.optionFieldControlValue(multiple, [
      "opt_active",
      "opt_deleted",
    ]),
    ["opt_active"],
  );
  assert.equal(
    customField.formatCustomFieldValue(
      single,
      JSON.stringify({ 1: "opt_deleted" }),
    ),
    "-",
  );

  const historicalTree = {
    logic: "and",
    conditions: [
      {
        field: "custom_1",
        operator: "in",
        values: ["node_a", "node_b"],
      },
      {
        logic: "or",
        conditions: [
          {
            field: "custom_2",
            operator: "in",
            values: ["node_a", "node_b"],
          },
        ],
      },
    ],
  };
  const normalizedTree = customField.normalizeCascaderFilterTree(
    historicalTree,
    [
      {
        field: "custom_1",
        fieldType: CUSTOM_FIELD_TYPE.CASCADER_SINGLE,
      },
      {
        field: "custom_2",
        fieldType: CUSTOM_FIELD_TYPE.CASCADER_MULTI,
      },
    ],
  );
  assert.deepEqual(normalizedTree.conditions[0].values, ["node_b"]);
  assert.deepEqual(normalizedTree.conditions[1].conditions[0].values, [
    "node_a",
    "node_b",
  ]);
  assert.deepEqual(historicalTree.conditions[0].values, ["node_a", "node_b"]);
  assert.deepEqual(
    customField.normalizeCascaderFilterValues(
      CUSTOM_FIELD_TYPE.CASCADER_MULTI,
      ["__all__"],
    ),
    ["__all__"],
  );

  const cascaderViewDescriptors = [
    {
      field: "custom_2",
      fieldType: CUSTOM_FIELD_TYPE.CASCADER_MULTI,
    },
  ];
  const mixedEmptyTree = {
    logic: "and",
    conditions: [
      {
        field: "custom_2",
        operator: "in",
        values: ["__empty__", "node_a"],
      },
    ],
  };
  assert.deepEqual(
    customField.normalizeCascaderViewFilterTree(
      mixedEmptyTree,
      cascaderViewDescriptors,
    ),
    {
      logic: "and",
      conditions: [
        {
          logic: "or",
          conditions: [
            { field: "custom_2", operator: "empty", values: [] },
            { field: "custom_2", operator: "in", values: ["node_a"] },
          ],
        },
      ],
    },
  );
  assert.deepEqual(mixedEmptyTree.conditions[0].values, [
    "__empty__",
    "node_a",
  ]);
  assert.deepEqual(
    customField.normalizeCascaderViewFilterTree(
      {
        logic: "and",
        conditions: [
          {
            field: "custom_2",
            operator: "in",
            values: ["__not_empty__", "node_a"],
          },
        ],
      },
      cascaderViewDescriptors,
    ),
    {
      logic: "and",
      conditions: [{ field: "custom_2", operator: "not_empty", values: [] }],
    },
  );
  assert.deepEqual(
    customField.normalizeCascaderViewFilterTree(
      {
        logic: "and",
        conditions: [
          {
            field: "custom_2",
            operator: "in",
            values: ["__all__"],
          },
          { field: "title", operator: "contains", values: ["保留"] },
        ],
      },
      cascaderViewDescriptors,
    ),
    {
      logic: "and",
      conditions: [{ field: "title", operator: "contains", values: ["保留"] }],
    },
  );
  assert.deepEqual(
    customField.normalizeCascaderViewFilterTree(
      {
        logic: "or",
        conditions: [
          {
            field: "custom_2",
            operator: "in",
            values: ["__empty__", "__not_empty__"],
          },
          { field: "title", operator: "contains", values: ["保留"] },
        ],
      },
      cascaderViewDescriptors,
    ),
    {
      logic: "or",
      conditions: [
        {
          field: "title",
          operator: "contains",
          values: ["保留"],
        },
      ],
    },
  );
  assert.deepEqual(NUMERIC_SEARCH_OPERATORS, [
    "eq",
    "ne",
    "between",
    "empty",
    "not_empty",
  ]);

  const [
    controlSource,
    boardSource,
    modalSource,
    panelSource,
    searchBarSource,
    checkboxPanelSource,
    statusPanelSource,
    cascaderControlSource,
    assigneeSource,
    numberCompareSource,
    dateRangeSource,
    viewFilterSource,
    ganttFilterSource,
    iterationSearchSource,
    numberRangeSource,
    listFieldsSource,
    workItemViewBusinessSource,
  ] = await Promise.all([
    source("src/components/common/CustomFieldControl.vue"),
    source("src/composables/useWorkItemBoard.js"),
    source("src/components/work-item/ViewEditModal.vue"),
    source("src/components/work-item/CascaderSearchPanel.vue"),
    source("src/components/work-item/WorkItemSearchBar.vue"),
    source("src/components/work-item/CheckboxMultiSelectPanel.vue"),
    source("src/components/work-item/StatusMultiPanel.vue"),
    source("src/components/common/CustomFieldCascader.vue"),
    source("src/components/common/AssigneeSelect.vue"),
    source("src/components/work-item/NumberCompareFilter.vue"),
    source("src/components/work-item/DateRangeFilter.vue"),
    source("src/components/work-item/ViewFilterGroup.vue"),
    source("src/views/workspace/board/gantt/GanttFilterBar.vue"),
    source("src/components/iteration/IterationWorkItemList.vue"),
    source("src/components/work-item/NumberRangeFilter.vue"),
    source("src/views/workspace/settings/ListFieldsTab.vue"),
    repoSource("internal/app/goteams/business/workitem_view.go"),
  ]);
  assert.equal(controlSource.match(/:value="optionControlValue"/g)?.length, 2);
  assert.match(controlSource, /:value="memberControlValue"/);
  assert.match(controlSource, /option-label-prop="label"/);
  assert.match(controlSource, /selectedMemberIds/);
  assert.match(boardSource, /normalizeCascaderFilterValues/);
  assert.match(
    boardSource,
    /descriptor\.kind === "custom"[\s\S]*customFieldMembers\.value/,
  );
  assert.match(modalSource, /normalizeCascaderFilterTree/);
  assert.equal(
    modalSource.match(/normalizeCascaderViewFilterTree\(/g)?.length,
    2,
  );
  assert.match(panelSource, /RadioGroup as ARadioGroup/);
  assert.match(panelSource, /:name="multiple \? undefined : radioGroupName"/);
  assert.match(panelSource, /allRecordsSelected/);
  assert.match(panelSource, /v-for="option in filteredLeadingOptions"/);
  assert.match(
    panelSource,
    /option\.label\.toLocaleLowerCase\(\)\.includes\(normalizedKeyword\.value\)/,
  );
  assert.match(
    panelSource,
    /const columns = computed\(\(\) => \{[\s\S]*if \(normalizedKeyword\.value\) return cols;/,
  );
  assert.doesNotMatch(panelSource, /allNonEmptySelected/);
  assert.doesNotMatch(panelSource, /:indeterminate=/);
  assert.match(
    panelSource,
    /function optionChecked\(key\) \{\s*return allRecordsSelected\.value \|\| selected\.value\.has\(String\(key\)\);\s*\}/,
  );
  assert.match(
    panelSource,
    /emitValues\(allRecordsSelected\.value \? \[\] : \[VIEW_VALUE_ALL\]\)/,
  );
  assert.match(
    panelSource,
    /VIEW_VALUE_EMPTY,\s*VIEW_VALUE_NOT_EMPTY,\s*\.\.\.allKeys\.value\.filter\(\(value\) => value !== key\)/,
  );
  assert.match(
    panelSource,
    /\[VIEW_VALUE_EMPTY, VIEW_VALUE_NOT_EMPTY\]\.filter/,
  );
  assert.match(
    searchBarSource,
    /values\.length === 1 && values\[0\] === VIEW_VALUE_EMPTY/,
  );
  assert.match(
    searchBarSource,
    /fieldMeta\(condition\.field\)\.kind === 'custom'[\s\S]*\$t\('member_search'\)/,
  );
  assert.equal(
    checkboxPanelSource.match(
      /:checked="selectedSet\.has\(String\(option\.value\)\)"/g,
    )?.length,
    2,
  );
  assert.match(checkboxPanelSource, /:indeterminate="someOptionsChecked"/);
  assert.match(
    checkboxPanelSource,
    /selectableOptions\.value\.every\([\s\S]*selectedSet\.value\.has\(String\(option\.value\)\)/,
  );
  assert.match(
    statusPanelSource,
    /:checked="selectedSet\.has\(option\.value\)"/,
  );
  assert.match(
    statusPanelSource,
    /:checked="selectedSet\.has\(String\(status\.id\)\)"/,
  );
  assert.match(statusPanelSource, /:indeterminate="someStatusesChecked"/);
  assert.match(cascaderControlSource, /:tree-check-strictly="multiple"/);
  assert.ok(
    (assigneeSource.match(/:checked="draft\.includes\(member\.id\)"/g) || [])
      .length >= 3,
  );
  assert.match(
    assigneeSource,
    /:placeholder="searchPlaceholder \|\| \$t\('assignee_searchPlaceholder'\)"/,
  );
  assert.match(numberCompareSource, /<a-radio-group :value="operator"/);
  assert.match(numberCompareSource, /<a-radio value="not_empty">/);
  assert.match(dateRangeSource, /"aria-pressed": active/);
  assert.match(
    viewFilterSource,
    /node\.operator !== "empty" && node\.operator !== "not_empty"/,
  );
  assert.equal(
    ganttFilterSource.match(/<CheckboxMultiSelectPanel/g)?.length,
    2,
  );
  assert.match(iterationSearchSource, /<WorkItemSearchBar/);
  assert.match(
    workItemViewBusinessSource,
    /values := selected\s*predicate := customJSONValueSQL \+ ` \?\| \{arg\}`\s*if field\.FieldType == define\.ViewFilterCustomTypeCascaderSingle \{\s*values = expandCascaderFilterKeys\(field\.Cascader, selected\)/,
  );
  assert.doesNotMatch(
    workItemViewBusinessSource,
    /if !includeEmpty \{\s*values = expandCascaderFilterKeys/,
  );
  const numberRangeStyle = numberRangeSource.match(
    /\.number-range-filter\s*\{([\s\S]*?)\}/,
  )?.[1];
  assert.ok(numberRangeStyle);
  assert.doesNotMatch(numberRangeStyle, /position:\s*absolute|inset:/);
  assert.match(
    listFieldsSource,
    /options:\s*\(field\.options \|\| \[\]\)\.map\(\(option\) => \(\{\s*key:\s*option\.key \|\| "",\s*name:\s*option\.name \|\| "",\s*\}\)\),/,
  );
  assert.doesNotMatch(
    listFieldsSource,
    /options:\s*\(field\.options \|\| \[\]\)\.map\(\(option\) => option\.name\)/,
  );

  console.log("custom field regression checks passed");
} finally {
  await vite.close();
}
