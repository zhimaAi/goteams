import assert from "node:assert/strict";
import { createApp, nextTick } from "vue";
import { createPinia } from "pinia";
import persistedState from "pinia-plugin-persistedstate";
import { useIssueStatisticsStore } from "../src/stores/issueStatistics.js";

const storage = new Map();
globalThis.localStorage = {
  getItem: (key) => storage.get(key) ?? null,
  setItem: (key, value) => storage.set(key, value),
};
function openSession() {
  const pinia = createPinia().use(persistedState);
  createApp({}).use(pinia);
  return useIssueStatisticsStore(pinia);
}
const preferences = openSession();
assert.equal(preferences.getThreshold(7, 10), 30);
assert.equal(preferences.setThreshold(7, 10, 15), true);
assert.equal(preferences.setThreshold(7, 11, 90), true);
assert.equal(preferences.setThreshold(8, 10, 120), true);
for (const invalid of [null, "60", 0, -1, 1.5, 525601]) {
  assert.equal(preferences.setThreshold(7, 10, invalid), false);
}
assert.equal(preferences.setThreshold(null, 10, 60), false);
await nextTick();
assert.ok(storage.has("goteams-issue-statistics"));
const restored = openSession();
assert.equal(restored.getThreshold(7, 10), 15);
assert.equal(restored.getThreshold(7, 11), 90);
assert.equal(restored.getThreshold(8, 10), 120);
assert.equal(restored.getThreshold(9, 10), 30);
restored.thresholds["7:10"] = 0;
assert.equal(restored.getThreshold(7, 10), 30);
console.log(
  "Statistics threshold default, persistence, user/workspace isolation and validation passed",
);
