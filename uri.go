package main

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func parseProxyURI(raw string) (Proxy, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return Proxy{}, fmt.Errorf("URI 无效: %w", err)
	}
	p := Proxy{Type: strings.ToLower(u.Scheme), Name: u.Fragment, Address: u.Hostname(), Port: uriPort(u.Port(), 443), Network: "tcp", TLS: true}
	if p.Name == "" {
		p.Name = p.Type + " 节点"
	}
	q := u.Query()
	switch p.Type {
	case "ss":
		return parseSS(u, p)
	case "vless":
		p.UUID = u.User.Username()
		fillTransport(&p, q)
		return p, nil
	case "trojan":
		p.Password = u.User.Username()
		fillTransport(&p, q)
		return p, nil
	default:
		return Proxy{}, fmt.Errorf("不支持的 URI 类型: %s", u.Scheme)
	}
}

func parseSS(u *url.URL, p Proxy) (Proxy, error) {
	p.TLS = false
	if u.User != nil {
		user := u.User.Username()
		if pass, ok := u.User.Password(); ok {
			p.Method, p.Password = user, pass
			return p, nil
		}
		method, password, err := decodeSSUserinfo(user)
		if err != nil {
			return Proxy{}, err
		}
		p.Method, p.Password = method, password
		return p, nil
	}
	decoded, err := decodeURLBase64(u.Host)
	if err != nil {
		return Proxy{}, fmt.Errorf("SS URI 编码无效: %w", err)
	}
	parts := strings.SplitN(string(decoded), "@", 2)
	if len(parts) != 2 {
		return Proxy{}, fmt.Errorf("SS URI 缺少服务器地址")
	}
	method, password, err := splitSSCredential(parts[0])
	if err != nil {
		return Proxy{}, err
	}
	host, port, err := splitHostPort(parts[1])
	if err != nil {
		return Proxy{}, err
	}
	p.Method, p.Password, p.Address, p.Port = method, password, host, port
	return p, nil
}

func decodeSSUserinfo(raw string) (string, string, error) {
	for _, enc := range []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.StdEncoding} {
		b, err := enc.DecodeString(raw)
		if err != nil {
			continue
		}
		method, password, err := splitSSCredential(string(b))
		if err == nil {
			return method, password, nil
		}
	}
	return "", "", fmt.Errorf("SS URI 编码无效")
}

func splitSSCredential(s string) (string, string, error) {
	method, password, ok := strings.Cut(s, ":")
	if !ok || method == "" || password == "" {
		return "", "", fmt.Errorf("SS URI 缺少密码")
	}
	return method, password, nil
}

func fillTransport(p *Proxy, q url.Values) {
	p.Network = valueOr(q.Get("type"), "tcp")
	p.ServerName = valueOr(q.Get("sni"), q.Get("peer"))
	p.Path = q.Get("path")
	p.Host = q.Get("host")
	p.ServiceName = q.Get("serviceName")
	p.Flow = q.Get("flow")
	p.PublicKey = q.Get("pbk")
	p.ShortID = q.Get("sid")
	p.TLS = q.Get("security") != "none"
}
func decodeURLBase64(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.StdEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("无法解码")
}
func splitHostPort(s string) (string, int, error) {
	i := strings.LastIndex(s, ":")
	if i < 1 {
		return "", 0, fmt.Errorf("地址缺少端口")
	}
	n, err := strconv.Atoi(s[i+1:])
	return s[:i], n, err
}
func uriPort(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
func valueOr(a, b string) string {
	if a == "" {
		return b
	}
	return a
}
