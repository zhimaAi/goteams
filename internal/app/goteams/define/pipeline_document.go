// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package define

// 产出文档上报接口的 result 取值。客户端按这三个值判断是否需要重传，不得随意扩展。
const (
	// PipelineDocumentAccepted 表示新建文档，客户端可以丢弃本地重试队列。
	PipelineDocumentAccepted = `accepted`
	// PipelineDocumentReplaced 表示同一 doc_key 内容已更新，revision 递增。
	PipelineDocumentReplaced = `replaced`
	// PipelineDocumentDuplicate 表示内容相同或旧包，云端未改动。
	PipelineDocumentDuplicate = `duplicate`
)

// 产出文档的存储状态。
const (
	PipelineDocumentStatePending = `pending`
	PipelineDocumentStateReady   = `ready`
	PipelineDocumentStateFailed  = `failed`
)

// 产出文档正文的读取形态：文本类由服务端从对象存储取出正文，其余只给签名链接。
const (
	PipelineDocumentContentMarkdown = `markdown`
	PipelineDocumentContentText     = `text`
	PipelineDocumentContentFile     = `file`
)

// PipelineTaskDocumentIndex 是渲染联动用的轻量文档索引：任务详情把它一起返回，
// 前端只对清单内命中的路径做可点击替换，避免任意路径被链接化。
type PipelineTaskDocumentIndex struct {
	DocumentId    int64  `json:"document_id"`
	DocKey        string `json:"doc_key"`
	Title         string `json:"title"`
	RelPath       string `json:"rel_path"`
	SourceAbsPath string `json:"source_abs_path"`
}
