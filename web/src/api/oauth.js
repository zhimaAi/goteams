import { request } from "./request";

// ======================== OAuth (third-party scan-code login) API ========================

/** Fetch the third-party sign-in providers that are switched on. */
export function fetchOauthProviders() {
  return request({
    url: "/api/auth/oauth/providers",
    method: "GET",
  });
}

/** Fetch a scan-code authorize entry for the sign-in page. */
export function fetchOauthAuthorize(provider) {
  return request({
    url: "/api/auth/oauth/authorize",
    method: "GET",
    params: { provider },
  });
}

/** Fetch a scan-code authorize entry for binding on the personal settings page. */
export function fetchUserOauthAuthorize(provider) {
  return request({
    url: "/api/user/oauth/authorize",
    method: "GET",
    params: { provider },
  });
}

/** Exchange a one-time sign-in ticket for a session. */
export function exchangeOauthTicket(ticket) {
  return request({
    url: "/api/auth/oauth/exchange",
    method: "POST",
    data: { ticket },
  });
}

/** Bind the scanned third-party account to an existing account and sign in. */
export function bindOauthAccount(data) {
  return request({
    url: "/api/auth/oauth/bind",
    method: "POST",
    data,
  });
}

/** Register a new tenant and bind the scanned third-party account. */
export function registerOauthAccount(data) {
  return request({
    url: "/api/auth/oauth/register",
    method: "POST",
    data,
  });
}

/** Fetch the current user's third-party binding list. */
export function fetchOauthBindings() {
  return request({
    url: "/api/user/oauth/bindings",
    method: "GET",
  });
}

/** Remove the binding of one provider. */
export function unbindOauthAccount(provider) {
  return request({
    url: `/api/user/oauth/bindings/${provider}`,
    method: "DELETE",
  });
}
