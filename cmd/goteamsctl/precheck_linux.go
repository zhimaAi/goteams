//go:build linux

package main

import (
	"fmt"
	"os"
	"syscall"
)

// checkDiskSpace requires at least minGB free space on the filesystem holding path.
func checkDiskSpace(path string, minGB uint64) error {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return err
	}
	freeGB := st.Bavail * uint64(st.Bsize) / (1024 * 1024 * 1024)
	if freeGB < minGB {
		return fmt.Errorf(`insufficient disk space at %s: %dGB free, %dGB required`, path, freeGB, minGB)
	}
	return nil
}

// checkMemory requires at least minGB of total RAM (reads /proc/meminfo).
func checkMemory(minGB uint64) error {
	raw, err := os.ReadFile(`/proc/meminfo`)
	if err != nil {
		return err
	}
	var totalKB uint64
	for _, line := range splitLines(raw) {
		if len(line) > 10 && string(line[:9]) == `MemTotal:` {
			_, _ = fmt.Sscanf(string(line[9:]), `%d`, &totalKB)
			break
		}
	}
	totalGB := totalKB / (1024 * 1024)
	if totalGB < minGB {
		return fmt.Errorf(`insufficient memory: %dGB total, %dGB required`, totalGB, minGB)
	}
	return nil
}

// splitLines splits /proc/meminfo content into trimmed lines.
func splitLines(raw []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, b := range raw {
		if b == '\n' {
			out = append(out, trimBytes(raw[start:i]))
			start = i + 1
		}
	}
	out = append(out, trimBytes(raw[start:]))
	return out
}

// trimBytes trims spaces around a byte slice.
func trimBytes(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t' || b[0] == '\r') {
		b = b[1:]
	}
	for len(b) > 0 && (b[len(b)-1] == ' ' || b[len(b)-1] == '\t' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
