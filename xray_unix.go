//go:build unix

package main

import (
	"os"
	"syscall"
)

// detachAttr 创建新会话，使子进程脱离终端信号，并在父进程退出后继续运行。
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// terminateProcess 以 SIGTERM 请求进程优雅退出。
func terminateProcess(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}
