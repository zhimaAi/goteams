// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

// XkfMemberRelationModel 是小客服客服与 goteams 成员的映射；开源版不接入小客服，保留字段供公开邀填使用。
type XkfMemberRelationModel struct {
	AdminId       int
	XkfStaffId    int64
	GoteamsUserId int
}

// GetXkfMemberByStaffId 开源版没有小客服成员映射，恒返回未命中。
func GetXkfMemberByStaffId(xkfStaffId int64) (XkfMemberRelationModel, bool, error) {
	return XkfMemberRelationModel{}, false, nil
}
