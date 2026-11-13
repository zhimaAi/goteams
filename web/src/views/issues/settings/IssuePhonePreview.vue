<template>
  <aside class="issue-phone-preview">
    <div class="issue-phone-preview__frame">
      <div class="issue-phone-preview__screen">
        <div class="issue-phone-preview__status-bar">
          <span class="issue-phone-preview__time">9:41</span>
          <span class="issue-phone-preview__icons">
            <WifiOutlined />
            <svg
              class="issue-phone-preview__battery"
              viewBox="0 0 26 13"
              aria-hidden="true"
            >
              <rect
                x="0.5"
                y="0.5"
                width="21"
                height="12"
                rx="3"
                fill="none"
                stroke="currentColor"
              />
              <rect
                x="2.5"
                y="2.5"
                width="15"
                height="8"
                rx="1.5"
                fill="currentColor"
              />
              <path d="M23.5 4v5a2.5 2.5 0 0 0 0-5z" fill="currentColor" />
            </svg>
          </span>
        </div>
        <div class="issue-phone-preview__tabs">
          <span class="issue-phone-preview__tab active">{{
            $t("issue_invite_create_issue")
          }}</span>
          <span class="issue-phone-preview__tab">{{
            $t("issue_invite_history")
          }}</span>
        </div>
        <div class="issue-phone-preview__body">
          <template v-if="mode === 'picker'">
            <template v-if="configEnabled">
              <img
                v-if="themeImage"
                :src="themeImage"
                class="issue-phone-preview__theme"
                alt=""
              />
              <div
                v-if="guideTitle || guideDescription"
                class="issue-phone-preview__guide"
              >
                <strong v-if="guideTitle">{{ guideTitle }}</strong>
                <p v-if="guideDescription">{{ guideDescription }}</p>
              </div>
            </template>
            <div class="issue-phone-preview__templates">
              <div
                v-for="item in visibleTemplates"
                :key="item.id"
                class="issue-phone-preview__template"
              >
                <WorkItemTypePill
                  :type="item.type_key"
                  :ws-id="props.wsId"
                  class="issue-phone-preview__type"
                />
                <span class="issue-phone-preview__name">{{ item.name }}</span>
                <span class="issue-phone-preview__create">{{
                  $t("issue_invite_create")
                }}</span>
              </div>
            </div>
          </template>
          <template v-else>
            <template v-if="configEnabled">
              <img
                v-if="themeImage"
                :src="themeImage"
                class="issue-phone-preview__theme"
                alt=""
              />
              <div
                v-if="guideTitle || guideDescription"
                class="issue-phone-preview__guide"
              >
                <strong>{{ guideTitle }}</strong>
                <p>{{ guideDescription }}</p>
              </div>
            </template>
            <div class="issue-phone-preview__fields">
              <div
                v-for="field in visibleFields"
                :key="field.label"
                class="issue-phone-preview__field"
              >
                <label>
                  <em
                    v-if="field.required"
                    class="issue-phone-preview__required"
                    >*</em
                  >
                  {{ field.label }}
                </label>
                <span class="issue-phone-preview__input">
                  {{ placeholderOf(field) }}
                </span>
              </div>
            </div>
          </template>
        </div>
      </div>
    </div>
  </aside>
</template>

<script setup>
import { computed } from "vue";
import { WifiOutlined } from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import WorkItemTypePill from "@/components/common/WorkItemTypePill.vue";

const props = defineProps({
  mode: { type: String, default: "picker" },
  wsId: { type: [String, Number], default: "" },
  templates: { type: Array, default: () => [] },
  fields: { type: Array, default: () => [] },
  guideTitle: { type: String, default: "" },
  guideDescription: { type: String, default: "" },
  themeImage: { type: String, default: "" },
  configEnabled: { type: Boolean, default: true },
});

const { t } = useI18n();

const visibleFields = computed(() =>
  props.fields.filter((field) => field.display !== false),
);

// The picker preview mirrors the public page, which only exposes enabled templates.
const visibleTemplates = computed(() =>
  props.templates.filter((item) => Number(item.enabled) === 1),
);

function placeholderOf(field) {
  if (field.widget === "textarea") return t("issue_invite_enter_placeholder");
  if (field.widget === "select" || field.widget === "date")
    return t("issue_invite_select_placeholder");
  return t("issue_invite_enter_placeholder");
}
</script>

<style scoped lang="less">
.issue-phone-preview {
  flex: 0 0 auto;
  width: 340px;
  padding: 12px;
  // 预览自己滚，不然字段一多就把这一列拉到页面外
  position: sticky;
  top: 0;
  max-height: calc(100vh - 120px);
  overflow-y: auto;

  &__frame {
    padding: 8px;
    background: #1f1f1f;
    border-radius: 32px;
  }

  &__screen {
    display: flex;
    flex-direction: column;
    overflow: hidden;
    min-height: 600px;
    border-radius: 24px;
    background: #f5f5f5;
  }

  &__status-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 14px 4px;
    color: #1f1f1f;
    font-size: 12px;
  }

  &__icons {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  &__battery {
    width: 14px;
    height: 7px;
  }

  &__tabs {
    display: flex;
    gap: 24px;
    padding: 4px 14px 0;
    background: #f5f5f5;
  }

  &__tab {
    padding: 6px 0 8px;
    color: #8b8b8b;
    font-size: 13px;
    border-bottom: 2px solid transparent;

    &.active {
      color: #0d68ff;
      border-bottom-color: #0d68ff;
      font-weight: 500;
    }
  }

  &__body {
    flex: 1;
    min-height: 0;
    padding: 8px 12px 12px;
  }

  &__theme {
    display: block;
    width: 100%;
    height: auto;
    border-radius: 8px;
  }

  &__guide {
    margin-top: 10px;
    padding: 10px 12px;
    background: #fff;
    border-radius: 8px;

    strong {
      display: block;
      color: #1f1f1f;
      font-size: 14px;
      font-weight: 600;
    }

    p {
      margin: 4px 0 0;
      color: #8b8b8b;
      font-size: 12px;
      line-height: 18px;
      word-break: break-all;
    }
  }

  &__templates {
    margin-top: 10px;
    background: #fff;
    border-radius: 8px;
  }

  &__template {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;

    & + & {
      border-top: 1px solid #f0f0f0;
    }
  }

  &__type {
    flex: 0 0 auto;
    display: inline-block;
    height: auto;
    padding: 1px 6px;
    border-radius: 3px;
    color: #fff;
    font-size: 11px;
    line-height: 17px;
  }

  &__name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    color: #262626;
    font-size: 13px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &__create {
    flex: 0 0 auto;
    color: #0d68ff;
    font-size: 13px;
  }

  &__fields {
    margin-top: 10px;
    background: #fff;
    border-radius: 8px;
    padding: 12px;
  }

  &__field {
    & + & {
      margin-top: 10px;
    }

    label {
      display: block;
      margin-bottom: 4px;
      color: #262626;
      font-size: 12px;
    }
  }

  &__required {
    margin-right: 2px;
    color: #ff4d4f;
    font-style: normal;
  }

  &__input {
    display: block;
    padding: 0 8px;
    height: 32px;
    line-height: 32px;
    border: 1px solid #d9d9d9;
    border-radius: 4px;
    background: #fff;
    color: #bfbfbf;
    font-size: 12px;
  }
}
</style>
