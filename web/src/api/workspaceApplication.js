import { request } from './request'

// ======================== Workspace Application Market API ========================

// List the application market entries with their enabled state in the workspace.
export function listWorkspaceApplications(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/applications`,
    method: 'GET',
  })
}

// Enable or disable an application for the workspace (data = { enabled: boolean }).
export function updateWorkspaceApplication(wsId, appKey, data) {
  return request({
    url: `/api/workspaces/${wsId}/applications/${appKey}`,
    method: 'PUT',
    data,
  })
}
