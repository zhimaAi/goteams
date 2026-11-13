<template>
  <div class="type-config-panel">
    <a-tabs v-model:activeKey="subTab" size="small">
      <a-tab-pane key="fields">
        <template #tab>
          <span><TableOutlined /> {{ $t('settings_attributeFields') }}</span>
        </template>
        <ListFieldsTab :type-key="typeKey" />
      </a-tab-pane>
      <a-tab-pane key="status">
        <template #tab>
          <span><CheckCircleOutlined /> {{ $t('settings_status') }}</span>
        </template>
        <StatusTab />
      </a-tab-pane>
      <a-tab-pane key="priority">
        <template #tab>
          <span><FlagOutlined /> {{ $t('settings_priority') }}</span>
        </template>
        <PriorityTab />
      </a-tab-pane>
    </a-tabs>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { TableOutlined, CheckCircleOutlined, FlagOutlined } from '@ant-design/icons-vue'
import ListFieldsTab from './ListFieldsTab.vue'
import StatusTab from './StatusTab.vue'
import PriorityTab from './PriorityTab.vue'

const props = defineProps({
  typeKey: { type: String, default: 'requirement' },
})
const subTab = ref('fields')
</script>

<style scoped lang="less">
.type-config-panel {
  padding-top: 8px;
}
</style>
