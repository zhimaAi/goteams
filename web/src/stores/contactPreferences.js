import { defineStore } from "pinia";
import { ref } from "vue";
import {
  getContactPreferences,
  recordRecentContacts,
  updateFavoriteContact,
} from "@/api/auth";

export const FAVORITE_CONTACT_LIMIT = 20;
export const RECENT_CONTACT_LIMIT = 20;

export const useContactPreferencesStore = defineStore(
  "contact-preferences",
  () => {
    const favoriteUserIds = ref([]);
    const recentUserIds = ref([]);
    const loaded = ref(false);
    let loadingPromise = null;

    function load(force = false) {
      if (loaded.value && !force) return Promise.resolve();
      if (loadingPromise) return loadingPromise;
      loadingPromise = getContactPreferences()
        .then((data) => {
          favoriteUserIds.value = (data?.favorite_user_ids || []).map(Number);
          recentUserIds.value = (data?.recent_user_ids || []).map(Number);
          loaded.value = true;
        })
        .finally(() => {
          loadingPromise = null;
        });
      return loadingPromise;
    }

    async function setFavorite(userId, isFavorite) {
      const id = Number(userId);
      await updateFavoriteContact(id, isFavorite);
      favoriteUserIds.value = isFavorite
        ? [...new Set([...favoriteUserIds.value, id])]
        : favoriteUserIds.value.filter((item) => item !== id);
    }

    async function recordRecent(userIds) {
      const ids = [...new Set((userIds || []).map(Number).filter(Boolean))];
      if (!ids.length) return false;
      try {
        await recordRecentContacts(ids);
      } catch {
        return false;
      }
      const selected = new Set(ids);
      recentUserIds.value = [
        ...ids.slice().reverse(),
        ...recentUserIds.value.filter((id) => !selected.has(id)),
      ].slice(0, RECENT_CONTACT_LIMIT);
      return true;
    }

    function clear() {
      favoriteUserIds.value = [];
      recentUserIds.value = [];
      loaded.value = false;
      loadingPromise = null;
    }

    return {
      favoriteUserIds,
      recentUserIds,
      loaded,
      load,
      setFavorite,
      recordRecent,
      clear,
    };
  },
);
