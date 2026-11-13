<template>
  <a-modal
    :open="open"
    :title="$t('home_footer_group_title')"
    :width="520"
    centered
    @update:open="(value) => emit('update:open', value)"
  >
    <div class="community-modal-content">
      <p>{{ $t("auth_registerCommunityTip") }}</p>
      <img
        :src="groupQrcode"
        class="community-qrcode"
        :alt="$t('auth_registerCommunityQrAlt')"
      />
    </div>
    <template #footer>
      <a-button type="primary" @click="emit('update:open', false)">
        {{ $t("common_confirm") }}
      </a-button>
    </template>
  </a-modal>
</template>

<script setup>
import groupQrcode from "@/image/home/group-qrcode.png";

defineProps({
  open: { type: Boolean, default: false },
});

const emit = defineEmits(["update:open"]);
</script>

<style scoped lang="less">
.community-modal-content {
  text-align: center;

  p {
    margin: 0 0 20px;
    color: #344054;
    line-height: 22px;
    text-align: left;
  }
}

.community-qrcode {
  display: block;
  width: 220px;
  height: 220px;
  margin: 0 auto;
  object-fit: contain;
}
</style>
