package define

// ExternalNotificationStrategyValidation contains fields checked before saving a strategy.
type ExternalNotificationStrategyValidation struct {
	Name       string
	Platform   string
	WebhookURL string
	MemberIDs  []int
	Events     []string
	AtMode     string
	AtRoles    []string
}
