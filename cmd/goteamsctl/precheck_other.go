//go:build !linux

package main

import "fmt"

// checkDiskSpace is only implemented for the supported linux target.
func checkDiskSpace(path string, minGB uint64) error {
	return fmt.Errorf(`disk precheck is only available on linux (target %s)`, path)
}

// checkMemory is only implemented for the supported linux target.
func checkMemory(minGB uint64) error {
	return fmt.Errorf(`memory precheck is only available on linux`)
}
