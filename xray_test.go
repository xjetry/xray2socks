package main

import (
	"encoding/json"
	"testing"
)

func TestBuildXrayConfig(t *testing.T) {
	b, err := buildXrayConfig(AppConfig{BindHost: "127.0.0.1", Proxies: []Proxy{{Name: "node", Type: "vless", LocalPort: 10808, Address: "example.com", Port: 443, UUID: "u", Network: "ws", Path: "/x", TLS: true}}})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got["inbounds"].([]any)) != 1 {
		t.Fatal("应生成一个 SOCKS5 入站")
	}
	out := got["outbounds"].([]any)[0].(map[string]any)
	if out["protocol"] != "vless" {
		t.Fatalf("协议错误: %v", out["protocol"])
	}
}

func TestBuildXrayConfigMultiListen(t *testing.T) {
	b, err := buildXrayConfig(AppConfig{Proxies: []Proxy{{Name: "n", Type: "ss", LocalPort: 1081, Address: "a", Port: 1, Method: "aes-128-gcm", Password: "p", Listen: "127.0.0.1,[::1]"}}})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	ins := got["inbounds"].([]any)
	if len(ins) != 2 {
		t.Fatalf("inbounds = %d", len(ins))
	}
	if ins[0].(map[string]any)["listen"] != "127.0.0.1" || ins[1].(map[string]any)["listen"] != "::1" {
		t.Fatalf("listen = %#v", ins)
	}
}

func TestValidateConfig(t *testing.T) {
	if validateConfig(defaultConfig()) == nil {
		t.Fatal("空代理配置应失败")
	}
	if err := validateConfig(AppConfig{Proxies: []Proxy{{Name: "x", Type: "ss", LocalPort: 1080, Address: "a", Port: 1, Method: "aes-128-gcm", Password: "p"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestParseProxyURI(t *testing.T) {
	p, err := parseProxyURI("vless://123e4567-e89b-12d3-a456-426614174000@example.com:443?type=ws&path=%2Fedge#vless1")
	if err != nil {
		t.Fatal(err)
	}
	if p.Type != "vless" || p.Network != "ws" || p.Name != "vless1" {
		t.Fatalf("解析结果错误: %+v", p)
	}
}

func TestParseSIP002SS(t *testing.T) {
	raw := "ss://MjAyMi1ibGFrZTMtYWVzLTEyOC1nY206NEJTc2xFdk5TM3c1Q3NmQ01xWElIUT09OkcrQjh0dWs2aDFaZGxsN2M4eHlYNlE9PQ@116.48.39.115:6123?#boil-hkt-2500m"
	p, err := parseProxyURI(raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.Type != "ss" || p.Address != "116.48.39.115" || p.Port != 6123 {
		t.Fatalf("endpoint: %+v", p)
	}
	if p.Method != "2022-blake3-aes-128-gcm" {
		t.Fatalf("method = %q", p.Method)
	}
	if p.Password != "4BSslEvNS3w5CsfCMqXIHQ==:G+B8tuk6h1Zdll7c8xyX6Q==" {
		t.Fatalf("password = %q", p.Password)
	}
	if p.Name != "boil-hkt-2500m" {
		t.Fatalf("name = %q", p.Name)
	}
}

func TestParseVLESSReality(t *testing.T) {
	raw := "vless://5a183330-2abd-4da0-a479-e488d592c345@nuro.xjetry.fun:8444?encryption=none&flow=xtls-rprx-vision&type=tcp&security=reality&sni=gateway.icloud.com&fp=chrome&pbk=S-Db0M-ZD4lrlUjOzWhCRkpzAWdxjjjdiGCooZvGZzk&sid=b08d8a84734517d7#nuro%20vless"
	p, err := parseProxyURI(raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.UUID != "5a183330-2abd-4da0-a479-e488d592c345" || p.Flow != "xtls-rprx-vision" || p.PublicKey == "" || p.ShortID != "b08d8a84734517d7" || p.ServerName != "gateway.icloud.com" {
		t.Fatalf("reality parse: %+v", p)
	}
}

func TestBuildXrayConfigSSProtocol(t *testing.T) {
	b, err := buildXrayConfig(AppConfig{Proxies: []Proxy{{Name: "x", Type: "ss", LocalPort: 1080, Address: "a", Port: 1, Method: "2022-blake3-aes-128-gcm", Password: "p"}}})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	out := got["outbounds"].([]any)[0].(map[string]any)
	if out["protocol"] != "shadowsocks" {
		t.Fatalf("ss outbound protocol = %v", out["protocol"])
	}
}
