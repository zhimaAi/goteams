<!--
 * NumberCompareFilter.vue
 * Numeric search condition: eq / ne / between / empty / not_empty.
 -->
<template>
  <a-popover
    v-model:open="open"
    trigger="click"
    placement="bottomLeft"
    overlay-class-name="number-compare-filter-popover"
  >
    <span class="number-compare-filter__trigger">{{ triggerText }}</span>
    <template #content>
      <div class="number-compare-filter">
        <a-radio-group :value="operator" @update:value="changeOperator">
          <a-radio value="eq">
            {{ $t("search_number_eq") }}
            <a-input-number
              v-if="operator === 'eq'"
              :value="numberValue(values[0])"
              :controls="false"
              @click.stop
              @update:value="(value) => emitSingle(value)"
            />
          </a-radio>
          <a-radio value="ne">
            {{ $t("search_number_ne") }}
            <a-input-number
              v-if="operator === 'ne'"
              :value="numberValue(values[0])"
              :controls="false"
              @click.stop
              @update:value="(value) => emitSingle(value)"
            />
          </a-radio>
          <a-radio value="between">
            {{ $t("search_number_range") }}
            <template v-if="operator === 'between'">
              <a-input-number
                :value="numberValue(values[0])"
                :placeholder="$t('search_number_min')"
                :controls="false"
                @click.stop
                @update:value="(value) => emitRange(0, value)"
              />
              <span>→</span>
              <a-input-number
                :value="numberValue(values[1])"
                :placeholder="$t('search_number_max')"
                :controls="false"
                @click.stop
                @update:value="(value) => emitRange(1, value)"
              />
            </template>
          </a-radio>
          <a-radio value="empty">{{ $t("view_op_empty") }}</a-radio>
          <a-radio value="not_empty">{{ $t("view_op_not_empty") }}</a-radio>
        </a-radio-group>
        <div class="number-compare-filter__footer">
          <a-button type="primary" size="small" @click="confirm">
            {{ $t("common_confirm") }}
          </a-button>
          <a-button size="small" @click="open = false">
            {{ $t("common_cancel") }}
          </a-button>
        </div>
      </div>
    </template>
  </a-popover>
</template>

<script setup>
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";

const props = defineProps({
  operator: { type: String, default: "eq" },
  values: { type: Array, default: () => [] },
});

const emit = defineEmits({
  change: (payload) => true,
});

const { t } = useI18n();
const open = ref(false);

const triggerText = computed(() => {
  if (props.operator === "empty") return t("view_op_empty");
  if (props.operator === "not_empty") return t("view_op_not_empty");
  if (props.operator === "between") {
    return (
      [props.values[0], props.values[1]].filter(Boolean).join(" → ") ||
      t("common_all")
    );
  }
  return props.values[0] || t("common_all");
});

function numberValue(value) {
  if (value === undefined || value === null || value === "") return null;
  const number = Number(value);
  return Number.isFinite(number) ? number : null;
}

function changeOperator(operator) {
  if (operator === "empty" || operator === "not_empty") {
    emit("change", { operator, values: [] });
    return;
  }
  emit("change", { operator, values: operator === "between" ? ["", ""] : [] });
}

function emitSingle(value) {
  emit("change", {
    operator: props.operator,
    values: value === null || value === undefined ? [] : [String(value)],
  });
}

function emitRange(index, value) {
  const next = [props.values[0] || "", props.values[1] || ""];
  next[index] = value === null || value === undefined ? "" : String(value);
  emit("change", {
    operator: "between",
    values: next.some((item) => item !== "") ? next : [],
  });
}

function confirm() {
  open.value = false;
}
</script>

<style scoped lang="less">
.number-compare-filter__trigger {
  display: inline-flex;
  min-width: 0;
  flex: 1;
  align-items: center;
  overflow: hidden;
  padding: 0 8px;
  color: var(--app-text-secondary);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
}

.number-compare-filter {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 240px;
  padding: 8px;
}

.number-compare-filter :deep(.ant-radio-wrapper) {
  display: flex;
  align-items: center;
  margin-inline-end: 0;
  margin-bottom: 8px;
}

.number-compare-filter :deep(.ant-input-number) {
  margin-left: 8px;
  width: 88px;
}

.number-compare-filter__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
