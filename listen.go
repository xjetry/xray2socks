package main

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

func parseListenAddrs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{"0.0.0.0"}
	}
	var addrs []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		part = strings.TrimPrefix(part, "[")
		part = strings.TrimSuffix(part, "]")
		addrs = append(addrs, part)
	}
	if len(addrs) == 0 {
		return []string{"0.0.0.0"}
	}
	return addrs
}

func socksListenAddrs(c AppConfig, p Proxy) []string {
	raw := p.Listen
	if raw == "" {
		raw = c.BindHost
	}
	return parseListenAddrs(raw)
}

func formatListenAddrs(addrs []string) string {
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if strings.Contains(a, ":") {
			parts = append(parts, "["+a+"]")
		} else {
			parts = append(parts, a)
		}
	}
	return strings.Join(parts, ",")
}

func usedLocalPorts(c AppConfig) map[int]bool {
	used := make(map[int]bool, len(c.Proxies))
	for _, p := range c.Proxies {
		used[p.LocalPort] = true
	}
	return used
}

func portFreeOn(addrs []string, port int) bool {
	var lns []net.Listener
	ok := true
	for _, addr := range addrs {
		ln, err := net.Listen("tcp", net.JoinHostPort(addr, strconv.Itoa(port)))
		if err != nil {
			ok = false
			break
		}
		lns = append(lns, ln)
	}
	for _, ln := range lns {
		_ = ln.Close()
	}
	return ok
}

func nextFreePort(c AppConfig, bind string) (int, error) {
	addrs := parseListenAddrs(bind)
	used := usedLocalPorts(c)
	for port := 1081; port <= 65535; port++ {
		if used[port] {
			continue
		}
		if portFreeOn(addrs, port) {
			return port, nil
		}
	}
	return 0, errors.New("没有可用的本地端口")
}

func isPortToken(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n >= 1 && n <= 65535
}
