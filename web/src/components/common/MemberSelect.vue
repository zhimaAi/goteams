<template>
  <a-select
    :value="modelValue"
    :placeholder="placeholder || $t('workspace_selectMembers')"
    :allow-clear="allowClear"
    :loading="loading"
    show-search
    :filter-option="filterOption"
    style="width: 100%"
    @update:value="$emit('update:modelValue', $event)"
    @change="$emit('change', $event)"
  >
    <a-select-option v-for="m in members" :key="m.user?.id || m.id" :value="m.user?.id || m.id">
      <a-space>
        <a-avatar size="small" :style="{ backgroundColor: '#1677ff' }">
          {{ (m.user?.name || m.name || '?').charAt(0) }}
        </a-avatar>
        <span>{{ m.user?.name || m.name }}</span>
        <span style="color: #999; font-size: 12px">
          ({{ (m.role === 'admin' || m.role_id === '1') ? $t('workspace_roleAdmin') : $t('workspace_roleMember') }})
        </span>
      </a-space>
    </a-select-option>
  </a-select>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { listMembers } from '@/api/workspace'

const { t: $t } = useI18n()

const props = defineProps({
  modelValue: { type: [String, Number], default: null },
  wsId: { type: String, required: true },
  placeholder: { type: String, default: '' },
  allowClear: { type: Boolean, default: true },
})

defineEmits(['update:modelValue', 'change'])

const members = ref([])
const loading = ref(false)

function filterOption(input, option) {
  return (option.children?.default?.()?.[1]?.children || '').toLowerCase().includes(input.toLowerCase())
}

onMounted(async () => {
  loading.value = true
  try {
    members.value = (await listMembers(props.wsId)) || []
  } catch {
    members.value = []
  } finally {
    loading.value = false
  }
})
</script>
