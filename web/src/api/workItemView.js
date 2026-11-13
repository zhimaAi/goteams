// workItemView.js
// Requirement/defect list view APIs (view CRUD + per-user view settings).
import { request } from "./request";

/**
 * List views (system views + own personal views, merged with user preferences).
 * @param {number|string} wsId - workspace id
 * @param {Object} params - { type: 'requirement' | 'defect' }
 * @returns {Promise<{list: Array, remember_last: boolean, search_expanded: boolean, search_filters: Object}>}
 */
export function listWorkItemViews(wsId, params) {
  return request({
    url: `/api/workspaces/${wsId}/views`,
    method: "GET",
    params,
  });
}

/**
 * Create a view.
 * @param {number|string} wsId - workspace id
 * @param {Object} data - { type, name, view_type, filters }
 * @returns {Promise<{id: string}>}
 */
export function createWorkItemView(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/views`,
    method: "POST",
    data,
  });
}

/**
 * Update a view's name and filters (the view type is immutable).
 * @param {number|string} wsId - workspace id
 * @param {number|string} viewId - view id
 * @param {Object} data - { name, filters }
 */
export function updateWorkItemView(wsId, viewId, data) {
  return request({
    url: `/api/workspaces/${wsId}/views/${viewId}`,
    method: "PUT",
    data,
  });
}

/**
 * Delete a view (the builtin "All" view is locked; system views need the manage permission).
 * @param {number|string} wsId - workspace id
 * @param {number|string} viewId - view id
 */
export function deleteWorkItemView(wsId, viewId) {
  return request({
    url: `/api/workspaces/${wsId}/views/${viewId}`,
    method: "DELETE",
  });
}

/**
 * Partially update per-user view and search settings.
 * @param {number|string} wsId - workspace id
 * @param {Object} data - type plus any view/search setting fields to update
 */
export function saveUserViewSettings(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/view-settings`,
    method: "PUT",
    data,
  });
}
