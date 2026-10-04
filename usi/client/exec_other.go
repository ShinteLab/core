//go:build !windows

package client

import "os/exec"

// hideConsole は Windows 以外では何もしない（コンソール窓が割り当てられることは無い）。
func hideConsole(_ *exec.Cmd) {}
