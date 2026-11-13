<template>
  <a-dropdown
    :open="open"
    :disabled="disabled"
    trigger="click"
    placement="bottomLeft"
    :overlay-style="{ minWidth: `${popupWidth}px` }"
    @open-change="handleOpenChange"
  >
    <div
      ref="controlRef"
      class="assignee-select"
      :class="{
        'assignee-select--small': size === 'small',
        'assignee-select--open': open,
        'assignee-select--disabled': disabled,
        'assignee-select--compact': compact,
        'assignee-select--clearable': canClearAll,
      }"
      :tabindex="disabled ? undefined : -1"
      @click.stop="openDropdown"
      @keydown.esc.stop.prevent="cancel"
    >
      <template v-if="compact">
        <span
          v-if="saving"
          class="assignee-select__saving"
          :aria-label="$t('assignee_saving')"
        >
          <a-spin size="14" />
        </span>
        <span
          class="assignee-select__compact-value"
          :class="{
            'assignee-select__compact-value--empty': !compactDisplayText,
          }"
          :title="compactDisplayText || undefined"
        >
          {{
            compactDisplayText ||
            placeholder ||
            $t("assignee_searchPlaceholder")
          }}
        </span>
        <button
          v-if="canClearAll"
          type="button"
          data-clear-all
          class="assignee-select__clear"
          :aria-label="$t('assignee_clearAll')"
          :title="$t('assignee_clearAll')"
          @click.stop="clearAll"
        >
          <CloseCircleFilled />
        </button>
        <DownOutlined class="assignee-select__arrow" />
      </template>
      <template v-else>
        <span
          v-if="saving"
          class="assignee-select__saving"
          :aria-label="$t('assignee_saving')"
        >
          <a-spin size="14" />
        </span>
        <span
          v-for="member in selectedMembers"
          :key="member.id"
          class="assignee-select__tag"
          :class="{ 'assignee-select__tag--departed': member.removed }"
        >
          <a-avatar
            :size="16"
            :src="member.avatar || undefined"
            class="assignee-select__tag-avatar"
          >
            {{ getInitial(member.name || member.username) }}
          </a-avatar>
          <span class="assignee-select__tag-name">{{
            member.name || member.username
          }}</span>
          <button
            type="button"
            class="assignee-select__remove"
            :disabled="disabled"
            :aria-label="$t('assignee_remove')"
            @click.stop="removeMember(member.id)"
          >
            <CloseOutlined />
          </button>
        </span>
        <input
          ref="searchInput"
          v-model="keyword"
          class="assignee-select__input"
          :disabled="disabled"
          :placeholder="
            selectedMembers.length
              ? ''
              : placeholder || $t('assignee_searchPlaceholder')
          "
          @click.stop="openDropdown"
        />
        <button
          v-if="canClearAll"
          type="button"
          data-clear-all
          class="assignee-select__clear"
          :aria-label="$t('assignee_clearAll')"
          :title="$t('assignee_clearAll')"
          @click.stop="clearAll"
        >
          <CloseCircleFilled />
        </button>
      </template>
    </div>

    <template #overlay>
      <div
        class="assignee-select__dropdown"
        :class="{
          'assignee-select__dropdown--compact': compact,
        }"
        @click.stop
        @keydown.esc.stop.prevent="cancel"
      >
        <label class="assignee-select__search">
          <SearchOutlined />
          <input
            ref="dropdownSearchInput"
            v-model="keyword"
            :placeholder="searchPlaceholder || $t('assignee_searchPlaceholder')"
          />
        </label>

        <div class="assignee-select__options">
          <template v-if="normalizedKeyword">
            <div v-if="searchResults.length" class="assignee-select__group">
              <div class="assignee-select__group-title">
                {{ $t("assignee_possible") }}
              </div>
              <AssigneeSelectOption
                v-for="member in searchResults"
                :key="member.id"
                :member="member"
                :checked="draft.includes(member.id)"
                :favorite="favoriteSet.has(member.id)"
                :favorite-disabled="
                  favoriteLimitReached && !favoriteSet.has(member.id)
                "
                :keyword="keyword"
                @toggle="toggleMember(member.id)"
                @favorite="toggleFavorite(member.id)"
              />
            </div>
            <div v-else class="assignee-select__empty">
              {{ $t("assignee_noMatch") }}
            </div>
          </template>

          <template v-else>
            <div v-if="favoriteMembers.length" class="assignee-select__group">
              <div class="assignee-select__group-title">
                {{ $t("assignee_favorites") }} ({{ favoriteUserIds.length }}/{{
                  FAVORITE_CONTACT_LIMIT
                }})
              </div>
              <AssigneeSelectOption
                v-for="member in favoriteMembers"
                :key="member.id"
                :member="member"
                :checked="draft.includes(member.id)"
                favorite
                @toggle="toggleMember(member.id)"
                @favorite="toggleFavorite(member.id)"
              />
            </div>
            <div v-if="recentMembers.length" class="assignee-select__group">
              <div class="assignee-select__group-title">
                {{ $t("assignee_recent") }}
              </div>
              <AssigneeSelectOption
                v-for="member in recentMembers"
                :key="member.id"
                :member="member"
                :checked="draft.includes(member.id)"
                :favorite="false"
                :favorite-disabled="favoriteLimitReached"
                @toggle="toggleMember(member.id)"
                @favorite="toggleFavorite(member.id)"
              />
            </div>
            <div v-if="otherMembers.length" class="assignee-select__group">
              <div class="assignee-select__group-title">
                {{ $t("assignee_possible") }}
              </div>
              <AssigneeSelectOption
                v-for="member in otherMembers"
                :key="member.id"
                :member="member"
                :checked="draft.includes(member.id)"
                :favorite="false"
                :favorite-disabled="favoriteLimitReached"
                @toggle="toggleMember(member.id)"
                @favorite="toggleFavorite(member.id)"
              />
            </div>
            <div
              v-if="!membersNormalized.length"
              class="assignee-select__empty"
            >
              {{ $t("assignee_noMatch") }}
            </div>
          </template>
        </div>
      </div>
    </template>
  </a-dropdown>
</template>

<script setup>
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
import { message } from "ant-design-vue";
import {
  CloseCircleFilled,
  CloseOutlined,
  DownOutlined,
  SearchOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import { getInitial } from "@/utils/avatar";
import AssigneeSelectOption from "@/components/common/AssigneeSelectOption.vue";
import {
  normalizeAssigneeIds,
  useAssigneeSelectOptions,
} from "@/composables/useAssigneeSelectOptions";
import {
  FAVORITE_CONTACT_LIMIT,
  useContactPreferencesStore,
} from "@/stores/contactPreferences";

const props = defineProps({
  value: { type: Array, default: () => [] },
  members: { type: Array, default: () => [] },
  size: { type: String, default: "small" },
  popupWidth: { type: Number, default: 280 },
  defaultOpen: { type: Boolean, default: true },
  disabled: { type: Boolean, default: false },
  placeholder: { type: String, default: "" },
  searchPlaceholder: { type: String, default: "" },
  compact: { type: Boolean, default: false },
});
const emit = defineEmits(["commit", "cancel", "save"]);
const { t: $t } = useI18n();
const preferences = useContactPreferencesStore();
const open = ref(false);
const keyword = ref("");
const saving = ref(false);
const draft = ref(normalizeIds(props.value));
const original = ref(normalizeIds(props.value));
const searchInput = ref(null);
const dropdownSearchInput = ref(null);
const controlRef = ref(null);
// How many rows the assignee tags currently wrap into. Drives the clear-all button below.
const rowCount = ref(1);
let focusFrame = 0;
// Ids pending to be recorded as "recent". We defer recording until the
// dropdown closes so the recent list order does not shift while the user is
// still choosing. The change takes effect on next open.
const pendingRecent = ref([]);
let cancelled = false;

const membersRef = computed(() => props.members);
const {
  favoriteUserIds,
  favoriteSet,
  membersNormalized,
  selectedMembers,
  normalizedKeyword,
  searchResults,
  favoriteMembers,
  recentMembers,
  otherMembers,
} = useAssigneeSelectOptions(membersRef, preferences, keyword, draft);
const favoriteLimitReached = computed(
  () => favoriteUserIds.value.length >= FAVORITE_CONTACT_LIMIT,
);
const compactDisplayText = computed(() =>
  selectedMembers.value
    .map((member) => member.name || member.username)
    .filter(Boolean)
    .join(" | "),
);
// Clear-all only earns its place once the tags wrap past two rows: below that, removing them one
// by one is quick and the button would only steal width from the tags. The compact variant renders
// a single text line instead of tags, so it always offers the button (it sits next to the arrow
// and costs no tag space).
const canClearAll = computed(() => {
  if (props.disabled || selectedMembers.value.length === 0) return false;
  return props.compact || rowCount.value > 2;
});

// measureRows counts distinct offsetTop values among the flow children, so it needs no hardcoded
// row height and follows whatever the browser laid out. The clear button itself is absolutely
// positioned and skipped via its data attribute.
// ponytail: recomputed on assignee changes only. These controls sit at a fixed px width, so a
// window resize cannot change the row count in practice; add a ResizeObserver if that stops holding.
function measureRows() {
  const el = controlRef.value;
  if (!el || props.compact) return;
  const tops = new Set();
  for (const child of el.children) {
    if (child.dataset.clearAll !== undefined) continue;
    tops.add(child.offsetTop);
  }
  rowCount.value = Math.max(1, tops.size);
}

function normalizeIds(value) {
  return normalizeAssigneeIds(value);
}

function sameIds(left, right) {
  return (
    left.length === right.length &&
    left.every((id, index) => id === right[index])
  );
}

function focusSearch() {
  cancelAnimationFrame(focusFrame);
  nextTick(() => {
    let attempts = 0;
    const focus = () => {
      if (!open.value || attempts++ >= 10) return;
      const input = dropdownSearchInput.value || searchInput.value;
      if (input) input.focus();
      focusFrame = requestAnimationFrame(focus);
    };
    focus();
  });
}

function openDropdown() {
  if (props.disabled || open.value) return;
  open.value = true;
  focusSearch();
}

function handleOpenChange(nextOpen) {
  if (nextOpen) {
    openDropdown();
    return;
  }
  // Keep the dropdown open while a save is in flight.
  if (saving.value) {
    open.value = true;
    return;
  }
  closeAndCommit();
}

async function saveDraft(nextIds) {
  const prev = [...draft.value];
  draft.value = nextIds;
  saving.value = true;
  try {
    await emit("save", [...nextIds]);
    original.value = [...nextIds];
    // Defer recording recent contacts until the dropdown closes, so the
    // recent list order stays stable during the current open session.
    pendingRecent.value = [
      ...new Set([...pendingRecent.value, ...nextIds.map(Number)]),
    ];
  } catch (err) {
    // Roll back the selection on failure and keep the dropdown open.
    draft.value = prev;
    message.error($t("assignee_saveFailed"));
  } finally {
    saving.value = false;
  }
}

function toggleMember(userId) {
  const id = Number(userId);
  const nextIds = draft.value.includes(id)
    ? draft.value.filter((item) => item !== id)
    : [...draft.value, id];
  // Clear the search keyword so the grouped list (favorites/recent/others)
  // is shown again after toggling a member, and keep focus in the search box.
  keyword.value = "";
  focusSearch();
  // Save immediately on every change; dropdown stays open during saving.
  saveDraft(nextIds);
}

function removeMember(userId) {
  openDropdown();
  toggleMember(userId);
}

// Drop every assignee at once. Keeps the dropdown focused when it is already open so the user can
// immediately pick a replacement.
function clearAll() {
  if (!canClearAll.value) return;
  keyword.value = "";
  if (open.value) focusSearch();
  saveDraft([]);
}

async function toggleFavorite(userId) {
  const id = Number(userId);
  const nextFavorite = !favoriteSet.value.has(id);
  if (nextFavorite && favoriteLimitReached.value) {
    message.warning($t("assignee_favoriteLimit"));
    return;
  }
  await preferences.setFavorite(id, nextFavorite);
  focusSearch();
}

async function flushRecent() {
  if (!pendingRecent.value.length) return;
  const ids = pendingRecent.value;
  pendingRecent.value = [];
  await preferences.recordRecent(ids);
}

function closeAndCommit() {
  if (!open.value) return;
  open.value = false;
  keyword.value = "";
  if (cancelled) {
    cancelled = false;
    pendingRecent.value = [];
    return;
  }
  flushRecent();
  if (!sameIds(draft.value, original.value)) emit("commit", [...draft.value]);
  else emit("cancel");
}

function cancel() {
  // Ignore cancel while a save is in flight to avoid losing the change.
  if (saving.value) return;
  cancelled = true;
  draft.value = [...original.value];
  open.value = false;
  keyword.value = "";
  flushRecent();
  emit("cancel");
  nextTick(() => {
    cancelled = false;
  });
}

watch(
  () => props.value,
  (next) => {
    // Sync external updates into local state only when idle, so a
    // successful save reflected back from the parent does not reset drafting.
    if (saving.value) return;
    const ids = normalizeIds(next);
    draft.value = [...ids];
    original.value = [...ids];
  },
);

// Tag count drives the row count, which in turn drives the clear-all button.
watch(selectedMembers, () => nextTick(measureRows));

onMounted(() => {
  if (props.defaultOpen && !props.disabled) {
    open.value = true;
    focusSearch();
  }
  nextTick(measureRows);
  preferences.load().catch(() => {});
});

onBeforeUnmount(() => cancelAnimationFrame(focusFrame));
</script>

<style scoped lang="less" src="./AssigneeSelect.less"></style>
