package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func (a *app) pidFile() string {
	return filepath.Join(filepath.Dir(a.file), "x2socks.pid")
}

func (a *app) logFile() string {
	return filepath.Join(filepath.Dir(a.file), "x2socks.log")
}

func stopPidFile(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		_ = os.Remove(path)
		return
	}
	proc, err := os.FindProcess(pid)
	if err == nil {
		_ = proc.Signal(syscall.SIGTERM)
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if err := proc.Signal(syscall.Signal(0)); err != nil {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		_ = proc.Kill()
	}
	_ = os.Remove(path)
}

func applyRuntime(a *app) error {
	stopPidFile(a.pidFile())
	if len(a.config.Proxies) == 0 {
		return nil
	}
	if err := validateConfig(a.config); err != nil {
		return err
	}
	bin, err := lookXray()
	if err != nil {
		return err
	}
	b, err := buildXrayConfig(a.config)
	if err != nil {
		return err
	}
	logf, err := os.OpenFile(a.logFile(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	cmd, err := startXrayLogDetach(bin, a.xrayFile(), b, logf, true)
	if err != nil {
		_ = logf.Close()
		return err
	}
	if err := os.WriteFile(a.pidFile(), []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0600); err != nil {
		_ = stopXray(cmd)
		_ = logf.Close()
		return err
	}
	for _, p := range a.config.Proxies {
		for _, addr := range socksListenAddrs(a.config, p) {
			if err := waitTCP(dialWaitAddr(addr, p.LocalPort), 5*time.Second); err != nil {
				stopPidFile(a.pidFile())
				return fmt.Errorf("本地 %s:%d 未监听: %w", addr, p.LocalPort, err)
			}
		}
	}
	return nil
}

func dialWaitAddr(listen string, port int) string {
	host := listen
	switch host {
	case "", "0.0.0.0":
		host = "127.0.0.1"
	case "::", "[::]":
		host = "::1"
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

var afterMutate = applyRuntime
