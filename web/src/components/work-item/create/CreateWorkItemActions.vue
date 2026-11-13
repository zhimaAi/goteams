<template>
  <div class="create-bottom">
    <a-space :size="12">
      <a-button
        v-if="canCreate"
        type="primary"
        :loading="creating"
        @click="$emit('create')"
      >
        {{ $t("common_create") }}
      </a-button>
      <a-button
        v-if="canCreate"
        class="continue-button"
        :loading="creating"
        @click="$emit('continue')"
      >
        {{ $t("requirement_submitContinue") }}
      </a-button>
      <a-button @click="$emit('cancel')">{{ $t("common_cancel") }}</a-button>
      <LocalDraftTips :draft="draft" @open="$emit('open-drafts')" />
    </a-space>
    <LocalDraftModal
      :open="draftModalOpen"
      :records="draftRecords"
      @close="$emit('close-drafts')"
      @restore="$emit('restore-draft', $event)"
    />
  </div>
</template>

<script setup>
import LocalDraftTips from "@/components/common/LocalDraftTips.vue";
import LocalDraftModal from "@/components/common/LocalDraftModal.vue";

defineProps({
  canCreate: { type: Boolean, default: false },
  creating: { type: Boolean, default: false },
  draft: { type: Object, required: true },
  draftModalOpen: { type: Boolean, default: false },
  draftRecords: { type: Array, default: () => [] },
});
defineEmits([
  "create",
  "continue",
  "cancel",
  "open-drafts",
  "close-drafts",
  "restore-draft",
]);
</script>

<style scoped lang="less">
.create-bottom {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 16px;
  height: 61px;
  padding: 10px 16px;
  border-top: 1px solid #e6eaf0;
}
.continue-button {
  border-color: #1677ff;
  color: #1677ff;
}
@media (max-width: 900px) {
  .create-bottom {
    position: sticky;
    bottom: 0;
    z-index: 2;
    background: var(--app-surface);
  }
}
</style>
