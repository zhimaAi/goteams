<template>
  <a-dropdown :trigger="['click']" :placement="placement" overlay-class-name="kb-create-menu">
    <slot />
    <template #overlay>
      <a-menu @click="onClick">
        <a-menu-item v-if="showFolder" key="folder">
          <FolderOutlined /> {{ $t('kb_newFolder') }}
        </a-menu-item>
        <a-menu-item key="document">
          <FileTextOutlined /> {{ $t('kb_newDocument') }}
        </a-menu-item>
        <a-menu-item key="link">
          <LinkOutlined /> {{ $t('kb_newLink') }}
        </a-menu-item>
        <a-menu-divider />
        <a-menu-item key="upload">
          <UploadOutlined /> {{ $t('kb_uploadFile') }}
        </a-menu-item>
      </a-menu>
    </template>
  </a-dropdown>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
import {
  FileTextOutlined,
  FolderOutlined,
  LinkOutlined,
  UploadOutlined,
} from '@ant-design/icons-vue'

defineProps({
  showFolder: { type: Boolean, default: true },
  placement: { type: String, default: 'bottomLeft' },
})

const emit = defineEmits(['select'])
const { t: $t } = useI18n()

function onClick({ key }) {
  emit('select', key)
}
</script>

<style scoped lang="less">
:global(.kb-create-menu .ant-dropdown-menu) {
  min-width: 240px;
  padding: 6px;
  border-radius: 8px;
}

:global(.kb-create-menu .ant-dropdown-menu-item) {
  min-height: 40px;
}
</style>
