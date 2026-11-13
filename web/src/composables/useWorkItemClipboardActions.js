// 工作项列表共用的复制标题、链接和描述动作。
import { unref } from "vue";
import { message } from "ant-design-vue";
import { getRequirement } from "@/api/workspace";
import {
  buildWorkItemPageUrl,
  combineDescriptionContent,
  descriptionContent,
  writeClipboardRichContent,
  writeClipboardText,
} from "@/utils/workItemClipboard";

export function useWorkItemClipboardActions(wsId, t, detailUrl) {
  async function copyPlainText(text, successKey) {
    if (!text) return;
    try {
      await writeClipboardText(text);
      message.success(t(successKey));
    } catch {
      message.error(t("requirement_copyTitleLinkFailed"));
    }
  }

  async function copyRich(content) {
    if (!content?.html && !content?.text) return;
    try {
      await writeClipboardRichContent(content);
      message.success(t("requirement_copySuccess"));
    } catch {
      message.error(t("requirement_copyTitleLinkFailed"));
    }
  }

  function workItemPlainText(item, action) {
    const workspaceId = unref(wsId);
    const title = item?.title || "";
    const link = detailUrl
      ? detailUrl(item?.id)
      : buildWorkItemPageUrl(workspaceId, item?.id);
    if (action === "copyTitle") return title;
    if (action === "copyLink") return link;
    if (action === "copyTitleLink")
      return [title, link].filter(Boolean).join("\n");
    return "";
  }

  function copyWorkItemsInfo(items, action) {
    const separator = action === "copyTitleLink" ? "\n\n" : "\n";
    const text = items
      .map((item) => workItemPlainText(item, action))
      .filter(Boolean)
      .join(separator);
    return copyPlainText(
      text,
      action === "copyTitleLink"
        ? "requirement_titleLinkCopied"
        : "requirement_copySuccess",
    );
  }

  async function copyWorkItemInfo(item, action) {
    if (["copyTitle", "copyLink", "copyTitleLink"].includes(action)) {
      return copyWorkItemsInfo([item], action);
    }
    const workspaceId = unref(wsId);
    const title = item?.title || "";
    const link = detailUrl
      ? detailUrl(item?.id)
      : buildWorkItemPageUrl(workspaceId, item?.id);
    if (!["copyDescription", "copyTitleLinkDescription"].includes(action)) {
      return;
    }
    const hide = message.loading(t("requirement_copying"), 0);
    let detail = null;
    try {
      detail = await getRequirement(workspaceId, item.id);
    } catch {
      /* 由请求拦截器统一提示 */
    } finally {
      hide();
    }
    if (!detail) return;
    const description = descriptionContent(
      detail.description || "",
      detail.description_mode,
    );
    if (action === "copyDescription") return copyRich(description);
    return copyRich(combineDescriptionContent(title, link, description));
  }

  return { copyWorkItemInfo, copyWorkItemsInfo };
}
