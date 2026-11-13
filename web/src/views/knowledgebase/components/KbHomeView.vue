<template>
  <div class="kb-home-view">
    <div class="kb-home-tabs">
      <div class="kb-segmented" role="tablist">
        <button
          type="button"
          role="tab"
          class="kb-segmented-item"
          :class="{ active: activeTab === 'activities' }"
          :aria-selected="activeTab === 'activities'"
          @click="activeTab = 'activities'"
        >
          {{ $t('kb_homeActivities') }}
        </button>
        <button
          type="button"
          role="tab"
          class="kb-segmented-item"
          :class="{ active: activeTab === 'favorites' }"
          :aria-selected="activeTab === 'favorites'"
          @click="activeTab = 'favorites'"
        >
          {{ $t('kb_homeFavorites') }}
        </button>
        <button
          type="button"
          role="tab"
          class="kb-segmented-item"
          :class="{ active: activeTab === 'created' }"
          :aria-selected="activeTab === 'created'"
          @click="activeTab = 'created'"
        >
          {{ $t('kb_homeCreated') }}
        </button>
      </div>
    </div>

    <div class="kb-home-body">
      <!-- 动态 -->
      <template v-if="activeTab === 'activities'">
        <div v-if="!activities.length && !activitiesLoading" class="kb-home-empty">
          <a-empty :description="$t('kb_activitiesEmpty')" :image="simpleImage" />
        </div>
        <div v-else class="kb-feed-list">
          <div v-for="activity in activities" :key="activity.id" class="kb-feed-item">
            <div class="kb-feed-header">
              <a-avatar :size="28" :src="activity.avatar || undefined" class="kb-feed-avatar">
                {{ (activity.user_name || 'U').charAt(0).toUpperCase() }}
              </a-avatar>
              <span class="kb-feed-user">{{ activity.user_name }}</span>
              <span class="kb-feed-action">{{ actionText(activity.action) }}</span>
              <span class="kb-feed-time">{{ formatTime(activity.created_at) }}</span>
            </div>
            <div class="kb-doc-card" @click="openActivity(activity)">
              <component
                :is="kbDocFileIcon(activity).icon"
                v-if="activity.item_type === 'document'"
                class="kb-doc-card-icon kb-icon-doc"
                :class="kbDocFileIcon(activity).modifier"
              />
              <FolderFilled v-else-if="activity.item_type === 'folder'" class="kb-doc-card-icon kb-icon-folder" />
              <LinkOutlined v-else class="kb-doc-card-icon" />
              <span class="kb-doc-card-title">{{ activity.item_title }}</span>
            </div>
          </div>
        </div>
      </template>

      <!-- 我关注的 -->
      <template v-else-if="activeTab === 'favorites'">
        <section v-if="favoriteFolders.length" class="kb-follow-section">
          <h3 class="kb-section-title">{{ $t('kb_favoriteFolders') }}</h3>
          <div class="kb-folder-grid">
            <div
              v-for="folder in favoriteFolders"
              :key="folder.id"
              class="kb-folder-card"
              @click="openFolder(folder)"
            >
              <FolderFilled class="kb-folder-card-icon" />
              <span class="kb-folder-card-title">{{ folder.title }}</span>
              <span class="kb-folder-card-count">
                {{ $t('kb_documentsCount', { count: folder.document_count || 0 }) }}
              </span>
            </div>
          </div>
        </section>
        <section v-if="favoriteDocuments.length" class="kb-follow-section">
          <div class="kb-fav-doc-list">
            <div
              v-for="doc in favoriteDocuments"
              :key="doc.id"
              class="kb-fav-doc-item"
              @click="openDoc(doc)"
            >
              <component
                :is="kbDocFileIcon(doc).icon"
                v-if="doc.item_type === 'document'"
                class="kb-fav-doc-icon kb-icon-doc"
                :class="kbDocFileIcon(doc).modifier"
              />
              <LinkOutlined v-else class="kb-fav-doc-icon" />
              <span class="kb-fav-doc-title">{{ doc.title }}</span>
              <span class="kb-fav-doc-meta">
                {{ formatTime(doc.created_at) }} · {{ doc.creator_name }}
              </span>
              <StarFilled class="kb-fav-doc-star" @click.stop="unfavorite(doc)" />
            </div>
          </div>
        </section>
        <div
          v-if="!favoriteFolders.length && !favoriteDocuments.length && !favoritesLoading"
          class="kb-home-empty"
        >
          <a-empty :description="$t('kb_favoritesEmpty')" :image="simpleImage" />
        </div>
      </template>

      <!-- 我创建的 -->
      <template v-else>
        <div v-if="!createdItems.length && !createdLoading" class="kb-home-empty">
          <a-empty :description="$t('kb_createdEmpty')" :image="simpleImage" />
        </div>
        <div v-else class="kb-feed-list">
          <div
            v-for="doc in createdItems"
            :key="doc.id"
            class="kb-created-item"
            @click="openDoc(doc)"
          >
            <component
              :is="kbDocFileIcon(doc).icon"
              v-if="doc.item_type === 'document'"
              class="kb-created-icon kb-icon-doc"
              :class="kbDocFileIcon(doc).modifier"
            />
            <FolderFilled v-else-if="doc.item_type === 'folder'" class="kb-created-icon kb-icon-folder" />
            <LinkOutlined v-else class="kb-created-icon" />
            <span class="kb-created-title">{{ doc.title }}</span>
            <span class="kb-created-meta">
              {{ formatTime(doc.created_at) }}
            </span>
            <StarFilled
              v-if="doc.favorite"
              class="kb-created-star is-active"
              @click.stop="toggleCreatedFavorite(doc)"
            />
            <StarOutlined
              v-else
              class="kb-created-star"
              @click.stop="toggleCreatedFavorite(doc)"
            />
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Empty } from 'ant-design-vue'
import {
  FolderFilled,
  LinkOutlined,
  StarFilled,
  StarOutlined,
} from '@ant-design/icons-vue'
import {
  getKnowledgeHomeFavorites,
  listKnowledgeActivities,
  listKnowledgeCreatedByMe,
  setKnowledgeFavorite,
  unsetKnowledgeFavorite,
} from '@/api/knowledgeBase'
import { useKnowledgeStore } from '@/stores/knowledge'
import { kbDocFileIcon } from '@/utils/kbFileIcon'
import { formatFriendlyLocalDate } from '@/utils/date'

const props = defineProps({
  wsId: { type: [Number, String], default: 0 },
})

const emit = defineEmits(['open-item', 'open-folder'])

const { t: $t } = useI18n()
const store = useKnowledgeStore()
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE

const activeTab = ref('activities')
const activities = ref([])
const activitiesLoading = ref(false)
const favoriteFolders = ref([])
const favoriteDocuments = ref([])
const favoritesLoading = ref(false)
const createdItems = ref([])
const createdLoading = ref(false)
const createdPage = ref(1)
const createdTotal = ref(0)
const createdFetched = ref(false)

watch(
  () => props.wsId,
  () => {
    activities.value = []
    favoriteFolders.value = []
    favoriteDocuments.value = []
    createdItems.value = []
    createdFetched.value = false
    activeTab.value = 'activities'
    loadActivities()
  },
  { immediate: true },
)

watch(activeTab, (tab) => {
  if (tab === 'favorites') loadFavorites()
  if (tab === 'created') loadCreated()
})

async function loadActivities() {
  if (!props.wsId) return
  activitiesLoading.value = true
  try {
    const res = await listKnowledgeActivities(props.wsId, { page: 1, size: 50 })
    activities.value = res?.items || []
  } catch {
    activities.value = []
  } finally {
    activitiesLoading.value = false
  }
}

async function loadFavorites() {
  if (!props.wsId || favoritesLoading.value) return
  favoritesLoading.value = true
  try {
    const res = await getKnowledgeHomeFavorites(props.wsId)
    favoriteFolders.value = res?.folders || []
    favoriteDocuments.value = res?.documents || []
  } catch {
    favoriteFolders.value = []
    favoriteDocuments.value = []
  } finally {
    favoritesLoading.value = false
  }
}

async function loadCreated() {
  if (!props.wsId || createdFetched.value) return
  createdLoading.value = true
  try {
    const res = await listKnowledgeCreatedByMe(props.wsId, {
      page: createdPage.value,
      size: 50,
    })
    createdItems.value = res?.items || []
    createdTotal.value = res?.total || 0
    createdFetched.value = true
  } catch {
    createdItems.value = []
  } finally {
    createdLoading.value = false
  }
}

function actionText(action) {
  switch (action) {
    case 'create':
      return $t('kb_actionCreate')
    case 'update':
      return $t('kb_actionUpdate')
    case 'restore':
      return $t('kb_actionRestore')
    case 'delete':
      return $t('kb_actionDelete')
    default:
      return action
  }
}

function openActivity(activity) {
  if (activity.item_type === 'folder') {
    emit('open-folder', { id: activity.item_id, title: activity.item_title })
  } else {
    emit('open-item', { id: activity.item_id, item_type: activity.item_type })
  }
}

function openDoc(doc) {
  emit('open-item', { id: doc.id, item_type: doc.item_type })
}

function openFolder(folder) {
  emit('open-folder', { id: folder.id, title: folder.title })
}

async function unfavorite(doc) {
  try {
    await unsetKnowledgeFavorite(props.wsId, doc.id)
    favoriteDocuments.value = favoriteDocuments.value.filter(
      (item) => Number(item.id) !== Number(doc.id),
    )
    store.refresh()
  } catch {
    // error toast handled by the request interceptor
  }
}

async function toggleCreatedFavorite(doc) {
  const next = !doc.favorite
  try {
    if (next) await setKnowledgeFavorite(props.wsId, doc.id)
    else await unsetKnowledgeFavorite(props.wsId, doc.id)
    doc.favorite = next
    const treeItem = store.items.find((item) => Number(item.id) === Number(doc.id))
    if (treeItem) treeItem.favorite = next
  } catch {
    // The unified interceptor already reports the failure.
  }
}

function formatTime(timestamp) {
  return timestamp
    ? formatFriendlyLocalDate(timestamp, $t('common_today'), $t('common_yesterday'))
    : ''
}
</script>

<style scoped lang="less">
.kb-home-view {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: #fff;
}

// Segmented control style for the three home tabs.
// Container is the light gray pill; the active tab is a white card with blue bold text,
// mirroring the design (浅灰底 + 白底选中 + 加粗蓝字).
.kb-home-tabs {
  flex-shrink: 0;
  padding: 16px 8px;
  padding-top: 0;
  background: #fff;
}

.kb-segmented {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 3px;
  background: #edeff2;
  border-radius: 6px;
}

.kb-segmented-item {
  padding: 6px 16px;
  background: transparent;
  border: 0;
  border-radius: 4px;
  font-size: 14px;
  line-height: 22px;
  color: #262626;
  cursor: pointer;
  transition:
    color 0.15s ease,
    background 0.15s ease,
    box-shadow 0.15s ease;

  &:hover:not(.active) {
    color: #1677ff;
  }

  &.active {
    background: #fff;
    color: #1677ff;
    font-weight: 600;
    box-shadow: 0 1px 2px rgb(0 0 0 / 6%);
  }
}

.kb-home-body {
  flex: 1;
  overflow-y: auto;
  padding: 0 8px;
}

.kb-home-empty {
  margin: 100px auto;
}

.kb-feed-list {
  width: 100%;
}

.kb-feed-item {
  padding-bottom: 22px;
  margin-bottom: 22px;
  border-bottom: 1px solid #f0f0f0;
}

.kb-feed-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.kb-feed-avatar {
  background: #1677ff;
  color: #fff;
  font-size: 15px;
  flex-shrink: 0;
}

.kb-feed-user {
  font-size: 14px;
  font-weight: 500;
  color: #172033;
}

.kb-feed-action {
  font-size: 14px;
  color: #595959;
}

.kb-feed-time {
  font-size: 12px;
  color: #bfbfbf;
  margin-left: auto;
}

// Document card inside a feed entry: pale blue (#F5F9FF) filled per the Figma design.
// No border, no shadow. File title is rendered as a link (blue + underline).
.kb-doc-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 20px;
  margin-left: 0;
  background: #f5f9ff;
  border: 0;
  border-radius: 12px;
  cursor: pointer;
  transition: background 0.15s ease;

  &:hover {
    background: #e6f0ff;
  }
}

// File-type icon glyphs use antd Filled with `fill="currentColor"`; each
// modifier drives the visible hue via CSS color, pixel-sampled from the Figma
// design. Keep in sync with KbTreeNode.
.kb-icon-doc {
  font-size: 18px;
  color: #8c8c8c;
  flex-shrink: 0;
}

.kb-icon-doc.kb-doc-icon--word {
  color: #4a8dff;
}

.kb-icon-doc.kb-doc-icon--excel {
  color: #00c090;
}

.kb-icon-doc.kb-doc-icon--csv {
  color: #00bdad;
}

.kb-icon-doc.kb-doc-icon--ppt {
  color: #fc7541;
}

.kb-icon-doc.kb-doc-icon--pdf {
  color: #e94848;
}

.kb-icon-doc.kb-doc-icon--ofd {
  color: #356ffe;
}

.kb-icon-doc.kb-doc-icon--image {
  color: #36d2ad;
}

.kb-icon-doc.kb-doc-icon--markdown {
  color: #20b2aa;
}

.kb-icon-doc.kb-doc-icon--zip {
  color: #ff5562;
}

.kb-icon-doc.kb-doc-icon--video {
  color: #627cfe;
}

.kb-icon-doc.kb-doc-icon--txt {
  color: #f9a006;
}

.kb-icon-doc.kb-doc-icon--html {
  color: #7a8699;
}

.kb-icon-doc.kb-doc-icon--unknown {
  color: #d0d8e1;
}

.kb-doc-card-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  line-height: 1.2;
  font-weight: 400;
  color: #3157e2;
  text-decoration: underline;
  text-underline-offset: 3px;
  text-decoration-thickness: 1px;
}

.kb-follow-section {
  margin-bottom: 32px;
  width: 100%;
}

.kb-section-title {
  font-size: 16px;
  font-weight: 600;
  color: #1f1f1f;
  margin-bottom: 16px;
}

.kb-folder-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 16px;
}

.kb-folder-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
  border: 1px solid #f0f0f0;
  border-radius: 8px;
  cursor: pointer;
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease;

  &:hover {
    border-color: #1677ff;
    box-shadow: 0 2px 8px rgb(22 119 255 / 10%);
  }
}

.kb-folder-card-icon {
  font-size: 24px;
  color: #faad14;
  flex-shrink: 0;
}

.kb-folder-card-title {
  flex: 1;
  font-size: 14px;
  color: #262626;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kb-folder-card-count {
  flex-shrink: 0;
  color: #8c8c8c;
  font-size: 12px;
}

.kb-fav-doc-list {
  border-top: 1px solid #f0f0f0;
}

// Figma design: each row = [icon] [title] [meta pushed right] [star pinned right].
.kb-fav-doc-item,
.kb-created-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 8px;
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;

  &:hover {
    background: #f5f5f5;
  }
}

.kb-fav-doc-icon,
.kb-created-icon {
  font-size: 16px;
  color: #8c8c8c;
  flex-shrink: 0;

  &.kb-icon-folder {
    color: #faad14;
  }

  &.kb-doc-icon--word {
    color: #4a8dff;
  }

  &.kb-doc-icon--excel {
    color: #00c090;
  }

  &.kb-doc-icon--csv {
    color: #00bdad;
  }

  &.kb-doc-icon--ppt {
    color: #fc7541;
  }

  &.kb-doc-icon--pdf {
    color: #e94848;
  }

  &.kb-doc-icon--ofd {
    color: #356ffe;
  }

  &.kb-doc-icon--image {
    color: #36d2ad;
  }

  &.kb-doc-icon--markdown {
    color: #20b2aa;
  }

  &.kb-doc-icon--zip {
    color: #ff5562;
  }

  &.kb-doc-icon--video {
    color: #627cfe;
  }

  &.kb-doc-icon--txt {
    color: #f9a006;
  }

  &.kb-doc-icon--html {
    color: #7a8699;
  }

  &.kb-doc-icon--unknown {
    color: #d0d8e1;
  }
}

.kb-fav-doc-title,
.kb-created-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  line-height: 1.2;
  color: #262626;
}

.kb-fav-doc-meta,
.kb-created-meta {
  flex-shrink: 0;
  margin-left: auto;
  margin-right: 8px;
  font-size: 14px;
  color: #8c8c8c;
  line-height: 1.2;
}

.kb-fav-doc-star {
  color: #fadb14;
  font-size: 16px;
  flex-shrink: 0;
}

.kb-created-star {
  flex-shrink: 0;
  color: #bfbfbf;
  font-size: 16px;

  &.is-active {
    color: #fadb14;
  }
}

// File-type icon colors come from antd Filled glyphs (SVG fill); per-modifier
// color overrides were removed when we switched to the Filled set in
// utils/kbFileIcon.js.
</style>