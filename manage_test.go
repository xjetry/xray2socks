package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	probeFn = func(Proxy) probeResult {
		return probeResult{OK: true, Status: 204, LatencyMs: 1}
	}
	afterMutate = func(*app) error { return nil }
	os.Exit(m.Run())
}

const testVLESS = "vless://123e4567-e89b-12d3-a456-426614174000@example.com:443?type=ws&path=%2Fedge#vless1"
const testTrojan = "trojan://secret@other.example:8443?security=tls&sni=other.example#trojan1"

func TestRunManageAddListRemove(t *testing.T) {
	c := defaultConfig()
	c, out, err := runManage(c, []string{"add", testVLESS, "1080"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Proxies) != 1 || c.Proxies[0].LocalPort != 1080 || c.Proxies[0].Name != "vless1" || c.Proxies[0].Listen != "0.0.0.0" {
		t.Fatalf("add: %+v (%s)", c.Proxies, out)
	}

	_, listed, err := runManage(c, []string{"list"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listed, "vless1") || !strings.Contains(listed, "1080") || !strings.Contains(listed, "1") {
		t.Fatalf("list = %q", listed)
	}

	c, _, err = runManage(c, []string{"remove", "1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Proxies) != 0 {
		t.Fatalf("remove: %+v", c.Proxies)
	}
}

func TestRunManageAddBind(t *testing.T) {
	c := defaultConfig()
	c, _, err := runManage(c, []string{"add", testVLESS, "1080", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Proxies[0].Listen != "127.0.0.1" {
		t.Fatalf("bind = %q", c.Proxies[0].Listen)
	}
}

func TestRunManageAddAutoPort(t *testing.T) {
	c := defaultConfig()
	c, _, err := runManage(c, []string{"add", testVLESS, "1081"})
	if err != nil {
		t.Fatal(err)
	}
	c, out, err := runManage(c, []string{"add", testTrojan})
	if err != nil {
		t.Fatal(err)
	}
	if c.Proxies[1].LocalPort == 1081 {
		t.Fatalf("auto port should skip used 1081: %+v %s", c.Proxies, out)
	}
	if c.Proxies[1].LocalPort < 1081 {
		t.Fatalf("auto port = %d", c.Proxies[1].LocalPort)
	}
	if c.Proxies[1].Listen != "0.0.0.0" {
		t.Fatalf("default bind = %q", c.Proxies[1].Listen)
	}
}

func TestRunManageAddBindOnly(t *testing.T) {
	c := defaultConfig()
	c, _, err := runManage(c, []string{"add", testVLESS, "127.0.0.1,::1"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Proxies[0].Listen != "127.0.0.1,::1" || c.Proxies[0].LocalPort < 1081 {
		t.Fatalf("add bind-only: %+v", c.Proxies[0])
	}
}

func TestRunManageAddRejectsDuplicatePort(t *testing.T) {
	c := defaultConfig()
	c, _, err := runManage(c, []string{"add", testVLESS, "1080"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := runManage(c, []string{"add", testTrojan, "1080"}); err == nil {
		t.Fatal("duplicate port should fail")
	}
}

func TestRunManageEdit(t *testing.T) {
	c := defaultConfig()
	c, _, err := runManage(c, []string{"add", testVLESS, "1080"})
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := runManage(c, []string{"edit", "1"}); err == nil {
		t.Fatal("edit without uri, port or bind should fail")
	}

	c, _, err = runManage(c, []string{"edit", "1", "--port", "1081"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Proxies[0].LocalPort != 1081 || c.Proxies[0].Name != "vless1" {
		t.Fatalf("edit port: %+v", c.Proxies[0])
	}

	c, _, err = runManage(c, []string{"edit", "1", "--uri", testTrojan})
	if err != nil {
		t.Fatal(err)
	}
	if c.Proxies[0].Type != "trojan" || c.Proxies[0].LocalPort != 1081 || c.Proxies[0].Name != "trojan1" {
		t.Fatalf("edit uri should keep port: %+v", c.Proxies[0])
	}

	c, _, err = runManage(c, []string{"edit", "1", "--uri", testVLESS, "--port", "1090"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Proxies[0].Type != "vless" || c.Proxies[0].LocalPort != 1090 {
		t.Fatalf("edit both: %+v", c.Proxies[0])
	}

	c, _, err = runManage(c, []string{"edit", "1", "--bind", "127.0.0.1,10.64.1.10"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Proxies[0].Listen != "127.0.0.1,10.64.1.10" {
		t.Fatalf("edit bind: %+v", c.Proxies[0])
	}
}

func TestRunManageInvalidID(t *testing.T) {
	c := defaultConfig()
	c, _, err := runManage(c, []string{"add", testVLESS, "1080"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := runManage(c, []string{"remove", "2"}); err == nil {
		t.Fatal("remove invalid id should fail")
	}
	if _, _, err := runManage(c, []string{"edit", "0", "--port", "1081"}); err == nil {
		t.Fatal("edit invalid id should fail")
	}
}

func TestRunTUIPersistsConfig(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.json")
	a, err := newApp(file)
	if err != nil {
		t.Fatal(err)
	}
	in := strings.Join([]string{
		"add " + testVLESS + " 1080",
		"edit 1 --port 1082",
		"list",
		"quit",
	}, "\n") + "\n"
	var out bytes.Buffer
	if err := runTUI(a, strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "1082") {
		t.Fatalf("tui output = %q", out.String())
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"localPort": 1082`) {
		t.Fatalf("config = %s", b)
	}
}

func TestAddSavesWhenUnreachable(t *testing.T) {
	old := probeFn
	t.Cleanup(func() { probeFn = old })
	probeFn = func(Proxy) probeResult { return probeResult{Err: "timeout"} }
	c := defaultConfig()
	c, _, err := runManage(c, []string{"add", testVLESS, "1080"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Proxies) != 1 {
		t.Fatalf("valid URI should save even if unreachable: %+v", c.Proxies)
	}
}

func TestListShowsLatency(t *testing.T) {
	old := probeFn
	t.Cleanup(func() { probeFn = old })
	n := 0
	probeFn = func(Proxy) probeResult {
		n++
		if n == 1 {
			return probeResult{OK: true, Status: 204, LatencyMs: 42}
		}
		return probeResult{Err: "timeout"}
	}
	c := defaultConfig()
	c, _, err := runManage(c, []string{"add", testVLESS, "1080"})
	if err != nil {
		t.Fatal(err)
	}
	c, _, err = runManage(c, []string{"add", testTrojan, "1081"})
	if err != nil {
		t.Fatal(err)
	}
	_, listed, err := runManage(c, []string{"list"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listed, "42ms") || !strings.Contains(listed, "down") {
		t.Fatalf("list should show latency and down: %q", listed)
	}
}

func TestRunManageTestURI(t *testing.T) {
	old := probeFn
	t.Cleanup(func() { probeFn = old })
	c := defaultConfig()
	c, _, err := runManage(c, []string{"add", testVLESS, "1080"})
	if err != nil {
		t.Fatal(err)
	}

	probeFn = func(Proxy) probeResult { return probeResult{OK: true, Status: 204, LatencyMs: 42} }
	next, out, err := runManage(c, []string{"test", testTrojan})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Proxies) != 1 || next.Proxies[0].Name != "vless1" {
		t.Fatalf("test must not change config: %+v", next.Proxies)
	}
	if !strings.Contains(out, "42ms") {
		t.Fatalf("test ok = %q", out)
	}

	probeFn = func(Proxy) probeResult { return probeResult{Err: "timeout"} }
	next, out, err = runManage(c, []string{"test", testVLESS})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Proxies) != 1 {
		t.Fatalf("unreachable test must not change config: %+v", next.Proxies)
	}
	if !strings.Contains(out, "down") || !strings.Contains(out, "timeout") {
		t.Fatalf("test down should include reason: %q", out)
	}

	if _, _, err := runManage(c, []string{"test", "not-a-uri"}); err == nil {
		t.Fatal("invalid URI should fail")
	}
}

const testSS = "ss://YWVzLTEyOC1nY206cGFzc3dvcmQ@ss.example:8388#ss1"

func TestRunManageAddChain(t *testing.T) {
	c := defaultConfig()
	c, _, err := runManage(c, []string{"add", testVLESS, testTrojan, "1085"})
	if err != nil {
		t.Fatal(err)
	}
	p := c.Proxies[0]
	if len(p.Chain) != 1 || p.Chain[0].Address != "other.example" || p.Chain[0].LocalPort != 0 || p.Chain[0].Listen != "" {
		t.Fatalf("chain = %+v", p.Chain)
	}
	if p.LocalPort != 1085 || p.Address != "example.com" {
		t.Fatalf("entry = %+v", p)
	}

	_, listed, err := runManage(c, []string{"list"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listed, "example.com:443 -> other.example:8443") {
		t.Fatalf("list should show chain target: %q", listed)
	}
}

func TestRunManageAddThreeHopChain(t *testing.T) {
	c := defaultConfig()
	c, _, err := runManage(c, []string{"add", testVLESS, testTrojan, testSS, "1085", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	p := c.Proxies[0]
	if len(p.Chain) != 2 || p.Chain[1].Address != "ss.example" {
		t.Fatalf("chain = %+v", p.Chain)
	}
	if p.Listen != "127.0.0.1" {
		t.Fatalf("bind = %q", p.Listen)
	}
}

func TestParseAddArgsRejects(t *testing.T) {
	for _, args := range [][]string{
		{"1080"},
		{testVLESS, "1080", "1081"},
		{testVLESS, "127.0.0.1", "10.0.0.1"},
	} {
		if _, _, _, err := parseAddArgs(args); err == nil {
			t.Fatalf("parseAddArgs(%v) should fail", args)
		}
	}
}

func TestEditChain(t *testing.T) {
	c := defaultConfig()
	c, _, err := runManage(c, []string{"add", testVLESS, testTrojan, "1080"})
	if err != nil {
		t.Fatal(err)
	}
	c, _, err = runManage(c, []string{"edit", "1", "--uri", testTrojan, "--uri", testSS})
	if err != nil {
		t.Fatal(err)
	}
	p := c.Proxies[0]
	if p.Type != "trojan" || len(p.Chain) != 1 || p.Chain[0].Address != "ss.example" {
		t.Fatalf("edit chain = %+v", p)
	}
	if p.LocalPort != 1080 || p.Listen != "0.0.0.0" {
		t.Fatalf("edit chain should keep port/bind: %+v", p)
	}

	c, _, err = runManage(c, []string{"edit", "1", "--uri", testVLESS})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Proxies[0].Chain) != 0 {
		t.Fatalf("single --uri should clear chain: %+v", c.Proxies[0])
	}
}

func TestRunManageTestChain(t *testing.T) {
	old := probeFn
	t.Cleanup(func() { probeFn = old })
	var probed Proxy
	probeFn = func(p Proxy) probeResult {
		probed = p
		return probeResult{OK: true, Status: 204, LatencyMs: 7}
	}
	c := defaultConfig()
	c, _, err := runManage(c, []string{"add", testVLESS, "1080"})
	if err != nil {
		t.Fatal(err)
	}
	_, out, err := runManage(c, []string{"test", testVLESS, testTrojan})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "7ms") {
		t.Fatalf("test chain out = %q", out)
	}
	if len(probed.Chain) != 1 || probed.Chain[0].Address != "other.example" {
		t.Fatalf("probe should include chain: %+v", probed)
	}
	if len(c.Proxies) != 1 || len(c.Proxies[0].Chain) != 0 {
		t.Fatalf("test must not change config: %+v", c.Proxies)
	}
}
