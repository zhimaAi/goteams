package validate

// WorkflowOptionOrder 检查请求是否提供了有效且唯一的配置项 ID。
func WorkflowOptionOrder(ids []int) bool {
	if len(ids) == 0 {
		return false
	}
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 || id > 2147483647 {
			return false
		}
		if _, exists := seen[id]; exists {
			return false
		}
		seen[id] = struct{}{}
	}
	return true
}
