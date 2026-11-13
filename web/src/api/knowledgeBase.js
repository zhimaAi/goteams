import { axios, request } from "./request";

// ======================== Knowledge Base API ========================

// Full non-deleted item list of the workspace (metadata only, no body).
export function getKnowledgeTree(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/kb/tree`,
    method: "GET",
  });
}

// Item detail with the current content (documents/links only).
export function getKnowledgeItem(wsId, itemId) {
  return request({
    url: `/api/workspaces/${wsId}/kb/items/${itemId}`,
    method: "GET",
  });
}

// Search by title or creator name (keyword, returns up to 50 rows).
export function searchKnowledgeItems(wsId, keyword) {
  return request({
    url: `/api/workspaces/${wsId}/kb/search`,
    method: "GET",
    params: { keyword },
  });
}

export function createKnowledgeFolder(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/kb/folders`,
    method: "POST",
    data,
  });
}

export function createKnowledgeDocument(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/kb/documents`,
    method: "POST",
    data,
  });
}

// Create a read-only knowledge document from any uploaded local file (multipart).
export function createKnowledgeDocumentFromFile(wsId, parentId, file) {
  const data = new FormData();
  data.append("parent_id", String(parentId || 0));
  data.append("file", file);
  return request({
    url: `/api/workspaces/${wsId}/kb/documents/from-file`,
    method: "POST",
    data,
    headers: { "Content-Type": "multipart/form-data" },
    timeout: 600000,
  });
}

// Download the original uploaded file, or a generated docx for an editor-created document.
export function downloadKnowledgeDocument(wsId, documentId) {
  const resourceOrigin =
    typeof window === "undefined" ? "" : window.location.origin;
  return axios.get(
    `/api/workspaces/${wsId}/kb/documents/${documentId}/download`,
    {
      responseType: "blob",
      timeout: 60000,
      headers: resourceOrigin
        ? { "X-GoTeams-Resource-Origin": resourceOrigin }
        : {},
    },
  );
}

export function createKnowledgeLink(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/kb/links`,
    method: "POST",
    data,
  });
}

// Rename a folder/document/link; sourceUrl optionally updates the link URL.
export function renameKnowledgeItem(wsId, itemId, title, sourceUrl) {
  return request({
    url: `/api/workspaces/${wsId}/kb/items/${itemId}`,
    method: "PUT",
    data: sourceUrl ? { title, source_url: sourceUrl } : { title },
  });
}

// Move a folder/document/link under another folder (0 = workspace root) and
// reorder: prevId is the sibling it is inserted after (0 = top, -1 = tail).
export function moveKnowledgeItem(wsId, itemId, parentId, prevId = -1) {
  return request({
    url: `/api/workspaces/${wsId}/kb/items/${itemId}/move`,
    method: "POST",
    data: { parent_id: parentId, prev_id: prevId },
  });
}

// Soft-delete a node (folder delete cascades to descendants).
export function deleteKnowledgeItem(wsId, itemId) {
  return request({
    url: `/api/workspaces/${wsId}/kb/items/${itemId}`,
    method: "DELETE",
  });
}

export function copyKnowledgeDocument(wsId, documentId, data) {
  return request({
    url: `/api/workspaces/${wsId}/kb/documents/${documentId}/copy`,
    method: "POST",
    data,
  });
}

// Optimistic autosave; a revision mismatch rejects with a 409 carrying the current server state.
export function saveKnowledgeDocumentContent(wsId, documentId, data) {
  return request({
    url: `/api/workspaces/${wsId}/kb/documents/${documentId}/content`,
    method: "PUT",
    data,
  });
}

// Convert an uploaded file document (content_mode=file) into an editable rich-text document.
// Errors are reported by the caller (silent avoids duplicate toasts with the interceptor).
export function convertKnowledgeDocument(wsId, documentId, data) {
  return request({
    url: `/api/workspaces/${wsId}/kb/documents/${documentId}/convert`,
    method: "POST",
    data,
    silent: true,
  });
}

export function listKnowledgeVersions(wsId, documentId, params) {
  return request({
    url: `/api/workspaces/${wsId}/kb/documents/${documentId}/versions`,
    method: "GET",
    params,
  });
}

export function getKnowledgeVersion(wsId, documentId, versionId) {
  return request({
    url: `/api/workspaces/${wsId}/kb/documents/${documentId}/versions/${versionId}`,
    method: "GET",
  });
}

export function createKnowledgeVersion(wsId, documentId, data) {
  return request({
    url: `/api/workspaces/${wsId}/kb/documents/${documentId}/versions`,
    method: "POST",
    data,
  });
}

export function restoreKnowledgeVersion(wsId, documentId, versionId, data) {
  return request({
    url: `/api/workspaces/${wsId}/kb/documents/${documentId}/versions/${versionId}/restore`,
    method: "POST",
    data,
  });
}

export function setKnowledgeFavorite(wsId, itemId) {
  return request({
    url: `/api/workspaces/${wsId}/kb/items/${itemId}/favorite`,
    method: "PUT",
  });
}

export function unsetKnowledgeFavorite(wsId, itemId) {
  return request({
    url: `/api/workspaces/${wsId}/kb/items/${itemId}/favorite`,
    method: "DELETE",
  });
}

export function listKnowledgeActivities(wsId, params) {
  return request({
    url: `/api/workspaces/${wsId}/kb/home/activities`,
    method: "GET",
    params,
  });
}

export function getKnowledgeHomeFavorites(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/kb/home/favorites`,
    method: "GET",
  });
}

export function listKnowledgeCreatedByMe(wsId, params) {
  return request({
    url: `/api/workspaces/${wsId}/kb/home/created-by-me`,
    method: "GET",
    params,
  });
}

// Upload an image for a document and return its directly accessible relative URL.
export function uploadKnowledgeImage(wsId, documentId, file) {
  const data = new FormData();
  data.append("file", file);
  return request({
    url: `/api/workspaces/${wsId}/kb/documents/${documentId}/attachments/images`,
    method: "POST",
    data,
    headers: { "Content-Type": "multipart/form-data" },
    timeout: 120000,
  });
}

// Upload a general file for a document and return its directly accessible relative URL.
export function uploadKnowledgeFile(wsId, documentId, file) {
  const data = new FormData();
  data.append("file", file);
  return request({
    url: `/api/workspaces/${wsId}/kb/documents/${documentId}/attachments/files`,
    method: "POST",
    data,
    headers: { "Content-Type": "multipart/form-data" },
    timeout: 600000,
  });
}

// Upload a video for a document and return its directly accessible relative URL.
export function uploadKnowledgeVideo(wsId, documentId, file) {
  const data = new FormData();
  data.append("file", file);
  return request({
    url: `/api/workspaces/${wsId}/kb/documents/${documentId}/attachments/videos`,
    method: "POST",
    data,
    headers: { "Content-Type": "multipart/form-data" },
    timeout: 600000,
  });
}
