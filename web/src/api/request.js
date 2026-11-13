import axios from "axios";
import { message } from "ant-design-vue";
import { useAuthStore } from "@/stores/auth";
import { useLocaleStore } from "@/stores/locale";
import { i18n } from "@/locales";
import { openQuotaModal } from "@/utils/quotaModal";
import { setLogoutReason } from "@/utils/logoutReason";
import { LICENSE_ROUTE_PATH } from "@/constants/license";

// Create the Axios instance.
const instance = axios.create({
  baseURL: "/",
  timeout: 15000,
  headers: { "Content-Type": "application/json" },
});

// A 401 from login means wrong credentials; a 401 from logout means the session is already gone.
// Neither is a forced sign-out, so they must not trigger the redirect or the login-page notice.
function isAuthEndpoint(url) {
  return Boolean(
    url &&
    (url.includes("/api/auth/login") || url.includes("/api/user/logout")),
  );
}

// Check whether the response is a plan quota exceeded error.
function isQuotaExceededError(data) {
  return Boolean(
    data &&
    data.data &&
    data.data.key &&
    data.data.scope &&
    typeof data.data.limit !== "undefined",
  );
}

// The commercial license gate answers with a fixed `license_*` reason in data.reason.
function isLicenseBlockedData(data) {
  const reason = data?.data?.reason;
  return typeof reason === "string" && reason.startsWith("license_");
}

// Module-level guard: once a license redirect is in flight, every further blocked
// response (including the logout call itself being rejected by the gate) must stay
// silent, otherwise the redirect and the toast would fight each other.
let licenseRedirecting = false;

// Installed blocked licenses preserve the session and enter the restricted renewal page;
// a never-activated instance still clears local identity and uses the public license page.
function handleLicenseBlocked(data) {
  if (licenseRedirecting) return;
  licenseRedirecting = true;
  const authStore = useAuthStore();
  const managementOnly = data?.data?.management_only === true;
  if (managementOnly && authStore.token) {
    goToLicensePage(data?.data?.redirect || "/members?tab=version");
    return;
  }
  authStore.clearSession();
  goToLicensePage(LICENSE_ROUTE_PATH);
}

// A blocked API answering on the redirect target itself would navigate to the same URL,
// hard-reloading the page and resetting the module guard above - an endless refresh loop.
function goToLicensePage(target) {
  if (window.location.pathname + window.location.search === target) return;
  window.location.href = target;
}

// Request interceptor - automatically inject token and language.
instance.interceptors.request.use((config) => {
  const authStore = useAuthStore();
  const localeStore = useLocaleStore();

  // Inject the session token.
  if (authStore.token) {
    config.headers.Authorization = `Bearer ${authStore.token}`;
  }

  // Inject the language preference.
  config.headers.lang = localeStore.currentLang;

  return config;
});

// Response interceptor - unified error handling.
instance.interceptors.response.use(
  (response) => {
    if (response.config.responseType === "blob") return response;
    const res = response.data;

    // Business error code handling.
    if (res.code !== 200) {
      // A license block outranks everything, even silent/background requests.
      if (isLicenseBlockedData(res)) {
        handleLicenseBlocked(res);
        return Promise.reject(
          new Error(res.message || i18n.global.t("common_requestFailed")),
        );
      }

      // Background requests (config.silent) must never interrupt the user with a toast.
      if (!response.config.silent) {
        // 401 redirects to login.
        if (res.code === 401 && !isAuthEndpoint(response.config?.url)) {
          // The reload below would wipe a toast, so hand the reason to the login page instead.
          setLogoutReason(res.message);
          const authStore = useAuthStore();
          authStore.logout();
          window.location.href = "/login";
          return Promise.reject(
            new Error(res.message || i18n.global.t("common_requestFailed")),
          );
        }

        message.error(res.message || i18n.global.t("common_requestFailed"));
      }

      return Promise.reject(
        new Error(res.message || i18n.global.t("common_requestFailed")),
      );
    }

    return response;
  },
  (error) => {
    // A license block outranks everything, even silent/background requests.
    if (error.response && isLicenseBlockedData(error.response.data)) {
      handleLicenseBlocked(error.response.data);
      return Promise.reject(error);
    }

    // Background requests fail quietly; the foreground request of the same view reports the problem.
    if (error.config?.silent) return Promise.reject(error);

    // HTTP error handling.
    if (error.response) {
      const { status, data } = error.response;
      const serverMessage = data?.message;
      switch (status) {
        case 401:
          if (isAuthEndpoint(error.config?.url)) {
            break;
          }
          // The redirect reloads the page and would wipe a toast, so the reason is handed to
          // the login page, which is where the user ends up looking.
          setLogoutReason(
            serverMessage || i18n.global.t("auth_sessionExpired"),
          );
          useAuthStore().logout();
          window.location.href = "/login";
          break;
        case 403:
          // Quota exceeded errors show a dedicated modal instead of just a message toast.
          if (isQuotaExceededError(data)) {
            openQuotaModal();
          } else {
            message.error(
              serverMessage || i18n.global.t("common_permissionDenied"),
            );
          }
          break;
        case 404:
          message.error(
            serverMessage || i18n.global.t("common_resourceNotFound"),
          );
          break;
        case 400:
          message.error(serverMessage || i18n.global.t("common_requestFailed"));
          break;
        case 500:
          message.error(serverMessage || i18n.global.t("common_serverError"));
          break;
        default:
          // 400 and other business errors are handled by the caller to avoid duplicate toasts
          break;
      }
    } else if (error.message && error.message.includes("timeout")) {
      message.error(i18n.global.t("common_requestTimeout"));
    } else {
      message.error(i18n.global.t("common_networkError"));
    }

    return Promise.reject(error);
  },
);

// Wrapper request methods (automatically extract the data field).
export async function request(config) {
  const response = await instance(config);
  return response.data.data;
}

export { instance as axios };
