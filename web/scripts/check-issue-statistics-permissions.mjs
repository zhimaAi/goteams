import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import * as vue from "vue";
import issueRoutes from "../src/router/issues.js";

const boardSource = readFileSync(
  new URL("../src/composables/useWorkItemBoard.js", import.meta.url),
  "utf8",
);
const initialTabs = boardSource.slice(
  boardSource.indexOf("  const enabledApplications ="),
  boardSource.indexOf("  function requestedBoardTab"),
);

// Statistics requires its own grant as well as work-item visibility.
{
  const permissions = vue.reactive(new Set(["requirement:view"]));
  const issueMetadata = vue.ref({ workspace: { workspace_type: "issue" } });
  const tabs = new Function(
    "context",
    `const { ref, computed, authStore, issueMetadata } = context;
     const surface = "issue";
     ${initialTabs}
     return computed(availableBoardTabs);`,
  )({
    ...vue,
    issueMetadata,
    authStore: { hasPermission: (key) => permissions.has(key) },
  });
  assert.ok(!tabs.value.includes("statistics"));
  permissions.add("issue_statistics:view");
  assert.ok(tabs.value.includes("statistics"));
  permissions.delete("issue_statistics:view");
  assert.ok(!tabs.value.includes("statistics"));
  permissions.add("issue_statistics:view");
  permissions.delete("requirement:view");
  assert.ok(!tabs.value.includes("statistics"));
  permissions.add("requirement:view");
  issueMetadata.value.workspace.workspace_type = "software";
  assert.ok(!tabs.value.includes("statistics"));

  const statistics = issueRoutes[1].children[0].children.find(
    (route) => route.name === "IssueStatistics",
  );
  assert.equal(statistics.meta.permission, "issue_statistics:view");
  assert.deepEqual(statistics.meta.anyPermission, ["requirement:view"]);
}
console.log(
  "Statistics grant, revocation, workspace type and route permissions passed",
);
