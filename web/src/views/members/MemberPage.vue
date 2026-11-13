<template>
  <div class="member-page">
    <a-tabs v-model:activeKey="activeTab" class="work-tabs">
      <template #leftExtra>
        <span class="page-icon"><BankOutlined /></span>
        <h3 class="page-title">{{ $t("enterprise_management") }}</h3>
      </template>
      <a-tab-pane
        v-if="canViewVersionInfo"
        key="version"
        :tab="$t('plan_versionInfo')"
      >
        <div class="plan-panel">
          <div class="plan-card">
            <div class="plan-version-row">
              <div class="plan-card-main">
                <div class="plan-icon"><CrownOutlined /></div>
                <div>
                  <div class="plan-title">
                    {{ $t("plan_currentVersion") }}：
                    <span>{{ planName }}</span>
                  </div>
                  <div class="plan-description">
                    {{ planDescription }}
                  </div>
                </div>
              </div>
              <div class="plan-card-actions">
                <a-button
                  v-if="licenseStore.isCommercial && authStore.isSuperAdmin()"
                  class="upgrade-btn"
                  @click="licenseContactOpen = true"
                >
                  {{ $t("license_contact_service") }}
                </a-button>
                <a-button
                  v-if="licenseStore.isCommercial && authStore.isSuperAdmin()"
                  type="primary"
                  class="upgrade-btn"
                  @click="licenseUpdateOpen = true"
                >
                  {{ $t("license_update_entry") }}
                </a-button>
                <a-button
                  v-if="!licenseStore.isCommercial"
                  class="upgrade-btn"
                  @click="contactOpen = true"
                >
                  {{ $t("plan_contactService") }}
                </a-button>
                <a-button
                  v-if="planUsage.plan_type !== 'enterprise'"
                  class="upgrade-btn"
                  @click="upgradeOpen = true"
                >
                  {{ $t("plan_upgradeVersion") }}
                </a-button>
              </div>
            </div>
          </div>

          <div class="usage-card">
            <div class="usage-card-title">{{ $t("plan_usageDetails") }}</div>
            <a-table
              :columns="usageColumns"
              :data-source="usageRows"
              :loading="planLoading"
              row-key="key"
              :pagination="false"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'usage'">
                  <div class="usage-progress">
                    <a-progress
                      :percent="usagePercent(record)"
                      :show-info="false"
                      :stroke-color="usageColor(record)"
                      :trail-color="'#f1f5f9'"
                      :size="[180, 8]"
                    />
                    <span :style="{ color: usageColor(record) }">
                      <template v-if="record.key === 'storage'">
                        {{ formatFileSize(record.used) }} / {{ record.limit }}G
                      </template>
                      <template v-else>
                        {{ record.used }} / {{ record.limit
                        }}{{ usageUnit(record) }}
                      </template>
                    </span>
                  </div>
                </template>
                <template v-else-if="column.key === 'enterprise'">
                  <span>{{ enterpriseLimitText(record) }}</span>
                </template>
              </template>
            </a-table>
          </div>
        </div>
      </a-tab-pane>

      <!-- Enterprise members -->
      <a-tab-pane
        v-if="canViewEnterpriseMembers"
        key="members"
        :tab="$t('plan_enterpriseMembers')"
      >
        <div class="members-layout">
          <div class="org-panel" :style="{ width: orgPanelWidth + 'px' }">
            <div class="org-panel-header">
              <span class="org-panel-title">{{
                $t("org_structureTitle")
              }}</span>
              <a-button
                v-if="canManageDepartments"
                type="text"
                size="small"
                class="org-add-button"
                :aria-label="$t('org_addTopDepartment')"
                @click="openCreateDepartment(0)"
              >
                <PlusOutlined />
              </a-button>
            </div>
            <div class="org-tree-wrap">
              <a-tree
                v-if="orgTreeData.length"
                v-model:expandedKeys="orgExpandedKeys"
                :tree-data="orgTreeData"
                :selected-keys="orgSelectedKeys"
                block-node
                @select="handleOrgSelect"
              >
                <template #title="{ title, key: nodeKey }">
                  <a-dropdown
                    v-if="nodeKey === ORG_ROOT_KEY"
                    :trigger="canRenameEnterprise ? ['contextmenu'] : []"
                  >
                    <span class="org-node org-node-root">
                      <span class="org-node-title">{{ title }}</span>
                      <span class="org-node-count">{{ members.length }}</span>
                    </span>
                    <template #overlay>
                      <a-menu @click="openEnterpriseRename">
                        <a-menu-item key="rename">
                          <EditOutlined /> {{ $t("enterprise_name_edit") }}
                        </a-menu-item>
                      </a-menu>
                    </template>
                  </a-dropdown>
                  <div v-else class="org-node">
                    <span class="org-node-title">{{ title }}</span>
                    <span class="org-node-count">{{
                      departmentMemberCounts[nodeKey] || 0
                    }}</span>
                    <a-dropdown
                      v-if="canManageDepartments"
                      :trigger="['click']"
                    >
                      <span
                        class="org-node-more"
                        :aria-label="$t('common_more')"
                        @click.stop
                      >
                        <MoreOutlined />
                      </span>
                      <template #overlay>
                        <a-menu
                          @click="
                            ({ key }) => handleDeptMenuClick(key, nodeKey)
                          "
                        >
                          <a-menu-item key="addChild">
                            <PlusOutlined /> {{ $t("org_addSubDepartment") }}
                          </a-menu-item>
                          <a-menu-item key="rename">
                            <EditOutlined /> {{ $t("org_renameDepartment") }}
                          </a-menu-item>
                          <a-menu-item key="delete">
                            <DeleteOutlined /> {{ $t("common_delete") }}
                          </a-menu-item>
                        </a-menu>
                      </template>
                    </a-dropdown>
                  </div>
                </template>
              </a-tree>
            </div>
          </div>
          <div
            class="org-resizer"
            :class="{ 'org-resizer--active': orgResizing }"
            role="separator"
            aria-orientation="vertical"
            @mousedown.prevent="startOrgResize"
          ></div>
          <div class="members-main">
            <div class="page-header">
              <div class="section-title">
                <span>{{ $t("workspace_enterpriseMembers") }}</span>
                <span class="member-count">{{ members.length }}</span>
                <a-tooltip :title="$t('member_logTitle')">
                  <a-button
                    type="text"
                    class="member-log-button"
                    :aria-label="$t('member_logTitle')"
                    @click="openMemberLogModal"
                  >
                    <FileTextOutlined />
                  </a-button>
                </a-tooltip>
              </div>
              <div class="header-actions">
                <a-input
                  v-model:value="searchKeyword"
                  allow-clear
                  autocomplete="off"
                  name="member-list-search"
                  class="member-search"
                  :placeholder="$t('workspace_searchMember')"
                >
                  <template #prefix><SearchOutlined /></template>
                </a-input>
                <a-popover
                  v-if="activeInvite"
                  trigger="hover"
                  placement="bottomRight"
                  @open-change="handleInvitePopoverOpen"
                >
                  <template #content>
                    <div class="invite-popover">
                      <div class="invite-progress-row">
                        <span>{{ $t("invite_progress") }}</span>
                        <span class="invite-progress-num"
                          >{{ activeInvite.accepted }}/{{
                            activeInvite.limit
                          }}</span
                        >
                      </div>
                      <a-input
                        :value="inviteLink"
                        readonly
                        class="invite-link-input"
                      />
                      <div class="invite-popover-actions">
                        <a-button size="small" @click="copyInviteLink">{{
                          $t("invite_copyLink")
                        }}</a-button>
                        <a-popconfirm
                          :title="$t('invite_stopConfirm')"
                          :ok-text="$t('common_confirm')"
                          :cancel-text="$t('common_cancel')"
                          @confirm="handleStopInvite"
                        >
                          <a-button size="small" danger>{{
                            $t("invite_stop")
                          }}</a-button>
                        </a-popconfirm>
                      </div>
                    </div>
                  </template>
                  <span class="invite-active-tag">
                    <span class="invite-pulse-dot"></span>
                    {{ $t("invite_activeLabel") }}
                    <span class="invite-countdown">{{
                      inviteCountdownText
                    }}</span>
                  </span>
                </a-popover>
                <a-button
                  v-else-if="authStore.hasPermission('member:invite')"
                  class="link-invite-button"
                  @click="openLinkInviteModal"
                >
                  <MailOutlined /> {{ $t("invite_linkInvite") }}
                </a-button>
                <a-dropdown :trigger="['click']">
                  <a-button class="batch-actions-button">
                    {{ $t("member_batchActions") }}
                    <DownOutlined class="batch-actions-arrow" />
                  </a-button>
                  <template #overlay>
                    <a-menu @click="handleBatchAction">
                      <a-menu-item
                        v-if="authStore.hasPermission('member:invite')"
                        key="import"
                      >
                        <ImportOutlined /> {{ $t("common_import") }}
                      </a-menu-item>
                      <a-menu-item key="export">
                        <ExportOutlined /> {{ $t("common_export") }}
                      </a-menu-item>
                    </a-menu>
                  </template>
                </a-dropdown>
                <a-button
                  v-if="authStore.hasPermission('member:invite')"
                  type="primary"
                  class="add-member-button"
                  @click="openInviteModal"
                >
                  <PlusOutlined /> {{ $t("workspace_addMember") }}
                </a-button>
              </div>
            </div>

            <div class="member-table-card">
              <a-table
                :columns="memberColumns"
                :data-source="filteredMembers"
                :loading="loading"
                row-key="id"
                :pagination="false"
                :scroll="{ x: 1000 }"
              >
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'user'">
                    <div class="user-cell">
                      <a-avatar
                        :size="40"
                        :src="getAvatarSrc(record) || undefined"
                        :style="
                          getAvatarSrc(record)
                            ? avatarImgStyle
                            : avatarFallbackStyle
                        "
                        class="member-avatar"
                      >
                        {{
                          getAvatarSrc(record) ? "" : getInitial(record.name)
                        }}
                      </a-avatar>
                      <div class="user-info">
                        <div class="user-name">
                          {{ record.name }}
                          <a-tag
                            v-if="isMemberDisabled(record)"
                            class="member-disabled-tag"
                          >
                            {{ $t("member_disabledTag") }}
                          </a-tag>
                        </div>
                        <div class="user-username">{{ record.username }}</div>
                      </div>
                    </div>
                  </template>
                  <template v-else-if="column.key === 'phone'">
                    <span class="member-contact">{{
                      record.phone || "-"
                    }}</span>
                  </template>
                  <template v-else-if="column.key === 'email'">
                    <a-tooltip :title="record.email || undefined">
                      <span class="member-email">{{
                        record.email || "-"
                      }}</span>
                    </a-tooltip>
                  </template>
                  <template v-else-if="column.key === 'role'">
                    <a-tag
                      :class="[
                        'role-tag',
                        { 'role-tag-admin': isAdminRole(record.role_id) },
                      ]"
                    >
                      {{ getRoleName(record.role_id) }}
                    </a-tag>
                  </template>
                  <template v-else-if="column.key === 'department'">
                    <a-tooltip
                      :title="memberDepartmentNames(record) || undefined"
                    >
                      <span class="member-department">{{
                        memberDepartmentNames(record) || "-"
                      }}</span>
                    </a-tooltip>
                  </template>
                  <template v-else-if="column.key === 'created_at'">
                    <span class="join-time">{{
                      formatJoinTime(record.created_at)
                    }}</span>
                    <a-tag
                      v-if="record.invited_by_link"
                      class="invite-link-tag"
                      >{{ $t("invite_linkTag") }}</a-tag
                    >
                  </template>
                  <template v-else-if="column.key === 'action'">
                    <a-space :size="12" class="action-links">
                      <a-tooltip
                        :title="
                          isTenantSuperAdmin(record)
                            ? $t('member_statusAdminLocked')
                            : undefined
                        "
                      >
                        <a-switch
                          v-if="canSeeStatusSwitch"
                          :checked="!isMemberDisabled(record)"
                          :disabled="!canToggleStatus(record)"
                          :loading="statusUpdatingId === record.id"
                          size="small"
                          @change="handleToggleStatus(record, $event)"
                        />
                      </a-tooltip>
                      <a-dropdown
                        v-if="
                          canEditMember(record) ||
                          canResetPassword(record) ||
                          canRemoveMember(record) ||
                          canTransferOwner(record)
                        "
                        :trigger="['click']"
                      >
                        <span
                          class="action-more"
                          :aria-label="$t('common_more')"
                        >
                          <MoreOutlined />
                        </span>
                        <template #overlay>
                          <a-menu
                            @click="
                              ({ key }) => handleMemberAction(key, record)
                            "
                          >
                            <a-menu-item
                              v-if="canEditMember(record)"
                              key="edit"
                            >
                              {{ $t("member_editInfo") }}
                            </a-menu-item>
                            <a-menu-item
                              v-if="canResetPassword(record)"
                              key="resetPwd"
                            >
                              {{ $t("workspace_resetPwd") }}
                            </a-menu-item>
                            <a-menu-item
                              v-if="canRemoveMember(record)"
                              key="remove"
                            >
                              {{ $t("common_remove") }}
                            </a-menu-item>
                            <a-menu-item
                              v-if="canTransferOwner(record)"
                              key="transfer"
                            >
                              {{ $t("owner_transfer") }}
                            </a-menu-item>
                          </a-menu>
                        </template>
                      </a-dropdown>
                    </a-space>
                  </template>
                </template>
              </a-table>
            </div>
          </div>
        </div>
      </a-tab-pane>

      <!-- Enterprise roles -->
      <a-tab-pane
        v-if="canViewEnterpriseRoleList"
        key="roles"
        :tab="$t('plan_enterpriseRoles')"
      >
        <ProjectRolesTab />
      </a-tab-pane>
      <!-- Project management -->
      <a-tab-pane
        v-if="canManageProjects"
        key="projects"
        :tab="$t('project_manage_title')"
      >
        <ProjectManageTab :active="activeTab === 'projects'" />
      </a-tab-pane>
      <!-- External system notification strategies -->
      <a-tab-pane
        v-if="canViewExternalNotification"
        key="external-notification"
        :tab="$t('external_notification_tab')"
      >
        <ExternalNotificationTab
          :active="activeTab === 'external-notification'"
        />
      </a-tab-pane>
      <!-- Storage space -->
      <a-tab-pane
        v-if="canViewStorage"
        key="storage"
        :tab="$t('storage_title')"
      >
        <StoragePage />
      </a-tab-pane>
      <!-- Enterprise SSO -->
      <a-tab-pane v-if="canManageSso" key="sso" :tab="$t('sso_tab_title')">
        <SsoTab />
      </a-tab-pane>
    </a-tabs>

    <!-- Upgrade plan dialog -->
    <UpgradeRenewalModal v-model:open="upgradeOpen" />
    <UpgradeRenewalModal
      v-model:open="contactOpen"
      :title="$t('plan_contactService')"
    />
    <MemberLogModal v-model:open="memberLogOpen" />

    <!-- Commercial license update / customer service dialogs -->
    <LicenseUpdateModal
      v-if="authStore.isSuperAdmin()"
      v-model:open="licenseUpdateOpen"
      @success="onLicenseUpdated"
    />
    <ContactServiceModal v-model:open="licenseContactOpen" />

    <!-- Add/Edit Member Dialog -->
    <a-modal
      v-model:open="inviteOpen"
      :title="$t(editingMember ? 'member_editInfo' : 'workspace_addMember')"
      width="520px"
      wrap-class-name="invite-member-modal"
      :destroy-on-close="true"
      @cancel="handleInviteCancel"
    >
      <div class="invite-avatar-section" v-if="false">
        <div class="invite-avatar-wrap">
          <a-avatar
            :size="88"
            :src="inviteAvatarPreview || undefined"
            class="invite-avatar"
          >
            {{ inviteAvatarPreview ? "" : getInitial(inviteForm.name) }}
          </a-avatar>
          <a-upload
            :show-upload-list="false"
            accept="image/png,image/jpeg"
            :before-upload="validateInviteAvatar"
            :custom-request="handleInviteAvatarUpload"
            :disabled="inviteAvatarUploading"
          >
            <button
              type="button"
              class="invite-avatar-camera"
              :aria-label="$t('member_avatarUpload')"
            >
              <LoadingOutlined v-if="inviteAvatarUploading" />
              <CameraOutlined v-else />
            </button>
          </a-upload>
        </div>
        <p class="invite-avatar-hint">{{ $t("member_avatarHint") }}</p>
      </div>

      <a-form
        ref="inviteFormRef"
        layout="vertical"
        class="invite-member-form"
        :model="inviteForm"
        :rules="inviteRules"
      >
        <a-form-item :label="$t('member_name')" name="name">
          <a-input
            v-model:value="inviteForm.name"
            :maxlength="100"
            :placeholder="$t('member_namePlaceholder')"
          />
        </a-form-item>
        <a-form-item :label="$t('member_account')" name="username">
          <a-input
            v-model:value="inviteForm.username"
            :disabled="editingTenantSuperAdmin"
            :maxlength="255"
            :placeholder="$t('member_accountPlaceholder')"
          />
        </a-form-item>
        <a-form-item :label="$t('member_role')" name="roleId">
          <a-select
            v-model:value="inviteForm.roleId"
            :disabled="editingTenantSuperAdmin"
            :placeholder="$t('member_rolePlaceholder')"
          >
            <a-select-option
              v-for="r in enterpriseRoles"
              :key="r.id"
              :value="Number(r.id)"
            >
              {{ getEnterpriseRoleDisplayName(r.name) }}
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="$t('member_phone')" name="phone">
          <a-input
            v-model:value="inviteForm.phone"
            :maxlength="20"
            :placeholder="$t('member_phonePlaceholder')"
          />
        </a-form-item>
        <a-form-item :label="$t('member_email')" name="email">
          <a-input
            v-model:value="inviteForm.email"
            :maxlength="255"
            :placeholder="$t('member_emailPlaceholder')"
          />
        </a-form-item>
        <a-form-item :label="$t('member_department')">
          <a-tree-select
            v-model:value="inviteForm.departmentIds"
            :tree-data="departmentSelectTree"
            multiple
            allow-clear
            tree-default-expand-all
            popup-class-name="member-department-tree-dropdown"
            :placeholder="$t('member_departmentPlaceholder')"
            :get-popup-container="(node) => node.parentNode"
          />
        </a-form-item>
      </a-form>

      <template #footer>
        <a-button @click="handleInviteCancel">{{
          $t("common_cancel")
        }}</a-button>
        <a-button
          type="primary"
          :loading="inviting"
          :disabled="inviteAvatarUploading"
          @click="handleInvite"
        >
          {{ $t(editingMember ? "common_save" : "common_add") }}
        </a-button>
      </template>
    </a-modal>

    <!-- Link invite: generate invitation modal -->
    <a-modal
      v-model:open="linkInviteOpen"
      :title="$t('invite_linkInvite')"
      width="420px"
      :destroy-on-close="true"
    >
      <a-form layout="vertical" class="link-invite-form">
        <a-form-item :label="$t('invite_limitLabel')" required>
          <a-input-number
            v-model:value="linkInviteForm.limit"
            :min="1"
            :max="9999"
            :precision="0"
            style="width: 100%"
          />
        </a-form-item>
        <a-form-item :label="$t('invite_expireLabel')" required>
          <a-input-number
            v-model:value="linkInviteForm.expireMinutes"
            :min="1"
            :max="10080"
            :precision="0"
            style="width: 100%"
          />
        </a-form-item>
        <a-form-item :label="$t('invite_phoneRequiredLabel')" required>
          <a-radio-group v-model:value="linkInviteForm.phoneRequired">
            <a-radio :value="true">{{ $t("invite_phoneRequiredYes") }}</a-radio>
            <a-radio :value="false">{{ $t("invite_phoneRequiredNo") }}</a-radio>
          </a-radio-group>
        </a-form-item>
        <a-form-item :label="$t('invite_emailRequiredLabel')" required>
          <a-radio-group v-model:value="linkInviteForm.emailRequired">
            <a-radio :value="true">{{ $t("invite_emailRequiredYes") }}</a-radio>
            <a-radio :value="false">{{ $t("invite_emailRequiredNo") }}</a-radio>
          </a-radio-group>
        </a-form-item>
        <a-form-item :label="$t('invite_roleLabel')" required>
          <a-select
            v-model:value="linkInviteForm.roleId"
            :placeholder="$t('member_rolePlaceholder')"
          >
            <a-select-option
              v-for="r in enterpriseRoles"
              :key="r.id"
              :value="Number(r.id)"
            >
              {{ getEnterpriseRoleDisplayName(r.name) }}
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="$t('member_department')">
          <a-tree-select
            v-model:value="linkInviteForm.departmentIds"
            :tree-data="departmentSelectTree"
            multiple
            allow-clear
            tree-default-expand-all
            popup-class-name="member-department-tree-dropdown"
            :placeholder="$t('member_departmentPlaceholder')"
            :get-popup-container="(node) => node.parentNode"
          />
        </a-form-item>
      </a-form>
      <template #footer>
        <a-button @click="linkInviteOpen = false">{{
          $t("common_cancel")
        }}</a-button>
        <a-button
          type="primary"
          :loading="linkInviteCreating"
          @click="handleGenerateInvite"
        >
          {{ $t("invite_generateLink") }}
        </a-button>
      </template>
    </a-modal>

    <!-- Link invite: result modal -->
    <a-modal
      v-model:open="linkResultOpen"
      :title="$t('invite_linkTitle')"
      width="480px"
      :footer="null"
      :destroy-on-close="true"
    >
      <p class="invite-result-tip">{{ inviteMessage }}</p>
      <div class="invite-result-link-row">
        <a
          :href="inviteLink"
          target="_blank"
          rel="noopener"
          class="invite-result-link"
          >{{ inviteLink }}</a
        >
        <a-button type="primary" size="small" @click="copyInviteLink">{{
          $t("invite_copy")
        }}</a-button>
      </div>
    </a-modal>

    <!-- Reset Password Dialog -->
    <a-modal
      v-model:open="resetPwdOpen"
      :title="$t('workspace_resetPwd')"
      @ok="handleResetPwdConfirm"
      :ok-text="$t('common_confirm')"
      :cancel-text="$t('common_cancel')"
      :destroy-on-close="true"
      @cancel="handleResetPwdCancel"
      @after-close="restoreSearchAfterReset"
    >
      <p>{{ $t("workspace_resetPwdFor", { name: resetPwdTarget?.name }) }}</p>
      <input
        class="password-autofill-anchor"
        name="username"
        type="text"
        autocomplete="username"
        :value="resetPwdTarget?.username || ''"
        tabindex="-1"
        aria-hidden="true"
        readonly
      />
      <a-input-password
        v-model:value="resetPwdValue"
        name="new-password"
        autocomplete="new-password"
        :placeholder="$t('workspace_newPwdPlaceholder')"
        style="margin-bottom: 8px"
      />
      <a-button size="small" @click="generatePwd">{{
        $t("workspace_regeneratePwd")
      }}</a-button>
    </a-modal>

    <!-- Department create/rename dialog -->
    <a-modal
      v-model:open="deptDialog.open"
      :title="deptDialogTitle"
      width="420px"
      :confirm-loading="deptDialog.saving"
      :ok-text="$t('common_confirm')"
      :cancel-text="$t('common_cancel')"
      :destroy-on-close="true"
      @ok="submitDeptDialog"
    >
      <a-input
        v-model:value="deptDialog.name"
        :maxlength="50"
        :placeholder="$t('org_departmentNamePlaceholder')"
        @press-enter="submitDeptDialog"
      />
    </a-modal>

    <!-- Enterprise rename dialog (org tree root) -->
    <a-modal
      v-model:open="enterpriseRename.open"
      :title="$t('enterprise_name_edit')"
      width="420px"
      :confirm-loading="enterpriseRename.saving"
      :ok-text="$t('common_confirm')"
      :cancel-text="$t('common_cancel')"
      :destroy-on-close="true"
      @ok="submitEnterpriseRename"
    >
      <a-input
        v-model:value="enterpriseRename.name"
        :maxlength="100"
        :placeholder="$t('enterprise_name_required')"
        @press-enter="submitEnterpriseRename"
      />
    </a-modal>

    <!-- Member added successfully dialog -->
    <a-modal
      v-model:open="addSuccess.open"
      :title="$t('member_addSuccessTitle')"
      width="480px"
      :destroy-on-close="true"
      :mask-closable="false"
    >
      <p class="add-success-tip">{{ $t("member_addSuccessTip") }}</p>
      <div class="add-success-rows">
        <div>
          {{ $t("member_addSuccessAddress") }}：{{ addSuccess.address }}
        </div>
        <div>
          {{ $t("member_addSuccessAccount") }}：{{ addSuccess.username }}
        </div>
        <div>
          {{ $t("member_addSuccessPassword") }}：{{ addSuccess.password }}
        </div>
      </div>
      <template #footer>
        <a-button @click="copyAddSuccessInfo">
          <CopyOutlined /> {{ $t("member_copyAccountPassword") }}
        </a-button>
        <a-button type="primary" @click="addSuccess.open = false">{{
          $t("common_done")
        }}</a-button>
      </template>
    </a-modal>

    <!-- Member Excel import dialog (mirrors the work-item import interaction) -->
    <ImportWorkItemsModal
      v-model:open="memberImportOpen"
      :ws-id="0"
      type-key="member"
      :template-api="downloadEnterpriseMemberImportTemplate"
      :preview-api="previewEnterpriseMemberImport"
      :import-api="importEnterpriseMembers"
      :tip-suffix="$t('member_importTipSuffix')"
      @imported="handleMemberImported"
    />

    <!-- Transfer the default system administrator role (owner only) -->
    <a-modal
      v-model:open="transferOpen"
      :title="$t('owner_transferTenantTitle')"
      :confirm-loading="transferSubmitting"
      :ok-text="$t('common_confirm')"
      :cancel-text="$t('common_cancel')"
      @ok="submitTransferOwner"
    >
      <p class="transfer-tip">{{ $t("owner_transferTenantTip") }}</p>
      <p class="transfer-tip">{{ $t("owner_transferTenantDemoteTip") }}</p>
      <div class="transfer-row">
        <span class="transfer-label">{{ $t("owner_transferTo") }}</span>
        <a-select
          v-model:value="transferTargetId"
          class="transfer-select"
          show-search
          option-filter-prop="label"
          :placeholder="$t('owner_transferSelectPlaceholder')"
          :options="transferCandidates"
        />
      </div>
    </a-modal>

    <!-- Remove a member, handing over the projects they run and optionally their unfinished items -->
    <a-modal
      v-model:open="removeOpen"
      :title="$t('workspace_confirmRemoveMember')"
      :confirm-loading="removeSubmitting"
      :ok-text="$t('common_remove')"
      :ok-button-props="{ danger: true }"
      :cancel-text="$t('common_cancel')"
      @ok="submitRemoveMember"
    >
      <p class="transfer-tip">{{ $t("member_removeMemberDesc") }}</p>
      <a-spin :spinning="removePreviewLoading">
        <div v-if="removeOwnedWorkspaces.length" class="transfer-row">
          <a-checkbox :checked="true" disabled>
            {{ $t("owner_transferProjectAdminRole") }}
          </a-checkbox>
          <span class="transfer-label">{{ $t("owner_transferTo") }}</span>
          <a-select
            v-model:value="removeOwnerTargetId"
            class="transfer-select"
            show-search
            option-filter-prop="label"
            :placeholder="$t('owner_transferSelectPlaceholder')"
            :options="removeCandidates"
          />
        </div>
        <p v-if="removeOwnedWorkspaces.length" class="transfer-hint">
          {{
            $t("owner_affectedProjects", {
              names: removeOwnedWorkspaces.map((w) => w.name).join("、"),
            })
          }}
        </p>
        <div v-if="removeUnfinishedCount > 0" class="transfer-row">
          <a-checkbox v-model:checked="removeTransferItems">
            {{
              $t("owner_transferUnfinishedItems", {
                n: removeUnfinishedCount,
              })
            }}
          </a-checkbox>
          <span class="transfer-label">{{ $t("owner_transferTo") }}</span>
          <a-select
            v-model:value="removeItemsTargetId"
            class="transfer-select"
            show-search
            option-filter-prop="label"
            :disabled="!removeTransferItems"
            :placeholder="$t('owner_transferSelectPlaceholder')"
            :options="removeCandidates"
          />
        </div>
      </a-spin>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch, watchEffect } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import { message, Modal } from "ant-design-vue";
import {
  BankOutlined,
  CameraOutlined,
  CopyOutlined,
  CrownOutlined,
  DeleteOutlined,
  DownOutlined,
  EditOutlined,
  ExportOutlined,
  FileTextOutlined,
  ImportOutlined,
  KeyOutlined,
  LoadingOutlined,
  MailOutlined,
  MoreOutlined,
  PlusOutlined,
  SearchOutlined,
} from "@ant-design/icons-vue";
import {
  listEnterpriseMembers,
  createEnterpriseMember,
  uploadEnterpriseMemberAvatar,
  listEnterpriseRoles,
  updateEnterpriseMember,
  removeEnterpriseMember,
  previewEnterpriseMemberRemoval,
  transferTenantOwner,
  resetEnterpriseMemberPassword,
  updateEnterpriseMemberStatus,
  getEnterprisePlanUsage,
  updateEnterpriseName,
  listEnterpriseDepartments,
  createEnterpriseDepartment,
  renameEnterpriseDepartment,
  deleteEnterpriseDepartment,
  exportEnterpriseMembers,
  importEnterpriseMembers,
  previewEnterpriseMemberImport,
  downloadEnterpriseMemberImportTemplate,
} from "@/api/workspace";
import {
  createMemberInvitation,
  getMemberInvitation,
  stopMemberInvitation,
} from "@/api/invite";
import { useAuthStore } from "@/stores/auth";
import { useWorkspaceStore } from "@/stores/workspace";
import { formatLocalDate } from "@/utils/date";
import { getAvatarSrc, getInitial } from "@/utils/avatar";
import ProjectRolesTab from "../workspace/settings/ProjectRolesTab.vue";
import ProjectManageTab from "./components/ProjectManageTab.vue";
import ImportWorkItemsModal from "@/components/work-item/ImportWorkItemsModal.vue";
import StoragePage from "./StoragePage.vue";
import ExternalNotificationTab from "./components/ExternalNotificationTab.vue";
import SsoTab from "./components/SsoTab.vue";
import UpgradeRenewalModal from "@/components/common/UpgradeRenewalModal.vue";
import MemberLogModal from "@/components/common/MemberLogModal.vue";
import LicenseUpdateModal from "@/components/license/LicenseUpdateModal.vue";
import ContactServiceModal from "@/components/license/ContactServiceModal.vue";
import { useLicenseStore } from "@/stores/license";

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const workspaceStore = useWorkspaceStore();
const licenseStore = useLicenseStore();

const licenseUpdateOpen = ref(false);
const licenseContactOpen = ref(false);

// 安装新授权后刷新授权快照与套餐用量（配额可能随新 Key 变化）。
async function onLicenseUpdated() {
  await licenseStore.refreshAfterInstall();
  if (canViewVersionInfo.value) {
    planLoading.value = true;
    try {
      planUsage.value = (await getEnterprisePlanUsage()) || planUsage.value;
    } finally {
      planLoading.value = false;
    }
  }
}

const TAB_KEYS = [
  "personalization",
  "version",
  "members",
  "roles",
  "projects",
  "external-notification",
  "storage",
  "sso",
];
const activeTab = ref(
  TAB_KEYS.includes(route.query.tab) ? route.query.tab : "version",
);
const members = ref([]);
const statusUpdatingId = ref(null);
const memberLogOpen = ref(false);
const planLoading = ref(false);
const planUsage = ref({
  plan_type: "experience",
  expire_at: 0,
  items: [],
});
const upgradeOpen = ref(false);
const contactOpen = ref(false);
const enterpriseRoles = ref([]);
const loading = ref(false);
const searchKeyword = ref("");

// ======================== Organization structure ========================
const ORG_ROOT_KEY = "root";
const ORG_PANEL_WIDTH_KEY = "member_org_panel_width";
const ORG_PANEL_MIN_WIDTH = 180;
const ORG_PANEL_MAX_WIDTH = 480;
const departments = ref([]);
const enterpriseName = ref("");
const orgSelectedKeys = ref([ORG_ROOT_KEY]);
const orgExpandedKeys = ref([ORG_ROOT_KEY]);
const orgPanelWidth = ref(loadOrgPanelWidth());
const orgResizing = ref(false);
const deptDialog = ref({
  open: false,
  mode: "create",
  parentId: 0,
  deptId: 0,
  name: "",
  saving: false,
});
const enterpriseRename = ref({ open: false, name: "", saving: false });
const addSuccess = ref({
  open: false,
  address: "",
  username: "",
  password: "",
});
const memberImportOpen = ref(false);

const inviteOpen = ref(false);
const editingMember = ref(null);
const editingTenantSuperAdmin = computed(() =>
  isTenantSuperAdmin(editingMember.value),
);
const inviting = ref(false);
const inviteForm = ref(createInviteForm());
const inviteFormRef = ref();
const inviteAvatarPreview = ref("");
const inviteAvatarPath = ref("");
const inviteAvatarUploading = ref(false);
let inviteAvatarUploadVersion = 0;

// Email stays optional, but a non-empty value must be a legal address; the error renders under
// the field like the other form items.
const inviteRules = computed(() => ({
  name: [
    {
      required: true,
      whitespace: true,
      message: t("member_namePlaceholder"),
      trigger: "blur",
    },
  ],
  username: [
    {
      required: true,
      whitespace: true,
      message: t("member_accountPlaceholder"),
      trigger: "blur",
    },
  ],
  email: [
    { type: "email", message: t("member_emailInvalid"), trigger: "blur" },
  ],
  roleId: [
    { required: true, message: t("member_rolePlaceholder"), trigger: "change" },
  ],
}));

const resetPwdOpen = ref(false);
const resetPwdTarget = ref(null);
const resetPwdValue = ref("");
const searchKeywordBeforeReset = ref("");

// ======================== Link invitation ========================
const linkInviteOpen = ref(false);
const linkInviteCreating = ref(false);
const linkInviteForm = ref({
  limit: 50,
  expireMinutes: 30,
  roleId: undefined,
  phoneRequired: undefined,
  emailRequired: undefined,
  departmentIds: [],
});
const linkResultOpen = ref(false);
// Current active invitation; null means no invitation in progress.
const activeInvite = ref(null);
const inviteCountdownText = ref("00:00");
let inviteCountdownTimer = null;

const inviteLink = computed(() =>
  activeInvite.value
    ? `${window.location.origin}/invite/m/${encodeURIComponent(activeInvite.value.token)}`
    : "",
);

const inviteMessage = computed(() =>
  t("invite_linkTip", {
    inviter: activeInvite.value?.inviterName || authStore.userInfo?.name || "",
    enterprise:
      activeInvite.value?.enterpriseName ||
      enterpriseName.value ||
      t("org_unnamedEnterprise"),
  }),
);

const licenseRestricted = computed(() => licenseStore.managementOnly);
const canViewVersionInfo = computed(
  () => licenseRestricted.value || authStore.hasPermission("plan:view"),
);
const canViewEnterpriseMembers = computed(
  () => !licenseRestricted.value && authStore.hasPermission("member:view"),
);
const canViewEnterpriseRoleList = computed(
  () =>
    !licenseRestricted.value &&
    authStore.hasPermission("member:view_role_list"),
);
const canViewStorage = computed(
  () => !licenseRestricted.value && authStore.hasPermission("storage:view"),
);
const canViewExternalNotification = computed(
  () =>
    !licenseRestricted.value &&
    authStore.hasPermission("external_notification:manage"),
);
const canManageSso = computed(() => authStore.hasPermission("sso:setting"));
const canManageProjects = computed(
  () =>
    !licenseRestricted.value &&
    authStore.hasEnterprisePermission("project:manage"),
);
const canManageDepartments = computed(() =>
  authStore.hasPermission("member:edit"),
);
const canRenameEnterprise = computed(() => authStore.isSuperAdmin());
const displayEnterpriseName = computed(
  () => enterpriseName.value || t("org_unnamedEnterprise"),
);
const departmentsById = computed(() => {
  const map = {};
  for (const item of departments.value) map[item.id] = item;
  return map;
});
const departmentChildren = computed(() => {
  const map = {};
  for (const item of departments.value) {
    (map[item.parent_id] ||= []).push(item);
  }
  return map;
});
const orgTreeData = computed(() => [
  {
    key: ORG_ROOT_KEY,
    title: displayEnterpriseName.value,
    children: buildOrgChildren("0"),
  },
]);
const departmentSelectTree = computed(() => buildSelectChildren("0"));
const selectedDepartmentId = computed(() => {
  const key = orgSelectedKeys.value[0];
  return key && key !== ORG_ROOT_KEY ? String(key) : "";
});
// Ids of the selected department plus all descendants; null means the enterprise root (no filter)
const selectedSubtreeIds = computed(() => {
  if (!selectedDepartmentId.value) return null;
  const ids = new Set();
  const walk = (id) => {
    ids.add(id);
    for (const child of departmentChildren.value[id] || []) walk(child.id);
  };
  walk(selectedDepartmentId.value);
  return ids;
});

// Member count per department node, subtree-inclusive and deduplicated
// (a member counts once toward a department and each of its ancestors).
const departmentMemberCounts = computed(() => {
  const counts = {};
  for (const member of members.value) {
    const nodes = new Set();
    for (const id of member.department_ids || []) {
      let current = String(id);
      while (current && current !== "0" && !nodes.has(current)) {
        nodes.add(current);
        current = departmentsById.value[current]?.parent_id;
      }
    }
    for (const node of nodes) counts[node] = (counts[node] || 0) + 1;
  }
  return counts;
});

function buildOrgChildren(parentId) {
  return (departmentChildren.value[parentId] || []).map((item) => ({
    key: item.id,
    title: item.name,
    children: buildOrgChildren(item.id),
  }));
}

function buildSelectChildren(parentId) {
  return (departmentChildren.value[parentId] || []).map((item) => ({
    value: Number(item.id),
    label: item.name,
    children: buildSelectChildren(item.id),
  }));
}
const visibleTabs = computed(() =>
  [
    canViewVersionInfo.value && "version",
    canViewEnterpriseMembers.value && "members",
    canViewEnterpriseRoleList.value && "roles",
    canManageProjects.value && "projects",
    canViewExternalNotification.value && "external-notification",
    canViewStorage.value && "storage",
    canManageSso.value && "sso",
  ].filter(Boolean),
);
const planName = computed(() =>
  licenseStore.isTrial
    ? t("plan_private_experience")
    : t("plan_private_enterprise"),
);
const planDescription = computed(() => {
  if (planUsage.value.plan_type !== "enterprise")
    return t("plan_trialDescription");
  if (!planUsage.value.expire_at) return t("plan_enterpriseActive");
  return t("plan_expireAt", {
    date: formatLocalDate(planUsage.value.expire_at),
  });
});
const usageRows = computed(() =>
  (planUsage.value.items || [])
    .filter((item) => item.key === "member" || item.key === "storage")
    .map((item) => ({
      ...item,
      label: usageLabel(item.key),
    })),
);
const usageColumns = computed(() => [
  {
    title: t("plan_featureName"),
    dataIndex: "label",
    key: "label",
    width: "28%",
  },
  { title: t("plan_usageProgress"), key: "usage", width: "42%" },
  ...(planUsage.value.plan_type !== "enterprise"
    ? [{ title: t("plan_enterprise"), key: "enterprise", align: "center" }]
    : []),
]);

const memberColumns = computed(() => [
  { title: t("member_member"), key: "user", width: 260 },
  { title: t("member_phone"), key: "phone", width: 150 },
  { title: t("member_email"), key: "email", width: 220 },
  { title: t("member_role"), key: "role", width: 130 },
  { title: t("member_department"), key: "department", width: 160 },
  {
    title: t("member_joinTime"),
    dataIndex: "created_at",
    key: "created_at",
    width: 150,
  },
  { title: t("member_action"), key: "action", width: 120, align: "right" },
]);

const filteredMembers = computed(() => {
  let list = members.value;
  const subtree = selectedSubtreeIds.value;
  if (subtree) {
    list = list.filter((member) =>
      (member.department_ids || []).some((id) => subtree.has(String(id))),
    );
  }
  const sourceKeyword = resetPwdOpen.value
    ? searchKeywordBeforeReset.value
    : searchKeyword.value;
  const keyword = sourceKeyword.trim().toLocaleLowerCase();
  if (!keyword) return list;
  return list.filter((member) =>
    [member.name, member.username].some((value) =>
      `${value || ""}`.toLocaleLowerCase().includes(keyword),
    ),
  );
});

onMounted(async () => {
  loading.value = true;
  planLoading.value = canViewVersionInfo.value;
  try {
    if (canViewExternalNotification.value)
      await workspaceStore.fetchWorkspaces();
    if (canViewVersionInfo.value) {
      planUsage.value = (await getEnterprisePlanUsage()) || planUsage.value;
    }
    if (!licenseRestricted.value) {
      await refreshEnterpriseRoles();
      if (canViewEnterpriseMembers.value) {
        await refreshDepartments();
        await refreshMembers();
      }
      if (authStore.hasPermission("member:invite")) {
        await refreshActiveInvite();
      }
    }
  } catch {
    /* error handled by interceptor */
  } finally {
    loading.value = false;
    planLoading.value = false;
  }
});

async function refreshEnterpriseRoles() {
  const roleList = await listEnterpriseRoles();
  enterpriseRoles.value = roleList || [];
}

// Roles created or deleted in the roles tab must show up in the member role selectors.
watch(activeTab, async (tab) => {
  if (tab !== "members" || !canViewEnterpriseMembers.value) return;
  try {
    await refreshEnterpriseRoles();
  } catch {
    /* error handled by interceptor */
  }
});

// Keep the active tab in the URL query so refresh stays on the current tab.
watch(activeTab, (tab) => {
  if (route.query.tab === tab) return;
  router.replace({ query: { ...route.query, tab } });
});

// The route is reused when only the query changes, so follow ?tab= back into the component.
watch(
  () => route.query.tab,
  (tab) => {
    if (TAB_KEYS.includes(tab) && tab !== activeTab.value) {
      activeTab.value = tab;
    }
  },
);

// The sidebar enterprise entry navigates here with addMember=1, so open the
// add-member modal right after landing on the members tab.
watch(
  () => route.query.addMember,
  async (addMember) => {
    if (addMember !== "1") return;
    if (!canViewEnterpriseMembers.value) return;
    activeTab.value = "members";
    if (authStore.hasPermission("member:invite")) {
      if (!enterpriseRoles.value.length) {
        try {
          await refreshEnterpriseRoles();
        } catch {
          /* error handled by interceptor */
        }
      }
      openInviteModal();
    }
    router.replace({ query: { ...route.query, addMember: undefined } });
  },
  { immediate: true },
);

// ======================== Organization: tree data & actions ========================

function loadOrgPanelWidth() {
  try {
    const stored = Number(localStorage.getItem(ORG_PANEL_WIDTH_KEY));
    if (stored >= ORG_PANEL_MIN_WIDTH && stored <= ORG_PANEL_MAX_WIDTH) {
      return stored;
    }
  } catch {
    /* localStorage unavailable */
  }
  return 240;
}

async function refreshDepartments() {
  try {
    const data = await listEnterpriseDepartments();
    enterpriseName.value = data?.enterprise_name || "";
    departments.value = (data?.items || []).map((item) => ({
      id: String(item.id),
      parent_id: String(item.parent_id || "0"),
      name: item.name,
    }));
    expandOrgNodes();
    if (
      selectedDepartmentId.value &&
      !departmentsById.value[selectedDepartmentId.value]
    ) {
      orgSelectedKeys.value = [ORG_ROOT_KEY];
    }
  } catch {
    /* handled by interceptor */
  }
}

// ponytail: fully re-expand on every refresh; switch to incremental expansion if preserving manual collapse matters
function expandOrgNodes() {
  orgExpandedKeys.value = [
    ORG_ROOT_KEY,
    ...departments.value.map((item) => item.id),
  ];
}

function handleOrgSelect(keys) {
  // Re-clicking the selected node makes antd deselect it; always keep one node selected
  if (keys.length) orgSelectedKeys.value = keys;
}

const deptDialogTitle = computed(() => {
  if (deptDialog.value.mode === "rename") return t("org_renameDepartment");
  return deptDialog.value.parentId
    ? t("org_addSubDepartment")
    : t("org_addTopDepartment");
});

function openCreateDepartment(parentId) {
  deptDialog.value = {
    open: true,
    mode: "create",
    parentId: Number(parentId) || 0,
    deptId: 0,
    name: "",
    saving: false,
  };
}

function handleDeptMenuClick(menuKey, nodeKey) {
  const dept = departmentsById.value[String(nodeKey)];
  if (!dept) return;
  if (menuKey === "addChild") {
    openCreateDepartment(dept.id);
  } else if (menuKey === "rename") {
    deptDialog.value = {
      open: true,
      mode: "rename",
      parentId: 0,
      deptId: Number(dept.id),
      name: dept.name,
      saving: false,
    };
  } else if (menuKey === "delete") {
    confirmDeleteDepartment(dept);
  }
}

function confirmDeleteDepartment(dept) {
  Modal.confirm({
    title: t("org_deleteConfirmTitle"),
    content: t("org_deleteConfirmContent", { name: dept.name }),
    okText: t("common_delete"),
    okType: "danger",
    cancelText: t("common_cancel"),
    async onOk() {
      try {
        await deleteEnterpriseDepartment(dept.id);
        await refreshDepartments();
        await refreshMembers();
      } catch {
        /* handled by interceptor */
      }
    },
  });
}

async function submitDeptDialog() {
  const dialog = deptDialog.value;
  const name = dialog.name.trim();
  if (!name) {
    message.warning(t("org_departmentNamePlaceholder"));
    return;
  }
  if (dialog.saving) return;
  dialog.saving = true;
  try {
    if (dialog.mode === "rename") {
      await renameEnterpriseDepartment(dialog.deptId, { name });
    } else {
      await createEnterpriseDepartment({ parentId: dialog.parentId, name });
    }
    dialog.open = false;
    await refreshDepartments();
  } catch {
    /* handled by interceptor */
  } finally {
    dialog.saving = false;
  }
}

function openEnterpriseRename() {
  enterpriseRename.value = {
    open: true,
    name: enterpriseName.value,
    saving: false,
  };
}

async function submitEnterpriseRename() {
  const state = enterpriseRename.value;
  const name = state.name.trim();
  if (!name) {
    message.error(t("enterprise_name_required"));
    return;
  }
  if (state.saving) return;
  state.saving = true;
  try {
    const result = await updateEnterpriseName({ enterprise_name: name });
    enterpriseName.value = result.enterprise_name;
    state.open = false;
    message.success(t("enterprise_name_updated"));
  } catch {
    /* handled by interceptor */
  } finally {
    state.saving = false;
  }
}

function memberDepartmentNames(record) {
  return (record.department_ids || [])
    .map((id) => departmentsById.value[String(id)]?.name)
    .filter(Boolean)
    .join(", ");
}

// ======================== Organization: panel width drag ========================

let orgResizeState = null;

function startOrgResize(event) {
  orgResizeState = { startX: event.clientX, startWidth: orgPanelWidth.value };
  orgResizing.value = true;
  document.addEventListener("mousemove", handleOrgResizeMove);
  document.addEventListener("mouseup", stopOrgResize);
  document.body.style.cursor = "col-resize";
  document.body.style.userSelect = "none";
}

function handleOrgResizeMove(event) {
  if (!orgResizeState) return;
  const width =
    orgResizeState.startWidth + event.clientX - orgResizeState.startX;
  orgPanelWidth.value = Math.min(
    ORG_PANEL_MAX_WIDTH,
    Math.max(ORG_PANEL_MIN_WIDTH, width),
  );
}

function stopOrgResize() {
  if (!orgResizeState) return;
  orgResizeState = null;
  orgResizing.value = false;
  document.removeEventListener("mousemove", handleOrgResizeMove);
  document.removeEventListener("mouseup", stopOrgResize);
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
  try {
    localStorage.setItem(ORG_PANEL_WIDTH_KEY, String(orgPanelWidth.value));
  } catch {
    /* localStorage unavailable */
  }
}

// ======================== Batch actions: import/export ========================

function handleBatchAction({ key }) {
  if (key === "export") {
    exportMembers();
  } else if (key === "import") {
    memberImportOpen.value = true;
  }
}

async function exportMembers() {
  try {
    const response = await exportEnterpriseMembers();
    const disposition = response.headers?.["content-disposition"] || "";
    const match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
    const filename = match
      ? decodeURIComponent(match[1])
      : `${t("workspace_enterpriseMembers")}.xlsx`;
    const url = URL.createObjectURL(response.data);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    link.click();
    URL.revokeObjectURL(url);
    message.success(t("workspace_exported"));
  } catch {
    /* handled by interceptor */
  }
}

function handleMemberImported() {
  // Import may auto-create missing departments, so both the tree and the list need a refresh
  refreshDepartments();
  refreshMembers();
}

onUnmounted(() => {
  avatarObjectUrls.forEach((url) => URL.revokeObjectURL(url));
  clearInterval(inviteCountdownTimer);
  stopOrgResize();
});

const avatarImgStyle = { backgroundColor: "transparent" };
const avatarFallbackStyle = { backgroundColor: "#0d68ff" };
const avatarObjectUrls = [];

async function refreshMembers() {
  members.value = (await listEnterpriseMembers()) || [];
}

// Join time shown to the minute (formatLocalDate includes seconds; take the first 16 chars = YYYY-MM-DD HH:mm).
function formatJoinTime(timestamp) {
  const text = formatLocalDate(timestamp, true);
  return text === "-" ? text : text.slice(0, 16);
}

// ======================== Link invitation ========================

function setActiveInvite(data) {
  if (!data || !data.token) {
    clearActiveInvite();
    return;
  }
  activeInvite.value = {
    token: data.token,
    limit: Number(data.invite_limit) || 0,
    accepted: Number(data.accepted_count) || 0,
    expiredAt: Number(data.expired_at) || 0,
    inviterName: data.inviter_name || "",
    enterpriseName: data.enterprise_name || "",
  };
  startInviteCountdown();
}

function clearActiveInvite() {
  activeInvite.value = null;
  clearInterval(inviteCountdownTimer);
  inviteCountdownTimer = null;
}

async function refreshActiveInvite() {
  try {
    setActiveInvite(await getMemberInvitation());
  } catch {
    /* handled by interceptor */
  }
}

function startInviteCountdown() {
  updateInviteCountdown();
  clearInterval(inviteCountdownTimer);
  inviteCountdownTimer = setInterval(updateInviteCountdown, 1000);
}

function updateInviteCountdown() {
  if (!activeInvite.value) return;
  const remain = activeInvite.value.expiredAt - Math.floor(Date.now() / 1000);
  if (remain <= 0) {
    // Countdown reached zero: auto-restore the initial state.
    clearActiveInvite();
    return;
  }
  const minutes = String(Math.floor(remain / 60)).padStart(2, "0");
  const seconds = String(remain % 60).padStart(2, "0");
  inviteCountdownText.value = `${minutes}:${seconds}`;
}

function openLinkInviteModal() {
  const memberRole = enterpriseRoles.value[1] || enterpriseRoles.value[0];
  linkInviteForm.value = {
    limit: 50,
    expireMinutes: 30,
    roleId: memberRole ? Number(memberRole.id) : undefined,
    phoneRequired: undefined,
    emailRequired: undefined,
    departmentIds: [],
  };
  linkInviteOpen.value = true;
}

async function handleGenerateInvite() {
  const {
    limit,
    expireMinutes,
    roleId,
    phoneRequired,
    emailRequired,
    departmentIds,
  } = linkInviteForm.value;
  // Both required flags must be explicitly chosen; linking is not allowed until then.
  if (
    !limit ||
    !expireMinutes ||
    !roleId ||
    phoneRequired === undefined ||
    emailRequired === undefined
  ) {
    message.warning(t("workspace_fillCompleteInfo"));
    return;
  }
  linkInviteCreating.value = true;
  try {
    const data = await createMemberInvitation({
      invite_limit: limit,
      expire_minutes: expireMinutes,
      role_id: roleId,
      phone_required: phoneRequired,
      email_required: emailRequired,
      department_ids: (departmentIds || []).map(Number),
    });
    setActiveInvite({ ...data, accepted_count: 0 });
    linkInviteOpen.value = false;
    linkResultOpen.value = true;
  } catch {
    /* handled by interceptor */
  } finally {
    linkInviteCreating.value = false;
  }
}

async function handleInvitePopoverOpen(open) {
  // Refresh the accepted count when hovering opens the popover.
  if (open) {
    await refreshActiveInvite();
    await refreshMembers();
  }
}

async function handleStopInvite() {
  try {
    await stopMemberInvitation();
    clearActiveInvite();
    message.success(t("invite_stopped"));
  } catch {
    /* handled by interceptor */
  }
}

// Copy text: navigator.clipboard is unavailable in an insecure HTTP context, fall back to a hidden textarea + execCommand.
async function copyText(text) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  textarea.select();
  try {
    if (!document.execCommand("copy"))
      throw new Error("execCommand copy failed");
  } finally {
    document.body.removeChild(textarea);
  }
}

async function copyInviteLink() {
  try {
    await copyText(`${inviteMessage.value}\n${inviteLink.value}`);
    message.success(t("invite_copied"));
  } catch {
    message.error(t("invite_copyFailed"));
  }
}

watchEffect(() => {
  if (!visibleTabs.value.includes(activeTab.value)) {
    activeTab.value = visibleTabs.value[0] || "members";
  }
});

function createInviteForm() {
  return {
    name: "",
    username: "",
    phone: "",
    email: "",
    roleId: undefined,
    departmentIds: [],
  };
}

function openInviteModal() {
  clearInviteForm();
  // Default to the plain member role (roles[0] is the admin role) and to the
  // department currently selected in the org tree, mirroring openLinkInviteModal.
  const memberRole = enterpriseRoles.value[1] || enterpriseRoles.value[0];
  if (memberRole) inviteForm.value.roleId = Number(memberRole.id);
  if (selectedDepartmentId.value) {
    inviteForm.value.departmentIds = [Number(selectedDepartmentId.value)];
  }
  inviteOpen.value = true;
}

function openEditModal(record) {
  clearInviteForm();
  editingMember.value = record;
  inviteForm.value = {
    name: record.name || "",
    username: record.username || "",
    phone: record.phone || "",
    email: record.email || "",
    roleId: Number(record.role_id),
    departmentIds: (record.department_ids || []).map(Number),
  };
  inviteAvatarPreview.value = getAvatarSrc(record) || "";
  inviteOpen.value = true;
}

function clearInviteAvatar() {
  inviteAvatarUploadVersion += 1;
  if (inviteAvatarPreview.value.startsWith("blob:")) {
    URL.revokeObjectURL(inviteAvatarPreview.value);
  }
  inviteAvatarPreview.value = "";
  inviteAvatarPath.value = "";
  inviteAvatarUploading.value = false;
}

function clearInviteForm() {
  inviteForm.value = createInviteForm();
  editingMember.value = null;
  clearInviteAvatar();
}

function handleInviteCancel() {
  inviteOpen.value = false;
  clearInviteForm();
}

function validateInviteAvatar(file) {
  const isImage = file.type === "image/jpeg" || file.type === "image/png";
  if (!isImage) {
    message.error(t("member_avatarInvalid"));
    return false;
  }
  if (file.size > 100 * 1024) {
    message.error(t("member_avatarTooLarge"));
    return false;
  }
  clearInviteAvatar();
  inviteAvatarPreview.value = URL.createObjectURL(file);
  return true;
}

async function handleInviteAvatarUpload({ file, onSuccess, onError }) {
  const uploadVersion = inviteAvatarUploadVersion;
  inviteAvatarUploading.value = true;
  try {
    const result = await uploadEnterpriseMemberAvatar(file);
    if (uploadVersion !== inviteAvatarUploadVersion) return;
    inviteAvatarPath.value = result?.url || "";
    if (!inviteAvatarPath.value)
      throw new Error("avatar upload returned no url");
    onSuccess(result);
  } catch (error) {
    if (uploadVersion === inviteAvatarUploadVersion) clearInviteAvatar();
    onError(error);
  } finally {
    if (uploadVersion === inviteAvatarUploadVersion) {
      inviteAvatarUploading.value = false;
    }
  }
}

function getRoleName(roleId) {
  const role = enterpriseRoles.value.find(
    (r) => Number(r.id) === Number(roleId),
  );
  if (!role) return t("member_member");
  return getEnterpriseRoleDisplayName(role.name);
}

function isAdminRole(roleId) {
  return Number(roleId) === Number(enterpriseRoles.value[0]?.id);
}

function getEnterpriseRoleDisplayName(name) {
  return name || "-";
}

function isTenantSuperAdmin(record) {
  if (!record) return false;
  // Ownership is transferable, so it comes from the backend flag, not from id === admin_id.
  return record.is_owner === true;
}

// Only the owner can hand their own role over; no permission key may grant this, or any
// enterprise admin could seize the tenant.
function canTransferOwner(record) {
  return isTenantSuperAdmin(record) && authStore.isSuperAdmin();
}

function canEditMember(record) {
  if (!authStore.hasPermission("member:edit")) return false;
  return (
    !isTenantSuperAdmin(record) ||
    String(record.id) === String(authStore.userInfo?.id)
  );
}

function canResetPassword(record) {
  if (!authStore.hasPermission("member:reset_password")) return false;
  return (
    !isTenantSuperAdmin(record) ||
    String(record.id) === String(authStore.userInfo?.id)
  );
}

function canRemoveMember(record) {
  if (!authStore.hasPermission("member:remove")) return false;
  return !isTenantSuperAdmin(record);
}

async function handleInvite() {
  if (inviteAvatarUploading.value) return;
  try {
    await inviteFormRef.value?.validate();
  } catch {
    return;
  }
  inviting.value = true;
  try {
    const data = {
      name: inviteForm.value.name.trim(),
      username: inviteForm.value.username.trim(),
      phone: inviteForm.value.phone.trim(),
      email: inviteForm.value.email.trim(),
      roleId: Number(inviteForm.value.roleId),
      avatar: inviteAvatarPath.value,
      departmentIds: (inviteForm.value.departmentIds || []).map(Number),
    };
    if (editingMember.value) {
      await updateEnterpriseMember(editingMember.value.id, data);
      message.success(t("member_updateSuccess"));
    } else {
      const password = randomPassword();
      await createEnterpriseMember({ ...data, password });
      addSuccess.value = {
        open: true,
        address: window.location.origin,
        username: data.username,
        password,
      };
    }
    inviteOpen.value = false;
    clearInviteForm();
    // Refresh the list.
    await refreshMembers();
  } catch {
    /* handled by interceptor */
  } finally {
    inviting.value = false;
  }
}

async function copyAddSuccessInfo() {
  const lines = [
    t("member_addSuccessTip"),
    `${t("member_addSuccessAddress")}：${addSuccess.value.address}`,
    `${t("member_addSuccessAccount")}：${addSuccess.value.username}`,
    `${t("member_addSuccessPassword")}：${addSuccess.value.password}`,
  ].join("\n");
  try {
    await copyText(lines);
    message.success(t("invite_copied"));
  } catch {
    message.error(t("invite_copyFailed"));
  }
}

function handleResetPwd(record) {
  searchKeywordBeforeReset.value = searchKeyword.value;
  resetPwdTarget.value = record;
  resetPwdValue.value = "";
  resetPwdOpen.value = true;
}

function restoreSearchAfterReset() {
  searchKeyword.value = searchKeywordBeforeReset.value;
}

function handleResetPwdCancel() {
  resetPwdOpen.value = false;
  restoreSearchAfterReset();
  resetPwdValue.value = "";
}

// 10-char random password; charset excludes easily-confused characters like 0/O/1/l
function randomPassword() {
  const chars = "ABCDEFGHJKMNPQRSTWXYZabcdefhijkmnprstwxyz2345678";
  let pwd = "";
  for (let i = 0; i < 10; i++) {
    pwd += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return pwd;
}

function generatePwd() {
  resetPwdValue.value = randomPassword();
}

async function handleResetPwdConfirm() {
  if (!resetPwdValue.value || !resetPwdTarget.value?.id) {
    message.warning(t("workspace_pwdRequired"));
    return;
  }
  try {
    await resetEnterpriseMemberPassword(resetPwdTarget.value.id, {
      password: resetPwdValue.value,
    });
    message.success(t("workspace_pwdReset"));
    resetPwdOpen.value = false;
    restoreSearchAfterReset();
  } catch {
    /* handled */
  }
}

// ---- Ownership transfer & member removal hand-over ----

const transferOpen = ref(false);
const transferSubmitting = ref(false);
const transferTargetId = ref(undefined);
const removeOpen = ref(false);
const removeSubmitting = ref(false);
const removePreviewLoading = ref(false);
const removeTarget = ref(null);
const removeOwnedWorkspaces = ref([]);
const removeUnfinishedCount = ref(0);
const removeOwnerTargetId = ref(undefined);
const removeItemsTargetId = ref(undefined);
const removeTransferItems = ref(false);

// Candidates for any hand-over: enabled members of the tenant, excluding the given user.
function buildCandidates(excludedId) {
  return members.value
    .filter(
      (m) => String(m.id) !== String(excludedId) && String(m.status) === "1",
    )
    .map((m) => ({ value: Number(m.id), label: m.name || m.username }));
}

const transferCandidates = computed(() =>
  buildCandidates(authStore.userInfo?.id),
);
const removeCandidates = computed(() =>
  buildCandidates(removeTarget.value?.id),
);

function openTransferOwner() {
  transferTargetId.value = undefined;
  transferOpen.value = true;
}

async function submitTransferOwner() {
  if (!transferTargetId.value) {
    message.warning(t("owner_transferSelectPlaceholder"));
    return;
  }
  transferSubmitting.value = true;
  try {
    await transferTenantOwner(transferTargetId.value);
    message.success(t("owner_transferred"));
    transferOpen.value = false;
    // ponytail: the operator just dropped from owner to plain member, so every cached permission,
    // menu and route guard is stale. Land on /my-work instead of reloading /members: the operator
    // no longer holds member:view, and the guard would bounce them to /403 -> /workspaces, a page
    // rendered without the workspace layout and therefore without any way to log out.
    window.location.href = "/my-work";
  } catch {
    /* handled */
  } finally {
    transferSubmitting.value = false;
  }
}

async function openRemoveMember(record) {
  removeTarget.value = record;
  removeOwnedWorkspaces.value = [];
  removeUnfinishedCount.value = 0;
  removeOwnerTargetId.value = undefined;
  removeItemsTargetId.value = undefined;
  removeTransferItems.value = false;
  removeOpen.value = true;
  removePreviewLoading.value = true;
  try {
    const preview = await previewEnterpriseMemberRemoval(record.id);
    removeOwnedWorkspaces.value = preview?.owned_workspaces || [];
    removeUnfinishedCount.value = Number(preview?.unfinished_count || 0);
  } catch {
    /* handled */
  } finally {
    removePreviewLoading.value = false;
  }
}

async function submitRemoveMember() {
  if (!removeTarget.value) return;
  // Handing the project admin role over is mandatory: those projects would otherwise lose their owner.
  if (removeOwnedWorkspaces.value.length && !removeOwnerTargetId.value) {
    message.warning(t("owner_transferSelectPlaceholder"));
    return;
  }
  if (removeTransferItems.value && !removeItemsTargetId.value) {
    message.warning(t("owner_transferSelectPlaceholder"));
    return;
  }
  const params = {};
  if (removeOwnedWorkspaces.value.length) {
    params.transferOwnerTo = removeOwnerTargetId.value;
  }
  if (removeTransferItems.value) {
    params.transferItemsTo = removeItemsTargetId.value;
  }
  removeSubmitting.value = true;
  try {
    await removeEnterpriseMember(removeTarget.value.id, params);
    message.success(t("workspace_memberRemoved"));
    removeOpen.value = false;
    await refreshMembers();
  } catch {
    /* handled */
  } finally {
    removeSubmitting.value = false;
  }
}

function openMemberLogModal() {
  memberLogOpen.value = true;
}

function usageLabel(key) {
  return t(`plan_quota_${key}`);
}

function formatFileSize(bytes) {
  if (!bytes || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const k = 1024;
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  const value = bytes / Math.pow(k, i);
  return `${value.toFixed(1)} ${units[i]}`;
}

function usageUnit(record) {
  return record.key === "storage" ? "G" : "";
}

function usagePercent(record) {
  if (!record.limit) return 0;
  if (record.key === "storage") {
    const usedGB = Number(record.used) / (1024 * 1024 * 1024);
    return Math.min(Math.round((usedGB / Number(record.limit)) * 100), 100);
  }
  return Math.min(
    Math.round((Number(record.used) / Number(record.limit)) * 100),
    100,
  );
}

function usageColor(record) {
  const percent = usagePercent(record);
  if (percent >= 100) return "#ef4444";
  if (percent > 80) return "#f59e0b";
  return planUsage.value.plan_type === "enterprise" ? "#7c3aed" : "#2563eb";
}

function enterpriseLimitText(record) {
  if (record.key === "storage") return `${record.enterprise_limit}G`;
  if (record.key === "member") return t("plan_memberEnterpriseLimit");
  if (record.key === "custom_field" || record.key === "automation_rule") {
    return t("plan_perProjectTotal", {
      perProject: 50,
      total: record.enterprise_limit,
    });
  }
  if (record.key === "automation_exec") {
    return t("plan_monthlyLimit", { value: record.enterprise_limit });
  }
  return String(record.enterprise_limit);
}

function isMemberDisabled(record) {
  return String(record?.status) === "0";
}

const canSeeStatusSwitch = computed(() =>
  authStore.hasPermission("member:edit"),
);

function canToggleStatus(record) {
  if (!authStore.hasPermission("member:edit")) return false;
  if (isTenantSuperAdmin(record)) return false;
  return String(record.id) !== String(authStore.userInfo?.id);
}

async function handleToggleStatus(record, checked) {
  const nextStatus = checked ? 1 : 0;
  statusUpdatingId.value = record.id;
  try {
    await updateEnterpriseMemberStatus(record.id, nextStatus);
    message.success(
      nextStatus === 1 ? t("member_enableSuccess") : t("member_disableSuccess"),
    );
    await refreshMembers();
  } catch {
    await refreshMembers();
  } finally {
    statusUpdatingId.value = null;
  }
}

// Row actions live in the "more" dropdown, so removal confirms via modal instead of popconfirm.
function handleMemberAction(menuKey, record) {
  if (menuKey === "edit") {
    openEditModal(record);
  } else if (menuKey === "resetPwd") {
    handleResetPwd(record);
  } else if (menuKey === "transfer") {
    openTransferOwner();
  } else if (menuKey === "remove") {
    openRemoveMember(record);
  }
}
</script>

<style scoped lang="less">
.member-page {
  display: flex;
  min-height: 0;
  flex: 1;
  flex-direction: column;
  color: #182b50;

  :deep(.ant-tabs) {
    min-height: 0;
    flex: 1;
  }

  :deep(.ant-tabs-nav) {
    margin-bottom: 0;
    border-bottom: 1px solid #e5e9f0;
  }

  :deep(.ant-tabs-nav-operations) {
    display: none;
  }

  :deep(.ant-tabs-extra-content) {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-right: 92px;
  }

  :deep(.ant-tabs-content-holder) {
    margin-top: 16px;
  }

  :deep(.ant-tabs-content-holder),
  :deep(.ant-tabs-content),
  :deep(.ant-tabs-tabpane) {
    min-height: 0;
    height: 100%;
  }
}

.members-layout {
  display: flex;
  height: 100%;
  min-height: 0;
}

.org-panel {
  display: flex;
  flex: none;
  min-width: 0;
  flex-direction: column;
  padding: 4px 0 16px;
}

.org-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 8px 14px;
}

.org-panel-title {
  color: #182b50;
  font-size: 15px;
  font-weight: 600;
}

.org-add-button {
  display: inline-flex;
  width: 26px;
  height: 26px;
  align-items: center;
  justify-content: center;
  padding: 0;
  border-radius: 6px;
  color: #657188;

  &:hover {
    color: #0d68ff;
    background: #eef4ff;
  }
}

.org-tree-wrap {
  min-height: 0;
  flex: 1;
  overflow: auto;
  padding-right: 6px;

  :deep(.ant-tree) {
    background: transparent;
    color: #182b50;
  }

  :deep(.ant-tree .ant-tree-treenode) {
    width: 100%;
    padding: 0;
    margin-bottom: 2px;
    align-items: center;
  }

  :deep(.ant-tree .ant-tree-switcher) {
    display: flex;
    width: 22px;
    align-self: stretch;
    align-items: center;
    justify-content: center;
    color: #8a94a6;
  }

  :deep(.ant-tree .ant-tree-switcher .ant-tree-switcher-icon) {
    font-size: 11px;
  }

  :deep(.ant-tree .ant-tree-node-content-wrapper) {
    display: flex;
    min-width: 0;
    min-height: 34px;
    flex: 1;
    align-items: center;
    padding: 0 8px;
    border-radius: 6px;
    font-size: 13px;
    transition: background-color 0.15s;
  }

  :deep(.ant-tree .ant-tree-node-content-wrapper:hover) {
    background: #f2f5fa;
  }

  :deep(.ant-tree .ant-tree-node-content-wrapper.ant-tree-node-selected) {
    background: #eef4ff;
    color: #0d68ff;
    font-weight: 500;
  }

  :deep(.ant-tree .ant-tree-title) {
    display: block;
    min-width: 0;
    flex: 1;
  }

  :deep(.ant-tree .ant-tree-indent-unit) {
    width: 14px;
  }
}

.org-node {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
}

.org-node-title {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.org-node-count {
  flex: none;
  color: #8a94a6;
  font-size: 12px;
  font-weight: 400;
}

.org-node-root {
  overflow: hidden;
  font-size: 14px;
  font-weight: 600;
}

.org-node-more {
  display: none;
  flex: none;
  width: 22px;
  height: 22px;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  color: #657188;

  &:hover {
    color: #0d68ff;
    background: #dbe7ff;
  }
}

.org-node:hover .org-node-more {
  display: inline-flex;
}

.org-resizer {
  flex: none;
  width: 7px;
  margin-right: 9px;
  cursor: col-resize;
  border-right: 1px solid #ebeef5;
  transition:
    border-color 0.2s,
    border-width 0.2s;

  &:hover,
  &.org-resizer--active {
    border-right-width: 3px;
    border-right-color: #0d68ff;
  }
}

.members-main {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
}

.page-icon {
  display: inline-flex;
  width: 24px;
  height: 24px;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  background: #0d68ff;
  color: #fff;
  font-size: 13px;
}

.page-title {
  margin: 0;
  color: #182b50;
  font-size: 16px;
  font-weight: 400;
  line-height: 24px;
  white-space: nowrap;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-bottom: 20px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 15px;
  font-weight: 600;
}

.member-count {
  display: inline-flex;
  min-width: 22px;
  height: 22px;
  align-items: center;
  justify-content: center;
  padding: 0 7px;
  border-radius: 11px;
  color: #0d68ff;
  background: #eef3ff;
  font-size: 12px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.member-search {
  width: 220px;
  height: 36px;
  border-radius: 6px;

  :deep(.ant-input-prefix) {
    color: #657188;
  }
}

.add-member-button {
  height: 36px;
  padding: 0 21px;
  border-radius: 8px;
  background: #0d68ff;
  box-shadow: 0 2px 4px rgba(13, 104, 255, 0.15);
  font-size: 13px;
}

.link-invite-button {
  height: 36px;
  padding: 0 16px;
  border-radius: 8px;
  border-color: #b9d3ff;
  color: #0d68ff;
  font-size: 13px;
}

.batch-actions-button {
  height: 36px;
  padding: 0 16px;
  border-radius: 8px;
  border-color: #b9d3ff;
  color: #0d68ff;
  font-size: 13px;
}

.batch-actions-arrow {
  margin-left: 2px;
  font-size: 11px;
}

.add-success-tip {
  margin: 8px 0 16px;
  color: #22b07d;
  font-size: 14px;
  font-weight: 600;
}

.add-success-rows {
  display: flex;
  flex-direction: column;
  gap: 10px;
  color: #182b50;
  font-size: 14px;
  word-break: break-all;
}

.invite-active-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 36px;
  padding: 0 16px;
  border: 1px solid #b9d3ff;
  border-radius: 8px;
  color: #0d68ff;
  background: #eef4ff;
  font-size: 13px;
  font-weight: 500;
  cursor: default;
  animation: invite-pulse 2s ease-in-out infinite;
}

.invite-pulse-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #0d68ff;
  animation: invite-dot-pulse 1.2s ease-in-out infinite;
}

.invite-countdown {
  font-family: "SFMono-Regular", Consolas, monospace;
  font-weight: 700;
}

@keyframes invite-pulse {
  0%,
  100% {
    box-shadow: 0 0 0 0 rgba(13, 104, 255, 0.25);
  }
  50% {
    box-shadow: 0 0 0 4px rgba(13, 104, 255, 0.08);
  }
}

@keyframes invite-dot-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.35;
  }
}

.invite-link-tag {
  margin-left: 8px;
  padding: 0 6px;
  border-color: #b9d3ff;
  border-radius: 4px;
  color: #0d68ff;
  background: #fff;
  font-size: 12px;
  line-height: 18px;
}

.invite-result-tip {
  margin: 8px 0 12px;
  color: #657188;
  font-size: 14px;
}

.invite-result-link-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border: 1px solid #e5e9f0;
  border-radius: 8px;
  background: #fafbfd;
}

.invite-result-link {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  color: #0d68ff;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
  word-break: break-all;
}

.member-table-card {
  min-height: 0;
  flex: 1;
  overflow-y: auto;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  background: #fff;

  :deep(.ant-table-thead > tr > th) {
    position: sticky;
    top: 0;
    z-index: 2;
    height: 48px;
    padding: 14px 16px;
    border-bottom: 1px solid #ebeef5;
    color: #657188;
    background: #fafbfd;
    font-size: 12px;
    font-weight: 600;
  }

  :deep(.ant-table-tbody > tr > td) {
    height: 64px;
    padding: 12px 16px;
    border-bottom: 0;
  }

  :deep(.ant-table-tbody > tr:hover > td) {
    background: #fafcff;
  }
}

.user-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.member-avatar {
  flex: none;
  color: #fff;
  background: #0d68ff;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.06);
  font-size: 20px;
}

.user-info {
  min-width: 0;
}

.user-name {
  overflow: hidden;
  color: #182b50;
  font-size: 14px;
  font-weight: 500;
  line-height: 20px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.member-disabled-tag {
  margin-left: 6px;
  padding: 0 6px;
  border-color: #ffa39e;
  color: #cf1322;
  background: #fff1f0;
  font-size: 12px;
  line-height: 18px;
}

.user-username,
.join-time {
  color: #8a94a6;
  font-size: 12px;
  line-height: 18px;
}

.member-contact,
.member-email,
.member-department {
  display: block;
  overflow: hidden;
  color: #657188;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.role-tag {
  margin: 0;
  padding: 2px 10px;
  border-color: #e5e9f0;
  border-radius: 6px;
  color: #657188;
  background: #f5f7fa;
  line-height: 22px;
}

.role-tag-admin {
  border-color: #b9d3ff;
  color: #0d68ff;
  background: #eef4ff;
}

.action-more {
  display: inline-flex;
  width: 24px;
  height: 24px;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  color: #657188;
  cursor: pointer;

  &:hover {
    color: #0d68ff;
    background: #dbe7ff;
  }
}

.password-autofill-anchor {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  opacity: 0;
  pointer-events: none;
}

@media (max-width: 900px) {
  .page-header {
    align-items: flex-start;
    gap: 12px;
  }

  .header-actions {
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .member-search {
    width: 200px;
  }
}

.plan-panel {
  height: 100%;
  min-height: 0;
  overflow-y: auto;
  padding-bottom: 24px;
}

.plan-card {
  margin-bottom: 24px;
  padding: 0 24px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #f8fafc;
}

.plan-version-row {
  display: flex;
  min-height: 116px;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.plan-card-actions {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 12px;
}

.upgrade-btn {
  height: 40px;
  border-radius: 10px;
  font-weight: 600;
  font-size: 14px;
  color: #fff;
  border: none;
  background: linear-gradient(135deg, rgb(124, 58, 237), rgb(91, 33, 182));
  box-shadow: rgba(124, 58, 237, 0.35) 0px 4px 14px;
}

.upgrade-btn:hover,
.upgrade-btn:focus {
  color: #fff;
  border: none;
  background: linear-gradient(135deg, rgb(124, 58, 237), rgb(91, 33, 182));
}

.plan-card-main {
  display: flex;
  align-items: center;
  gap: 16px;
}

.plan-icon {
  display: flex;
  width: 48px;
  height: 48px;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  color: #2563eb;
  background: linear-gradient(135deg, #eff6ff, #dbeafe);
  font-size: 22px;
}

.plan-title {
  margin-bottom: 4px;
  color: #0f172a;
  font-size: 18px;
  font-weight: 700;

  span {
    color: #2563eb;
  }
}

.plan-description {
  color: #64748b;
  font-size: 13px;
}

.usage-card {
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #fff;

  :deep(.ant-table-thead > tr > th) {
    padding: 14px 24px;
    color: #64748b;
    background: #fff;
    font-size: 13px;
  }

  :deep(.ant-table-tbody > tr > td) {
    padding: 14px 24px;
    border-bottom-color: #f1f5f9;
  }
}

.usage-card-title {
  padding: 16px 24px;
  border-bottom: 1px solid #f1f5f9;
  color: #0f172a;
  background: #f8fafc;
  font-size: 14px;
  font-weight: 600;
}

.usage-progress {
  display: flex;
  align-items: center;
  gap: 12px;

  :deep(.ant-progress) {
    width: min(100%, 260px);
    margin: 0;
  }

  span {
    min-width: 70px;
    font-size: 13px;
    font-weight: 600;
    text-align: right;
    white-space: nowrap;
  }
}

.member-log-button {
  display: inline-flex;
  width: 24px;
  height: 24px;
  align-items: center;
  justify-content: center;
  padding: 0;
  color: #6b7a99;

  &:hover {
    color: #0d68ff;
  }
}

.password-autofill-anchor {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  opacity: 0;
  pointer-events: none;
}

@media (max-width: 900px) {
  .page-header {
    align-items: flex-start;
    gap: 12px;
  }

  .header-actions {
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .member-search {
    width: 200px;
  }
}
</style>

<style lang="less">
.invite-popover {
  width: 280px;

  .invite-progress-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 10px;
    color: #657188;
    font-size: 13px;

    .invite-progress-num {
      color: #0d68ff;
      font-weight: 700;
      font-family: "SFMono-Regular", Consolas, monospace;
    }
  }

  .invite-link-input {
    margin-bottom: 12px;

    input {
      color: #657188;
      font-size: 12px;
    }
  }

  .invite-popover-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
}

.invite-member-modal {
  .ant-modal {
    max-width: calc(100vw - 32px);
    padding-bottom: 24px;
  }

  .ant-modal-content {
    padding: 20px 24px 24px;
    border-radius: 8px;
    box-shadow:
      0 6px 8px rgba(0, 0, 0, 0.08),
      0 3px 3px rgba(0, 0, 0, 0.12),
      0 9px 14px rgba(0, 0, 0, 0.05);
  }

  .ant-modal-header {
    margin-bottom: 0;
  }

  .ant-modal-title {
    color: rgba(0, 0, 0, 0.88);
    font-size: 16px;
    font-weight: 600;
    line-height: 24px;
  }

  .ant-modal-close {
    top: 12px;
    right: 12px;
    width: 32px;
    height: 32px;
  }

  .ant-modal-footer {
    margin-top: 24px;
  }

  .ant-modal-footer .ant-btn {
    height: 32px;
    padding: 0 16px;
    border-radius: 6px;
  }

  .ant-modal-footer .ant-btn-primary {
    background: #3157e2;
  }

  .invite-avatar-section {
    display: flex;
    height: 180px;
    flex-direction: column;
    align-items: center;
    justify-content: flex-end;
  }

  .invite-avatar-wrap {
    position: relative;
    width: 88px;
    height: 88px;
    margin-bottom: 20px;
  }

  .invite-avatar {
    border: 3px solid #fff;
    color: #fff;
    background: #0d68ff;
    box-shadow: 0 4px 14px rgba(13, 104, 255, 0.15);
    font-size: 32px;
  }

  .invite-avatar-camera {
    position: absolute;
    right: 0;
    bottom: 0;
    display: flex;
    width: 32px;
    height: 32px;
    align-items: center;
    justify-content: center;
    padding: 0;
    border: 2px solid #fff;
    border-radius: 50%;
    color: #fff;
    background: #0d68ff;
    box-shadow: 0 3px 4px rgba(13, 104, 255, 0.3);
    cursor: pointer;
    font-size: 13px;
  }

  .invite-avatar-hint {
    margin: 0;
    color: rgba(0, 0, 0, 0.45);
    font-size: 14px;
    line-height: 22px;
  }

  .invite-member-form {
    margin-top: 16px;
  }

  .invite-member-form .ant-form-item {
    margin-bottom: 20px;
  }

  .invite-member-form .ant-form-item:last-child {
    margin-bottom: 0;
  }

  .invite-member-form .ant-form-item-label {
    padding-bottom: 4px;
  }

  .invite-member-form .ant-form-item-label > label {
    height: 22px;
    color: #262626;
    font-size: 14px;
    line-height: 22px;
  }

  .invite-member-form .ant-form-item-required::before {
    color: #fb363f !important;
  }

  .invite-member-form .ant-input,
  .invite-member-form .ant-input-affix-wrapper,
  .invite-member-form .ant-select-selector {
    border-radius: 6px !important;
  }

  .invite-member-form .ant-input,
  .invite-member-form .ant-input-affix-wrapper,
  .invite-member-form .ant-select-single,
  .invite-member-form .ant-select-selector {
    min-height: 32px;
  }
}

.member-department-tree-dropdown {
  .ant-select-tree-list-holder-inner {
    align-items: stretch;
  }

  .ant-select-tree-treenode {
    width: 100%;
  }

  .ant-select-tree-node-content-wrapper {
    flex: 1;
  }
}

.transfer-tip {
  margin: 0 0 8px;
  color: #5a6a85;
}

.transfer-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
}

.transfer-label {
  flex-shrink: 0;
  color: #182b50;
}

.transfer-select {
  flex: 1;
  min-width: 0;
}

.transfer-hint {
  margin: 6px 0 0;
  color: #8c8c8c;
  font-size: 12px;
}
</style>
