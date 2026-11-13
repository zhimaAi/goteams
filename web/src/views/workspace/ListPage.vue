<template>
  <div class="workspace-list-page">
    <div class="page-header">
      <h2>{{ $t('workspace_title') }}</h2>
      <a-button v-if="canCreateProject" type="primary" @click="createModalOpen = true">
        <PlusOutlined /> {{ $t('workspace_create') }}
      </a-button>
    </div>

    <a-row :gutter="[16, 16]">
      <a-col v-for="ws in workspaces" :key="ws.id" :xs="24" :sm="12" :lg="8" :xl="6">
        <a-card hoverable @click="router.push(`/workspace/${ws.id}/board`)">
          <a-card-meta :title="ws.name" :description="ws.description || $t('common_noData')" />
        </a-card>
      </a-col>
    </a-row>

    <a-empty v-if="!workspaces.length" :description="$t('workspace_noWorkspace')" />

    <CreateWorkspaceModal v-model:open="createModalOpen" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { PlusOutlined } from '@ant-design/icons-vue'
import CreateWorkspaceModal from '@/components/layout/CreateWorkspaceModal.vue'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const workspaceStore = useWorkspaceStore()
const authStore = useAuthStore()
const workspaces = computed(() => workspaceStore.workspaces)
const canCreateProject = computed(() => authStore.hasEnterprisePermission('project:create'))
const createModalOpen = ref(false)

onMounted(async () => {
  try { await workspaceStore.fetchWorkspaces() }
  catch { /* empty */ }
})
</script>

<style scoped lang="less">
.workspace-list-page {
  padding: 24px;
  max-width: 1200px;
  margin: 0 auto;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
  h2 { margin: 0; }
}
</style>
