// 编排头像预设与解析工具。
// 预设头像托管在 web/public/avatars 下：URL 即文件名、无构建 hash，dev 与生产地址一致。
// 云端把该相对路径字符串存入步骤/专家的 avatar 字段；客户端本地有同名资源，接相对路径直接渲染。
// 旧数据为空字符串时由 resolve 方法回退到预设，保证界面始终有图。

export const PIPELINE_STEP_AVATARS = [
  "/avatars/agent-avatar-1.png",
  "/avatars/agent-avatar-2.png",
  "/avatars/agent-avatar-3.png",
  "/avatars/agent-avatar-4.png",
  "/avatars/agent-avatar-5.png",
  "/avatars/agent-avatar-6.png",
  "/avatars/agent-avatar-7.png",
];

// 专家团成员与流水线步骤共用同一套人物预设，保证两端视觉一致。
export const AGENT_AVATARS = PIPELINE_STEP_AVATARS;

// 专家团整体头像预设（用流水线/团队图标，偏组织语义）。
export const EXPERT_GROUP_AVATARS = [
  "/avatars/pipeline-avatar-1.svg",
  "/avatars/pipeline-avatar-2.svg",
  "/avatars/pipeline-avatar-3.svg",
  "/avatars/pipeline-avatar-4.svg",
  "/avatars/pipeline-avatar-5.svg",
  "/avatars/pipeline-avatar-6.svg",
  "/avatars/pipeline-avatar-7.svg",
];

// 流水线与专家团共用同一套组织头像，和客户端列表回退图一致。
export const PIPELINE_AVATARS = EXPERT_GROUP_AVATARS;

// resolveAvatar 优先返回已存的真实头像，否则按 index 回退到预设。
export function resolveAvatar(avatar, presets, index = 0) {
  if (avatar) return avatar;
  const list = presets && presets.length ? presets : AGENT_AVATARS;
  return list[Math.max(index, 0) % list.length];
}
