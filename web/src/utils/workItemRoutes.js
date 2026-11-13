// Page surfaces share work-item data but own their navigation destinations.
export function workItemRoute(surface, page, wsId, id, query = {}) {
  const names =
    surface === "issue"
      ? {
          list: "IssueList",
          dashboard: "IssueDashboard",
          create: "IssueCreate",
          detail: "IssueDetail",
          history: "IssueHistoryDetail",
        }
      : {
          list: "RequirementBoard",
          dashboard: "RequirementBoard",
          create: "CreateWorkItem",
          detail: "WorkItemDetail",
          history: "WorkItemHistoryDetail",
        };
  return {
    name: names[page],
    params: { wsId, ...(id ? { id } : {}) },
    query:
      page === "dashboard" && surface !== "issue"
        ? { ...query, tab: "dashboard" }
        : query,
  };
}

export function legacyIssueRoute(to) {
  const wsId = to.params.wsId;
  const query = { ...to.query };
  if (to.name === "WorkspaceSettings" && query.section === "issue-invite") {
    delete query.section;
    return {
      name: "IssueSettings",
      params: { wsId },
      query,
      hash: to.hash,
      replace: true,
    };
  }
  return null;
}

// Issue workspace applications share one persistent header. Preserve old links and their deep-link queries.
export function issueWorkItemRoute(to) {
  const applicationPage = {
    "issue-invite": "IssueInviteManage",
    dashboard: "IssueDashboard",
    knowledge: "IssueKnowledge",
    planning: "IssuePlanning",
    iteration: "IssueIteration",
    gantt: "IssueGantt",
  }[to.query?.tab];
  if (to.name === "RequirementBoard" && typeof applicationPage === "string") {
    const query = { ...to.query };
    delete query.tab;
    return {
      name: applicationPage,
      params: to.params,
      query,
      hash: to.hash,
      replace: true,
    };
  }

  const name = {
    RequirementBoard: "IssueList",
    CreateWorkItem: "IssueCreate",
    WorkItemDetail: "IssueDetail",
    WorkItemHistoryDetail: "IssueHistoryDetail",
  }[to.name];
  if (!name) return null;
  if (
    to.name === "RequirementBoard" &&
    to.query.tab &&
    !["requirement", "defect"].includes(to.query.tab)
  )
    return null;
  return {
    name,
    params: to.params,
    query: to.query,
    hash: to.hash,
    replace: true,
  };
}
