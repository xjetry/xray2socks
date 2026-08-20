package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	unitX2socks      = "/etc/systemd/system/x2socks.service"
	unitXray2socks   = "/etc/systemd/system/xray2socks.service"
	dirX2socks       = "/etc/x2socks"
	dirXray2socks    = "/etc/xray2socks"
	legacyBinaryName = "xray2socks"
)

func uninstall(configFile string, purge bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	configFile, err = filepath.Abs(configFile)
	if err != nil {
		return err
	}
	paths := uninstallPaths(exe, configFile, purge)
	stopServices()
	stopPidFile(filepath.Join(filepath.Dir(configFile), "x2socks.pid"))
	for _, p := range paths.files {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("删除 %s: %w", p, err)
		}
	}
	if purge {
		for _, d := range paths.dirs {
			if err := os.RemoveAll(d); err != nil {
				return fmt.Errorf("删除 %s: %w", d, err)
			}
		}
	}
	return nil
}

type uninstallSet struct {
	files []string
	dirs  []string
}

func uninstallPaths(exe, configFile string, purge bool) uninstallSet {
	files := []string{
		exe,
		filepath.Join(filepath.Dir(exe), legacyBinaryName),
		unitX2socks,
		unitXray2socks,
	}
	var dirs []string
	if purge {
		dir := filepath.Dir(configFile)
		files = append(files, configFile, filepath.Join(dir, "xray-runtime.json"), filepath.Join(dir, "x2socks.pid"), filepath.Join(dir, "x2socks.log"))
		dirs = []string{dirX2socks, dirXray2socks}
	}
	return uninstallSet{files: uniqueStrings(files), dirs: dirs}
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func stopServices() {
	if runtime.GOOS != "linux" {
		return
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return
	}
	_ = exec.Command("systemctl", "disable", "--now", "x2socks").Run()
	_ = exec.Command("systemctl", "disable", "--now", "xray2socks").Run()
	_ = exec.Command("systemctl", "daemon-reload").Run()
}
