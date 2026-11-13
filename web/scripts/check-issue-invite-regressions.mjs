import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { compileScript, parse } from "@vue/compiler-sfc";
import * as vue from "vue";
import dayjs from "dayjs";
import { useIssueOpenid } from "../src/composables/useIssueOpenid.js";

// Exercise the real SFC setup functions without a browser or a new test framework.
function loadComponent(file, dependencies) {
  const { descriptor } = parse(
    readFileSync(new URL(file, import.meta.url), "utf8"),
  );
  const script = compileScript(descriptor, { id: file });
  const bindings = Object.entries(script.imports)
    .map(
      ([name, { source, imported }]) =>
        `const ${name} = dependencies[${JSON.stringify(source)}][${JSON.stringify(imported)}];`,
    )
    .join("\n");
  const source = script.content
    .replace(/^import\s[\s\S]*?from\s*["'][^"']+["'];?\s*$/gm, "")
    .replace("export default", "return");
  const component = new Function("dependencies", `${bindings}\n${source}`)(
    dependencies,
  );
  component.render = () => null;
  return component;
}

const renderer = vue.createRenderer({
  createComment: () => ({}),
  insert() {},
  remove() {},
  parentNode: () => null,
  nextSibling: () => null,
});
function mount(component, props) {
  const app = renderer.createApp(component, props);
  app.mount({});
  return { state: app._instance.setupState, unmount: () => app.unmount() };
}
const flush = async () => {
  await new Promise(setImmediate);
  await vue.nextTick();
};
const storage = () => {
  const entries = new Map();
  return {
    getItem: (key) => entries.get(key),
    setItem: (key, value) => entries.set(key, value),
  };
};
globalThis.localStorage = storage();
globalThis.sessionStorage = storage();
const scrolls = [];
globalThis.window = { scrollTo: (options) => scrolls.push(options) };
const viewportMeta = { content: "width=device-width, initial-scale=1.0" };
globalThis.document = {
  querySelector: (selector) => {
    assert.equal(selector, 'meta[name="viewport"]');
    return viewportMeta;
  },
  body: { style: { touchAction: "auto" } },
};

const inviteKey = "abcdefghijklmnop";
const route = vue.reactive({
  name: "IssueInviteTemplate",
  params: { key: inviteKey, templateKey: "template-a" },
  query: { openid: "AUTO_TEST_issue_invite", title: "link title" },
});
const router = {
  async push({ name, params, query }) {
    Object.assign(route, { name, params, query });
    await vue.nextTick();
  },
};
const schema = [
  {
    field_key: "title",
    field_type: "text",
    default_value: "default title",
    link_param: "title",
  },
  { field_key: "description", field_type: "textarea" },
  { field_key: "custom:1", field_type: "select_multi" },
  { field_key: "planned_start_date", field_type: "date" },
];
let rejectSubmit = true;
let submitted;
let historyHandler = async (_key, _openid, page) => ({
  items: [{ requirement_id: 1, status_name: "latest" }],
  page,
  total: 30,
});
let uploadHandler = async () => ({ id: 7, url: "/test-attachment.txt" });
const api = {
  async getPublicIssueTemplate(_key, templateKey) {
    return { template_key: templateKey, fields_json: JSON.stringify(schema) };
  },
  async getPublicIssueInvite() {
    return { templates: [{ template_key: "template-a" }] };
  },
  listPublicIssueHistory: (...args) => historyHandler(...args),
  async submitPublicIssue(...args) {
    submitted = args;
    if (rejectSubmit) throw new Error("expected submission failure");
  },
  uploadPublicIssueAttachment: (...args) => uploadHandler(...args),
  async deletePublicIssueAttachment() {},
};
const dependencies = {
  vue,
  "vue-router": { useRoute: () => route, useRouter: () => router },
  "vue-i18n": { useI18n: () => ({ t: (key) => key }) },
  "ant-design-vue": { message: { success() {}, warning() {}, error() {} } },
  "@ant-design/icons-vue": {},
  dayjs: { default: dayjs },
  "@/api/issueInvite": api,
  "@/composables/useIssueOpenid": { useIssueOpenid },
  "@/composables/useIssueInviteLang": {
    useIssueInviteLang: () => ({ currentLang: vue.ref("zh-CN") }),
  },
  "./components/IssueForm.vue": {},
  "./components/IssueHistoryDetail.vue": {},
  "./components/IssueHistoryList.vue": {},
  "./components/IssueTemplatePicker.vue": {},
};
const Form = loadComponent(
  "../src/views/issue-invite/components/IssueForm.vue",
  dependencies,
);
const Page = loadComponent(
  "../src/views/issue-invite/IssueInvitePage.vue",
  dependencies,
);
const pageMount = mount(Page);
const page = pageMount.state;
await flush();
assert.equal(
  viewportMeta.content,
  "width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no",
);
assert.equal(document.body.style.touchAction, "pan-x pan-y");
assert.equal(page.state, "form");
const mountForm = () =>
  mount(Form, {
    template: page.selectedTemplate,
    linkValues: page.linkValues,
    draft: page.createDraft,
    inviteKey,
    openid: page.resolvedOpenid,
    onSubmit: (payload) => page.submitIssue(payload),
  });
let form = mountForm();
assert.equal(form.state.formState.title, "link title");
Object.assign(form.state.formState, {
  title: "unsaved title",
  description: "",
  "custom:1": ["a", "b"],
  planned_start_date: dayjs("2026-10-22"),
});
await form.state.handleAttachmentUpload({ name: "test.txt", size: 10 });
form.unmount();
page.switchTab("history");
await flush();
assert.equal(page.history.items[0].status_name, "latest");
assert.deepEqual(scrolls.at(-1), { top: 0, left: 0, behavior: "instant" });
page.switchTab("create");
await flush();
form = mountForm();
assert.equal(form.state.formState.title, "unsaved title");
assert.equal(form.state.formState.description, "");
assert.deepEqual(form.state.formState["custom:1"], ["a", "b"]);
assert.equal(
  form.state.formState.planned_start_date.format("YYYY-MM-DD"),
  "2026-10-22",
);
assert.equal(form.state.uploadedFiles[0].id, 7);

// Failure keeps both the content and attachment ids for a retry.
form.state.submit();
await flush();
assert.deepEqual(submitted[5], [7]);
assert.equal(form.state.uploadedFiles[0].id, 7);
assert.equal(page.state, "form");
rejectSubmit = false;
form.state.submit();
await flush();
assert.equal(route.name, "IssueInviteHistory");
assert.equal(page.history.page, 1);
form.unmount();
page.switchTab("create");
await flush();
form = mountForm();
assert.equal(form.state.formState.title, "link title");
assert.equal(form.state.uploadedFiles.length, 0);

// Changing template, submitter or invitation must not reuse another draft.
form.state.formState.title = "draft A";
const draftA = page.createDraft;
form.unmount();
for (const [params, query] of [
  [{ key: inviteKey, templateKey: "template-b" }, route.query],
  [
    { key: inviteKey, templateKey: "template-a" },
    { openid: "another-submitter" },
  ],
  [{ key: "ponmlkjihgfedcba", templateKey: "template-a" }, route.query],
]) {
  await router.push({ name: "IssueInviteTemplate", params, query });
  await flush();
  assert.notEqual(page.createDraft, draftA);
}
await router.push({
  name: "IssueInviteTemplate",
  params: { key: inviteKey, templateKey: "template-a" },
  query: { openid: "AUTO_TEST_issue_invite" },
});
await flush();
assert.equal(page.createDraft.values.title, "draft A");

// An upload finishing after a tab switch still belongs to the original draft.
form = mountForm();
let finishUpload;
uploadHandler = () =>
  new Promise((resolve) => {
    finishUpload = resolve;
  });
const uploading = form.state.handleAttachmentUpload({
  name: "pending.txt",
  size: 10,
});
form.unmount();
form = mountForm();
assert.equal(form.state.attachmentUploading, true);
const previousSubmit = submitted;
form.state.submit();
assert.equal(submitted, previousSubmit);
finishUpload({ id: 8, url: "/pending.txt" });
await uploading;
assert.equal(form.state.attachmentUploading, false);
assert.equal(form.state.uploadedFiles[0].id, 8);
await form.state.handleAttachmentDelete({ id: 8 });
assert.equal(page.createDraft.files.length, 0);
form.unmount();

// Slow page responses must not overwrite a newer page or a different tab.
page.switchTab("history");
await flush();
let finishHistory;
historyHandler = () =>
  new Promise((resolve) => {
    finishHistory = resolve;
  });
const slowPage = page.loadHistory(2);
historyHandler = async () => ({ items: [], page: 3, total: 30 });
await page.loadHistory(3);
finishHistory({ items: [], page: 2, total: 30 });
await slowPage;
assert.equal(page.history.page, 3);
historyHandler = () =>
  new Promise((resolve) => {
    finishHistory = resolve;
  });
const staleHistory = page.loadHistory(1);
page.switchTab("create");
await flush();
const scrollCount = scrolls.length;
finishHistory({ items: [], page: 1 });
await staleHistory;
assert.equal(page.state, "form");
assert.equal(scrolls.length, scrollCount);

// Common links retain the picker navigation and restore a draft after reselecting its template.
await router.push({
  name: "IssueInviteTemplate",
  params: { key: inviteKey, templateKey: "template-a" },
  query: { ...route.query, open_mode: "common" },
});
await flush();
historyHandler = async () => ({ items: [], page: 1 });
page.switchTab("history");
await flush();
page.switchTab("create");
await flush();
assert.equal(route.name, "IssueInvite");
page.goCreate("template-a");
await flush();
form = mountForm();
assert.equal(form.state.formState.title, "draft A");
form.unmount();

page.switchTab("history");
await flush();
historyHandler = () =>
  new Promise((resolve) => {
    finishHistory = resolve;
  });
const abandonedHistory = page.loadHistory(2);
const finalScrollCount = scrolls.length;
pageMount.unmount();
assert.equal(viewportMeta.content, "width=device-width, initial-scale=1.0");
assert.equal(document.body.style.touchAction, "auto");
finishHistory({ items: [], page: 2 });
await abandonedHistory;
assert.equal(scrolls.length, finalScrollCount);
console.log("issue invitation regression checks passed");

// Exercise visibility and draft isolation in the real comment component.
const commentItems = [
  {
    id: "4",
    user_id: "7",
    content: "internal mention",
    source: "user",
    at_invite: "1",
    parent_id: "0",
    is_public: false,
  },
  {
    id: "3",
    user_id: "7",
    content: "public reply",
    source: "user",
    parent_id: "2",
    is_public: true,
  },
  {
    id: "2",
    user_id: "0",
    content: "invitee question",
    source: "issue_invite",
    parent_id: "0",
    is_public: true,
  },
  {
    id: "1",
    user_id: "7",
    content: "internal",
    source: "user",
    parent_id: "0",
    is_public: false,
  },
];
let commentPayload;
let rejectComment = false;
let commentUpdatePayload;
const Comments = loadComponent(
  "../src/components/common/RequirementComments.vue",
  {
    ...dependencies,
    "ant-design-vue": { Empty: {}, message: {}, Modal: {} },
    "@/stores/auth": { useAuthStore: () => ({ userInfo: { id: 7 } }) },
    "@/utils/date": {},
    "@/utils/imageViewer": { closeImageViewer() {} },
    "@/components/common/CommentRichTextInput.vue": {},
    "@/components/common/RichTextEditor.vue": {},
    "@/api/workspace": {
      listComments: async () => commentItems,
      uploadRequirementAttachments: async () => [{ id: 801 }],
      updateComment: async (_ws, _req, _id, payload) => {
        commentUpdatePayload = payload;
      },
      createComment: async (_ws, _requirement, payload) => {
        commentPayload = payload;
        if (rejectComment) throw new Error("expected comment failure");
      },
    },
  },
);
// Only the DOM text/HTML boundary is mocked; state and submission run in the SFC.
globalThis.document = {
  createElement: () => ({
    textContent: "",
    get innerHTML() {
      return this.textContent
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;");
    },
    set innerHTML(value) {
      this.textContent = value.replace(/<[^>]*>/g, "");
    },
  }),
};
const commentsMount = mount(Comments, {
  open: true,
  wsId: 15,
  requirementId: 281,
  inviteFiller: true,
  canEdit: true,
});
const commentsState = commentsMount.state;
await flush();
assert.deepEqual(
  commentsState.visibleComments.map((item) => item.id),
  ["4", "1"],
);
assert.equal(commentsState.internalCommentCount, 2);
assert.equal(commentsState.publicCommentCount, 2);
commentsState.commentText = 'internal <span data-invite="true">@invitee</span>';
commentsState.startReply(commentItems[0]);
commentsState.replyDraft = "internal reply draft";
commentsState.startCommentEdit(commentItems[0]);
commentsState.commentEditDraft = "internal edit draft";
commentsState.commentTab = "public";
await flush();
assert.deepEqual(
  commentsState.commentRenderList.map((entry) => entry.comment.id),
  ["2", "3"],
);
assert.equal(commentsState.commentText, "");
assert.equal(commentsState.replyTargetId, 0);
assert.equal(commentsState.editingCommentId, 0);
commentsState.commentText = "<p><strong>public &lt;literal&gt;</strong></p>";
commentsState.commentImageFileIds = [701];
commentsState.pendingCommentFiles = [{ name: "public.txt", size: 10 }];
await commentsState.addDetailComment();
assert.equal(commentPayload.is_public, true);
assert.equal(
  commentPayload.content,
  "<p><strong>public &lt;literal&gt;</strong></p>",
);
assert.deepEqual([...commentPayload.image_file_ids], [701]);
assert.deepEqual(commentPayload.attachment_ids, [801]);
assert.equal(commentsState.commentText, "");
commentsState.commentText = "<p><br></p>";
assert.equal(commentsState.canPublishComment, false);
commentsState.commentText = "";
commentsState.startCommentEdit(commentItems[1]);
commentsState.commentEditDraft = '<p><img src="/public-image.png"></p>';
commentsState.commentEditImageFileIds = [702];
await commentsState.saveCommentEdit(commentItems[1]);
assert.equal(
  commentUpdatePayload.content,
  '<p><img src="/public-image.png"></p>',
);
assert.deepEqual([...commentUpdatePayload.image_file_ids], [702]);
assert.deepEqual([...commentsState.commentEditImageFileIds], []);
commentsState.startReply(commentItems[2]);
commentsState.replyDraft = "public reply draft";
rejectComment = true;
await commentsState.submitReply();
assert.equal(commentPayload.is_public, true);
assert.equal(commentPayload.parent_id, 2);
assert.equal(commentsState.replyDraft, "public reply draft");
assert.equal(commentsState.replySubmitting, false);
commentsState.commentTab = "internal";
await flush();
assert.equal(
  commentsState.commentText,
  'internal <span data-invite="true">@invitee</span>',
);
assert.equal(commentsState.replyDraft, "internal reply draft");
assert.equal(commentsState.commentEditDraft, "internal edit draft");
rejectComment = false;
await commentsState.addDetailComment();
assert.equal(commentPayload.is_public, false);
assert.equal(commentPayload.at_invite, true);
commentsState.commentTab = "public";
await flush();
assert.equal(commentsState.replyDraft, "public reply draft");
commentsMount.unmount();
const normalComments = mount(Comments, {
  open: true,
  wsId: 16,
  requirementId: 1,
});
await flush();
assert.equal(normalComments.state.visibleComments.length, 4);
assert.equal(normalComments.state.isPublic, false);
normalComments.unmount();

// An omitted visibility flag must follow the same private default in counts and rows.
commentItems.splice(
  0,
  commentItems.length,
  { id: "5", content: "missing visibility", source: "user", parent_id: "0" },
  { id: "4", content: "null visibility", is_public: null, parent_id: "0" },
  { id: "3", content: "zero visibility", is_public: 0, parent_id: "0" },
  { id: "2", content: "empty visibility", is_public: "", parent_id: "0" },
  { id: "1", content: "private", is_public: false, parent_id: "0" },
);
const unmarkedComments = mount(Comments, {
  open: true,
  wsId: 15,
  requirementId: 281,
  inviteFiller: true,
});
await flush();
assert.equal(unmarkedComments.state.internalCommentCount, 5);
assert.equal(unmarkedComments.state.visibleComments.length, 5);
assert.equal(unmarkedComments.state.commentRenderList.length, 5);
unmarkedComments.state.commentTab = "public";
await flush();
assert.equal(unmarkedComments.state.publicCommentCount, 0);
assert.equal(unmarkedComments.state.visibleComments.length, 0);
unmarkedComments.unmount();

// Historical invitee comments and direct staff replies remain in the public conversation.
commentItems.splice(
  0,
  commentItems.length,
  { id: "5", source: "user", parent_id: "0", content: "internal" },
  { id: "4", source: "user", parent_id: "0", content: "internal" },
  { id: "3", source: "user", parent_id: "0", content: "internal" },
  { id: "2", source: "issue_invite", parent_id: "0", content: "question" },
  { id: "1", source: "user", parent_id: "2", content: "answer" },
);
const historicalComments = mount(Comments, {
  open: true,
  wsId: 15,
  requirementId: 281,
  inviteFiller: true,
});
await flush();
assert.equal(historicalComments.state.internalCommentCount, 3);
assert.equal(historicalComments.state.publicCommentCount, 2);
assert.deepEqual(
  historicalComments.state.visibleComments.map((item) => item.id),
  ["5", "4", "3"],
);
historicalComments.state.commentTab = "public";
await flush();
assert.deepEqual(
  historicalComments.state.visibleComments.map((item) => item.id),
  ["2", "1"],
);
assert.equal(
  historicalComments.state.commentRenderList[1].replyTo,
  "issue_invite_filler",
);
historicalComments.unmount();

commentItems.push(
  {
    id: "6",
    source: "user",
    parent_id: "0",
    at_invite: "1",
    content: "legacy public mention",
  },
  {
    id: "7",
    source: "user",
    parent_id: "0",
    at_invite: "1",
    is_public: false,
    content: "explicit internal mention",
  },
  {
    id: "8",
    source: "user",
    parent_id: "2",
    is_public: false,
    content: "explicit internal flag",
  },
);
const explicitComments = mount(Comments, {
  open: true,
  wsId: 15,
  requirementId: 281,
  inviteFiller: true,
});
await flush();
assert.equal(explicitComments.state.internalCommentCount, 5);
assert.equal(explicitComments.state.publicCommentCount, 3);
assert.deepEqual(
  explicitComments.state.visibleComments.map((item) => item.id),
  ["5", "4", "3", "7", "8"],
);
explicitComments.unmount();
console.log("comment visibility regression checks passed");
