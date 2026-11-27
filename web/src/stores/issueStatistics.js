import { defineStore } from "pinia";
import { ref } from "vue";

export const useIssueStatisticsStore = defineStore(
  "issue-statistics",
  () => {
    const thresholds = ref({});
    const valid = (minutes) =>
      Number.isInteger(minutes) && minutes >= 1 && minutes <= 525600;

    function getThreshold(userId, wsId) {
      const minutes = thresholds.value?.[`${userId}:${wsId}`];
      return userId && wsId && valid(minutes) ? minutes : 30;
    }

    function setThreshold(userId, wsId, minutes) {
      if (!userId || !wsId || !valid(minutes)) return false;
      thresholds.value = {
        ...thresholds.value,
        [`${userId}:${wsId}`]: minutes,
      };
      return true;
    }

    return { thresholds, getThreshold, setThreshold };
  },
  { persist: { key: "goteams-issue-statistics" } },
);
