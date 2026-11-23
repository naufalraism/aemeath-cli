//go:build windows

package cli

import (
	"os"
	"os/exec"
	"syscall"
)

// Windows has no SIGTERM. The server is started in a process group of its own
// and stopped with Ctrl+Break sent to that group, which Go programs receive as
// os.Interrupt, like Ctrl+C: the app's graceful shutdown and OnShutdown hooks
// run, as on other systems.

var generateConsoleCtrlEvent = syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")

func prepareServer(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// askToStop sends Ctrl+Break to the server's process group. It fails when
// aemeath dev has no console (the caller then kills the server).
func askToStop(p *os.Process) error {
	if ok, _, err := generateConsoleCtrlEvent.Call(syscall.CTRL_BREAK_EVENT, uintptr(p.Pid)); ok == 0 {
		return err
	}
	return nil
}
