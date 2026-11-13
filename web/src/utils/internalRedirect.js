/** Same-origin path used as a post-login return URL. Reject protocol-relative and off-site values. */
export function safeInternalPath(raw) {
  if (typeof raw !== "string") return "";
  const value = raw.trim();
  if (!value.startsWith("/") || value.startsWith("//")) return "";
  return value;
}
