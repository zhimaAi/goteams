import issueRoutes from "./issues";
import { createRouter, createWebHistory } from "vue-router";

const routes = [
  ...issueRoutes,
  // Community deployments enter the application instead of the hosted marketing site.
  { path: "/", redirect: "/my-work" },
  { path: "/home", redirect: "/my-work" },
  { path: "/pricing", redirect: "/my-work" },

  // Login page.
  {
    path: "/login",
    name: "Login",
    component: () => import("@/views/login/index.vue"),
    meta: { title: "登录" },
  },
  // TeamsBoard browser-login confirmation. Unauthenticated visitors are sent to /login with redirect.
  {
    path: "/client-login",
    name: "ClientLogin",
    component: () => import("@/views/client-login/index.vue"),
    meta: { title: "确认登录" },
  },

  // Commercial license page (public; the license gate decides who lands here).
  {
    path: "/license",
    name: "License",
    component: () => import("@/views/license/LicensePage.vue"),
    meta: { title: "开源版授权" },
  },
  // Public issue invitation pages; distinct from member invitation acceptance.
  // The `i` segment avoids colliding with the member route below.
  {
    path: "/invite/i/:key",
    name: "IssueInvite",
    component: () => import("@/views/issue-invite/IssueInvitePage.vue"),
    meta: { title: "邀填工单", public: true },
  },
  // The submitter's own history gets a dedicated address, just like the template form.
  // The static `history` segment wins over the dynamic template route below.
  {
    path: "/invite/i/:key/history",
    name: "IssueInviteHistory",
    component: () => import("@/views/issue-invite/IssueInvitePage.vue"),
    meta: { title: "邀填工单", public: true },
  },
  // The submitter's own detail gets its own address under the history segment.
  {
    path: "/invite/i/:key/history/:requirementId",
    name: "IssueInviteHistoryDetail",
    component: () => import("@/views/issue-invite/IssueInvitePage.vue"),
    meta: { title: "工单详情", public: true },
  },
  {
    path: "/invite/i/:key/:templateKey",
    name: "IssueInviteTemplate",
    component: () => import("@/views/issue-invite/IssueInvitePage.vue"),
    meta: { title: "邀填工单", public: true },
  },
  // Member invitation accept page (public, token authenticated).
  // The `m` segment distinguishes the 64-char member token from the 16-char issue key.
  {
    path: "/invite/m/:token",
    name: "InviteAccept",
    component: () => import("@/views/invite/InvitePage.vue"),
    meta: { title: "加入 GoTeams" },
  },
  {
    path: "/workspaces",
    name: "WorkspaceList",
    component: () => import("@/views/workspace/ListPage.vue"),
    meta: { title: "工作空间" },
  },
  {
    path: "/404",
    name: "NotFound",
    component: () => import("@/views/error/404.vue"),
    meta: { title: "404" },
  },
  {
    path: "/403",
    name: "Forbidden",
    component: () => import("@/views/error/403.vue"),
    meta: { title: "403" },
  },
  {
    path: "/workspace/:wsId/access-denied",
    name: "WorkspaceAccessDenied",
    component: () => import("@/views/workspace/WorkspaceAccessDeniedPage.vue"),
    meta: { title: "无权限" },
  },

  // My Work (standalone page, uses WorkspaceLayout to keep the sidebar but does not depend on wsId).
  {
    path: "/my-work",
    name: "MyWork",
    component: () => import("@/layouts/WorkspaceLayout.vue"),
    children: [
      {
        path: "",
        component: () => import("@/views/my-work/MyWorkPage.vue"),
        meta: { title: "我的工作" },
      },
    ],
  },

  // Team work (standalone page, not depending on wsId).
  {
    path: "/team-work",
    name: "TeamWork",
    component: () => import("@/layouts/WorkspaceLayout.vue"),
    children: [
      {
        path: "",
        component: () => import("@/views/team-work/TeamWorkPage.vue"),
        meta: {
          title: "团队工作",
          anyPermission: [
            "team_work:view_today",
            "team_work:view_yesterday",
            "team_work:view_tomorrow",
            "team_work:view_custom_date",
            "team_work:view_unscheduled",
          ],
        },
      },
    ],
  },

  // Member management (standalone page, not depending on wsId).
  {
    path: "/members",
    name: "MemberList",
    component: () => import("@/layouts/WorkspaceLayout.vue"),
    children: [
      {
        path: "",
        component: () => import("@/views/members/MemberPage.vue"),
        meta: {
          title: "企业管理",
          anyPermission: [
            "personalization:setting",
            "plan:view",
            "member:view",
            "member:view_role_list",
            "storage:view",
            "external_notification:manage",
          ],
        },
      },
    ],
  },

  // Pipeline (standalone page, not depending on wsId).
  {
    path: "/pipelines",
    component: () => import("@/layouts/WorkspaceLayout.vue"),
    children: [
      {
        path: "",
        name: "PipelinePage",
        component: () => import("@/views/pipeline/PipelinePage.vue"),
        meta: { title: "Agent员工" },
      },
      {
        // The settings page merged into the "pipelines" tab of the expert
        // pipeline page; keep the route as a redirect for old links.
        path: "settings/:pipelineId?",
        redirect: (to) => ({
          path: "/pipelines",
          query: {
            tab: "pipelines",
            ...(to.params.pipelineId
              ? { pipelineId: to.params.pipelineId }
              : {}),
          },
        }),
      },
      {
        path: "setup-guide",
        name: "PipelineSetupGuide",
        component: () => import("@/views/pipeline/PipelineSetupGuidePage.vue"),
        meta: { title: "流水线设置说明", permission: "pipeline:view" },
      },
      {
        // Documents produced by one client task: full-page form of the same panel
        // that opens as a drawer from a document reference inside a conversation.
        // 权限跟随工作项查看权限（与需求详情 Agent tab 的沟通记录一致），
        // pipeline:view 仅作兜底：只有企业级流水线权限的成员在 Agent 员工页也能看。
        path: "logs/:taskId/documents",
        name: "pipeline-task-documents",
        component: () =>
          import("@/views/pipeline/PipelineTaskDocumentsPage.vue"),
        meta: {
          title: "产出文档",
          anyPermission: ["requirement:view", "defect:view", "pipeline:view"],
        },
      },
    ],
  },

  // Profile (standalone page, not depending on wsId).
  {
    path: "/profile",
    name: "Profile",
    component: () => import("@/layouts/WorkspaceLayout.vue"),
    children: [
      {
        path: "",
        component: () => import("@/views/profile/ProfilePage.vue"),
        meta: { title: "个人中心" },
      },
    ],
  },

  // Notifications are tenant-wide and do not depend on a workspace.
  {
    path: "/notifications",
    component: () => import("@/layouts/WorkspaceLayout.vue"),
    children: [
      {
        path: "",
        name: "Notifications",
        component: () => import("@/views/notifications/NotificationPage.vue"),
        meta: { title: "消息中心" },
      },
    ],
  },

  // Change detail of one recorded description change; opened in a new tab from the history list.
  // The design shows a full-width page without the sidebar, so it deliberately sits outside
  // WorkspaceLayout instead of nesting under the workspace route below.
  {
    path: "/workspace/:wsId/work-items/:id/history/:historyId",
    name: "WorkItemHistoryDetail",
    component: () => import("@/views/workspace/WorkItemHistoryDetailPage.vue"),
    meta: {
      title: "变更详情",
      anyPermission: ["requirement:view", "defect:view"],
    },
  },

  // Workspace layout (Sidebar + Content).
  {
    path: "/workspace/:wsId",
    component: () => import("@/layouts/WorkspaceLayout.vue"),
    children: [
      {
        path: "",
        redirect: (to) => ({ path: `/workspace/${to.params.wsId}/board` }),
      },
      {
        path: "board",
        name: "RequirementBoard",
        component: () => import("@/views/workspace/BoardPage.vue"),
        meta: {
          title: "需求看板",
          anyPermission: [
            "planning:view",
            "requirement:view",
            "defect:view",
            "version:view",
            "dashboard:view",
            "kb:view",
            "kb:manage",
            "ticket:copy_link",
            "ticket:manage",
            "external_notification:manage",
          ],
        },
      },
      {
        path: "knowledge",
        name: "KnowledgeBase",
        component: () => import("@/views/knowledgebase/KnowledgeBaseView.vue"),
        meta: {
          title: "知识库",
          anyPermission: ["kb:view", "kb:manage"],
        },
      },
      {
        path: "work-items/:id",
        name: "WorkItemDetail",
        component: () => import("@/views/workspace/WorkItemDetailPage.vue"),
        meta: {
          title: "工作项详情",
          anyPermission: ["requirement:view", "defect:view"],
        },
      },
      {
        path: "versions",
        name: "VersionList",
        component: () => import("@/views/workspace/VersionPage.vue"),
        meta: { title: "版本迭代", permission: "version:view" },
      },
      {
        path: "settings",
        name: "WorkspaceSettings",
        component: () => import("@/views/workspace/SettingsPage.vue"),
        meta: {
          title: "空间设置",
          anyPermission: [
            "project_info:edit",
            "project_member:add",
            "project_member:remove",
            "project_member:change_role",
            "project_role:add",
            "project_role:edit",
            "project_role:delete",
            "requirement_setting:field",
            "requirement_setting:status",
            "requirement_setting:priority",
            "requirement_setting:attribute",
            "defect_setting:field",
            "defect_setting:status",
            "defect_setting:priority",
            "defect_setting:attribute",
            "automation:view",
            "automation:manage",
            "application_market:view",
            "application_market:manage",
            "planning:edit",
            "workspace:end",
            "ticket:manage",
          ],
        },
      },
      {
        path: "settings/issue-invite/new",
        name: "IssueInviteTemplateCreate",
        redirect: (to) => ({
          name: "IssueTemplateCreate",
          params: { wsId: to.params.wsId },
          query: to.query,
          hash: to.hash,
        }),
        meta: {
          title: "新增邀填工单模板",
          permission: "ticket:manage",
        },
      },
      {
        path: "settings/issue-invite/:templateId",
        name: "IssueInviteTemplateSettings",
        redirect: (to) => ({
          name: "IssueTemplateSettings",
          params: to.params,
          query: to.query,
          hash: to.hash,
        }),
        meta: {
          title: "邀填工单模板设置",
          permission: "ticket:manage",
        },
      },
      {
        path: "create",
        name: "CreateWorkItem",
        component: () =>
          import("@/views/workspace/create/CreateWorkItemPage.vue"),
        meta: {
          title: "创建工作项",
          anyPermission: ["requirement:create", "defect:create"],
        },
      },
    ],
  },

  // Enterprise SSO entry: DingTalk/Feishu wait for a click; other providers redirect to backend authorization.
  // 必须显式注册，且放在通配 404/回首页之前，否则会被 /:pathMatch(.*)* 兜底走掉。
  {
    path: "/sso/login",
    name: "SsoLogin",
    component: () => import("@/views/sso/SsoLoginPage.vue"),
    meta: { title: "登录", public: true },
  },

  // Default redirect.
  { path: "/:pathMatch(.*)*", redirect: "/" },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router;
