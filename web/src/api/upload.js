import { request } from "@/api/request";

// Upload timeouts are per-endpoint, not the 15s global default: a single file may be up to 200MB,
// and the server must also finish writing it to object storage before responding.
// 600000 covers a 200MB file on a ~3Mbps uplink; the image cap is smaller so it gets a shorter one.
const IMAGE_UPLOAD_TIMEOUT = 120000;
const FILE_UPLOAD_TIMEOUT = 600000;

// Upload an inline image from the requirement rich-text editor and return a directly accessible image URL (OSS URL or local /uploads/{key} address).
export function uploadRequirementImage(wsId, file) {
  const formData = new FormData();
  formData.append("file", file);
  // multipart/form-data must be set explicitly: the axios instance defaults the global Content-Type to application/json,
  // and without overriding it, FormData is sent as application/json, so the backend c.FormFile("file") cannot parse the file (appears as "nothing was sent").
  return request({
    url: `/api/workspaces/${wsId}/requirements/image`,
    method: "POST",
    data: formData,
    headers: { "Content-Type": "multipart/form-data" },
    timeout: IMAGE_UPLOAD_TIMEOUT,
  });
}

// Upload a document inserted into the requirement rich-text editor.
export function uploadRequirementFile(wsId, file) {
  const formData = new FormData();
  formData.append("file", file);
  return request({
    url: `/api/workspaces/${wsId}/requirements/file`,
    method: "POST",
    data: formData,
    headers: { "Content-Type": "multipart/form-data" },
    timeout: FILE_UPLOAD_TIMEOUT,
  });
}

// Upload a video inserted into the requirement rich-text editor.
export function uploadRequirementVideo(wsId, file) {
  const formData = new FormData();
  formData.append("file", file);
  return request({
    url: `/api/workspaces/${wsId}/requirements/video`,
    method: "POST",
    data: formData,
    headers: { "Content-Type": "multipart/form-data" },
    timeout: FILE_UPLOAD_TIMEOUT,
  });
}
