import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import vm from "node:vm";
import { computed, ref } from "vue";
import { workItemRoute } from "../src/utils/workItemRoutes.js";

const board = await readFile(
  new URL("../src/composables/useWorkItemBoard.js", import.meta.url),
  "utf8",
);
const detail = await readFile(
  new URL("../src/composables/useWorkItemDetail.js", import.meta.url),
  "utf8",
);
function functionSource(source, name) {
  const start = source.search(new RegExp(`  (?:async )?function ${name}\\(`));
  assert.ok(start >= 0, `${name} must exist`);
  return source.slice(start, source.indexOf("\n  }", start) + 4);
}

const parent = { id: "10", parent_id: "0", status: "completed" };
const child = { id: "11", parent_id: "10", status: "planned" };
const nextChild = { id: "12", parent_id: "10", status: "planned" };
const requests = [];
const context = {
  computed,
  activeTab: ref("requirement"),
  requirements: ref([parent]),
  childrenMap: ref({}),
  expandedRowKeys: ref([]),
  childLoadingKeys: ref(new Set()),
  childLoadPromises: new Map(),
  childLoadTokens: new Map(),
  listContextSequence: 1,
  childLoadSequence: 0,
  currentViewId: ref("completed-view"),
  currentListSort: ref({ field: "id", direction: "asc" }),
  categoryFilterParam: () => "7",
  wsId: ref("21"),
  listRequirements: async (wsId, params) => {
    requests.push({ wsId, ...params });
    // The saved view matches the parent, but excludes its planned children.
    const list = params.view_id ? [] : [params.page === 1 ? child : nextChild];
    return { list, total: params.view_id ? 0 : 2, page: params.page };
  },
};
vm.createContext(context);
vm.runInContext(
  functionSource(board, "loadChildren") +
    board.slice(
      board.indexOf("  const treeList = computed("),
      board.indexOf("  const boardNavigationItems = computed("),
    ) +
    "\nglobalThis.rows = treeList;",
  context,
);

assert.equal(await context.loadChildren(parent.id), true);
assert.equal(context.childrenMap.value[parent.id].list[0].id, child.id);
assert.equal(requests[0].wsId, "21");
assert.equal(requests[0].parent_id, parent.id);
assert.equal(requests[0].type, "requirement");
assert.equal(requests[0].view_id, undefined);
assert.equal(requests[0].category_id, "7");
assert.equal(requests[0].sort_field, "id");
assert.deepEqual(
  Array.from(context.rows.value, (row) => row.id),
  [parent.id],
);

context.expandedRowKeys.value = [parent.id];
assert.deepEqual(
  Array.from(context.rows.value, (row) => row.id),
  [parent.id, child.id, "load-more-10-2"],
);
await context.loadChildren(parent.id, 2);
assert.deepEqual(
  Array.from(context.rows.value, (row) => row.id),
  [parent.id, child.id, nextChild.id],
);
context.requirements.value = [parent, child];
assert.equal(context.rows.value.filter((row) => row.id === child.id).length, 1);
context.expandedRowKeys.value = [];
assert.deepEqual(
  Array.from(context.rows.value, (row) => row.id),
  [parent.id, child.id],
);
context.requirements.value = [child];
assert.equal(context.rows.value[0].id, child.id);

// Ignore an expansion response after the list context has changed.
let resolve;
context.listRequirements = () => new Promise((done) => (resolve = done));
const pending = context.loadChildren("99");
context.listContextSequence += 1;
resolve({ list: [child], total: 1 });
assert.equal(await pending, false);
assert.equal(context.childrenMap.value["99"], undefined);

for (const surface of ["issue", "workspace"]) {
  const opened = [];
  const navigation = {
    surface,
    props: { wsId: "21", requirementId: parent.id },
    workItemRoute,
    router: {
      resolve: (route) => {
        assert.equal(
          route.name,
          surface === "issue" ? "IssueDetail" : "WorkItemDetail",
        );
        assert.equal(route.params.id, child.id);
        assert.equal(route.params.wsId, "21");
        return { href: `/details/${route.params.id}` };
      },
    },
    window: { open: (...args) => opened.push(args) },
  };
  vm.runInNewContext(
    functionSource(detail, "openRelatedInNewWindow"),
    navigation,
  );
  navigation.openRelatedInNewWindow(child.id);
  assert.deepEqual(opened, [["/details/11", "_blank", "noopener,noreferrer"]]);
  assert.equal(navigation.props.requirementId, parent.id);
}

console.log(
  "child requirements: expansion, pagination, stale responses and detail routes passed",
);
