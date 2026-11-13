<template>
  <div class="access-denied-page">
    <a-spin v-if="loading" />
    <div v-else class="access-notice">
      <div class="access-illustration" aria-hidden="true">
        <UserOutlined class="access-user" />
        <LockOutlined class="access-lock" />
      </div>
      <div class="access-content">
        <h1>{{ $t("workspace_accessNoticeTitle") }}</h1>
        <p>{{ $t("workspace_accessDenied") }}</p>
        <p>
          {{ $t("workspace_contactAdminPrefix") }}
          <a
            v-if="requestStatus !== 'pending'"
            href="#"
            @click.prevent="open = true"
            >{{ $t("workspace_roleAdmin") }}</a
          >
          <span v-else>{{ $t("workspace_roleAdmin") }}</span>
        </p>
        <a-button
          v-if="requestStatus !== 'pending'"
          type="primary"
          @click="open = true"
        >
          {{ $t("workspace_requestAccess") }}
        </a-button>
        <span v-else class="access-pending">{{
          $t("workspace_accessPending")
        }}</span>
      </div>
    </div>
    <a-modal
      v-model:open="open"
      :title="$t('workspace_accessReason')"
      :ok-text="$t('common_ok')"
      :cancel-text="$t('common_cancel')"
      :ok-button-props="{
        disabled: !reason.trim() || [...reason.trim()].length > 30,
        loading: submitting,
      }"
      @ok="submit"
    >
      <label for="access-reason" class="reason-label">
        {{ $t("workspace_accessReason") }} <span aria-hidden="true">*</span>
      </label>
      <a-textarea
        id="access-reason"
        :value="reason"
        :placeholder="$t('workspace_accessReasonPlaceholder')"
        :rows="3"
        @input="onReasonInput"
      />
      <div class="reason-count">{{ [...reason].length }}/30</div>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { message } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import { LockOutlined, UserOutlined } from "@ant-design/icons-vue";
import { getWorkspaceAccess, requestWorkspaceAccess } from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const authStore = useAuthStore();
const requestStatus = ref("");
const loading = ref(true);
const open = ref(false);
const reason = ref("");
const submitting = ref(false);

watch(
  () => route.params.wsId,
  async (wsId) => {
    if (!wsId) return;
    loading.value = true;
    try {
      const access = await getWorkspaceAccess(wsId);
      if (access.is_member) {
        await authStore.refreshProjectPermissions(wsId);
        await router.replace(
          typeof route.query.from === "string" &&
            route.query.from.startsWith(`/workspace/${wsId}/`)
            ? route.query.from
            : `/workspace/${wsId}/board`,
        );
        return;
      }
      requestStatus.value = access.request_status;
    } catch (error) {
      router.replace(error.response?.status === 404 ? "/404" : "/403");
    } finally {
      loading.value = false;
    }
  },
  { immediate: true },
);

async function submit() {
  const value = reason.value.trim();
  if (!value || [...value].length > 30 || submitting.value) return;
  submitting.value = true;
  try {
    await requestWorkspaceAccess(route.params.wsId, value);
    requestStatus.value = "pending";
    open.value = false;
    reason.value = "";
    message.success(t("workspace_accessSubmitted"));
  } catch (error) {
    if (error.response?.status === 409) {
      try {
        const access = await getWorkspaceAccess(route.params.wsId);
        requestStatus.value = access.request_status;
        if (access.request_status === "pending") open.value = false;
        if (access.is_member)
          router.replace(`/workspace/${route.params.wsId}/board`);
      } catch {
        // Keep the dialog open so a failed status refresh does not lose the reason.
      }
    }
  } finally {
    submitting.value = false;
  }
}

function onReasonInput(event) {
  const limited = [...event.target.value].slice(0, 30).join("");
  event.target.value = limited;
  reason.value = limited;
}
</script>

<style scoped lang="less">
.access-denied-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
  padding: 32px 20px;
}
.access-notice {
  display: flex;
  align-items: center;
  gap: 48px;
  max-width: 760px;
}
.access-illustration {
  position: relative;
  display: grid;
  place-items: center;
  flex: 0 0 180px;
  width: 180px;
  height: 180px;
  border-radius: 50%;
  background: #eef4ff;
  color: #91a7cc;
}
.access-user {
  font-size: 88px;
}
.access-lock {
  position: absolute;
  right: 22px;
  bottom: 18px;
  display: grid;
  place-items: center;
  width: 58px;
  height: 58px;
  border: 4px solid #fff;
  border-radius: 50%;
  background: #4c85d6;
  color: #fff;
  font-size: 25px;
}
.access-content h1 {
  margin: 0 0 16px;
  color: var(--app-text-primary, #1f2937);
  font-size: 22px;
  font-weight: 600;
}
.access-content p {
  margin: 0 0 8px;
  color: var(--app-text-secondary, #596579);
  line-height: 1.6;
}
.access-content a {
  color: #1677ff;
}
.access-content .ant-btn,
.access-pending {
  margin-top: 16px;
}
.access-pending {
  display: inline-block;
  color: var(--app-text-secondary, #596579);
}
.reason-label {
  display: block;
  margin-bottom: 8px;
}
.reason-label span {
  color: #ff4d4f;
}
.reason-count {
  color: var(--app-text-secondary);
  text-align: right;
  font-size: 12px;
}
@media (max-width: 560px) {
  .access-notice {
    flex-direction: column;
    align-items: flex-start;
    gap: 24px;
  }
  .access-illustration {
    align-self: center;
    flex-basis: 140px;
    width: 140px;
    height: 140px;
  }
  .access-user {
    font-size: 72px;
  }
  .access-lock {
    right: 12px;
    bottom: 10px;
    width: 48px;
    height: 48px;
    font-size: 20px;
  }
}
</style>
