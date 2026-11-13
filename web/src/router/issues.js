export default [
  {
    path: "/issues/:wsId/items/:id/history/:historyId",
    component: () => import("@/layouts/IssueLayout.vue"),
    props: { standalone: true },
    children: [
      {
        path: "",
        name: "IssueHistoryDetail",
        component: () => import("@/views/issues/IssueHistoryDetailPage.vue"),
      },
    ],
    meta: {
      workItemSurface: "issue",
      titleKey: "requirement_history_detailTitle",
      anyPermission: ["requirement:view", "defect:view"],
    },
  },
  {
    path: "/issues/:wsId",
    component: () => import("@/layouts/IssueLayout.vue"),
    meta: { workItemSurface: "issue" },
    children: [
      {
        path: "",
        component: () => import("@/layouts/IssueBoardLayout.vue"),
        children: [
          {
            path: "",
            name: "IssueList",
            component: () => import("@/views/issues/IssueListPage.vue"),
            meta: {
              titleKey: "issue_management",
              anyPermission: ["requirement:view", "defect:view"],
            },
          },
          {
            path: "dashboard",
            name: "IssueDashboard",
            component: () => import("@/views/issues/IssueDashboardPage.vue"),
            meta: {
              titleKey: "issue_management",
              permission: "dashboard:view",
            },
          },
          {
            path: "invite",
            name: "IssueInviteManage",
            component: () => import("@/views/issues/IssueInviteManagePage.vue"),
            meta: {
              titleKey: "app_issue_invite",
              anyPermission: [
                "workspace:view",
                "ticket:copy_link",
                "ticket:manage",
              ],
            },
          },
          {
            path: "knowledge",
            name: "IssueKnowledge",
            component: () => import("@/views/issues/IssueApplicationPage.vue"),
            meta: {
              boardTab: "knowledge",
              titleKey: "issue_management",
              anyPermission: ["kb:view", "kb:manage"],
            },
          },
          {
            path: "planning",
            name: "IssuePlanning",
            component: () => import("@/views/issues/IssueApplicationPage.vue"),
            meta: {
              boardTab: "planning",
              titleKey: "issue_management",
              permission: "planning:view",
            },
          },
          {
            path: "iteration",
            name: "IssueIteration",
            component: () => import("@/views/issues/IssueApplicationPage.vue"),
            meta: {
              boardTab: "iteration",
              titleKey: "issue_management",
              permission: "version:view",
            },
          },
          {
            path: "gantt",
            name: "IssueGantt",
            component: () => import("@/views/issues/IssueApplicationPage.vue"),
            meta: {
              boardTab: "gantt",
              titleKey: "issue_management",
              permission: "gantt:view",
            },
          },
        ],
      },
      {
        path: "create",
        name: "IssueCreate",
        component: () => import("@/views/issues/IssueCreatePage.vue"),
        meta: {
          titleKey: "issue_invite_create_issue",
          anyPermission: ["requirement:create", "defect:create"],
        },
      },
      {
        path: "items/:id",
        name: "IssueDetail",
        component: () => import("@/views/issues/IssueDetailPage.vue"),
        meta: {
          titleKey: "issue_detail",
          anyPermission: ["requirement:view", "defect:view"],
        },
      },

      {
        path: "settings",
        name: "IssueSettings",
        component: () => import("@/views/issues/IssueSettingsPage.vue"),
        meta: { titleKey: "issue_settings", permission: "ticket:manage" },
      },
      {
        path: "settings/templates/new",
        name: "IssueTemplateCreate",
        component: () =>
          import("@/views/issues/settings/IssueTemplateSettingsPage.vue"),
        meta: {
          titleKey: "issue_invite_add_template",
          permission: "ticket:manage",
        },
      },
      {
        path: "settings/templates/:templateId",
        name: "IssueTemplateSettings",
        component: () =>
          import("@/views/issues/settings/IssueTemplateSettingsPage.vue"),
        meta: {
          titleKey: "issue_invite_template_settings",
          permission: "ticket:manage",
        },
      },
    ],
  },
];
