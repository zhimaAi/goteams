import { Extension } from "@tiptap/core";
import { Plugin, PluginKey, TextSelection } from "@tiptap/pm/state";
import { Decoration, DecorationSet } from "@tiptap/pm/view";

// Shows a "+" above hovered first-row cells (insert a column to the right) and
// left of hovered first-column cells (insert a row below).
const tableInsertKey = new PluginKey("tableInsertControls");
const cellTypeNames = new Set(["tableCell", "tableHeader"]);

function findHoveredCell(view, domTarget) {
  const cellDom = domTarget?.closest?.("td, th");
  if (!cellDom || !view.dom.contains(cellDom)) return null;
  let $pos;
  try {
    $pos = view.state.doc.resolve(view.posAtDOM(cellDom, 0));
  } catch {
    return null;
  }
  for (let depth = $pos.depth; depth > 0; depth -= 1) {
    const node = $pos.node(depth);
    if (!cellTypeNames.has(node.type.name)) continue;
    const row = $pos.node(depth - 1);
    const table = $pos.node(depth - 2);
    if (!row || !table) return null;
    return {
      pos: $pos.before(depth),
      isFirstRow: table.child(0) === row,
      isFirstCol: row.child(0) === node,
    };
  }
  return null;
}

function sameCell(previous, next) {
  return (
    (previous?.pos ?? -1) === (next?.pos ?? -1) &&
    Boolean(previous?.isFirstRow) === Boolean(next?.isFirstRow) &&
    Boolean(previous?.isFirstCol) === Boolean(next?.isFirstCol)
  );
}

// The insert buttons are absolutely positioned outside their cells, so moving
// the cursor onto one crosses a gap where no cell is hovered. Hiding is
// delayed to keep the buttons clickable during that crossing.
const HIDE_DELAY_MS = 300;

// Tiptap table commands read state.selection, so the caret has to be moved
// into the hovered cell with its own transaction before running the command.
function insertButton(editor, cellPos, className, commandName) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = `table-insert-btn ${className}`;
  button.textContent = "+";
  button.setAttribute("contenteditable", "false");
  button.addEventListener("mousedown", (event) => {
    event.preventDefault();
    event.stopPropagation();
    const { state, view } = editor;
    const $pos = state.doc.resolve(cellPos + 1);
    view.dispatch(state.tr.setSelection(TextSelection.near($pos)));
    editor.commands[commandName]();
  });
  return button;
}

export const TableInsertControls = Extension.create({
  name: "tableInsertControls",
  addProseMirrorPlugins() {
    const editor = this.editor;
    return [
      new Plugin({
        key: tableInsertKey,
        state: {
          init: () => null,
          apply(transaction, value) {
            const meta = transaction.getMeta(tableInsertKey);
            if (meta !== undefined) return meta;
            if (!value || !transaction.docChanged) return value;
            const mapped = transaction.mapping.mapResult(value.pos);
            return mapped.deleted ? null : { ...value, pos: mapped.pos };
          },
        },
        props: {
          decorations(state) {
            const hovered = tableInsertKey.getState(state);
            if (!hovered) return DecorationSet.empty;
            const widgets = [];
            if (hovered.isFirstRow) {
              widgets.push(
                Decoration.widget(
                  hovered.pos + 1,
                  () =>
                    insertButton(
                      editor,
                      hovered.pos,
                      "table-insert-btn--column",
                      "addColumnAfter",
                    ),
                  { side: -1, ignoreMutation: () => true },
                ),
              );
            }
            if (hovered.isFirstCol) {
              widgets.push(
                Decoration.widget(
                  hovered.pos + 1,
                  () =>
                    insertButton(
                      editor,
                      hovered.pos,
                      "table-insert-btn--row",
                      "addRowAfter",
                    ),
                  { side: -1, ignoreMutation: () => true },
                ),
              );
            }
            if (!widgets.length) return DecorationSet.empty;
            return DecorationSet.create(state.doc, widgets);
          },
        },
        view(view) {
          let hideTimer = null;
          const cancelHide = () => {
            if (hideTimer) {
              clearTimeout(hideTimer);
              hideTimer = null;
            }
          };
          const scheduleHide = () => {
            cancelHide();
            hideTimer = setTimeout(() => {
              hideTimer = null;
              if (!tableInsertKey.getState(view.state)) return;
              view.dispatch(view.state.tr.setMeta(tableInsertKey, null));
            }, HIDE_DELAY_MS);
          };
          const handleMouseOver = (event) => {
            const next = findHoveredCell(view, event.target);
            if (!next) {
              scheduleHide();
              return;
            }
            cancelHide();
            if (sameCell(tableInsertKey.getState(view.state), next)) return;
            view.dispatch(view.state.tr.setMeta(tableInsertKey, next));
          };
          const handleMouseLeave = scheduleHide;
          view.dom.addEventListener("mouseover", handleMouseOver);
          view.dom.addEventListener("mouseleave", handleMouseLeave);
          return {
            destroy() {
              cancelHide();
              view.dom.removeEventListener("mouseover", handleMouseOver);
              view.dom.removeEventListener("mouseleave", handleMouseLeave);
            },
          };
        },
      }),
    ];
  },
});
