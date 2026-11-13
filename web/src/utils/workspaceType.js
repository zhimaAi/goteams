// Existing workspaces keep their previous entry until they have an explicit type.
export function usesIssuePages(workspaceType, applications = []) {
  return workspaceType
    ? workspaceType === "issue"
    : applications.includes("issue");
}
