package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func lookXray() (string, error) {
	if b := strings.TrimSpace(os.Getenv("XRAY_BIN")); b != "" {
		return b, nil
	}
	if p, err := exec.LookPath("xray"); err == nil {
		return p, nil
	}
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), "xray")
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, nil
		}
	}
	return "", errors.New("未找到 xray，请安装 Xray-core 或设置 XRAY_BIN")
}

func startXray(bin, cfgPath string, cfg []byte) (*exec.Cmd, error) {
	return startXrayLog(bin, cfgPath, cfg, os.Stderr)
}

func startXrayLog(bin, cfgPath string, cfg []byte, log io.Writer) (*exec.Cmd, error) {
	return startXrayLogDetach(bin, cfgPath, cfg, log, false)
}

func startXrayLogDetach(bin, cfgPath string, cfg []byte, log io.Writer, detach bool) (*exec.Cmd, error) {
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(cfgPath, append(cfg, '\n'), 0600); err != nil {
		return nil, err
	}
	if log == nil {
		log = io.Discard
	}
	cmd := exec.Command(bin, "run", "-c", cfgPath)
	cmd.Stdout = log
	cmd.Stderr = log
	if detach {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 Xray 失败: %w", err)
	}
	if detach {
		go cmd.Wait()
	}
	return cmd, nil
}

func stopXray(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			return nil
		}
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil
		}
		return err
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		return <-done
	}
}

func waitTCP(addr string, d time.Duration) error {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("等待端口超时: %s", addr)
}

func buildXrayConfig(c AppConfig) ([]byte, error) {
	inbounds := make([]map[string]any, 0, len(c.Proxies))
	outbounds := make([]map[string]any, 0, len(c.Proxies))
	rules := make([]map[string]any, 0, len(c.Proxies))
	for i, p := range c.Proxies {
		outTag := "proxy-" + strconv.Itoa(i+1)
		addrs := socksListenAddrs(c, p)
		inTags := make([]string, 0, len(addrs))
		for j, addr := range addrs {
			inTag := "socks-" + strconv.Itoa(i+1)
			if len(addrs) > 1 {
				inTag += "-" + strconv.Itoa(j+1)
			}
			inTags = append(inTags, inTag)
			inbounds = append(inbounds, map[string]any{"tag": inTag, "listen": addr, "port": p.LocalPort, "protocol": "socks", "settings": map[string]any{"auth": "noauth", "udp": true}, "sniffing": map[string]any{"enabled": true, "destOverride": []string{"http", "tls", "quic"}}})
		}
		out := map[string]any{"tag": outTag, "protocol": xrayProtocol(p.Type), "settings": outboundSettings(p)}
		if p.Type == "vless" || p.Type == "trojan" {
			out["streamSettings"] = streamSettings(p)
		}
		outbounds = append(outbounds, out)
		rules = append(rules, map[string]any{"type": "field", "inboundTag": inTags, "outboundTag": outTag})
	}
	return json.Marshal(map[string]any{"log": map[string]any{"loglevel": "warning"}, "inbounds": inbounds, "outbounds": outbounds, "routing": map[string]any{"domainStrategy": "AsIs", "rules": rules}})
}

func xrayProtocol(t string) string {
	if t == "ss" {
		return "shadowsocks"
	}
	return t
}

func outboundSettings(p Proxy) map[string]any {
	switch p.Type {
	case "ss":
		return map[string]any{"servers": []map[string]any{{"address": p.Address, "port": p.Port, "method": p.Method, "password": p.Password}}}
	case "trojan":
		return map[string]any{"servers": []map[string]any{{"address": p.Address, "port": p.Port, "password": p.Password, "email": p.Name}}}
	default:
		return map[string]any{"vnext": []map[string]any{{"address": p.Address, "port": p.Port, "users": []map[string]any{{"id": p.UUID, "encryption": "none", "flow": p.Flow}}}}}
	}
}

func streamSettings(p Proxy) map[string]any {
	network := p.Network
	if network == "" {
		network = "tcp"
	}
	s := map[string]any{"network": network}
	if p.PublicKey != "" {
		s["security"] = "reality"
		s["realitySettings"] = map[string]any{"serverName": p.ServerName, "publicKey": p.PublicKey, "shortId": p.ShortID, "fingerprint": "chrome"}
	} else if p.TLS {
		s["security"] = "tls"
		s["tlsSettings"] = map[string]any{"serverName": p.ServerName, "allowInsecure": false}
	}
	if network == "ws" {
		s["wsSettings"] = map[string]any{"path": p.Path, "headers": map[string]string{"Host": p.Host}}
	}
	if network == "grpc" {
		s["grpcSettings"] = map[string]any{"serviceName": p.ServiceName, "multiMode": false}
	}
	return s
}
