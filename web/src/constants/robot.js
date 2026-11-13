// Robot protocol values shared by SLA, automation, and enterprise notifications.
export const robotAtModes = {
  dingtalk: ["mobile", "at_mobiles", "at_user_ids"],
  feishu: ["user_id"],
  wecom: ["mobile", "mentioned_list", "mentioned_mobile_list"],
};
export const robotAtModeLabels = {
  at_mobiles: "@atMobiles",
  at_user_ids: "@atUserIds",
  user_id: "@user_id",
  mentioned_list: "@mentioned_list",
  mentioned_mobile_list: "@mentioned_mobile_list",
};
export const robotUrlPlaceholders = {
  feishu: "https://open.feishu.cn/open-apis/bot/v2/hook/...",
  wecom: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=...",
  dingtalk: "https://oapi.dingtalk.com/robot/send?access_token=...",
};
