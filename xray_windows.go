//go:build windows

package main

import (
	"os"
	"syscall"
)

var procGenerateConsoleCtrlEvent = syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")

// detachAttr 将子进程放入独立进程组：不响应父控制台的 Ctrl+C 事件，
// 且父进程退出后继续运行（Windows 子进程默认不随父进程结束）。
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// terminateProcess 向进程组发送 CTRL_BREAK_EVENT 请求优雅退出。
// 失败时调用方可回退到 Kill（stopXray/stopPidFile 均有超时强杀兜底）。
func terminateProcess(p *os.Process) error {
	r1, _, callErr := procGenerateConsoleCtrlEvent.Call(uintptr(syscall.CTRL_BREAK_EVENT), uintptr(p.Pid))
	if r1 == 0 {
		return callErr
	}
	return nil
}
