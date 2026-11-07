//go:build !windows

package app

import "runtime"

// osVersion names the system as feedback tells it.
func osVersion() string { return runtime.GOOS }
