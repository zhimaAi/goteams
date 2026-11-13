<template>
  <a-badge :status="badgeStatus" :text="label" />
</template>

<script setup>
import { computed } from 'vue'
import { REQUIREMENT_STATUS } from '@/constants/workItem'

const props = defineProps({
  status: { type: String, required: true },
})

const statusMap = {
  default: 'default',
  blue: 'processing',
  orange: 'warning',
  green: 'success',
  red: 'error',
}

const config = computed(() => {
  const key = Object.keys(REQUIREMENT_STATUS).find(
    (k) => REQUIREMENT_STATUS[k].labelKey === props.status || k === props.status
  )
  return REQUIREMENT_STATUS[key] || { color: 'default' }
})

const badgeStatus = computed(() => statusMap[config.value.color] || 'default')

const label = computed(() => {
  // If a Chinese status name is passed in, use it directly; otherwise try to look it up from constants.
  return props.status
})
</script>
