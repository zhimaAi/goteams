<template>
  <WorkItemList :model="model" />
</template>
<script setup>
import { inject } from "vue";
import WorkItemList from "@/components/work-item/WorkItemList.vue";
const model = inject("issueBoard");
</script>
