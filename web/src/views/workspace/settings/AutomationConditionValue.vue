<script>
import { defineComponent, h } from "vue";
import { useI18n } from "vue-i18n";
import { Select, Input, InputNumber, DatePicker } from "ant-design-vue";
import dayjs from "dayjs";
import { conditionFieldOptions } from "@/utils/automationConditions";
function conditionValuePresent(value) {
  return (
    value !== undefined &&
    value !== null &&
    value !== "" &&
    (!Array.isArray(value) || value.length > 0)
  );
}
function datetimeValueValid(value) {
  return (
    typeof value === "string" &&
    /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(value) &&
    dayjs(value).isValid() &&
    dayjs(value).format("YYYY-MM-DD HH:mm:ss") === value
  );
}
function referenceMissing(options, value) {
  return (Array.isArray(value) ? value : [value])
    .filter((v) => v !== undefined && v !== null && v !== "")
    .some((v) => !options.some((o) => String(o.value) === String(v)));
}
function withMissingOption(options, value, t) {
  return [
    ...options,
    ...(Array.isArray(value) ? value : [value])
      .filter(
        (v) =>
          v !== undefined &&
          v !== null &&
          v !== "" &&
          !options.some((o) => String(o.value) === String(v)),
      )
      .map((v) => ({
        value: v,
        label: t("automation_invalidOptionValue", { value: v }),
        class: "automation-invalid-option",
      })),
  ];
}
export default defineComponent({
  props: {
    modelValue: null,
    field: Object,
    operator: String,
    metadata: Object,
    triggerType: String,
    showError: Boolean,
  },
  emits: ["update:modelValue"],
  setup(componentProps, { emit }) {
    const { t } = useI18n();
    return () => {
      const field = componentProps.field || {};
      const options = conditionFieldOptions(
        field,
        componentProps.metadata,
        componentProps.triggerType,
      );
      const usesOptions =
        options.length > 0 ||
        ["select_single", "select_multi", "user", "user_multi"].includes(
          field.type,
        ) ||
        ["status", "priority", "assignee", "created_by", "version"].includes(
          field.key,
        );
      if (usesOptions)
        return h(Select, {
          value: componentProps.modelValue,
          options: withMissingOption(options, componentProps.modelValue, t),
          status:
            referenceMissing(options, componentProps.modelValue) ||
            componentProps.showError
              ? "error"
              : undefined,
          mode:
            ["select_multi", "user_multi"].includes(field.type) ||
            ["in", "not_in"].includes(componentProps.operator)
              ? "multiple"
              : undefined,
          style: "width:100%",
          "onUpdate:value": (value) => emit("update:modelValue", value),
        });
      if (["number", "numeric"].includes(field.type))
        return h(InputNumber, {
          value: componentProps.modelValue,
          status: componentProps.showError ? "error" : undefined,
          style: "width:100%",
          "onUpdate:value": (value) => emit("update:modelValue", value),
        });
      if (field.type === "datetime") {
        const valid = datetimeValueValid(componentProps.modelValue);
        return h(DatePicker, {
          value: valid ? dayjs(componentProps.modelValue) : null,
          showTime: { format: "HH:mm:ss" },
          format: "YYYY-MM-DD HH:mm:ss",
          status:
            componentProps.showError ||
            (conditionValuePresent(componentProps.modelValue) && !valid)
              ? "error"
              : undefined,
          style: "width:100%",
          onChange: (value) =>
            emit(
              "update:modelValue",
              value ? value.format("YYYY-MM-DD HH:mm:ss") : null,
            ),
        });
      }
      if (field.type === "date")
        return h(DatePicker, {
          value: componentProps.modelValue
            ? dayjs.unix(Number(componentProps.modelValue))
            : null,
          status: componentProps.showError ? "error" : undefined,
          style: "width:100%",
          onChange: (value) =>
            emit("update:modelValue", value ? value.unix() : null),
        });
      return h(Input, {
        value: componentProps.modelValue,
        status: componentProps.showError ? "error" : undefined,
        placeholder: t("automation_value"),
        "onUpdate:value": (value) => emit("update:modelValue", value),
      });
    };
  },
});
</script>
