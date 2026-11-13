<template>
  <a-layout class="main-layout">
    <!-- Top header bar -->
    <a-layout-header class="main-header">
      <div class="header-left">
        <div class="logo" @click="$router.push('/workspaces')">
          <AppstoreOutlined class="logo-icon" />
          <span class="logo-text">GoTeams</span>
        </div>
      </div>
      <div class="header-center">
        <a-menu
          v-model:selectedKeys="menuKeys"
          mode="horizontal"
          :style="{ lineHeight: '48px', borderBottom: 'none' }"
          @click="handleMenuClick"
        >
          <a-menu-item key="/workspaces">
            <ProjectOutlined />
            {{ $t('menu_workspaces') }}
          </a-menu-item>
          <a-menu-item key="/my-work">
            <HomeOutlined />
            {{ $t('menu_myWork') }}
          </a-menu-item>
          <a-menu-item key="/profile">
            <UserOutlined />
            {{ $t('menu_profile') }}
          </a-menu-item>
        </a-menu>
      </div>
      <div class="header-right">
        <a-switch
          :checked="localeStore.isZhCn"
          checked-children="ZH"
          un-checked-children="EN"
          size="small"
          style="margin-right: 12px"
          @change="localeStore.toggleLang"
        />
        <a-dropdown>
          <span class="user-info">
            <a-avatar size="small" :style="{ backgroundColor: '#1677ff' }">
              {{ userInitial }}
            </a-avatar>
            <span class="user-name">{{ authStore.userInfo?.name }}</span>
          </span>
          <template #overlay>
            <a-menu>
              <a-menu-item key="profile" @click="$router.push('/profile')">
                <UserOutlined /> {{ $t('menu_profile') }}
              </a-menu-item>
              <a-menu-divider />
              <a-menu-item key="logout" @click="handleLogout">
                <LogoutOutlined /> {{ $t('auth_logout') }}
              </a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
      </div>
    </a-layout-header>

    <!-- Main content -->
    <a-layout-content class="main-content">
      <router-view />
    </a-layout-content>
  </a-layout>
</template>

<script setup>
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useLocaleStore } from '@/stores/locale'
import {
  AppstoreOutlined,
  ProjectOutlined,
  HomeOutlined,
  UserOutlined,
  LogoutOutlined,
} from '@ant-design/icons-vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const localeStore = useLocaleStore()

const menuKeys = computed(() => {
  const path = route.path
  if (path.startsWith('/workspace/')) return ['/workspaces']
  if (path === '/my-work') return ['/my-work']
  if (path === '/profile') return ['/profile']
  return ['/workspaces']
})

const userInitial = computed(() => authStore.userInfo?.name?.charAt(0) || 'U')

function handleMenuClick({ key }) {
  router.push(key)
}

function handleLogout() {
  authStore.logout()
  router.push('/login')
}
</script>

<style scoped lang="less">
.main-layout {
  min-height: 100vh;
  background: #f0f2f5;
}

.main-header {
  background: #fff;
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
  height: 48px;
  line-height: 48px;
  position: sticky;
  top: 0;
  z-index: 100;
}

.header-left {
  display: flex;
  align-items: center;
}

.logo {
  display: flex;
  align-items: center;
  cursor: pointer;
  gap: 8px;

  .logo-icon {
    font-size: 20px;
    color: #1677ff;
  }

  .logo-text {
    font-size: 18px;
    font-weight: 600;
    color: #333;
    letter-spacing: 1px;
  }
}

.header-center {
  flex: 1;
  display: flex;
  justify-content: center;

  :deep(.ant-menu-horizontal) {
    border-bottom: none;
  }
}

.header-right {
  display: flex;
  align-items: center;
}

.user-info {
  display: flex;
  align-items: center;
  cursor: pointer;
  gap: 8px;

  .user-name {
    font-size: 14px;
    color: #333;
  }
}

.main-content {
  min-height: calc(100vh - 48px);
}
</style>
