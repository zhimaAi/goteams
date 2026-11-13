<template>
  <svg
    class="kb-svg-icon"
    :width="size"
    :height="size"
    :viewBox="viewBox"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    aria-hidden="true"
  >
    <!-- 目录：文档 + 3 条横线（设计稿 SVG） -->
    <template v-if="name === 'directory'">
      <path
        d="M2.66602 1.99967C2.66602 1.63148 2.96449 1.33301 3.33268 1.33301H9.99935L13.3327 4.66634V13.9997C13.3327 14.3679 13.0342 14.6663 12.666 14.6663H3.33268C2.96449 14.6663 2.66602 14.3679 2.66602 13.9997V1.99967Z"
        stroke="currentColor"
        stroke-width="1.3"
        stroke-linejoin="round"
      />
      <path
        d="M5.33398 6.05176H10.6673"
        stroke="currentColor"
        stroke-width="1.3"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M5.33398 8.63086H10.6673"
        stroke="currentColor"
        stroke-width="1.3"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M5.33398 11.209H10.6673"
        stroke="currentColor"
        stroke-width="1.3"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </template>

    <!-- 主页：房子（设计稿 SVG，fill） -->
    <template v-else-if="name === 'home'">
      <path
        d="M4.3746 8.18781C3.745 8.81897 2.66663 8.37307 2.66663 7.48157V7.33333C2.66663 7.24444 2.68329 7.16111 2.71663 7.08333C2.74996 7.00556 2.79996 6.93333 2.86663 6.86667L7.53329 2.2C7.59996 2.13333 7.67218 2.08333 7.74996 2.05C7.82774 2.01667 7.91107 2 7.99996 2C8.08885 2 8.17218 2.01667 8.24996 2.05C8.32774 2.08333 8.39996 2.13333 8.46663 2.2L8.69784 2.42715C9.09392 2.81629 9.09714 3.45361 8.70499 3.84672L4.3746 8.18781ZM12.3333 13.3333C11.781 13.3333 11.3333 12.8856 11.3333 12.3333V8.29755C11.3333 8.03233 11.2279 7.77798 11.0404 7.59044L9.42373 5.97377C9.03321 5.58325 9.03345 4.94984 9.42398 4.55931C9.81527 4.16803 10.4502 4.16866 10.8404 4.56103L13.1333 6.86667C13.1888 6.93333 13.2362 7.00556 13.2753 7.08333C13.314 7.16111 13.3333 7.24444 13.3333 7.33333V12.6667C13.3333 12.8556 13.2693 13.0138 13.1413 13.1413C13.0137 13.2693 12.8555 13.3333 12.6666 13.3333H12.3333ZM3.33329 13.3333C3.1444 13.3333 2.98596 13.2693 2.85796 13.1413C2.7304 13.0138 2.66663 12.8556 2.66663 12.6667V11.7142C2.66663 11.449 2.77198 11.1946 2.95952 11.0071L3.64996 10.3167C4.02514 9.94149 4.66663 10.2072 4.66663 10.7378C4.66663 11.0667 4.93326 11.3333 5.26218 11.3333H9.33329C9.88558 11.3333 10.3333 11.781 10.3333 12.3333C10.3333 12.8856 9.88558 13.3333 9.33329 13.3333H3.33329Z"
        fill="currentColor"
      />
    </template>

    <!-- 收起箭头：实心右三角（设计稿 SVG） -->
    <template v-else-if="name === 'caret-collapsed'">
      <path
        d="M6.41296 12L10.413 8L6.41296 4L6.41296 12Z"
        fill="currentColor"
        stroke="currentColor"
        stroke-width="0.888889"
        stroke-linejoin="round"
      />
    </template>

    <!-- 展开箭头：实心下三角（设计稿 SVG） -->
    <template v-else-if="name === 'caret-expanded'">
      <path
        d="M4 6L8 10L12 6L4 6Z"
        fill="currentColor"
        stroke="currentColor"
        stroke-width="0.888889"
        stroke-linejoin="round"
      />
    </template>

    <!-- 折叠全部：方框 + 上箭头（设计稿 SVG） -->
    <template v-else-if="name === 'collapse-all'">
      <path
        d="M2 3.33333C2 2.59695 2.59695 2 3.33333 2H12.6667C13.403 2 14 2.59695 14 3.33333V12.6667C14 13.403 13.403 14 12.6667 14H3.33333C2.59695 14 2 13.403 2 12.6667V3.33333Z"
        stroke="currentColor"
        stroke-width="1.33333"
        stroke-linejoin="round"
      />
      <path
        d="M2 10.667H14"
        stroke="currentColor"
        stroke-width="1.33333"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M6.66663 5.33301L7.99996 6.66634L9.33329 5.33301"
        stroke="currentColor"
        stroke-width="1.33333"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M2 8.66699V12.667"
        stroke="currentColor"
        stroke-width="1.33333"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M14 8.66699V12.667"
        stroke="currentColor"
        stroke-width="1.33333"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </template>

    <!-- 展开全部：方框 + 下箭头（镜像折叠全部） -->
    <template v-else-if="name === 'expand-all'">
      <path
        d="M2 3.33333C2 2.59695 2.59695 2 3.33333 2H12.6667C13.403 2 14 2.59695 14 3.33333V12.6667C14 13.403 13.403 14 12.6667 14H3.33333C2.59695 14 2 13.403 2 12.6667V3.33333Z"
        stroke="currentColor"
        stroke-width="1.33333"
        stroke-linejoin="round"
      />
      <path
        d="M2 10.667H14"
        stroke="currentColor"
        stroke-width="1.33333"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M6.66663 10.667L7.99996 9.33367L9.33329 10.667"
        stroke="currentColor"
        stroke-width="1.33333"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M2 8.66699V12.667"
        stroke="currentColor"
        stroke-width="1.33333"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M14 8.66699V12.667"
        stroke="currentColor"
        stroke-width="1.33333"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </template>

    <!-- 排序：双向排序箭头（设计稿 SVG，16×16 viewBox） -->
    <template v-else-if="name === 'sort'">
      <path
        d="M11.6216 2.05371V13.797"
        stroke="currentColor"
        stroke-width="1.18919"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M3 9.18848H7.75676L3 13.9452H7.75676"
        stroke="currentColor"
        stroke-width="1.18919"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M3 6.81047L5.4604 2.05371L7.75676 6.81047"
        stroke="currentColor"
        stroke-width="1.18919"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M13.9999 11.5674L11.6215 13.9458L9.24316 11.5674"
        stroke="currentColor"
        stroke-width="1.18919"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </template>
  </svg>
</template>

<script setup>
import { computed } from 'vue'

// KbIcon renders the exact SVG glyphs from the Figma design.
// Colors use currentColor so callers control the hue via CSS `color`.
// Available names: directory | home | caret-collapsed | caret-expanded
//                 | collapse-all | expand-all | sort
const props = defineProps({
  name: { type: String, required: true },
  size: { type: [Number, String], default: 16 },
})

// Each glyph keeps its own viewBox from the Figma export.
const VIEWBOX = {
  directory: '0 0 16 16',
  home: '0 0 16 16',
  'caret-collapsed': '0 0 16 16',
  'caret-expanded': '0 0 16 16',
  'collapse-all': '0 0 16 16',
  'expand-all': '0 0 16 16',
  sort: '0 0 16 16',
}

const viewBox = computed(() => VIEWBOX[props.name] || '0 0 16 16')
</script>

<style scoped lang="less">
.kb-svg-icon {
  display: inline-block;
  flex-shrink: 0;
  vertical-align: -0.125em;
}
</style>