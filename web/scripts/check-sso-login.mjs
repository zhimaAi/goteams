import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { compileScript, compileTemplate, parse } from "@vue/compiler-sfc";
import * as vue from "vue";

async function loadSetup(path) {
  const filename = new URL(`../src/${path}`, import.meta.url);
  const source = await readFile(filename, "utf8");
  const { descriptor } = parse(source, { filename: filename.pathname });
  const script = compileScript(descriptor, { id: "sso-check" });
  const template = compileTemplate({
    source: descriptor.template.content,
    filename: filename.pathname,
    id: "sso-check",
    compilerOptions: { bindingMetadata: script.bindings },
  });
  assert.deepEqual(template.errors, []);
  // Execute the real component setup with boundary mocks, without a DOM or extra test dependencies.
  const code = script.content
    .replace(/^import[\s\S]*?from\s+["'][^"']+["'];?\s*$/gm, "")
    .replace("export default", "return");
  return (overrides = {}) => {
    const mounted = [];
    const unmounted = [];
    const imports = Object.fromEntries(
      Object.entries(script.imports).map(([name, entry]) => [
        name,
        entry.source === "vue" ? vue[entry.imported] : name,
      ]),
    );
    const mocks = {
      ...imports,
      onMounted: (fn) => mounted.push(fn),
      onUnmounted: (fn) => unmounted.push(fn),
      useRoute: () => ({ query: {} }),
      useRouter: () => ({}),
      useI18n: () => ({ t: (key) => key }),
      useLocaleStore: () => ({ currentLang: "zh-CN" }),
      useAuthStore: () => ({}),
      safeInternalPath: () => "",
      getRememberedTab: (_user, scope) =>
        scope === "auth-tab" ? "register" : "dingtalk",
      ...overrides,
    };
    const component = new Function(...Object.keys(mocks), code)(
      ...Object.values(mocks),
    );
    return { state: component.setup({}, { expose() {} }), mounted, unmounted };
  };
}

const ssoSetup = await loadSetup("views/sso/SsoLoginPage.vue");
const loginSetup = await loadSetup("views/login/index.vue");
const scopeId = "scope+/=?";
const authorization = `/api/sso/authorize?scope_id=${encodeURIComponent(scopeId)}`;

function entry(query, result) {
  const redirects = [];
  const calls = [];
  const component = ssoSetup({
    useRoute: () => ({ query }),
    useRouter: () => ({ replace: (path) => redirects.push(path) }),
    getSsoEntry: async (scope) => {
      calls.push(scope);
      if (result instanceof Error) throw result;
      return await result;
    },
    window: { location: { replace: (path) => redirects.push(path) } },
  });
  return { ...component, redirects, calls };
}

for (const provider of ["dingtalk", "feishu"]) {
  const page = entry({ scope_id: scopeId }, { enabled: true, provider });
  page.state.authorize();
  assert.deepEqual(page.redirects, [], "loading must block authorization");
  await page.mounted[0]();
  assert.equal(page.state.loading.value, false);
  assert.equal(
    page.state.providerPresentation.value.label,
    provider === "dingtalk" ? "auth_loginModeDingtalk" : "auth_loginModeFeishu",
  );
  assert.deepEqual(page.calls, [scopeId]);
  assert.deepEqual(
    page.redirects,
    [],
    "supported platforms must wait for a click",
  );
  page.state.authorize();
  page.state.authorize();
  assert.deepEqual(
    page.redirects,
    [authorization],
    "authorization must encode scope and block repeat clicks",
  );
}

for (const provider of ["oidc", "oauth2"]) {
  const page = entry({ scope_id: scopeId }, { enabled: true, provider });
  await page.mounted[0]();
  assert.deepEqual(page.redirects, [authorization]);
}

for (const query of [{}, { scope_id: "" }, { scope_id: [scopeId] }]) {
  const page = entry(query, { enabled: true, provider: "dingtalk" });
  await page.mounted[0]();
  assert.deepEqual(page.calls, []);
  assert.deepEqual(page.redirects, ["/login"]);
}
for (const result of [null, { enabled: false }, new Error("unavailable")]) {
  const page = entry({ scope_id: scopeId }, result);
  await page.mounted[0]();
  assert.deepEqual(page.redirects, ["/login"]);
}

let resolveEntry;
const leftPage = entry(
  { scope_id: scopeId },
  new Promise((resolve) => {
    resolveEntry = resolve;
  }),
);
const pending = leftPage.mounted[0]();
leftPage.unmounted.forEach((fn) => fn());
resolveEntry({ enabled: true, provider: "oidc" });
await pending;
assert.deepEqual(
  leftPage.redirects,
  [],
  "late requests must not redirect after password navigation",
);

for (const [query, tab, mode] of [
  [{ mode: "password" }, "login", "password"],
  [{}, "register", "dingtalk"],
  [{ mode: "scan" }, "register", "dingtalk"],
  [{ mode: ["password"] }, "register", "dingtalk"],
  [
    { mode: "password", oauth_ticket: "ticket", provider: "feishu" },
    "login",
    "feishu",
  ],
]) {
  const page = loginSetup({
    useRoute: () => ({ query }),
    setRememberedTab: () =>
      assert.fail("initial display must not overwrite remembered choices"),
  });
  assert.equal(page.state.activeTab.value, tab);
  assert.equal(page.state.loginMode.value, mode);
  assert.equal(
    page.state.oauthCallbackPending.value,
    Boolean(query.oauth_ticket),
  );
}

const tokenPage = loginSetup({
  useRoute: () => ({ query: { mode: "password", token: "token" } }),
  takeLogoutReason: () => "",
  useAuthStore: () => ({
    loginWithToken: async (token) => assert.equal(token, "token"),
  }),
  useRouter: () => ({
    replace: async (path) => assert.equal(path, "/my-work"),
  }),
  fetchOauthProviders: () => assert.fail("token sign-in must take priority"),
});
await tokenPage.mounted[0]();

const sidebarSetup = await loadSetup("layouts/Sidebar.vue");
function sidebar(logout) {
  const redirects = [];
  const component = sidebarSetup({
    watch: () => {},
    onBeforeUnmount: () => {},
    useAuthStore: () => ({ logout }),
    useWorkspaceStore: () => ({ workspaces: [] }),
    useThemeStore: () => ({}),
    useNotificationStore: () => ({}),
    useRouter: () => ({ push: (path) => redirects.push(path) }),
    window: { location: { replace: (path) => redirects.push(path) } },
  });
  return { ...component, redirects };
}

for (const result of [
  { login_entry_url: "https://example.com/sso/login?scope_id=tenant" },
  { login_entry_url: "" },
  null,
]) {
  const page = sidebar(async () => result);
  await page.state.handleUserMenuClick({ key: "logout" });
  assert.deepEqual(page.redirects, [result?.login_entry_url || "/"]);
}

let finishLogout;
let logoutCalls = 0;
const exiting = sidebar(() => {
  logoutCalls += 1;
  return new Promise((resolve) => {
    finishLogout = resolve;
  });
});
const logoutPending = exiting.state.handleUserMenuClick({ key: "logout" });
await exiting.state.handleUserMenuClick({ key: "logout" });
assert.equal(logoutCalls, 1, "repeat clicks must not start another logout");
assert.deepEqual(exiting.redirects, [], "wait for the server's tenant entry");
finishLogout({ login_entry_url: "/sso/login?scope_id=tenant" });
await logoutPending;
assert.deepEqual(exiting.redirects, ["/sso/login?scope_id=tenant"]);
console.log("SSO login regressions: ok");
