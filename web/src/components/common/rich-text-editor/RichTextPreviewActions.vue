<template>
  <div
    class="compact-action-panel"
    :class="{ collapsed: actionsCollapsed, 'read-only': readOnly }"
  >
    <a-button
      type="text"
      size="small"
      class="compact-icon-button compact-expand-button"
      :aria-label="$t('requirement_expand_description_actions')"
      :tabindex="actionsCollapsed ? 0 : -1"
      @click="actionsCollapsed = false"
    >
      <EllipsisOutlined />
    </a-button>
    <div class="compact-expanded-actions" :aria-hidden="actionsCollapsed">
      <a-button
        v-if="!readOnly"
        type="text"
        size="small"
        class="compact-edit-button"
        :tabindex="actionsCollapsed ? -1 : 0"
        @click="$emit('edit')"
      >
        <EditOutlined /> {{ $t("common_edit") }}
      </a-button>
      <a-button
        type="text"
        size="small"
        class="compact-icon-button"
        :aria-label="$t('requirement_maximize_description')"
        :tabindex="actionsCollapsed ? -1 : 0"
        @click="$emit('preview')"
      >
        <ExpandOutlined />
      </a-button>
      <a-button
        type="text"
        size="small"
        class="compact-icon-button"
        :aria-label="$t('requirement_collapse_description_actions')"
        :tabindex="actionsCollapsed ? -1 : 0"
        @click="actionsCollapsed = true"
      >
        <CaretLeftOutlined />
      </a-button>
    </div>
  </div>
</template>

<script setup>
import { ref } from "vue";
import {
  CaretLeftOutlined,
  EditOutlined,
  EllipsisOutlined,
  ExpandOutlined,
} from "@ant-design/icons-vue";

defineProps({ readOnly: { type: Boolean, default: false } });
defineEmits(["edit", "preview"]);
const actionsCollapsed = ref(false);
</script>

<style scoped lang="less">
.compact-action-panel {
  position: relative;
  display: flex;
  align-items: center;
  box-sizing: border-box;
  width: 130px;
  height: 32px;
  overflow: hidden;
  border: 1px solid #e6eaf1;
  border-radius: 9px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 5px 16px rgba(23, 35, 60, 0.07);
  transition: width 0.16s ease;
}
.compact-action-panel.collapsed {
  width: 32px;
}
.compact-action-panel.read-only:not(.collapsed) {
  width: 58px;
}
.compact-edit-button {
  width: 70px;
  height: 24px;
  padding: 0 10px;
  color: #17233c;
}
.compact-expanded-actions {
  position: absolute;
  top: 3px;
  left: 3px;
  display: flex;
  align-items: center;
  gap: 2px;
  opacity: 1;
  visibility: visible;
  transition:
    opacity 0.08s ease 0.08s,
    visibility 0s linear 0.08s;
}
.compact-action-panel.collapsed .compact-expanded-actions {
  opacity: 0;
  visibility: hidden;
  pointer-events: none;
  transition-delay: 0s;
}
.compact-icon-button {
  width: 24px;
  min-width: 24px;
  height: 24px;
  padding: 0;
  color: #8c8c8c;
}
.compact-icon-button:hover {
  background: #f5f7fa;
  color: #595959;
}
.compact-expand-button {
  position: absolute;
  top: 3px;
  right: 3px;
  opacity: 0;
  visibility: hidden;
  pointer-events: none;
  transition:
    opacity 0.08s ease,
    visibility 0s linear 0.08s;
}
.compact-action-panel.collapsed .compact-expand-button {
  opacity: 1;
  visibility: visible;
  pointer-events: auto;
  transition-delay: 0.08s;
}
</style>
