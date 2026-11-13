# 邀填工单前端集成 TODO

> 路由、菜单、项目 tab、应用卡片、权限目录和组件入口已接入；领域函数体仍保留 TODO。

1. `router/index.js`
   - [x] 新增路径 `/invite/i/:key` 与 `/invite/i/:key/:templateKey`，指向 `IssueInvitePage.vue`。
   - [x] 成员邀请路由改为 `/invite/m/:token`；`i`/`m` 前缀区分 16 位工单 Key 与 64 位成员 Token。
   - [x] 通过 `meta.public` 跳过登录权限守卫。
2. `workspace/settings/ApplicationMarketPanel.vue`
   - [x] 增加 `issue` 应用图标、配置按钮和应用状态事件。
   - [x] 关闭后当前 `issue-invite` section 由可见菜单回退逻辑跳走。
3. `workspace/SettingsPage.vue`
   - [x] 应用开启且拥有 `ticket:manage` 时显示“邀填工单”菜单并渲染 `IssueInviteSettings`。
   - [x] WorkspaceSettings 路由权限列表补充 `ticket:manage`；仅有复制权限不允许进入配置页。
4. `workspace/BoardPage.vue`
   - [x] 在知识库之后增加邀填工单 tab，应用开启时显示并渲染 `IssueInvitePanel`。
   - [x] tab 可见性与按钮权限分开：入口按应用状态，复制按 `ticket:copy_link`，设置按 `ticket:manage`。
5. `workspace/settings/ProjectRolesTab.vue`
   - [x] 权限列表请求携带 `wsId`；后端仅在应用开启时返回 ticket 权限组。
6. i18n
   - [x] 已补充当前组件与应用/权限目录使用的中英文平铺 key。
   - [ ] 后续函数实现时补充复制结果、提交结果及具体校验失败 key。
   - [x] 禁止点分隔或嵌套 JSON，所有用户可见文本使用 `$t()`。
7. URL
   - [x] API 与组件接口仅接收公开邀请/模板 Key，不从数据库 ID 生成公开链接。
   - [x] 链接主体使用路径传参 `/invite/i/{key}`、`/invite/i/{key}/{templateKey}`，不再拼接查询串。
   - [x] `openid` 为运行时动态访客标识，保留为可选 `?openid=` 查询参数；页面仍兼容旧 `?k=`/`?t=`。
