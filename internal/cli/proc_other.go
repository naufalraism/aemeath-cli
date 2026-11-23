//go:build !windows

package cli

import (
	"os"
	"os/exec"
	"syscall"
)

func prepareServer(*exec.Cmd) {}

// askToStop sends SIGTERM, which the app's graceful shutdown handles.
func askToStop(p *os.Process) error { return p.Signal(syscall.SIGTERM) }
