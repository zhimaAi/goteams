<!-- VersionPage.vue: legacy route reusing the canonical iteration panel. -->
<template>
  <IterationPanel :key="wsId" :ws-id="wsId" />
</template>

<script setup>
import { computed } from "vue";
import { useRoute } from "vue-router";
import IterationPanel from "@/components/iteration/IterationPanel.vue";

const route = useRoute();
const wsId = computed(() => route.params.wsId);
</script>
