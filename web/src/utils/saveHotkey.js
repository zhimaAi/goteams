// Ctrl/Cmd+S is only taken over while something is actually editing.
// With no registered handler the browser keeps its native behaviour, so the
// shortcut is never swallowed silently.
const handlers = new Set();

export function isSaveHotkey(event) {
  if (!(event.metaKey || event.ctrlKey) || event.shiftKey) return false;
  const key = event.key;
  return (
    event.code === "KeyS" ||
    key === "s" ||
    key === "S" ||
    event.keyCode === 83 ||
    event.which === 83
  );
}

// registerSaveHotkey subscribes a save callback and returns its unsubscribe.
export function registerSaveHotkey(handler) {
  handlers.add(handler);
  return () => handlers.delete(handler);
}

// installSaveHotkeyInterceptor binds the single app-wide capture listener.
export function installSaveHotkeyInterceptor() {
  if (window.__goteamsSaveHotkeyInstalled) return;
  window.__goteamsSaveHotkeyInstalled = true;
  window.addEventListener(
    "keydown",
    (event) => {
      if (!handlers.size || !isSaveHotkey(event)) return;
      event.preventDefault();
      [...handlers].forEach((handler) => handler());
    },
    { capture: true, passive: false },
  );
}
