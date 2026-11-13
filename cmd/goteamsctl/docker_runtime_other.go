//go:build !linux

package main

import "errors"

// ensureDockerRuntime is only implemented for the supported linux target; requirePlatform
// already refuses to install the commercial package anywhere else.
func ensureDockerRuntime(pkgDir string, manifest *packageManifest, assumeYes bool) error {
	return errors.New(`docker runtime installation is only available on linux`)
}
