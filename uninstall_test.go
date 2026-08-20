package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestUninstallPathsWithoutPurgeLeavesConfig(t *testing.T) {
	got := uninstallPaths("/usr/local/bin/x2socks", "/etc/x2socks/config.json", false)
	if !slices.Contains(got.files, "/usr/local/bin/x2socks") || !slices.Contains(got.files, "/usr/local/bin/xray2socks") {
		t.Fatalf("files = %v", got.files)
	}
	if slices.Contains(got.files, "/etc/x2socks/config.json") || len(got.dirs) != 0 {
		t.Fatalf("purge files leaked: %+v", got)
	}
}

func TestUninstallPathsPurgeIncludesConfig(t *testing.T) {
	got := uninstallPaths("/usr/local/bin/x2socks", "/etc/x2socks/config.json", true)
	want := []string{"/etc/x2socks/config.json", "/etc/x2socks/xray-runtime.json", "/etc/x2socks/x2socks.pid", "/etc/x2socks/x2socks.log", "/etc/x2socks", "/etc/xray2socks"}
	for _, w := range want {
		if w == "/etc/x2socks" || w == "/etc/xray2socks" {
			if !slices.Contains(got.dirs, w) {
				t.Fatalf("dirs = %v, missing %s", got.dirs, w)
			}
			continue
		}
		if !slices.Contains(got.files, w) {
			t.Fatalf("files = %v, missing %s", got.files, w)
		}
	}
}

func TestUninstallRemovesFiles(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "x2socks")
	legacy := filepath.Join(dir, "xray2socks")
	cfg := filepath.Join(dir, "config.json")
	runtime := filepath.Join(dir, "xray-runtime.json")
	for _, p := range []string{bin, legacy, cfg, runtime} {
		if err := os.WriteFile(p, []byte("x\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	set := uninstallPaths(bin, cfg, true)
	for _, p := range set.files {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	for _, p := range []string{bin, legacy, cfg, runtime} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s still exists", p)
		}
	}
}
