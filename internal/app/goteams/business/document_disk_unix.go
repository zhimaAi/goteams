// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

//go:build !windows

package business

import "syscall"

// stagedDiskFree 返回暂存目录所在文件系统的可用字节数；取不到时 ok=false，调用方不做背压。
func stagedDiskFree(path string) (int64, bool) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, false
	}
	return int64(stat.Bavail) * int64(stat.Bsize), true
}
