import assert from "node:assert/strict";
import { createPinia, setActivePinia } from "pinia";
import { createServer } from "vite";

const vite = await createServer({
  root: process.cwd(),
  server: { middlewareMode: true },
  appType: "custom",
});

try {
  setActivePinia(createPinia());
  const { useWorkspaceStore } = await vite.ssrLoadModule(
    "/src/stores/workspace.js",
  );
  const { useAuthStore } = await vite.ssrLoadModule("/src/stores/auth.js");
  const workspaceStore = useWorkspaceStore();
  const authStore = useAuthStore();

  workspaceStore.workspaces = [{ id: "1", name: "test" }];
  workspaceStore.loaded = true;
  authStore.logout();

  const stale = workspaceStore.loaded || workspaceStore.workspaces.length > 0;
  const { axios } = await vite.ssrLoadModule("/src/api/request.js");
  for (const result of [
    { login_entry_url: "https://example.com/sso/login?scope_id=tenant" },
    { login_entry_url: "" },
    new Error("network unavailable"),
  ]) {
    authStore.token = "logout-session";
    axios.defaults.adapter = async (config) => {
      assert.equal(config.url, "/api/user/logout");
      assert.equal(config.headers.Authorization, "Bearer logout-session");
      assert.equal(authStore.token, "", "local logout must remain synchronous");
      if (result instanceof Error) {
        result.config = { ...config, silent: true };
        throw result;
      }
      return {
        config,
        status: 200,
        data: { code: 200, data: result },
      };
    };
    const pending = authStore.logout();
    assert.equal(authStore.token, "");
    assert.equal(await pending, result instanceof Error ? null : result);
  }
  assert.equal(
    await authStore.logout(),
    null,
    "anonymous logout skips the API",
  );
  console.log(
    `WORKSPACES_AFTER_LOGOUT=${JSON.stringify(workspaceStore.workspaces)}`,
  );
  console.log(`LOADED_AFTER_LOGOUT=${workspaceStore.loaded}`);
  console.log(`VERDICT=${stale ? "RED_STALE_WORKSPACE" : "GREEN_CLEARED"}`);
  process.exitCode = stale ? 1 : 0;
} finally {
  await vite.close();
}
