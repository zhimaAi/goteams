// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

//go:build windows

package business

// stagedDiskFree 在 Windows 开发机上不做暂存盘水位判断（生产部署在 Linux），返回 ok=false 即跳过背压。
func stagedDiskFree(_ string) (int64, bool) {
	return 0, false
}
