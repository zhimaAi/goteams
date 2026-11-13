import { request } from "./request";

// Public invitation requests are anonymous: the visitor has no app session, so the
// display language travels in the `Lang` header (the server reads it via lib_i18n.GetLang).
function publicHeaders(lang) {
  return lang ? { Lang: lang } : {};
}

// Return authenticated sharing metadata for a workspace.
export function getIssueInviteShare(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/issue-invite/share`,
    method: "GET",
  });
}

// Return the invitation guide configuration for a workspace.
export function getIssueInviteConfig(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/issue-invite/config`,
    method: "GET",
  });
}

// Update the invitation guide configuration for a workspace.
export function updateIssueInviteConfig(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/issue-invite/config`,
    method: "PATCH",
    data,
  });
}

// Upload the invitation theme image (750*360, within 500KB) and return its URL.
export function uploadIssueInviteThemeImage(wsId, file) {
  const formData = new FormData();
  formData.append("file", file);
  return request({
    url: `/api/workspaces/${wsId}/issue-invite/theme-image`,
    method: "POST",
    data: formData,
    headers: { "Content-Type": "multipart/form-data" },
  });
}

// Return invitation templates for a workspace.
export function listIssueInviteTemplates(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/issue-invite/templates`,
    method: "GET",
  });
}

// Create one independent invitation template.
export function createIssueInviteTemplate(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/issue-invite/templates`,
    method: "POST",
    data,
  });
}

// Return one invitation template.
export function getIssueInviteTemplate(wsId, templateId) {
  return request({
    url: `/api/workspaces/${wsId}/issue-invite/templates/${templateId}`,
    method: "GET",
  });
}

// Update one invitation template.
export function updateIssueInviteTemplate(wsId, templateId, data) {
  return request({
    url: `/api/workspaces/${wsId}/issue-invite/templates/${templateId}`,
    method: "PATCH",
    data,
  });
}

// Remove one invitation template.
export function removeIssueInviteTemplate(wsId, templateId) {
  return request({
    url: `/api/workspaces/${wsId}/issue-invite/templates/${templateId}`,
    method: "DELETE",
  });
}

// Replace the complete invitation-template order.
export function reorderIssueInviteTemplates(wsId, templateIds) {
  return request({
    url: `/api/workspaces/${wsId}/issue-invite/templates/order`,
    method: "PUT",
    data: { template_ids: templateIds },
  });
}

// Return a public general-link configuration.
export function getPublicIssueInvite(inviteKey, openid, lang) {
  return request({
    url: `/api/public/issue-invites/${encodeURIComponent(inviteKey)}`,
    method: "GET",
    params: { openid },
    headers: publicHeaders(lang),
  });
}

// Return one public copied-template schema.
export function getPublicIssueTemplate(inviteKey, templateKey, openid, lang) {
  return request({
    url: `/api/public/issue-invites/${encodeURIComponent(inviteKey)}/templates/${encodeURIComponent(templateKey)}`,
    method: "GET",
    params: { openid },
    headers: publicHeaders(lang),
  });
}

// Submit one public issue.
export function submitPublicIssue(
  inviteKey,
  templateKey,
  openid,
  fields,
  lang,
  fileIds,
) {
  return request({
    url: `/api/public/issue-invites/${encodeURIComponent(inviteKey)}/templates/${encodeURIComponent(templateKey)}/requirements`,
    method: "POST",
    data: { openid, fields, file_ids: fileIds },
    headers: publicHeaders(lang),
  });
}

// Return one fixed-size public issue-history page.
export function listPublicIssueHistory(inviteKey, openid, page, lang) {
  return request({
    url: `/api/public/issue-invites/${encodeURIComponent(inviteKey)}/requirements`,
    method: "GET",
    params: { openid, page },
    headers: publicHeaders(lang),
  });
}

// Return one submitter-owned read-only public issue.
export function getPublicIssueDetail(inviteKey, openid, requirementId, lang) {
  return request({
    url: `/api/public/issue-invites/${encodeURIComponent(inviteKey)}/requirements/${requirementId}`,
    method: "GET",
    params: { openid },
    headers: publicHeaders(lang),
  });
}

// Return the merged handle-record feed (comments, creation, status changes) of one public issue.
export function getPublicIssueHandleRecords(
  inviteKey,
  openid,
  requirementId,
  lang,
) {
  return request({
    url: `/api/public/issue-invites/${encodeURIComponent(inviteKey)}/requirements/${requirementId}/handle-records`,
    method: "GET",
    params: { openid },
    headers: publicHeaders(lang),
  });
}

// Post one submitter comment on a public issue.
export function createPublicIssueComment(
  inviteKey,
  openid,
  requirementId,
  content,
  lang,
) {
  return request({
    url: `/api/public/issue-invites/${encodeURIComponent(inviteKey)}/requirements/${requirementId}/comments`,
    method: "POST",
    data: { openid, content },
    headers: publicHeaders(lang),
  });
}

// Withdraw one's own public issue; the server moves it to the template's recall status.
export function recallPublicIssue(inviteKey, openid, requirementId, lang) {
  return request({
    url: `/api/public/issue-invites/${encodeURIComponent(inviteKey)}/requirements/${requirementId}/recall`,
    method: "POST",
    data: { openid },
    headers: publicHeaders(lang),
  });
}

// Urge the assignees of one's own public issue.
export function urgePublicIssue(inviteKey, openid, requirementId, lang) {
  return request({
    url: `/api/public/issue-invites/${encodeURIComponent(inviteKey)}/requirements/${requirementId}/urge`,
    method: "POST",
    data: { openid },
    headers: publicHeaders(lang),
  });
}

// Edit and resubmit one's own public issue; the server replaces the form content and resets the status.
export function resubmitPublicIssue(
  inviteKey,
  openid,
  requirementId,
  fields,
  fileIds,
  lang,
) {
  return request({
    url: `/api/public/issue-invites/${encodeURIComponent(inviteKey)}/requirements/${requirementId}`,
    method: "PUT",
    data: { openid, fields, file_ids: fileIds },
    headers: publicHeaders(lang),
  });
}

// Upload one attachment from the public issue form and return its accessible url.
export function uploadPublicIssueAttachment(inviteKey, openid, lang, file) {
  const formData = new FormData();
  formData.append("file", file);
  return request({
    url: `/api/public/issue-invites/${encodeURIComponent(inviteKey)}/attachments`,
    method: "POST",
    params: { openid },
    data: formData,
    headers: { "Content-Type": "multipart/form-data", ...publicHeaders(lang) },
    // Attachments may be up to 200MB; the 15s global default would abort long before that.
    timeout: 600000,
  });
}

// Remove one not-yet-submitted attachment of the public issue form.
export function deletePublicIssueAttachment(
  inviteKey,
  openid,
  attachmentId,
  lang,
) {
  return request({
    url: `/api/public/issue-invites/${encodeURIComponent(inviteKey)}/attachments/${attachmentId}`,
    method: "DELETE",
    params: { openid },
    headers: publicHeaders(lang),
  });
}
