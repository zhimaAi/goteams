package define

const (
	// TableSkills 保存系统级技能，与租户和账号无关。
	TableSkills = "skills"
	// SkillReviewPending 表示待审核。
	SkillReviewPending = "pending"
	// SkillReviewApproved 表示审核通过。
	SkillReviewApproved = "approved"
	// SkillReviewRejected 表示审核失败。
	SkillReviewRejected = "rejected"
	// SkillSourceGitHub 表示 GitHub 来源。
	SkillSourceGitHub = "github"
	// SkillSourceSkillHub 表示 SkillHub 来源。
	SkillSourceSkillHub = "skillhub"
	// SkillSourceClawHub 表示 ClawHub 来源。
	SkillSourceClawHub = "clawhub"
	// MsgSkillGitHubURLInvalid 表示 GitHub 来源地址不是合法的仓库链接。
	MsgSkillGitHubURLInvalid = "skill_github_url_invalid"
	// MsgSkillReviewWithSourceChange 表示来源变更和人工审核需要分开提交。
	MsgSkillReviewWithSourceChange = "skill_review_with_source_change"
	// MsgSkillReviewConflict 表示审核期间技能已被其他请求修改。
	MsgSkillReviewConflict = "skill_review_conflict"
	// MsgSkillSourceURLExists 表示来源地址已存在。
	MsgSkillSourceURLExists = "skill_source_url_exists"
)
