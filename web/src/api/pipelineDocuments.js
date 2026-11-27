import { request } from "./request";

// Documents produced by one pipeline task. The response is flat (rel_path on every
// row); the tree is built on the client so a single document can be located directly.
export function listPipelineTaskDocuments(taskId) {
  return request({
    url: `/api/pipeline-tasks/${taskId}/documents`,
    method: "GET",
  });
}

// One document's metadata. Text-like documents come back with the body already
// inlined (content), binary ones only carry metadata and need a signed link.
export function getPipelineDocument(documentId, options = {}) {
  return request({
    url: `/api/pipeline-documents/${documentId}`,
    method: "GET",
    signal: options.signal,
    silent: options.silent,
  });
}

// Request a fresh 2-hour signature when viewing, downloading or retrying an image.
export function getPipelineDocumentLink(
  documentId,
  disposition = "inline",
  options = {},
) {
  return request({
    url: `/api/pipeline-documents/${documentId}/link`,
    method: "GET",
    params: { disposition },
    signal: options.signal,
    silent: options.silent,
  });
}
