<template>
  <a-tag :color="tagColor">{{ label }}</a-tag>
</template>

<script setup>
import { computed } from 'vue'
import { REQUIREMENT_PRIORITY } from '@/constants/workItem'

const props = defineProps({
  priority: { type: String, required: true },
})

const colorMap = {
  red: 'red',
  orange: 'orange',
  blue: 'blue',
  default: 'default',
}

const config = computed(() => {
  const key = Object.keys(REQUIREMENT_PRIORITY).find(
    (k) => REQUIREMENT_PRIORITY[k].labelKey === props.priority || k === props.priority
  )
  return REQUIREMENT_PRIORITY[key] || { color: 'default' }
})

const tagColor = computed(() => colorMap[config.value.color] || 'default')
const label = computed(() => props.priority)
</script>
