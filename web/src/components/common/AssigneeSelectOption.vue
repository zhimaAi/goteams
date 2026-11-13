<template>
  <button
    type="button"
    class="assignee-select__option"
    :class="{ 'assignee-select__option--checked': checked }"
    @click="$emit('toggle')"
  >
    <span class="assignee-select__checkbox">
      <CheckOutlined v-if="checked" />
    </span>
    <span class="assignee-select__avatar-wrap">
      <span class="assignee-select__avatar">
        <img v-if="member.avatar" :src="member.avatar" alt="" />
        <template v-else>{{
          getInitial(member.name || member.username)
        }}</template>
      </span>
    </span>
    <span class="assignee-select__identity">
      <span class="assignee-select__name">
        <template
          v-for="(part, index) in highlighted(displayName)"
          :key="index"
        >
          <mark v-if="part.matched">{{ part.text }}</mark>
          <template v-else>{{ part.text }}</template>
        </template>
      </span>
    </span>
    <span
      role="button"
      :tabindex="favoriteDisabled ? -1 : 0"
      :title="
        favoriteDisabled
          ? $t('assignee_favoriteLimit')
          : $t('assignee_toggleFavorite')
      "
      class="assignee-select__star"
      :class="{
        'assignee-select__star--active': favorite,
        'assignee-select__star--disabled': favoriteDisabled,
      }"
      @click.stop="!favoriteDisabled && $emit('favorite')"
    >
      <StarFilled v-if="favorite" />
      <StarOutlined v-else />
    </span>
  </button>
</template>

<script setup>
import { computed } from "vue";
import { CheckOutlined, StarFilled, StarOutlined } from "@ant-design/icons-vue";
import { getInitial } from "@/utils/avatar";

const props = defineProps({
  member: { type: Object, required: true },
  checked: { type: Boolean, default: false },
  favorite: { type: Boolean, default: false },
  favoriteDisabled: { type: Boolean, default: false },
  keyword: { type: String, default: "" },
});

defineEmits(["toggle", "favorite"]);

const displayName = computed(() => props.member.name || "");

function highlighted(text) {
  const value = String(text || "");
  const query = props.keyword.trim();
  if (!query) return [{ text: value, matched: false }];
  const index = value.toLocaleLowerCase().indexOf(query.toLocaleLowerCase());
  if (index < 0) return [{ text: value, matched: false }];
  return [
    { text: value.slice(0, index), matched: false },
    { text: value.slice(index, index + query.length), matched: true },
    { text: value.slice(index + query.length), matched: false },
  ].filter((part) => part.text);
}
</script>

<style scoped lang="less" src="./AssigneeSelect.less"></style>
