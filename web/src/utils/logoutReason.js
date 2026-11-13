// Carries the reason for a forced sign-out across the redirect to the login page.
//
// The 401 handler cannot simply show a toast: it calls `window.location.href = "/login"`
// immediately afterwards, and the full page reload destroys the toast before it is read.
// Stashing the message in sessionStorage lets the login page show it after the reload,
// which is also where the user is actually looking.
const KEY = "goteams_logout_reason";

export function setLogoutReason(reason) {
  if (!reason) return;
  try {
    sessionStorage.setItem(KEY, reason);
  } catch {
    // Private mode or a full quota: losing the reason must never break the redirect.
  }
}

// Reads the pending reason and clears it, so it is shown exactly once.
export function takeLogoutReason() {
  try {
    const reason = sessionStorage.getItem(KEY);
    if (reason) sessionStorage.removeItem(KEY);
    return reason || "";
  } catch {
    return "";
  }
}
