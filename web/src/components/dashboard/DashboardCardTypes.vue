<template>
  <section class="dashboard-card-types">
    <h3>{{ $t("dashboard_basic_information") }}</h3>
    <div class="dashboard-card-types__grid">
      <button
        v-for="item in availableTypes"
        :key="item.type"
        type="button"
        class="dashboard-card-types__item"
        :disabled="item.type === 'announcement' && hasAnnouncement"
        @click="emit('select', item.type)"
      >
        <span
          class="dashboard-card-types__icon"
          :style="{ backgroundColor: item.color }"
          ><component :is="item.icon"
        /></span>
        <span class="dashboard-card-types__copy"
          ><strong>{{ $t(`dashboard_card_${item.type}`) }}</strong
          ><small>{{ $t(`dashboard_card_${item.type}_desc`) }}</small></span
        >
      </button>
    </div>
    <a-tabs
      :active-key="activeTab"
      class="dashboard-card-types__tabs"
      @change="emit('update:tab', $event)"
    >
      <a-tab-pane
        v-for="group in chartGroups"
        :key="group.key"
        :tab="$t(group.title)"
      >
        <div class="dashboard-card-types__charts">
          <button
            v-for="item in group.entries"
            :key="item.key"
            type="button"
            class="dashboard-card-types__chart"
            @click="emit('select', item.key)"
          >
            <svg viewBox="0 0 180 90" aria-hidden="true">
              <template v-if="item.icon === 'sla_risk'">
                <g
                  v-for="(width, index) in [100, 82, 91]"
                  :key="index"
                  :opacity="1 - index * 0.22"
                >
                  <rect
                    x="22"
                    :y="17 + index * 21"
                    width="13"
                    height="13"
                    rx="4"
                    fill="#fa8c16"
                  />
                  <rect
                    x="44"
                    :y="19 + index * 21"
                    :width="width"
                    height="9"
                    rx="4"
                    fill="#ffd5ad"
                  />
                </g>
              </template>
              <template v-else-if="item.icon === 'sla_distribution'">
                <circle
                  cx="90"
                  cy="46"
                  r="27"
                  fill="none"
                  stroke="#21bd84"
                  stroke-width="13"
                />
                <circle
                  cx="90"
                  cy="46"
                  r="27"
                  fill="none"
                  stroke="#fa8c16"
                  stroke-width="13"
                  stroke-dasharray="56 170"
                  transform="rotate(-90 90 46)"
                />
                <circle
                  cx="90"
                  cy="46"
                  r="27"
                  fill="none"
                  stroke="#f5222d"
                  stroke-width="13"
                  stroke-dasharray="25 170"
                  transform="rotate(-90 90 46)"
                />
              </template>
              <template v-else-if="item.icon === 'bar'">
                <path d="M8 82 H174" stroke="#e5e7eb" />
                <rect
                  v-for="(height, index) in [40, 58, 72, 48, 63]"
                  :key="index"
                  :x="14 + index * 32"
                  :y="82 - height"
                  width="18"
                  :height="height"
                  rx="3"
                  fill="#1890ff"
                  :opacity="1 - index * 0.13"
                />
              </template>
              <path
                v-else-if="item.icon === 'line'"
                d="M8 75 Q30 46 47 60 T91 34 T133 55 T173 14"
                fill="none"
                :stroke="item.scope === 'defect' ? '#52c41a' : '#1890ff'"
                stroke-width="3"
              />
              <template v-else-if="item.icon === 'step'">
                <path d="M8 82 H174" stroke="#e5e7eb" />
                <rect
                  x="16"
                  y="66"
                  width="18"
                  height="16"
                  rx="3"
                  fill="#95de64"
                />
                <rect
                  x="48"
                  y="54"
                  width="18"
                  height="28"
                  rx="3"
                  fill="#73d13d"
                />
                <rect
                  x="80"
                  y="40"
                  width="18"
                  height="42"
                  rx="3"
                  fill="#52c41a"
                />
                <rect
                  x="112"
                  y="26"
                  width="18"
                  height="56"
                  rx="3"
                  fill="#389e0d"
                />
                <path
                  d="M22 58 L57 46 L89 32 L121 18"
                  fill="none"
                  stroke="#237804"
                  stroke-width="2.5"
                />
              </template>
              <template v-else-if="item.icon === 'rank'">
                <rect
                  x="10"
                  y="16"
                  width="120"
                  height="12"
                  rx="6"
                  fill="#1677ff"
                />
                <rect
                  x="10"
                  y="38"
                  width="86"
                  height="12"
                  rx="6"
                  fill="#1677ff"
                  opacity="0.75"
                />
                <rect
                  x="10"
                  y="60"
                  width="54"
                  height="12"
                  rx="6"
                  fill="#1677ff"
                  opacity="0.5"
                />
                <path
                  d="M140 62 L168 26 M168 26 H150 M168 26 V44"
                  fill="none"
                  stroke="#fa8c16"
                  stroke-width="3"
                />
              </template>
              <template v-else-if="item.icon === 'burndown'">
                <path d="M8 82 H174" stroke="#e5e7eb" />
                <path
                  d="M14 20 L170 78"
                  fill="none"
                  stroke="#faad14"
                  stroke-width="3"
                  stroke-dasharray="7 5"
                />
                <path
                  d="M14 22 L60 34 L104 52 L170 74"
                  fill="none"
                  stroke="#1677ff"
                  stroke-width="3"
                />
              </template>
              <template v-else-if="item.icon === 'workload'">
                <g fill="#306bff">
                  <circle cx="53" cy="29" r="17" />
                  <path d="M25 77a28 28 0 0 1 56 0Z" />
                </g>
                <g fill="#aac4ff">
                  <circle cx="125" cy="29" r="17" />
                  <path d="M97 77a28 28 0 0 1 56 0Z" />
                </g>
              </template>
              <template v-else-if="item.icon === 'category'">
                <g v-for="(width, index) in [96, 72, 86]" :key="index">
                  <rect
                    x="24"
                    :y="17 + index * 22"
                    width="14"
                    height="14"
                    rx="3"
                    fill="#1677ff"
                  />
                  <rect
                    x="46"
                    :y="21 + index * 22"
                    :width="width"
                    height="6"
                    rx="3"
                    fill="#b4ceff"
                  />
                </g>
              </template>
              <template v-else>
                <circle
                  cx="78"
                  cy="45"
                  r="32"
                  fill="none"
                  stroke="#52c41a"
                  stroke-width="19"
                />
                <circle
                  cx="78"
                  cy="45"
                  r="32"
                  fill="none"
                  stroke="#1890ff"
                  stroke-width="19"
                  stroke-dasharray="60 142"
                />
                <circle cx="133" cy="28" r="4" fill="#52c41a" />
                <circle cx="133" cy="45" r="4" fill="#1890ff" />
                <path
                  d="M144 28H171 M144 45H171"
                  stroke="#e5e7eb"
                  stroke-width="5"
                />
              </template>
            </svg>
            <span>{{ t(`dashboard_chart_${item.key}`) }}</span>
          </button>
        </div>
      </a-tab-pane>
    </a-tabs>
  </section>
</template>
<script setup>
import { computed } from "vue";
import { useDashboardWorkItems } from "@/composables/useDashboardWorkItems";
import { useDashboardI18n } from "@/composables/useDashboardI18n";
import { chartEntries } from "./dashboardCharts";
import {
  AppstoreOutlined,
  FileTextOutlined,
  InfoCircleOutlined,
  NotificationOutlined,
} from "@ant-design/icons-vue";
const props = defineProps({
  hasAnnouncement: { type: Boolean, default: false },
  canAddSla: { type: Boolean, default: false },
  tab: { type: String, default: "basic" },
});
const emit = defineEmits({
  "update:tab": (value) => value === "basic" || value === "sla",
  select: (value) =>
    [
      "project_info",
      "announcement",
      "metric",
      "sla_metric",
      "text",
      ...chartEntries.map((item) => item.key),
    ].includes(value),
});
const types = [
  { type: "project_info", icon: InfoCircleOutlined, color: "#26bf88" },
  { type: "announcement", icon: NotificationOutlined, color: "#722ed1" },
  { type: "metric", icon: AppstoreOutlined, color: "#1677ff" },
  { type: "text", icon: FileTextOutlined, color: "#8c8c8c" },
];
const { t } = useDashboardI18n();
const { isIssueWorkspace } = useDashboardWorkItems();
const availableTypes = computed(() =>
  isIssueWorkspace.value && props.canAddSla
    ? [
        ...types,
        { type: "sla_metric", icon: AppstoreOutlined, color: "#fa8c16" },
      ]
    : types,
);
const chartGroups = computed(() => {
  const entries = chartEntries.filter(
    (entry) => !entry.issueOnly || isIssueWorkspace.value,
  );
  const groups = [
    {
      key: "basic",
      title: "dashboard_chart_basic",
      entries: entries.filter((entry) => !entry.sla),
    },
  ];
  if (isIssueWorkspace.value && props.canAddSla) {
    groups.push({
      key: "sla",
      title: "dashboard_chart_sla",
      entries: entries.filter((entry) => entry.sla),
    });
  }
  return groups;
});
const activeTab = computed(() =>
  chartGroups.value.some((group) => group.key === props.tab)
    ? props.tab
    : "basic",
);
</script>
<style scoped lang="less">
.dashboard-card-types__tabs {
  margin-top: 22px;

  :deep(.ant-tabs-tab) {
    margin: 0 8px 0 0;
    padding: 5px 14px;
    border-radius: 20px;
    background: #f1f3f5;
  }
  :deep(.ant-tabs-tab.ant-tabs-tab-active) {
    background: var(--ant-color-primary, #1677ff);
  }
  :deep(.ant-tabs-tab-active .ant-tabs-tab-btn) {
    color: white;
  }
  :deep(.ant-tabs-ink-bar) {
    display: none;
  }
  :deep(.ant-tabs-nav::before) {
    border-bottom: none;
  }
}
.dashboard-card-types__charts {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
.dashboard-card-types__chart {
  padding: 10px;
  border: 1px solid #ededed;
  border-radius: 10px;
  background: white;
  cursor: pointer;
  min-width: 0;
  svg {
    display: block;
    width: 100%;
    background: #fafbfc;
    margin-bottom: 10px;
  }
  span {
    font-size: 12px;
    color: #595959;
  }
  &:hover,
  &:focus-visible {
    border-color: #1677ff;
  }
}
@media (max-width: 620px) {
  .dashboard-card-types__charts {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
.dashboard-card-types h3 {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 0 0 16px;
  font-size: 15px;
  font-weight: 600;
  color: #15284b;
  &::before {
    content: "";
    width: 3px;
    height: 16px;
    border-radius: 2px;
    background: #1677ff;
  }
}
.dashboard-card-types__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}
.dashboard-card-types__item {
  display: flex;
  align-items: center;
  gap: 16px;
  min-height: 92px;
  padding: 18px 20px;
  border: 1px solid #ededed;
  border-radius: 10px;
  background: #fff;
  text-align: left;
  cursor: pointer;
  &:hover,
  &:focus-visible {
    border-color: #1677ff;
    background: #f0f7ff;
    outline: none;
  }
  &:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
}
.dashboard-card-types__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  flex-shrink: 0;
  color: #fff;
  border-radius: 10px;
  font-size: 23px;
}
.dashboard-card-types__copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  strong {
    color: #15284b;
    font-size: 16px;
    font-weight: 600;
  }
  small {
    color: #8c8c8c;
    font-size: 13px;
  }
}
@media (max-width: 620px) {
  .dashboard-card-types__grid {
    grid-template-columns: 1fr;
  }
}
</style>
