<template>
  <div class="error-page">
    <a-result
      status="403"
      title="403"
      :sub-title="$t('errorPage_noPermission')"
    >
      <template #extra>
        <a-button type="primary" @click="$router.push('/workspaces')">{{ $t('common_returnWorkspace') }}</a-button>
      </template>
    </a-result>
  </div>
</template>

<style scoped>
.error-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
}
</style>
