package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

type probeResult struct {
	OK        bool
	Status    int
	LatencyMs int64
	Err       string
}

var probeFn = probeProxy

func probeAll(proxies []Proxy) []probeResult {
	out := make([]probeResult, len(proxies))
	var wg sync.WaitGroup
	lim := make(chan struct{}, 4)
	for i, p := range proxies {
		wg.Add(1)
		lim <- struct{}{}
		go func(i int, p Proxy) {
			defer wg.Done()
			defer func() { <-lim }()
			out[i] = probeFn(p)
		}(i, p)
	}
	wg.Wait()
	return out
}

func probeProxy(p Proxy) probeResult {
	p.LocalPort = 1
	if err := validateConfig(AppConfig{Proxies: []Proxy{p}}); err != nil {
		return probeResult{Err: err.Error()}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return probeResult{Err: "无法分配测试端口"}
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	p.LocalPort = port
	b, err := buildXrayConfig(AppConfig{Proxies: []Proxy{p}})
	if err != nil {
		return probeResult{Err: err.Error()}
	}
	bin, err := lookXray()
	if err != nil {
		return probeResult{Err: err.Error()}
	}
	tmp, err := os.CreateTemp("", "xray2socks-probe-*.json")
	if err != nil {
		return probeResult{Err: err.Error()}
	}
	cfgPath := tmp.Name()
	_ = tmp.Close()
	cmd, err := startXrayLog(bin, cfgPath, b, io.Discard)
	if err != nil {
		_ = os.Remove(cfgPath)
		return probeResult{Err: err.Error()}
	}
	defer func() {
		_ = stopXray(cmd)
		_ = os.Remove(cfgPath)
	}()
	if err := waitTCP(fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second); err != nil {
		return probeResult{Err: err.Error()}
	}
	dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", port), nil, proxy.Direct)
	if err != nil {
		return probeResult{Err: err.Error()}
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				return dialer.Dial(network, address)
			},
			TLSHandshakeTimeout: 4 * time.Second,
		},
		Timeout: 5 * time.Second,
	}
	started := time.Now()
	resp, err := client.Get("https://www.gstatic.com/generate_204")
	if err != nil {
		return probeResult{Err: err.Error(), LatencyMs: time.Since(started).Milliseconds()}
	}
	defer resp.Body.Close()
	ms := time.Since(started).Milliseconds()
	ok := resp.StatusCode >= 200 && resp.StatusCode < 400
	if !ok {
		return probeResult{Status: resp.StatusCode, LatencyMs: ms, Err: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	return probeResult{OK: true, Status: resp.StatusCode, LatencyMs: ms}
}

func latencyText(r probeResult) string {
	if r.OK {
		return fmt.Sprintf("%dms", r.LatencyMs)
	}
	if r.Err != "" {
		return "down  " + r.Err
	}
	return "down"
}
