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

func TestStartStopFakeXray(t *testing.T) {
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
	if err := a.startLocked(); err != nil {
		t.Fatal(err)
	}
	if !a.running() {
		t.Fatal("should be running")
	}
	if _, err := os.Stat(a.xrayFile()); err != nil {
		t.Fatal(err)
	}
	if err := a.stopLocked(); err != nil {
		t.Fatal(err)
	}
	if a.running() {
		t.Fatal("should be stopped")
	}
}
