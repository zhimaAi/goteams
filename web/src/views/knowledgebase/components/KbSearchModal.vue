<template>
  <a-modal
    v-model:open="visible"
    :title="$t('common_search')"
    :width="650"
    :footer="null"
    @after-close="reset"
  >
    <div class="kb-search-modal">
      <a-input
        v-model:value="keyword"
        :placeholder="$t('kb_searchPlaceholder')"
        allow-clear
        @input="onInput"
        @press-enter="openFirst"
      >
        <template #prefix><SearchOutlined /></template>
      </a-input>

      <div class="kb-search-results">
        <a-empty
          v-if="!keyword"
          :description="$t('kb_searchEmptyHint')"
          :image="simpleImage"
        />
        <a-empty
          v-else-if="!loading && !results.length"
          :description="$t('kb_searchNoResult')"
          :image="simpleImage"
        />
        <div
          v-for="item in results"
          :key="item.id"
          class="kb-search-item"
          :class="{ active: activeIndex === results.indexOf(item) }"
          @click="openItem(item)"
          @mouseenter="activeIndex = results.indexOf(item)"
        >
          <FolderOpenFilled v-if="item.item_type === 'folder'" class="kb-search-icon kb-icon-folder" />
          <FileTextOutlined v-else-if="item.item_type === 'document'" class="kb-search-icon kb-icon-doc" />
          <LinkOutlined v-else class="kb-search-icon" />
          <span class="kb-search-name">{{ item.title }}</span>
          <span class="kb-search-meta">
            {{ item.creator_name }}
            <template v-if="item.folder_title"> · {{ item.folder_title }}</template>
          </span>
        </div>
      </div>
    </div>
  </a-modal>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Empty } from 'ant-design-vue'
import {
  FileTextOutlined,
  FolderOpenFilled,
  LinkOutlined,
  SearchOutlined,
} from '@ant-design/icons-vue'
import { searchKnowledgeItems } from '@/api/knowledgeBase'

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [Number, String], default: 0 },
})

const emit = defineEmits(['update:open', 'open-item'])

const { t: $t } = useI18n()
const keyword = ref('')
const results = ref([])
const loading = ref(false)
const activeIndex = ref(-1)
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE

const visible = computed({
  get: () => props.open,
  set: (value) => emit('update:open', value),
})

let debounceTimer = null
let searchSequence = 0

function onInput() {
  clearTimeout(debounceTimer)
  debounceTimer = setTimeout(search, 300)
}

async function search() {
  const kw = keyword.value.trim()
  if (!kw) {
    results.value = []
    return
  }
  const sequence = ++searchSequence
  loading.value = true
  try {
    const nextResults = (await searchKnowledgeItems(props.wsId, kw)) || []
    if (sequence !== searchSequence || kw !== keyword.value.trim()) return
    results.value = nextResults
    activeIndex.value = results.value.length ? 0 : -1
  } catch {
    if (sequence === searchSequence) results.value = []
  } finally {
    if (sequence === searchSequence) loading.value = false
  }
}

function openItem(item) {
  if (item.item_type === 'folder') {
    emit('open-item', { type: 'folder', item })
  } else {
    emit('open-item', { type: 'doc', item })
  }
  visible.value = false
}

function openFirst() {
  if (results.value.length) openItem(results.value[activeIndex.value >= 0 ? activeIndex.value : 0])
}

function reset() {
  searchSequence += 1
  keyword.value = ''
  results.value = []
  activeIndex.value = -1
  clearTimeout(debounceTimer)
  loading.value = false
}

onBeforeUnmount(() => {
  searchSequence += 1
  clearTimeout(debounceTimer)
})

watch(
  () => props.open,
  (open) => {
    if (open) {
      setTimeout(() => {
        const input = document.querySelector('.kb-search-modal .ant-input')
        input?.focus()
      }, 50)
    }
  },
)
</script>

<style scoped lang="less">
.kb-search-modal {
  padding: 4px;
}

.kb-search-results {
  margin-top: 8px;
  min-height: 120px;
  max-height: 320px;
  overflow-y: auto;
}

.kb-search-item {
  display: flex;
  align-items: center;
  height: 48px;
  padding: 0 12px;
  border-radius: 6px;
  cursor: pointer;

  &:hover,
  &.active {
    background: #f5f5f5;
  }
}

.kb-search-icon {
  color: #8c8c8c;
  margin-right: 10px;
  font-size: 16px;
  flex-shrink: 0;

  &.kb-icon-folder {
    color: #faad14;
  }

  &.kb-icon-doc {
    color: #4096ff;
  }
}

.kb-search-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #262626;
}

.kb-search-meta {
  flex-shrink: 0;
  font-size: 12px;
  color: #8c8c8c;
}
</style>
