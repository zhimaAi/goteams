<!-- WorkItemRowActionMenu.vue：需求与缺陷列表共用的行操作菜单。 -->
<template>
  <a-dropdown v-if="hasActions" :trigger="['click']" placement="bottomLeft">
    <slot />
    <template #overlay>
      <a-menu @click="handleMenuClick">
        <a-menu-item v-if="canCreateChild" key="child">
          <PlusOutlined /> {{ $t("requirement_createChild") }}
        </a-menu-item>
        <a-menu-item v-if="canDuplicate" key="duplicate">
          <CopyOutlined /> {{ $t("common_copy") }}
        </a-menu-item>
        <a-menu-divider v-if="canDelete && (canCreateChild || canDuplicate)" />
        <a-menu-item v-if="canDelete" key="delete" danger>
          <DeleteOutlined /> {{ $t("common_delete") }}
        </a-menu-item>
        <a-menu-divider
          v-if="canCopy && (canCreateChild || canDuplicate || canDelete)"
        />
        <template v-if="canCopy">
          <a-menu-item key="copyTitleLink">
            <ShareAltOutlined /> {{ $t("requirement_copyTitleLink") }}
          </a-menu-item>
          <a-menu-item key="copyTitle">
            <FontSizeOutlined /> {{ $t("requirement_copyTitle") }}
          </a-menu-item>
          <a-menu-item key="copyLink">
            <LinkOutlined /> {{ $t("requirement_copyLink") }}
          </a-menu-item>
          <a-menu-item key="copyDescription">
            <FileTextOutlined /> {{ $t("requirement_copyDescription") }}
          </a-menu-item>
          <a-menu-item key="copyTitleLinkDescription">
            <SnippetsOutlined />
            {{ $t("requirement_copyTitleLinkDescription") }}
          </a-menu-item>
        </template>
      </a-menu>
    </template>
  </a-dropdown>
</template>

<script setup>
import { computed } from "vue";
import {
  CopyOutlined,
  DeleteOutlined,
  FileTextOutlined,
  FontSizeOutlined,
  LinkOutlined,
  PlusOutlined,
  ShareAltOutlined,
  SnippetsOutlined,
} from "@ant-design/icons-vue";

const props = defineProps({
  item: { type: Object, required: true },
  canCreateChild: { type: Boolean, default: false },
  canDuplicate: { type: Boolean, default: false },
  canDelete: { type: Boolean, default: false },
  canCopy: { type: Boolean, default: false },
});
const emit = defineEmits({
  action: (key, item) => Boolean(key && item?.id),
});

const hasActions = computed(
  () =>
    props.canCreateChild ||
    props.canDuplicate ||
    props.canDelete ||
    props.canCopy,
);

function handleMenuClick({ key, domEvent }) {
  domEvent?.stopPropagation?.();
  emit("action", key, props.item);
}
</script>
