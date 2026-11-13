// 本地草稿纯规则的自动检查：node scripts/check-local-draft.mjs
// 使用 node:assert，不引入测试框架。
import assert from "node:assert";
import {
  EXPIRE_AFTER,
  MAX_VERSIONS,
  VERSION_INTERVAL,
  applyPosted,
  applyRetarget,
  applySave,
  createDraftSessionId,
  filterVisible,
  isCompatibleRecord,
  isExpired,
} from "../src/utils/localDraftRecord.js";

const t0 = 1_800_000_000_000;
assert.equal(
  createDraftSessionId({ randomUUID: () => "native-uuid" }),
  "native-uuid",
);
assert.match(createDraftSessionId({}), /^\d+-[a-z0-9]+$/);

const meta = {
  key: "draft-create-1-s1",
  kind: "create",
  userId: "1",
  wsId: "10",
  type: "requirement",
  targetId: "s1",
};

// 初始化空内容不创建记录
assert.equal(applySave(null, { ...meta, title: "", html: "" }, t0), null);

// TAPD：全新创建会话第一次有效保存只建立最新内容，不产生历史版本
const first = applySave(
  null,
  { ...meta, title: "标题", html: "<p>正文</p>" },
  t0,
);
assert.equal(first.versions.length, 0);
assert.equal(first.title, "标题");
assert.equal(first.isPosted, false);
assert.equal(isCompatibleRecord(first), true);
assert.equal(isCompatibleRecord({ ...first, schemaVersion: 0 }), false);

// 退出后进入全新会话，恢复 first 再修改：第一次写入新会话仍不能生成历史版本
const restoredFirstEdit = applySave(
  null,
  {
    ...meta,
    key: "draft-create-1-s2",
    targetId: "s2",
    title: "标题-恢复后修改",
    html: "<p>正文-恢复后修改</p>",
  },
  t0 + 30_000,
);
assert.equal(restoredFirstEdit.versions.length, 0);
assert.equal(first.title, "标题");

// 标题和正文都没变 → 不写入
assert.equal(
  applySave(first, { ...meta, title: "标题", html: "<p>正文</p>" }, t0 + 1000),
  null,
);

// TAPD：已有最新内容但还没有历史版本时，下一次有效保存建立第一个子版本
const within = applySave(
  first,
  { ...meta, title: "标题2", html: "<p>正文2</p>" },
  t0 + 60_000,
);
assert.equal(within.versions.length, 1);
assert.equal(within.versions[0].title, "标题2");
assert.equal(within.title, "标题2");

// 距上一版本超过 3 分钟 → 新增版本
const beyond = applySave(
  within,
  { ...meta, title: "标题3", html: "<p>正文3</p>" },
  within.versions[0].savedAt + VERSION_INTERVAL + 1000,
);
assert.equal(beyond.versions.length, 2);

// 版本超过 5 个 → 丢弃最早的
let record = beyond;
for (let i = 4; i <= 8; i += 1) {
  record = applySave(
    record,
    { ...meta, title: `标题${i}`, html: `<p>正文${i}</p>` },
    t0 + i * (VERSION_INTERVAL + 1000),
  );
}
assert.equal(record.versions.length, MAX_VERSIONS);
assert.equal(record.versions[0].title, "标题4");
assert.equal(record.versions[record.versions.length - 1].title, "标题8");

// 主动清空标题和正文：仍视为有效修改，记录和版本保留
const cleared = applySave(
  record,
  { ...meta, title: "", html: "" },
  t0 + 9 * (VERSION_INTERVAL + 1000),
);
assert.equal(cleared.title, "");
assert.equal(cleared.html, "");
assert.equal(cleared.versions.length, MAX_VERSIONS);

// 过期：以最后一次保存起算 3 天
assert.equal(
  isExpired(
    applySave(
      null,
      { ...meta, key: "draft-create-1-old", title: "旧", html: "旧" },
      t0 - EXPIRE_AFTER - 1000,
    ),
    t0,
  ),
  true,
);
assert.equal(isExpired(first, t0), false);

// 可见范围：用户 + 工作空间 + 类型
const visible = filterVisible(
  [
    { ...first, key: "draft-create-2-x", userId: "2" },
    { ...first, key: "draft-create-1-w2", wsId: "20" },
    { ...first, key: "draft-create-1-d1", type: "defect" },
    first,
  ],
  { userId: "1", wsId: "10", type: "requirement" },
  t0,
);
assert.equal(visible.length, 1);
assert.equal(visible[0].key, "draft-create-1-s1");

// TAPD：新建与详情共用同一个缓存池，不按单个工作项拆分
const sharedPool = filterVisible(
  [
    {
      ...first,
      key: "draft-item-1-10-101",
      kind: "work-item",
      targetId: "101",
    },
    {
      ...first,
      key: "draft-item-1-10-102",
      kind: "work-item",
      targetId: "102",
    },
    first,
  ],
  { userId: "1", wsId: "10", type: "requirement" },
  t0,
);
assert.equal(sharedPool.length, 3);

// TAPD：编辑器模式随内容一起缓存，子版本同样带上，旧记录缺字段时降级为空
const withMode = applySave(
  null,
  {
    ...meta,
    key: "draft-create-1-m1",
    title: "md",
    html: "<p>md</p>",
    mode: "markdown",
  },
  t0,
);
assert.equal(withMode.mode, "markdown");
assert.equal(
  applySave(
    withMode,
    { ...meta, title: "md2", html: "<p>md2</p>", mode: "markdown" },
    t0 + 60_000,
  ).versions[0].mode,
  "markdown",
);
const legacy = { ...first };
delete legacy.mode;
assert.equal(
  applySave(legacy, { ...meta, title: "x", html: "<p>x</p>" }, t0 + 60_000)
    .mode,
  "",
);

// markPosted 只标记已提交，不刷新 savedAt
const posted = applyPosted(first);
assert.equal(posted.isPosted, true);
assert.equal(posted.savedAt, first.savedAt);
assert.equal(applyPosted(posted), null);

// retarget：切换工作空间/类型，同 key 原地更新；范围没变不写
const moved = applyRetarget(first, { wsId: "20", type: "defect" });
assert.equal(moved.wsId, "20");
assert.equal(moved.type, "defect");
assert.equal(applyRetarget(moved, { wsId: "20", type: "defect" }), null);

console.log("local-draft record rules OK");
