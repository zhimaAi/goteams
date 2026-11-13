import { h } from "vue";
import { message } from "ant-design-vue";

export function showWorkItemUpdateSuccess(text) {
  const dismiss = (event) => {
    event.currentTarget.style.pointerEvents = "none";
    close();
  };
  const close = message.success({
    content: () =>
      h(
        "span",
        {
          // Ant's message API does not forward hover events to the notice.
          onVnodeMounted: ({ el }) => {
            el.closest(".ant-message-notice-content").addEventListener(
              "mouseenter",
              dismiss,
              { once: true },
            );
          },
          onVnodeBeforeUnmount: ({ el }) => {
            el.closest(".ant-message-notice-content").removeEventListener(
              "mouseenter",
              dismiss,
            );
          },
        },
        text,
      ),
  });
}

export function showWorkItemActionSuccess(router, text, result, viewText) {
  if (!result?.id || !result?.workspace_id) {
    message.success(text);
    return;
  }
  const target = router.resolve({
    name:
      router.currentRoute?.value?.meta?.workItemSurface === "issue" &&
      String(router.currentRoute.value.params.wsId) ===
        String(result.workspace_id)
        ? "IssueDetail"
        : "WorkItemDetail",
    params: { wsId: result.workspace_id, id: result.id },
  });
  message.success({
    duration: 5,
    content: () =>
      h("span", { class: "work-item-action-toast" }, [
        h("span", text),
        h(
          "a",
          {
            href: target.href,
            target: "_blank",
            rel: "noopener noreferrer",
            style: "margin-left: 6px",
          },
          viewText,
        ),
      ]),
  });
}
