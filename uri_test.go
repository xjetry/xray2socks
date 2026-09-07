package main

import "testing"

func TestParseSocksURI(t *testing.T) {
	p, err := parseProxyURI("socks5://user:pass@10.0.0.1:1080#hop")
	if err != nil {
		t.Fatal(err)
	}
	if p.Type != "socks" || p.Username != "user" || p.Password != "pass" || p.Port != 1080 || p.TLS {
		t.Fatalf("socks5 = %+v", p)
	}
	p, err = parseProxyURI("socks://10.0.0.1#plain")
	if err != nil {
		t.Fatal(err)
	}
	if p.Type != "socks" || p.Port != 1080 || p.Username != "" || p.Password != "" {
		t.Fatalf("socks 默认端口 1080、无凭据: %+v", p)
	}
}

func TestParseHTTPURI(t *testing.T) {
	p, err := parseProxyURI("http://u:p@10.0.0.1:8080#h")
	if err != nil {
		t.Fatal(err)
	}
	if p.Type != "http" || p.TLS || p.Port != 8080 || p.Username != "u" || p.Password != "p" {
		t.Fatalf("http = %+v", p)
	}
	p, err = parseProxyURI("http://10.0.0.1#h")
	if err != nil {
		t.Fatal(err)
	}
	if p.Port != 80 {
		t.Fatalf("http 默认端口 80: %+v", p)
	}
	p, err = parseProxyURI("https://10.0.0.1#h")
	if err != nil {
		t.Fatal(err)
	}
	if p.Type != "http" || !p.TLS || p.Port != 443 {
		t.Fatalf("https = %+v", p)
	}
}
