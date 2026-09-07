package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLookXrayUsesEnv(t *testing.T) {
	t.Setenv("XRAY_BIN", "/opt/custom-xray")
	got, err := lookXray()
	if err != nil || got != "/opt/custom-xray" {
		t.Fatalf("lookXray = %q, %v", got, err)
	}
}

func TestLookXrayMissing(t *testing.T) {
	t.Setenv("XRAY_BIN", "")
	t.Setenv("PATH", t.TempDir())
	if _, err := lookXray(); err == nil {
		t.Fatal("expected error when xray is missing")
	}
}

func TestStartDetachedFakeXray(t *testing.T) {
	bin, err := filepath.Abs("testdata/fake-xray")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bin, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XRAY_BIN", bin)
	dir := t.TempDir()
	a, err := newApp(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	a.config.Proxies = []Proxy{{Name: "x", Type: "ss", LocalPort: 1080, Address: "a", Port: 1, Method: "aes-128-gcm", Password: "p"}}
	if err := startDetached(a); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.xrayFile()); err != nil {
		t.Fatalf("runtime 配置未写入: %v", err)
	}
	if !pidAlive(a.pidFile()) {
		t.Fatal("pid 应存活")
	}
	stopPidFile(a.pidFile())
	if pidAlive(a.pidFile()) {
		t.Fatal("stop 后 pid 应清理")
	}
}

func TestRunCommandMutateWithoutXray(t *testing.T) {
	t.Setenv("XRAY_BIN", "")
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	f := filepath.Join(dir, "config.json")
	if err := runCommand(f, []string{"add", testVLESS, "1080"}); err != nil {
		t.Fatalf("编辑不应依赖 xray 二进制: %v", err)
	}
	a, err := newApp(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.config.Proxies) != 1 {
		t.Fatalf("配置应已保存: %+v", a.config)
	}
	if pidAlive(a.pidFile()) {
		t.Fatal("编辑不应启动 Xray")
	}
}
