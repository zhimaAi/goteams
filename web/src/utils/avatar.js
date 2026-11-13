// Common avatar-related utility methods.
// The backend returns a publicly accessible avatar URL directly; the frontend needs no extra request and uses the field as-is.

/**
 * Get the avatar URL.
 * Handles the differing field names across APIs: the project member list returns user_avatar
 * (an alias of users.avatar), while enterprise member / add member return avatar. Returns
 * undefined when there is no avatar.
 * @param {Object} user Member object
 * @returns {string|undefined} Avatar URL, or undefined when absent
 */
export function getAvatarSrc(user) {
  if (!user) return undefined;
  return user.user_avatar || user.avatar || undefined;
}

/**
 * Take the first character of a name as the avatar fallback (uppercased for Latin, first
 * character for CJK, empty string for blank input).
 * Accepts either a name string or a member object (user_name / name / user_username / username).
 * @param {string|Object} userOrName Name string or member object
 * @returns {string} First character (uppercased), or an empty string when no valid name
 */
export function getInitial(userOrName) {
  const name =
    typeof userOrName === "string"
      ? userOrName
      : userOrName?.user_name ||
        userOrName?.name ||
        userOrName?.user_username ||
        userOrName?.username ||
        "";
  if (!name) return "";
  const ch = `${name}`.trim().charAt(0);
  if (!ch) return "";
  return ch.toLocaleUpperCase();
}
