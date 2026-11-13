import { computed } from "vue";
import { getAvatarSrc } from "@/utils/avatar";

export function normalizeAssigneeIds(value) {
  return [...new Set((value || []).map(Number).filter(Boolean))];
}

export function useAssigneeSelectOptions(members, preferences, keyword, draft) {
  const favoriteUserIds = computed(() => preferences.favoriteUserIds);
  const favoriteSet = computed(() => new Set(favoriteUserIds.value));
  const membersNormalized = computed(() =>
    members.value
      .map((member) => ({
        source: member,
        id: Number(member.user_id ?? member.user?.id ?? member.id),
        name: member.user_name ?? member.user?.name ?? member.name ?? "",
        username:
          member.user_username ??
          member.user?.username ??
          member.username ??
          "",
        avatar: getAvatarSrc(member.user || member),
        // Removed/disabled members stay in the map so already-picked ones still render a name
        // and avatar, but they are filtered out of every candidate list.
        removed: Boolean(member.removed),
        disabled: String(member.user_status ?? member.status) === "0",
      }))
      .filter((member) => member.id > 0),
  );
  const selectableMembers = computed(() =>
    membersNormalized.value.filter(
      (member) => !member.removed && !member.disabled,
    ),
  );
  const memberMap = computed(
    () => new Map(membersNormalized.value.map((member) => [member.id, member])),
  );
  const selectedMembers = computed(() =>
    draft.value.map((id) => memberMap.value.get(id)).filter(Boolean),
  );
  const normalizedKeyword = computed(() =>
    keyword.value.trim().toLocaleLowerCase(),
  );
  const searchResults = computed(() => {
    const query = normalizedKeyword.value;
    if (!query) return [];
    return selectableMembers.value.filter((member) =>
      member.name.toLocaleLowerCase().includes(query),
    );
  });
  const favoriteMembers = computed(() =>
    favoriteUserIds.value
      .map((id) => memberMap.value.get(id))
      .filter((member) => member && !member.removed && !member.disabled),
  );
  const recentMembers = computed(() => {
    const favorites = favoriteSet.value;
    return preferences.recentUserIds
      .filter((id) => !favorites.has(id))
      .map((id) => memberMap.value.get(id))
      .filter((member) => member && !member.removed && !member.disabled);
  });
  const groupedIds = computed(
    () =>
      new Set(
        [...favoriteMembers.value, ...recentMembers.value].map(
          (member) => member.id,
        ),
      ),
  );
  const otherMembers = computed(() =>
    selectableMembers.value.filter(
      (member) => !groupedIds.value.has(member.id),
    ),
  );
  return {
    favoriteUserIds,
    favoriteSet,
    membersNormalized,
    selectableMembers,
    selectedMembers,
    normalizedKeyword,
    searchResults,
    favoriteMembers,
    recentMembers,
    otherMembers,
  };
}
