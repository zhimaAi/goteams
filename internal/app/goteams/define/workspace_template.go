// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package define

// CopyWorkspaceTemplateModel carries the parameters for copying a template
// workspace's configuration into a newly created workspace.
type CopyWorkspaceTemplateModel struct {
	AdminID             int // destination admin_id the new workspace belongs to
	OperatorID          int // current user_id recorded on configuration actions
	TemplateWorkspaceID int // source template workspace id
	NewWorkspaceID      int // newly created workspace id to copy into
}

// Offline template package constants. The path is fixed by the requirement so operations can
// replace the package without a rebuild; limits are here rather than in the ini files because
// they do not vary per environment.
const (
	// TemplatePackagePath is the package location relative to the process working directory. The
	// binary also embeds a default copy (assets/template, package templateassets): a file at this
	// path wins when present, otherwise the embedded package is used — the commercial deployment
	// ships no assets directory.
	TemplatePackagePath = `assets/template/template.tar.gz`
	// TemplatePackageMaxBytes bounds the compressed package.
	TemplatePackageMaxBytes = 64 << 20
	// TemplatePackageMaxEntryBytes bounds one package entry.
	TemplatePackageMaxEntryBytes = 32 << 20
)

// Template package export message key. A package that cannot be read only ever reaches the logs
// (the copy degrades to the seed), so those failures get no key, and a
// forbidden export answers a bare 403 with no message at all.
const MsgTemplatePackageTooLarge = "template_package_too_large"
