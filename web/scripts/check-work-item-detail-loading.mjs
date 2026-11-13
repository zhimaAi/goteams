import {
  buildIssueProcessingRecords,
  processingActionKey,
} from "../src/utils/issueProcessingRecords.js";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import vm from "node:vm";

const source = await readFile(
  new URL("../src/composables/useWorkItemDetail.js", import.meta.url),
  "utf8",
);
function functionSource(text, name) {
  const start = text.search(new RegExp(`  (?:async )?function ${name}\\(`));
  assert.ok(start >= 0, `${name} must exist`);
  return text.slice(start, text.indexOf("\n  }", start) + 4);
}
function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

// Execute the production loader and watcher with controllable I/O. No timers or
// source-pattern assertions: assertions below observe requests and UI state.
function harness() {
  const calls = [],
    events = [],
    waits = [];
  let cleanup;
  const props = { open: true, wsId: 19, requirementId: 188 };
  async function api(name, wsId, id) {
    calls.push({ name, wsId, id });
    assert.ok(wsId && (id === undefined || id), "request needs a target");
    const index = waits.findIndex(
      (wait) => wait.name === name && wait.matches(wsId, id),
    );
    if (index >= 0) {
      const wait = waits.splice(index, 1)[0];
      wait.entered.resolve();
      await wait.result.promise;
    }
    if (name === "metadata") return { permissions: [wsId] };
    if (name === "detail")
      return {
        id,
        workspace_id: wsId,
        type: "requirement",
        title: `item ${id}`,
      };
    if (name === "children") return { list: [{ id, type: "requirement" }] };
    return [{ id, wsId }];
  }
  const context = {
    props,
    watch: (_, callback) => {
      context.run = callback;
    },
    emit: (name, payload) => events.push({ name, payload }),
    authStore: { consumeProjectMetadata: () => null },
    getWorkspaceMetadata: (ws) => api("metadata", ws),
    getWorkItemDisplaySettings: (ws) => api("display", ws),
    getRequirement: (ws, id) => api("detail", ws, id),
    recordRequirementVisit: (ws, id) => api("visit", ws, id),
    listRequirements: (ws, params) => api("children", ws, params.parent_id),
    listRequirementAttachments: (ws, id) => api("attachments", ws, id),
    listRequirementHistories: (ws, id) => api("history", ws, id),
    updateRequirement: (ws, id) => api("update", ws, id),
    buildPlannedDateUpdates: (field, value) => ({ [field]: value }),
    consumeWorkLogEditorRequest: async () => {},
    fieldEditable: () => true,
    parseCustomFieldValue: () => "",
    draft: {
      setTarget: async () => {},
      list: async () => [],
    },
  };
  for (const [, name] of source.matchAll(/\b(\w+)\.value/g)) {
    context[name] ||= { value: null };
  }
  context.detailReloadKey.value = 0;
  context.historyRequestSequence = 0;
  const declarations = source.slice(
    source.indexOf("  let detailLoadSequence"),
    source.indexOf("  const loading = ref(false)"),
  );
  const names = [
    "loadOptions",
    "loadCurrentDetail",
    "loadDetail",
    "setAttachments",
    "reloadAttachments",
    "resetHistories",
    "handleDetailTabChange",
    "refreshAfterUpdate",
    "updateField",
    "detailUpdateEvent",
    "isFinalStatus",
    "openStatusEditor",
    "handleStatusTransitionSuccess",
  ];
  const watcher = source.slice(
    source.indexOf(
      "  watch(\n    () => [props.open, props.wsId, props.requirementId]",
    ),
    source.indexOf("  watch(\n    () => props.workLogEditorToken"),
  );
  vm.runInNewContext(
    declarations +
      "let statusTransitionContext;\n" +
      names.map((name) => functionSource(source, name)).join("\n") +
      watcher,
    context,
  );
  return {
    context,
    calls,
    events,
    props,
    delay(name, matches = () => true) {
      const wait = { name, matches, entered: deferred(), result: deferred() };
      waits.push(wait);
      return wait;
    },
    change(next = {}) {
      Object.assign(props, next);
      cleanup?.();
      return context.run(
        [props.open, props.wsId, props.requirementId],
        undefined,
        (callback) => {
          cleanup = callback;
        },
      );
    },
    stop() {
      cleanup?.();
    },
  };
}

for (const stage of ["metadata", "display", "detail"]) {
  const h = harness(),
    wait = h.delay(stage);
  const pending = h.change();
  await wait.entered.promise;
  await h.change({ open: false, requirementId: null });
  const count = h.calls.length;
  wait.result.resolve();
  await pending;
  assert.equal(h.calls.length, count, `${stage}: closed loader stops requests`);
  assert.equal(h.context.detail.value, null);
  assert.equal(h.context.loading.value, false);
}

for (const rejectOld of [false, true]) {
  const h = harness(),
    old = h.delay("detail"),
    current = h.delay("detail");
  const first = h.change();
  await old.entered.promise;
  const second = h.change({ requirementId: 189 });
  await current.entered.promise;
  if (rejectOld) old.result.reject(new Error("old request failed"));
  else old.result.resolve();
  await first;
  assert.equal(
    h.context.loading.value,
    true,
    "old finally cannot stop new loading",
  );
  current.result.resolve();
  await second;
  assert.equal(h.context.detail.value.id, 189);
  assert.deepEqual(
    h.events
      .filter((event) => event.name === "loaded")
      .map((event) => event.payload.id),
    [189],
  );
}

for (const next of [
  { requirementId: 189 },
  { wsId: 20 },
  { open: false, requirementId: null },
]) {
  const h = harness(),
    wait = h.delay("attachments");
  const pending = h.change();
  await wait.entered.promise;
  await h.change(next);
  const attachments = h.context.attachments.value;
  const children = h.context.childRequirements.value;
  wait.result.resolve();
  await pending;
  assert.equal(
    h.context.attachments.value,
    attachments,
    "old attachments cannot overwrite",
  );
  assert.equal(
    h.context.childRequirements.value,
    children,
    "old children cannot overwrite",
  );
}

{
  const h = harness(),
    wait = h.delay("detail");
  const pending = h.change();
  await wait.entered.promise;
  await h.change({ open: false, requirementId: null });
  await h.change({ open: true, requirementId: 188 });
  const detail = h.context.detail.value;
  wait.result.resolve();
  await pending;
  assert.equal(
    h.context.detail.value,
    detail,
    "reopening same ID invalidates first load",
  );
  assert.equal(h.events.filter((event) => event.name === "loaded").length, 1);
}

{
  const h = harness(),
    wait = h.delay("detail");
  const pending = h.change();
  await wait.entered.promise;
  h.stop();
  const count = h.calls.length;
  wait.result.resolve();
  await pending;
  assert.equal(
    h.calls.length,
    count,
    "watcher disposal stops follow-up requests",
  );
  assert.equal(h.context.detail.value, null);
}

for (const name of ["attachments", "history", "update"]) {
  const h = harness();
  await h.change();
  const wait = h.delay(name);
  const pending =
    name === "attachments"
      ? h.context.reloadAttachments()
      : name === "history"
        ? h.context.handleDetailTabChange("history")
        : h.context.updateField("title", "saved A");
  await wait.entered.promise;
  await h.change({ requirementId: 189 });
  const detail = h.context.detail.value;
  const attachments = h.context.attachments.value;
  const count = h.calls.length;
  wait.result.resolve();
  await pending;
  assert.equal(h.context.detail.value, detail);
  assert.equal(h.context.attachments.value, attachments);
  assert.equal(
    h.calls.length,
    count,
    `${name}: obsolete operation cannot request B`,
  );
  if (name === "history") assert.equal(h.context.historyLoaded.value, false);
  if (name === "update") {
    const event = h.events.at(-1);
    assert.equal(event.payload.action, "refresh");
    assert.equal(event.payload.id, 188, "save event belongs to A");
  }
}

{
  const h = harness();
  h.props.keepAlive = true;
  h.props.initialTab = "aging";
  await h.change();
  assert.equal(h.context.detailTab.value, "aging");
  h.context.detailTab.value = "history";
  await h.context.updateField("title", "saved");
  assert.equal(h.context.detailTab.value, "history", "refresh preserves tab");
  const count = h.calls.length;
  await h.change({ open: false });
  await h.change({ open: true });
  assert.equal(h.calls.length, count, "keepAlive reuses a fully loaded detail");
  assert.equal(h.context.detailTab.value, "history");
}

{
  const h = harness();
  h.props.keepAlive = true;
  const wait = h.delay("attachments");
  const pending = h.change();
  await wait.entered.promise;
  await h.change({ open: false });
  wait.result.resolve();
  await pending;
  await h.change({ open: true });
  assert.equal(
    h.context.attachments.value[0].id,
    188,
    "incomplete keepAlive load retries",
  );
}

{
  const text = await readFile(
    new URL("../src/composables/useLocalDraft.js", import.meta.url),
    "utf8",
  );
  const wait = deferred();
  const context = {
    flush: () => wait.promise,
    scope: { targetId: "189", wsId: "19" },
    hasUnposted: {},
    hasRecords: {},
    lastSavedAt: {},
  };
  vm.runInNewContext(functionSource(text, "setTarget"), context);
  let current = true;
  const pending = context.setTarget("188", { wsId: "19" }, () => current);
  current = false;
  wait.resolve();
  await pending;
  assert.equal(
    context.scope.targetId,
    "189",
    "stale task cannot retarget draft",
  );
  await context.setTarget("190");
  assert.equal(
    context.scope.targetId,
    "190",
    "existing draft callers still work",
  );
}

// A stale failure arriving after B finished must not clear the new detail.
{
  const h = harness(),
    wait = h.delay("detail");
  const pending = h.change();
  await wait.entered.promise;
  await h.change({ requirementId: 189 });
  const detail = h.context.detail.value;
  wait.result.reject(new Error("late A failure"));
  await pending;
  assert.equal(h.context.detail.value, detail);
}

// Metadata and settings responses belong to their original workspace.
for (const name of ["metadata", "display"]) {
  const h = harness(),
    wait = h.delay(name);
  const pending = h.change();
  await wait.entered.promise;
  await h.change({ wsId: 20 });
  const metadata = h.context.workspaceMetadata.value;
  wait.result.resolve();
  await pending;
  assert.equal(h.context.workspaceMetadata.value, metadata);
  assert.equal(h.context.workspacePermissions.value[0], 20);
}

{
  const h = harness();
  await h.change();
  h.context.titleDraft.value = "unsaved title";
  h.context.descriptionDraft.value = "unsaved description";
  h.context.editingTitle.value = true;
  await h.context.loadCurrentDetail();
  assert.equal(h.context.titleDraft.value, "unsaved title");
  assert.equal(h.context.descriptionDraft.value, "unsaved description");
  h.context.openStatusEditor();
  assert.equal(h.context.editingProperty.value, "status");
  h.context.openStatusEditor("header_status");
  assert.equal(h.context.editingProperty.value, "header_status");
  h.context.editingProperty.value = "";
  h.context.fieldEditable = () => false;
  h.context.openStatusEditor("header_status");
  assert.equal(h.context.editingProperty.value, "");
  h.context.fieldEditable = () => true;
  h.context.statusTransitionOn.value = true;
  h.context.openStatusEditor("header_status");
  assert.equal(h.context.transitionOpen.value, true);
  assert.equal(h.context.editingProperty.value, "");
  await h.change({ requirementId: 189 });
  const count = h.calls.length;
  await h.context.handleStatusTransitionSuccess({ status_id: 2 });
  assert.equal(h.calls.length, count, "status success for A must not reload B");
  assert.equal(h.events.at(-1).payload.id, 188);
}

// Both panels share one request and invalidation; failed loads remain retryable.
{
  const h = harness();
  await h.change();
  const wait = h.delay("history");
  const processing = h.context.handleDetailTabChange("processing");
  await wait.entered.promise;
  await h.context.handleDetailTabChange("history");
  assert.equal(h.calls.filter((call) => call.name === "history").length, 1);
  wait.result.resolve();
  await processing;
  assert.equal(h.context.historyLoaded.value, true);
  await h.context.handleDetailTabChange("history");
  assert.equal(h.calls.filter((call) => call.name === "history").length, 1);
  h.context.resetHistories();
  const failure = h.delay("history");
  const failed = h.context.handleDetailTabChange("processing");
  await failure.entered.promise;
  failure.result.reject(new Error("history unavailable"));
  await assert.rejects(failed);
  assert.equal(h.context.historyLoading.value, false);
  assert.equal(h.context.historyLoaded.value, false);
  await h.context.handleDetailTabChange("processing");
  assert.equal(h.context.historyLoaded.value, true);
}

// Creation, status, and both assignment sources share the processing projection.
{
  const histories = [
    { id: 6, action: "field_update", field_name: "assignee" },
    { id: 5, action: "status_change", field_name: "status" },
    { id: 4, action: "field_update", field_name: "title" },
    { id: 3, action: "issue_recall", field_name: "status" },
    { id: 2, action: "created" },
    { id: 1, action: "assignee_change", field_name: "assignee" },
  ];
  assert.deepEqual(
    buildIssueProcessingRecords(histories, { id: 9, created_at: 1 }).map(
      (item) => item.id,
    ),
    [6, 5, 3, 2, 1],
  );
  const legacy = [
    { id: 8, action: "status_change", field_name: "status", created_at: 20 },
  ];
  const result = buildIssueProcessingRecords(legacy, {
    id: 9,
    created_at: 10,
    creator_name: "Creator",
    status_name: "Current",
    assignee_names: ["Current assignee"],
  });
  assert.equal(result.length, 2);
  assert.equal(result[1].action, "created");
  assert.equal(result[1].user_name, "Creator");
  assert.equal(result[1].processing_snapshot, undefined);
  assert.equal(legacy.length, 1);
  assert.equal(
    processingActionKey({ action: "created" }),
    "issue_processing_created",
  );
  assert.equal(
    processingActionKey({ action: "field_update", field_name: "assignee" }),
    "issue_processing_assigned",
  );
  assert.equal(
    processingActionKey({ action: "issue_recall", field_name: "title" }),
    "",
  );
  assert.deepEqual(buildIssueProcessingRecords([], null), []);
}

// Snapshot fields win over current metadata; unknown old state stays unknown.
{
  const timeline = await readFile(
    new URL(
      "../src/components/work-item/IssueProcessingRecords.vue",
      import.meta.url,
    ),
    "utf8",
  );
  const context = { props: { valueText: (_, value) => value } };
  for (const name of [
    "isStatusEvent",
    "processingStatusText",
    "processingAssigneeText",
  ]) {
    const fn = timeline.match(new RegExp(`function ${name}\\([^]*?\n\}`))[0];
    vm.runInNewContext(fn, context);
  }
  assert.equal(
    context.processingStatusText({ action: "created", new_value: "A title" }),
    "—",
  );
  assert.equal(
    context.processingStatusText({
      action: "status_change",
      new_value: "Old status",
    }),
    "Old status",
  );
  const item = {
    action: "status_change",
    new_value: "Old status",
    processing_snapshot: {
      status_name: "Snapshot status",
      assignee_names: ["A", "B"],
    },
    assignee_names: ["Wrong"],
  };
  assert.equal(context.processingStatusText(item), "Snapshot status");
  assert.equal(context.processingAssigneeText(item), "A、B");
  assert.equal(
    context.processingAssigneeText({ assignee_names: ["#7"] }),
    "#7",
  );
  assert.equal(
    context.processingAssigneeText({
      processing_snapshot: { assignee_names: [] },
      assignee_names: ["Wrong"],
    }),
    "—",
  );
}

// The narrow timeline indicator follows horizontal scrolling and resizing.
{
  const timeline = await readFile(
    new URL(
      "../src/components/work-item/IssueProcessingRecords.vue",
      import.meta.url,
    ),
    "utf8",
  );
  const fn = timeline.match(/function syncScroll\(\) \{[\s\S]*?\n\}/)[0];
  const scrollEl = {
    value: { clientWidth: 636, scrollWidth: 1464, scrollLeft: 0 },
  };
  const scrollThumbStyle = { value: {} };
  const context = { scrollEl, scrollThumbStyle };
  vm.runInNewContext(fn, context);
  context.syncScroll();
  assert.equal(Number.parseFloat(scrollThumbStyle.value.marginLeft), 0);
  assert.ok(
    Math.abs(Number.parseFloat(scrollThumbStyle.value.width) - 43.44) < 0.01,
  );
  scrollEl.value.scrollLeft = 828;
  context.syncScroll();
  assert.ok(
    Math.abs(
      Number.parseFloat(scrollThumbStyle.value.width) +
        Number.parseFloat(scrollThumbStyle.value.marginLeft) -
        100,
    ) < 0.01,
  );
  scrollEl.value = { clientWidth: 1464, scrollWidth: 1464, scrollLeft: 0 };
  context.syncScroll();
  assert.equal(scrollThumbStyle.value.width, "100%");
  scrollEl.value = null;
  context.syncScroll();
  assert.equal(scrollThumbStyle.value.width, "100%");
}

console.log("work item detail loading regressions: ok");
