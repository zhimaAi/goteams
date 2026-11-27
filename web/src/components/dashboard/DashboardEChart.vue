<template>
  <div
    ref="container"
    class="dashboard-echart"
    role="img"
    :aria-label="label"
  />
</template>

<script setup>
import { onMounted, onBeforeUnmount, ref, watch } from "vue";
import { init, use } from "echarts/core";
import { LineChart, BarChart, PieChart } from "echarts/charts";
import {
  GridComponent,
  LegendComponent,
  TooltipComponent,
  MarkLineComponent,
  TitleComponent,
  AriaComponent,
} from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";

use([
  LineChart,
  BarChart,
  PieChart,
  GridComponent,
  LegendComponent,
  TooltipComponent,
  MarkLineComponent,
  TitleComponent,
  AriaComponent,
  CanvasRenderer,
]);
const props = defineProps({
  option: { type: Object, required: true },
  label: { type: String, required: true },
});
const container = ref(null);
let chart;
let observer;
onMounted(() => {
  chart = init(container.value);
  chart.setOption(props.option);
  observer = new ResizeObserver(() => chart.resize());
  observer.observe(container.value);
});
watch(
  () => props.option,
  (option) => chart?.setOption(option, { notMerge: true }),
  { deep: true },
);
onBeforeUnmount(() => {
  observer?.disconnect();
  chart?.dispose();
});
</script>

<style scoped lang="less">
.dashboard-echart {
  width: 100%;
  height: 280px;
  min-width: 0;
}
</style>
