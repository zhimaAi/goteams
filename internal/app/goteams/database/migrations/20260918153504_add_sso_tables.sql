-- +goose Up
-- +goose StatementBegin

-- 企业 SSO 登录配置表：一个租户一行，同一时刻只有一个 provider 生效。
-- app_secret 明文落库且永不出参（与 api_tokens 口径一致），不新增加密基础设施。
CREATE TABLE IF NOT EXISTS sso_configs (
    id                SERIAL PRIMARY KEY,
    admin_id          INTEGER      NOT NULL,
    enabled           SMALLINT     NOT NULL DEFAULT 0,
    provider          VARCHAR(20)  NOT NULL DEFAULT '',
    app_key           VARCHAR(200) NOT NULL DEFAULT '',
    app_secret        VARCHAR(500) NOT NULL DEFAULT '',
    agent_id          VARCHAR(100) NOT NULL DEFAULT '',
    issuer_url        VARCHAR(500) NOT NULL DEFAULT '',
    authorize_url     VARCHAR(500) NOT NULL DEFAULT '',
    token_url         VARCHAR(500) NOT NULL DEFAULT '',
    userinfo_url      VARCHAR(500) NOT NULL DEFAULT '',
    scopes            VARCHAR(255) NOT NULL DEFAULT '',
    external_id_field VARCHAR(100) NOT NULL DEFAULT '',
    callback_key      VARCHAR(32)  NOT NULL,
    created_at        BIGINT       NOT NULL DEFAULT 0,
    updated_at        BIGINT       NOT NULL DEFAULT 0
);

COMMENT ON TABLE "sso_configs" IS '企业SSO登录配置表（一个租户一行，单平台生效）';
COMMENT ON COLUMN "sso_configs"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "sso_configs"."enabled" IS 'SSO开关: 0=关闭, 1=开启；关闭保留全部配置';
COMMENT ON COLUMN "sso_configs"."provider" IS '当前生效平台: oidc/oauth2/dingtalk/wecom/feishu';
COMMENT ON COLUMN "sso_configs"."app_key" IS '应用标识，按平台语义: 钉钉AppKey、企微CorpId、飞书App ID、OIDC/OAuth2 Client ID';
COMMENT ON COLUMN "sso_configs"."app_secret" IS '应用密钥（AppSecret/CorpSecret/Client Secret），明文落库、永不出参';
COMMENT ON COLUMN "sso_configs"."agent_id" IS '钉钉、企业微信必填的应用AgentId；其余平台为空';
COMMENT ON COLUMN "sso_configs"."issuer_url" IS 'OIDC必需：discovery基址，实际端点由 {issuer}/.well-known/openid-configuration 派生';
COMMENT ON COLUMN "sso_configs"."authorize_url" IS '通用OAuth2.0必需：授权端点';
COMMENT ON COLUMN "sso_configs"."token_url" IS '通用OAuth2.0必需：令牌端点';
COMMENT ON COLUMN "sso_configs"."userinfo_url" IS '通用OAuth2.0必需：用户信息端点';
COMMENT ON COLUMN "sso_configs"."scopes" IS 'OIDC/OAuth2.0 请求的scope，逗号分隔';
COMMENT ON COLUMN "sso_configs"."external_id_field" IS 'OAuth2.0必需：从userinfo取哪个字段作唯一标识；OIDC固定用sub';
COMMENT ON COLUMN "sso_configs"."callback_key" IS '回调地址里的租户标识：随机生成一次后永久固定，回调路由按它反查admin_id，不由admin_id推导';
COMMENT ON COLUMN "sso_configs"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "sso_configs"."updated_at" IS '更新时间（Unix时间戳）';

-- 一个租户一份SSO配置。
CREATE UNIQUE INDEX IF NOT EXISTS uq_sso_configs_admin ON sso_configs(admin_id);
-- 回调按 key 单点定位租户；随机值不重叠，唯一索引兼作并发写入兜底。
CREATE UNIQUE INDEX IF NOT EXISTS uq_sso_configs_callback_key ON sso_configs(callback_key);

-- 成员账号同步配置：与SSO登录配置相互独立存储（需求明确要求）。
CREATE TABLE IF NOT EXISTS sso_sync_configs (
    id                SERIAL PRIMARY KEY,
    admin_id          INTEGER      NOT NULL,
    enabled           SMALLINT     NOT NULL DEFAULT 0,
    auto_sync_period  VARCHAR(20)  NOT NULL DEFAULT 'none',
    weekly_day        SMALLINT     NOT NULL DEFAULT 0,
    scim_base_url     VARCHAR(500) NOT NULL DEFAULT '',
    scim_token        VARCHAR(500) NOT NULL DEFAULT '',
    user_field_mapping TEXT,
    dept_field_mapping TEXT,
    last_synced_at    BIGINT       NOT NULL DEFAULT 0,
    last_synced_count INTEGER      NOT NULL DEFAULT 0,
    last_sync_status  VARCHAR(20)  NOT NULL DEFAULT '',
    last_failed_count INTEGER      NOT NULL DEFAULT 0,
    last_error        VARCHAR(500) NOT NULL DEFAULT '',
    created_at        BIGINT       NOT NULL DEFAULT 0,
    updated_at        BIGINT       NOT NULL DEFAULT 0
);

COMMENT ON TABLE "sso_sync_configs" IS '企业SSO成员账号同步配置表';
COMMENT ON COLUMN "sso_sync_configs"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "sso_sync_configs"."enabled" IS '同步开关: 0=关闭, 1=开启';
COMMENT ON COLUMN "sso_sync_configs"."auto_sync_period" IS '自动同步周期: none/hourly/daily/weekly；none时不参与定时调度';
COMMENT ON COLUMN "sso_sync_configs"."weekly_day" IS '周同步的执行日: 1=周一…7=周日，仅weekly使用';
COMMENT ON COLUMN "sso_sync_configs"."scim_base_url" IS 'SCIM Base URL，仅OIDC/OAuth2.0使用';
COMMENT ON COLUMN "sso_sync_configs"."scim_token" IS 'SCIM Bearer Token，明文落库、永不出参';
COMMENT ON COLUMN "sso_sync_configs"."user_field_mapping" IS '用户字段映射JSON（IdP字段→本地字段）';
COMMENT ON COLUMN "sso_sync_configs"."dept_field_mapping" IS '部门字段映射JSON（IdP字段→本地字段）';
COMMENT ON COLUMN "sso_sync_configs"."last_synced_at" IS '上次同步完成时间（Unix时间戳），0=从未同步';
COMMENT ON COLUMN "sso_sync_configs"."last_synced_count" IS '上次同步处理的成员数';
COMMENT ON COLUMN "sso_sync_configs"."last_sync_status" IS '上次同步结果: success=全部成功, partial=跑完但有失败, failed=整体失败；空=从未同步';
COMMENT ON COLUMN "sso_sync_configs"."last_failed_count" IS '上次同步写入失败的成员数，成功时为0';
COMMENT ON COLUMN "sso_sync_configs"."last_error" IS '上次同步失败原因，成功时清空';
COMMENT ON COLUMN "sso_sync_configs"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "sso_sync_configs"."updated_at" IS '更新时间（Unix时间戳）';

CREATE UNIQUE INDEX IF NOT EXISTS uq_sso_sync_configs_admin ON sso_sync_configs(admin_id);

-- SSO身份绑定表：登录与成员同步共用的身份映射，只存身份与展示资料，不存任何第三方令牌。
CREATE TABLE IF NOT EXISTS user_sso_bindings (
    id           SERIAL PRIMARY KEY,
    admin_id     INTEGER      NOT NULL,
    user_id      INTEGER      NOT NULL,
    provider     VARCHAR(20)  NOT NULL,
    app_key      VARCHAR(200) NOT NULL DEFAULT '',
    external_id  VARCHAR(200) NOT NULL,
    username     VARCHAR(255) NOT NULL DEFAULT '',
    email        VARCHAR(255) NOT NULL DEFAULT '',
    job_title    VARCHAR(100) NOT NULL DEFAULT '',
    employee_no  VARCHAR(100) NOT NULL DEFAULT '',
    nickname     VARCHAR(100) NOT NULL DEFAULT '',
    avatar       VARCHAR(500) NOT NULL DEFAULT '',
    created_at   BIGINT       NOT NULL DEFAULT 0,
    updated_at   BIGINT       NOT NULL DEFAULT 0
);

COMMENT ON TABLE "user_sso_bindings" IS '企业SSO身份绑定表（登录与成员同步共用的身份映射）';
COMMENT ON COLUMN "user_sso_bindings"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "user_sso_bindings"."user_id" IS '绑定的GoTeams成员ID';
COMMENT ON COLUMN "user_sso_bindings"."provider" IS '平台标识: oidc/oauth2/dingtalk/wecom/feishu';
COMMENT ON COLUMN "user_sso_bindings"."app_key" IS '建立绑定时所用应用标识；切换平台或换应用时整批清空绑定行';
COMMENT ON COLUMN "user_sso_bindings"."external_id" IS 'IdP侧唯一标识（OIDC sub、钉钉/企微userid、飞书open_id）；身份匹配唯一依据';
COMMENT ON COLUMN "user_sso_bindings"."username" IS 'IdP侧登录账号，仅存资料与展示用，不参与身份匹配';
COMMENT ON COLUMN "user_sso_bindings"."email" IS 'IdP侧邮箱，仅存资料与展示用，不参与身份匹配（邮箱可被IdP改写，按它认领等于账号接管路径）';
COMMENT ON COLUMN "user_sso_bindings"."job_title" IS 'IdP侧职位，仅存资料与展示用';
COMMENT ON COLUMN "user_sso_bindings"."employee_no" IS 'IdP侧工号，仅存资料与展示用';
COMMENT ON COLUMN "user_sso_bindings"."nickname" IS '展示用姓名';
COMMENT ON COLUMN "user_sso_bindings"."avatar" IS '展示用头像URL';
COMMENT ON COLUMN "user_sso_bindings"."created_at" IS '绑定时间（Unix时间戳）';
COMMENT ON COLUMN "user_sso_bindings"."updated_at" IS '更新时间（Unix时间戳）';

-- 一个IdP身份只对应一个成员；并发建号靠唯一索引兜底。
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_sso_bindings_identity ON user_sso_bindings(admin_id, provider, app_key, external_id);
-- 一个成员在一个应用下只有一个绑定。
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_sso_bindings_user_app ON user_sso_bindings(user_id, provider, app_key);
-- 按成员查绑定。
CREATE INDEX IF NOT EXISTS idx_user_sso_bindings_admin_user ON user_sso_bindings(admin_id, user_id);

-- 部门身份关联表：部门同步幂等与「上级部门」解析的唯一依据，因此不给 departments 扩列。
CREATE TABLE IF NOT EXISTS department_sso_bindings (
    id            SERIAL PRIMARY KEY,
    admin_id      INTEGER      NOT NULL,
    provider      VARCHAR(20)  NOT NULL,
    app_key       VARCHAR(200) NOT NULL DEFAULT '',
    department_id INTEGER      NOT NULL,
    external_id   VARCHAR(200) NOT NULL,
    created_at    BIGINT       NOT NULL DEFAULT 0,
    updated_at    BIGINT       NOT NULL DEFAULT 0
);

COMMENT ON TABLE "department_sso_bindings" IS '企业SSO部门身份关联表';
COMMENT ON COLUMN "department_sso_bindings"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "department_sso_bindings"."provider" IS '平台标识';
COMMENT ON COLUMN "department_sso_bindings"."app_key" IS '建立绑定时所用应用标识；切换平台时整批清空';
COMMENT ON COLUMN "department_sso_bindings"."department_id" IS '本地部门ID';
COMMENT ON COLUMN "department_sso_bindings"."external_id" IS 'IdP侧部门标识';
COMMENT ON COLUMN "department_sso_bindings"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "department_sso_bindings"."updated_at" IS '更新时间（Unix时间戳）';

-- 一个IdP部门只对应一个本地部门。
CREATE UNIQUE INDEX IF NOT EXISTS uq_department_sso_bindings_identity ON department_sso_bindings(admin_id, provider, app_key, external_id);
-- 一个本地部门在一个应用下只有一个上游。
CREATE UNIQUE INDEX IF NOT EXISTS uq_department_sso_bindings_dept_app ON department_sso_bindings(department_id, provider, app_key);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS department_sso_bindings;
DROP TABLE IF EXISTS user_sso_bindings;
DROP TABLE IF EXISTS sso_sync_configs;
DROP TABLE IF EXISTS sso_configs;

-- +goose StatementEnd
