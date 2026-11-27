import { ref, watchPostEffect } from "vue";
import Sortable from "sortablejs";

export function useStatusTransitionFieldOrder(
  fields,
  container,
  disabled,
  persist = () => true,
) {
  const ordering = ref(false);

  async function move(from, to) {
    if (
      disabled.value ||
      ordering.value ||
      from < 0 ||
      to < 0 ||
      from >= fields.value.length ||
      to >= fields.value.length ||
      from === to
    )
      return false;
    const previous = fields.value;
    const next = [...previous];
    next.splice(to, 0, ...next.splice(from, 1));
    fields.value = next;
    ordering.value = true;
    let saved = false;
    try {
      saved = await persist();
      return saved;
    } finally {
      // A new configuration or workspace must not be overwritten by an old failure.
      if (!saved && fields.value === next) fields.value = previous;
      ordering.value = false;
    }
  }

  watchPostEffect((cleanup) => {
    const element = container();
    if (!element || disabled.value || ordering.value || fields.value.length < 2)
      return;
    const sortable = Sortable.create(element, {
      handle: ".workflow-option-handle",
      draggable: ".transition-sortable-field",
      animation: 180,
      easing: "cubic-bezier(0.22, 0.61, 0.36, 1)",
      forceFallback: true,
      fallbackOnBody: true,
      fallbackTolerance: 3,
      ghostClass: "transition-field-placeholder",
      fallbackClass: "transition-field-drag-preview",
      onStart() {
        Sortable.ghost?.setAttribute("inert", "");
      },
      onEnd({ oldDraggableIndex, newDraggableIndex, item, from }) {
        if (
          oldDraggableIndex == null ||
          newDraggableIndex == null ||
          oldDraggableIndex === newDraggableIndex
        )
          return;
        const rows = [...from.children].filter(
          (row) => row !== item && row.matches(".transition-sortable-field"),
        );
        // Restore the DOM before Vue applies the reordered array.
        from.insertBefore(item, rows[oldDraggableIndex] || null);
        void move(oldDraggableIndex, newDraggableIndex);
      },
    });
    cleanup(() => sortable.destroy());
  });

  function moveField(key, offset) {
    const index = fields.value.findIndex((field) => field.key === key);
    return move(index, index + offset);
  }

  return { ordering, moveField };
}
