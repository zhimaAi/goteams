import { ref, watchPostEffect } from "vue";
import Sortable from "sortablejs";

export function useWorkflowOptionOrder(
  items,
  container,
  disabled,
  save,
  dragOptions = {},
) {
  const ordering = ref(false);

  async function move(from, to) {
    if (
      disabled.value ||
      ordering.value ||
      from < 0 ||
      to < 0 ||
      from >= items.value.length ||
      to >= items.value.length ||
      from === to
    ) {
      return;
    }
    const previous = items.value;
    const next = [...previous];
    const [item] = next.splice(from, 1);
    next.splice(to, 0, item);
    items.value = next.map((entry, index) => ({
      ...entry,
      sort_order: index + 1,
    }));
    const pendingItems = items.value;
    ordering.value = true;
    try {
      if (
        !(await save(next.map((entry) => entry.id))) &&
        items.value === pendingItems
      ) {
        items.value = previous;
      }
    } finally {
      ordering.value = false;
    }
  }

  watchPostEffect((cleanup) => {
    const element = container();
    if (!element || disabled.value || ordering.value || items.value.length < 2)
      return;
    const sortable = Sortable.create(element, {
      handle: ".workflow-option-handle",
      draggable: "[data-row-key], .workflow-option-row",
      animation: 150,
      ...dragOptions,
      onStart() {
        Sortable.ghost?.setAttribute("inert", "");
      },
      onEnd({ oldDraggableIndex, newDraggableIndex, item, from }) {
        if (
          oldDraggableIndex == null ||
          oldDraggableIndex === newDraggableIndex
        )
          return;
        const rows = [...from.children].filter(
          (row) =>
            row !== item && row.matches("[data-row-key], .workflow-option-row"),
        );
        // Restore Vue's expected DOM order, then commit the drop in the same tick.
        from.insertBefore(item, rows[oldDraggableIndex] || null);
        if (newDraggableIndex != null)
          void move(oldDraggableIndex, newDraggableIndex);
      },
    });
    cleanup(() => sortable.destroy());
  });

  function moveOption(id, offset) {
    const index = items.value.findIndex((item) => item.id === id);
    if (index >= 0) void move(index, index + offset);
  }

  return { ordering, moveOption };
}
